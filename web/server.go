package web

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/sirrobot01/scroblarr/pkg/logger"
	"html/template"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirrobot01/scroblarr/internal/config"
)

//go:embed templates/*.html
var templateFS embed.FS

// Server represents the web UI server
type Server struct {
	templates *template.Template
	logger    zerolog.Logger
}

// New creates a new web UI server
func New() *Server {

	// Create a new template with functions, then parse files
	tmpl := template.New("")
	templates := template.Must(tmpl.ParseFS(templateFS, "templates/*.html"))

	return &Server{
		templates: templates,
		logger:    logger.NewLogger("web"),
	}
}

// Start starts the web server
func (s *Server) Start(ctx context.Context) error {
	cfg := config.Get()
	// Set up API routes
	//http.HandleFunc("/api/config", s.handleConfig)
	http.HandleFunc("/api/auth/trakt", s.handleTraktAuth)
	http.HandleFunc("/api/auth/trakt/poll", s.handleTraktPoll)

	// Set up simple page handlers that just serve the base HTML
	http.HandleFunc("/", s.IndexHandler)
	http.HandleFunc("/auth", s.AuthHandler)
	http.HandleFunc("/settings", s.ConfigHandler)

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Port)
	s.logger.Info().Msgf("Starting web UI at http://localhost%s", addr)

	srv := &http.Server{
		Addr:         addr,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error().Err(err).Msgf("Error starting server")
			stop()
		}
	}()

	<-ctx.Done()
	s.logger.Info().Msg("Shutting down gracefully...")
	return srv.Shutdown(context.Background())
}

func (s *Server) Stop() {
	s.logger.Info().Msg("Shutting down gracefully...")
}

// IndexHandler renders the single-page app shell
func (s *Server) IndexHandler(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"Page":  "index",
		"Title": "",
	}
	if err := s.templates.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, fmt.Sprintf("Error rendering template: %v", err), http.StatusInternalServerError)
	}
}

func (s *Server) AuthHandler(w http.ResponseWriter, r *http.Request) {
	cfg := config.Get()
	data := map[string]any{
		"Page":              "auth",
		"Title":             "Authentication",
		"TraktEnabled":      cfg.TraktEnabled,
		"TraktClientID":     cfg.TraktDetails.ClientID,
		"TraktClientSecret": cfg.TraktDetails.ClientSecret,
	}
	if err := s.templates.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, fmt.Sprintf("Error rendering template: %v", err), http.StatusInternalServerError)
	}
}

func (s *Server) ConfigHandler(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"Page":  "settings",
		"Title": "Settings",
	}
	if err := s.templates.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, fmt.Sprintf("Error rendering template: %v", err), http.StatusInternalServerError)
	}
}

// handleConfigAPI handles the API for getting/updating configuration
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodGet {
		// Return current config
		cfg := config.Get()
		err := json.NewEncoder(w).Encode(cfg)
		if err != nil {
			return
		}
		return
	}

	if r.Method == http.MethodPost {
		// Update config
		var cfg config.Config
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, fmt.Sprintf("Error parsing request: %v", err), http.StatusBadRequest)
			return
		}
		// Validate
		if err := cfg.Validate(); err != nil {
			http.Error(w, fmt.Sprintf("Invalid configuration: %v", err), http.StatusBadRequest)
			return
		}

		// Save to file

		if err := cfg.Save(); err != nil {
			http.Error(w, fmt.Sprintf("Error saving config: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(map[string]string{"status": "success"})
		if err != nil {
			return
		}
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handleTraktDeviceAuth initiates the Trakt device authentication flow
func (s *Server) handleTraktAuth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	cfg := config.Get()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clientId := r.FormValue("client_id")
	if clientId == "" {
		clientId = cfg.TraktDetails.ClientID
	}
	if clientId == "" {
		http.Error(w, "Client ID is required", http.StatusBadRequest)
		return
	}

	// Make request to Trakt API to get device code
	payload := map[string]string{
		"client_id": clientId,
	}
	jsonPayload, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", "https://api.trakt.tv/oauth/device/code", bytes.NewBuffer(jsonPayload))
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to contact Trakt API", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Parse response from Trakt
	var deviceCodeResp struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURL string `json:"verification_url"`
		ExpiresIn       int    `json:"expires_in"`
		Interval        int    `json:"interval"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&deviceCodeResp); err != nil {
		http.Error(w, "Failed to parse Trakt response", http.StatusInternalServerError)
		return
	}

	// Update config with client id
	cfg.TraktDetails.ClientID = clientId
	if err := cfg.Save(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save config: %v", err), http.StatusInternalServerError)
		return
	}

	// Return device code info to client
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(deviceCodeResp)
	if err != nil {
		return
	}
}

// handleTraktDevicePoll polls Trakt API to check if user has authorized the device
func (s *Server) handleTraktPoll(w http.ResponseWriter, r *http.Request) {

	cfg := config.Get()
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clientId := r.FormValue("client_id")
	if clientId == "" {
		clientId = cfg.TraktDetails.ClientID
	}
	if clientId == "" {
		http.Error(w, "Client ID is required", http.StatusBadRequest)
		return
	}

	clientSecret := r.FormValue("client_secret")
	if clientSecret == "" {
		clientSecret = cfg.TraktDetails.ClientSecret
	}
	if clientSecret == "" {
		http.Error(w, "Client secret is required", http.StatusBadRequest)
		return
	}

	// Parse request
	var request struct {
		DeviceCode string `json:"device_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Make request to Trakt API to check token status
	payload := map[string]string{
		"client_id":     clientId,
		"code":          request.DeviceCode,
		"client_secret": clientSecret,
	}
	jsonPayload, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", "https://api.trakt.tv/oauth/device/token", bytes.NewBuffer(jsonPayload))
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to contact Trakt API")
		http.Error(w, "Failed to contact Trakt API", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// If successful, response will be 200 with token data
	// If pending, response will be 400 with error "authorization_pending"
	if resp.StatusCode == http.StatusOK {
		// Parse token data
		var tokenResp traktTokenResponse
		if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
			http.Error(w, "Failed to parse Trakt response", http.StatusInternalServerError)
			return
		}
		err := s.saveTraktToken(tokenResp)
		if err != nil {
			return
		}
		var data = map[string]string{
			"success": "true",
		}

		// Update config with client id and secret
		cfg.TraktDetails.ClientID = clientId
		cfg.TraktDetails.ClientSecret = clientSecret
		if err := cfg.Save(); err != nil {
			http.Error(w, fmt.Sprintf("Failed to save config: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		err = json.NewEncoder(w).Encode(data)
		if err != nil {
			return
		}
		return
	}

	// Parse error response for pending authorization
	var errorResp struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}

	switch resp.StatusCode {
	case http.StatusBadRequest:
		// Pending
		errorResp.Error = "pending"
		errorResp.ErrorDescription = "Authorization pending"
	case http.StatusNotFound:
		// Invalid Device Code
		errorResp.Error = "invalid_device_code"
		errorResp.ErrorDescription = "Invalid device code"
	case http.StatusConflict:
		errorResp.Error = "already_used"
		errorResp.ErrorDescription = "Device code already used"
	default:
		errorResp.Error = "unknown"
		errorResp.ErrorDescription = "Unknown error"
	}

	w.WriteHeader(http.StatusBadRequest)
	err = json.NewEncoder(w).Encode(errorResp)
	if err != nil {
		return
	}
}

// saveTraktToken saves the Trakt access token to configuration
func (s *Server) saveTraktToken(token traktTokenResponse) error {
	// Get current config
	cfg := config.Get()

	trakt := cfg.Trakt
	if trakt == nil {
		trakt = &config.Trakt{}
	}

	trakt.AccessToken = token.AccessToken
	trakt.RefreshToken = token.RefreshToken
	trakt.ExpiresIn = token.ExpiresIn
	trakt.TokenType = token.TokenType

	// Update config
	cfg.Trakt = trakt
	// Save config
	if err := cfg.SaveTrakt(); err != nil {
		return fmt.Errorf("failed to save Trakt token: %w", err)
	}
	return nil
}
