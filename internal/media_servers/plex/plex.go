package plex

import (
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/sirrobot01/scroblarr/internal/config"
	"github.com/sirrobot01/scroblarr/internal/types"
	"github.com/sirrobot01/scroblarr/pkg/logger"
	"github.com/sirrobot01/scroblarr/pkg/misc"
	"github.com/sirrobot01/scroblarr/pkg/request"
	"net/http"
	"net/url"
	"strings"
)

// Plex  implements the Server interface for Plex Media Server
type Plex struct {
	name      string
	config    config.Server
	logger    zerolog.Logger
	client    *request.Client
	libraries []types.Library
}

// Session represents a session in Plex
type Session struct {
	MediaContainer struct {
		Size     int        `json:"size"`
		Metadata []Metadata `json:"Metadata"`
	} `json:"MediaContainer"`
}

// Metadata represents a media item in Plex
type Metadata struct {
	RatingKey        string `json:"ratingKey"`
	Key              string `json:"key"`
	Title            string `json:"title"`
	Type             string `json:"type"`
	Year             int    `json:"year"`
	Duration         int64  `json:"duration"`
	ViewOffset       int64  `json:"viewOffset"`
	GrandparentTitle string `json:"grandparentTitle"`
	ParentIndex      int    `json:"parentIndex"`
	Index            int    `json:"index"`
	Guid             string `json:"guid"`
	Player           struct {
		State string `json:"state"`
	} `json:"Player"`
	ViewedAt int64 `json:"viewedAt"`
	User     struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"User"`
	LibrariesSectionID  string `json:"librarySectionID"`
	LibrariesSectionKey string `json:"librarySectionKey"`
	LibraryName         string `json:"librarySectionTitle"`
	AccountId           int    `json:"accountID"` // Added for compatibility with Plex API
}

// New creates a new Plex client
func New(name string, config config.Server) (*Plex, error) {
	if config.URL == "" || config.Token == "" {
		return nil, fmt.Errorf("missing required Plex configuration")
	}

	// Remove trailing slash if present
	config.URL = strings.TrimSuffix(config.URL, "/")

	headers := map[string]string{
		"Accept":       "application/json",
		"X-Plex-Token": config.Token,
	}
	_logger := logger.NewLogger("plex")
	client := request.New(
		request.WithHeaders(headers),
		request.WithLogger(_logger),
	)

	s := &Plex{
		name:   name,
		config: config,
		logger: _logger,
		client: client,
	}
	if err := s.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to Plex: %w", err)
	}
	return s, nil
}

func (p *Plex) plexItemsToMediaSessions(items []Metadata) []types.MediaSession {
	var sessions []types.MediaSession
	for _, item := range items {
		if p.config.Username != "" && item.User.Title != "" && item.User.Title != p.config.Username {
			// If a username is set in the config, filter sessions by that user
			continue
		}
		session := types.MediaSession{
			SessionID:  item.RatingKey,
			Title:      item.Title,
			Type:       item.Type,
			Year:       item.Year,
			Duration:   item.Duration,
			ViewOffset: item.ViewOffset,
			State:      item.Player.State,
			Progress:   misc.CalculateProgress(item.ViewOffset, item.Duration),
		}

		// Extract IDs from guid
		if strings.Contains(item.Guid, "imdb") {
			parts := strings.Split(item.Guid, "//")
			if len(parts) > 1 {
				session.IMDBID = strings.TrimSuffix(parts[1], "?")
			}
		} else if strings.Contains(item.Guid, "tvdb") {
			parts := strings.Split(item.Guid, "//")
			if len(parts) > 1 {
				session.TVDBID = strings.TrimSuffix(parts[1], "?")
			}
		}

		// Handle TV shows
		if item.Type == "episode" {
			session.ShowTitle = item.GrandparentTitle
			session.EpisodeTitle = item.Title
			session.SeasonNum = item.ParentIndex
			session.EpisodeNum = item.Index
		}

		session.User = types.User{
			ID:       item.User.ID,
			Username: item.User.Title,
		}

		sessions = append(sessions, session)
	}
	return sessions
}

// GetSessions returns currently active sessions from Plex
func (p *Plex) GetSessions() ([]types.MediaSession, error) {
	url := fmt.Sprintf("%s/status/sessions", p.config.URL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("plex API returned status code %d", resp.StatusCode)
	}

	var container Session
	if err := json.NewDecoder(resp.Body).Decode(&container); err != nil {
		return nil, err
	}

	sessions := p.plexItemsToMediaSessions(container.MediaContainer.Metadata)

	return sessions, nil
}

// GetWatchHistory returns the watch history from Plex
func (p *Plex) GetWatchHistory() ([]types.MediaSession, error) {

	var allWatchedItems []types.MediaSession

	// For each section, get watched items
	//url := fmt.Sprintf("%s/status/sessions/history/all", s.config.URL)
	//
	//req, err := http.NewRequest("GET", url, nil)
	//if err != nil {
	//	return nil, err
	//}
	//
	//resp, err := s.client.Do(req)
	//if err != nil {
	//	return nil, err
	//}
	//
	//if resp.StatusCode != http.StatusOK {
	//	err := resp.Body.Close()
	//	if err != nil {
	//		return nil, err
	//	}
	//	s.logger.Error().
	//		Int("status_code", resp.StatusCode).
	//		Msg("Failed to get watch history for section")
	//	return nil, fmt.Errorf("plex API returned status code %d", resp.StatusCode)
	//}
	//
	//var container Session
	//if err := json.NewDecoder(resp.Body).Decode(&container); err != nil {
	//	if err := resp.Body.Close(); err != nil {
	//		return nil, fmt.Errorf("error decoding Plex response: %w", err)
	//	}
	//	return nil, err
	//}
	//if err := resp.Body.Close(); err != nil {
	//	return nil, err
	//}
	//
	//// Process each watched item
	//for _, item := range container.MediaContainer.Metadata {
	//	session := types.MediaSession{
	//		SessionID:  item.RatingKey,
	//		Title:      item.Title,
	//		Type:       item.Type,
	//		Year:       item.Year,
	//		Duration:   item.Duration,
	//		ViewOffset: item.Duration, // For history items, assume they're complete
	//		State:      "stopped",     // History items are completed
	//		Progress:   100,           // 100% progress for history items
	//		ViewedAt:   item.ViewedAt,
	//	}
	//
	//	// Extract IDs from guid
	//	if strings.Contains(item.Guid, "imdb") {
	//		parts := strings.Split(item.Guid, "//")
	//		if len(parts) > 1 {
	//			session.IMDBID = strings.TrimSuffix(parts[1], "?")
	//		}
	//	} else if strings.Contains(item.Guid, "tvdb") {
	//		parts := strings.Split(item.Guid, "//")
	//		if len(parts) > 1 {
	//			session.TVDBID = strings.TrimSuffix(parts[1], "?")
	//		}
	//	}
	//
	//	// Handle TV shows
	//	if item.Type == "episode" {
	//		session.ShowTitle = item.GrandparentTitle
	//		session.EpisodeTitle = item.Title
	//		session.SeasonNum = item.ParentIndex
	//		session.EpisodeNum = item.Index
	//	}
	//
	//	// Add user info if available
	//	session.User = types.User{
	//		ID:       item.User.ID,
	//		Username: item.User.Title,
	//	}
	//
	//	allWatchedItems = append(allWatchedItems, session)
	//}
	//
	//s.logger.Info().
	//	Int("count", len(allWatchedItems)).
	//	Msg("Retrieved watch history from Plex")

	return allWatchedItems, nil
}

// GetServerType returns the type of this server
func (p *Plex) GetServerType() string {
	return "plex"
}

// GetName returns the name of the server
func (p *Plex) GetName() string {
	return p.name
}

func (p *Plex) SyncHistory(session types.MediaSession) error {
	//TODO implement me
	return nil
}

func (p *Plex) Scrobble(session types.MediaSession, action string) error {
	results, err := p.search(session)
	if err != nil {
		return fmt.Errorf("failed to search for media: %w", err)
	}
	if len(results) == 0 {
		return fmt.Errorf("no matching media found for scrobble")
	}
	errChan := make(chan error, len(results))
	for _, item := range results {
		go func(item types.MediaSession) {
			if err := p.scrobble(item.SessionID, session); err != nil {
				errChan <- fmt.Errorf("failed to scrobble item %s: %w", item.Title, err)
			}
		}(item)
	}
	close(errChan)
	var scrobbleErrors []error
	for err := range errChan {
		if err != nil {
			scrobbleErrors = append(scrobbleErrors, err)
		}
	}
	if len(scrobbleErrors) > 0 {
		return fmt.Errorf("scrobble errors occurred: %v", scrobbleErrors)
	}
	p.logger.Trace().
		Str("action", action).
		Str("title", session.Title).
		Str("item", session.SessionID).
		Msgf("Scrobbled to %s", p.name)
	return nil
}

func (p *Plex) scrobble(key string, item types.MediaSession) error {
	query := url.Values{}
	query.Add("key", key)
	query.Add("state", item.State)
	query.Add("time", fmt.Sprintf("%d", item.ViewOffset))
	query.Add("identifier", "com.plexapp.plugins.library")

	_url := fmt.Sprintf("%s/:/progress", p.config.URL)
	_url += "?" + query.Encode()
	req, err := http.NewRequest("GET", _url, nil)
	if err != nil {
		return fmt.Errorf("failed to create scrobble request: %w", err)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send scrobble request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("plex API returned status code %d", resp.StatusCode)
	}
	return nil
}

// Helper function to calculate progress percentage

func (p *Plex) GetConfig() config.Server {
	return p.config
}

func (p *Plex) Connect() error {
	libraries, err := p.getLibraries()
	if err != nil {
		return fmt.Errorf("failed to connect to Plex: %w", err)
	}
	p.libraries = libraries
	p.logger.Info().Msgf("Connected to Plex server: %s with %d libraries", p.name, len(p.libraries))
	return nil
}
