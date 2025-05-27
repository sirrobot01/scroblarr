package emby_jellyfin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/sirrobot01/scroblarr/internal/config"
	"github.com/sirrobot01/scroblarr/pkg/request"
	"io"
	"net/http"
	"strings"
)

func getClient(config config.Server, logger zerolog.Logger) (*request.Client, error) {
	switch config.Type {
	case "emby":
		return embyClient(config, logger)
	case "jellyfin":
		return jellyfinClient(config, logger)
	default:
		return nil, fmt.Errorf("unsupported server type: %s", config.Type)
	}
}

func jellyfinClient(config config.Server, logger zerolog.Logger) (*request.Client, error) {
	if config.Token == "" && config.Username != "" && config.Password != "" {
		// Authenticate with Jellyfin API
		authURL := fmt.Sprintf("%s/Users/AuthenticateByName", config.URL)
		authData := map[string]string{
			"Username": config.Username,
			"Pw":       config.Password,
		}

		jsonData, err := json.Marshal(authData)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize authentication data: %w", err)
		}

		req, err := http.NewRequest("POST", authURL, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// hash username
		deviceID := hashString(config.Username)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "MediaBrowser Client=\"Scroblarr\", Device=\"Scroblarr\", Version=\"1.0\", DeviceId=\""+deviceID+"\"")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to send request: %w", err)
		}
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				return
			}
		}(resp.Body)

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("jellyfin API returned status code %d", resp.StatusCode)
		}

		var authResponse struct {
			Token string `json:"AccessToken"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&authResponse); err != nil {
			return nil, fmt.Errorf("failed to decode authentication response: %w", err)
		}

		config.Token = authResponse.Token
	}

	// Remove trailing slash if present
	config.URL = strings.TrimSuffix(config.URL, "/")
	headers := map[string]string{
		"Accept":        "application/json",
		"Authorization": fmt.Sprintf("MediaBrowser Token=%s", config.Token),
		"Content-Type":  "application/json",
	}
	client := request.New(
		request.WithHeaders(headers),
		request.WithLogger(logger),
	)
	return client, nil
}

func embyClient(config config.Server, logger zerolog.Logger) (*request.Client, error) {
	if config.Token == "" && (config.Username != "" && config.Password != "") {
		// Authenticate with Jellyfin API
		authURL := fmt.Sprintf("%s/Users/AuthenticateByName", config.URL)
		authData := map[string]string{
			"Username": config.Username,
			"Pw":       config.Password,
		}

		jsonData, err := json.Marshal(authData)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize authentication data: %w", err)
		}

		req, err := http.NewRequest("POST", authURL, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// hash username
		deviceID := hashString(config.Username)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Emby Client=\"Scroblarr\", Device=\"Scroblarr\", Version=\"1.0\", DeviceId=\""+deviceID+"\"")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to send request: %w", err)
		}
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				return
			}
		}(resp.Body)

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("jellyfin API returned status code %d", resp.StatusCode)
		}

		var authResponse struct {
			Token string `json:"AccessToken"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&authResponse); err != nil {
			return nil, fmt.Errorf("failed to decode authentication response: %w", err)
		}

		config.Token = authResponse.Token
	}

	config.URL = strings.TrimSuffix(config.URL, "/")

	headers := map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json",
		"X-Emby-Token": config.Token,
	}
	client := request.New(
		request.WithHeaders(headers),
		request.WithLogger(logger),
	)
	return client, nil
}
