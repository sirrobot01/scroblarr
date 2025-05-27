package trakt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/sirrobot01/scroblarr/internal/config"
	"github.com/sirrobot01/scroblarr/internal/types"
	"github.com/sirrobot01/scroblarr/pkg/logger"
	"github.com/sirrobot01/scroblarr/pkg/request"
	"github.com/sirrobot01/scroblarr/pkg/version"
	"io"
	"net/http"
)

type Client struct {
	APIBaseURL string
	config     *config.Trakt
	logger     zerolog.Logger
	client     *request.Client
}

func (t *Client) GetSyncFrom() bool {
	return false
}

func New() *Client {
	cfg := config.Get().Trakt
	if cfg == nil {
		return nil
	}
	headers := map[string]string{
		"Content-Type":      "application/json",
		"trakt-api-version": "2",
		"Authorization":     "Bearer " + cfg.AccessToken,
		"trakt-api-key":     "4ee97aae28ec4797b76a7c97d2655286e3c113124028339c9c08d9ab12a2f81a",
	}
	_logger := logger.NewLogger("trakt")
	client := request.New(
		request.WithHeaders(headers),
		request.WithLogger(_logger),
	)
	c := &Client{
		APIBaseURL: "https://api.trakt.tv",
		config:     cfg,
		logger:     _logger,
		client:     client,
	}

	// Connect
	return c
}

// Scrobble sends a scrobble update to Trakt
func (t *Client) Scrobble(session types.MediaSession, action string) error {
	url := fmt.Sprintf("%s/scrobble/%s", t.APIBaseURL, action)

	// Prepare request body
	payload := ScrobbleRequest{
		Progress:   session.Progress,
		AppVersion: fmt.Sprintf("scroblarr/%s", version.GetInfo()),
	}

	// Set the appropriate media type
	if session.Type == "movie" {
		payload.Movie = &Movie{
			Title: session.Title,
			Year:  session.Year,
			IDs:   make(map[string]string),
		}
		if session.IMDBID != "" {
			payload.Movie.IDs["imdb"] = session.IMDBID
		}
	} else if session.Type == "episode" {
		payload.Episode = &Episode{
			Title:  session.EpisodeTitle,
			Season: session.SeasonNum,
			Number: session.EpisodeNum,
			IDs:    make(map[string]string),
		}
		payload.Show = &Show{
			Title: session.ShowTitle,
			IDs:   make(map[string]string),
		}
		if session.TVDBID != "" {
			payload.Show.IDs["tvdb"] = session.TVDBID
		}
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Prepare request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		fmt.Println(string(bodyBytes))
		return fmt.Errorf("trakt API error: %d", resp.StatusCode)
	}

	return nil
}

// SyncHistory syncs a single completed item to Trakt
func (t *Client) SyncHistory(session types.MediaSession) error {
	url := fmt.Sprintf("%s/sync/history", t.APIBaseURL)

	// Prepare history data based on media type
	var historyData map[string]interface{}

	if session.Type == "movie" {
		movies := []map[string]interface{}{
			{
				"title": session.Title,
				"year":  session.Year,
				"ids":   map[string]string{},
			},
		}

		if session.IMDBID != "" {
			movies[0]["ids"].(map[string]string)["imdb"] = session.IMDBID
		}

		historyData = map[string]interface{}{
			"movies": movies,
		}
	} else if session.Type == "episode" {
		episodes := []map[string]interface{}{
			{
				"title":  session.EpisodeTitle,
				"season": session.SeasonNum,
				"number": session.EpisodeNum,
				"ids":    map[string]string{},
			},
		}

		historyData = map[string]interface{}{
			"episodes": episodes,
		}
	} else {
		return fmt.Errorf("unsupported media type: %s", session.Type)
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(historyData)
	if err != nil {
		return fmt.Errorf("failed to marshal history data: %w", err)
	}

	// Prepare request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("trakt API error: %d", resp.StatusCode)
	}

	return nil
}

// GetSessions returns currently active sessions from Emby
func (t *Client) GetSessions() ([]types.MediaSession, error) {
	// This would need to be implemented
	return []types.MediaSession{}, nil
}

// GetWatchHistory returns the watch history from Emby
func (t *Client) GetWatchHistory() ([]types.MediaSession, error) {
	// This would need to be implemented
	return []types.MediaSession{}, nil
}

// GetServerType returns the type of this server
func (t *Client) GetServerType() string {
	return "trakt"
}

func (t *Client) GetSyncTo() []string {
	return []string{}
}

func (t *Client) Connect() error {
	// This would need to be implemented
	return nil
}
