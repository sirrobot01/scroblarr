package emby_jellyfin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/sirrobot01/scroblarr/internal/config"
	"github.com/sirrobot01/scroblarr/internal/types"
	"github.com/sirrobot01/scroblarr/pkg/misc"
	"github.com/sirrobot01/scroblarr/pkg/request"
	"hash/fnv"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// BaseServer implements the BaseServer interface for Jellyfin
type BaseServer struct {
	name   string
	config config.Server
	logger zerolog.Logger
	client *request.Client
}

// GetName returns the name of the server
func (s *BaseServer) GetName() string {
	return s.name
}

// Session represents a session in Jellyfin
type Session struct {
	ID             string         `json:"Id"`
	UserID         string         `json:"UserId"`
	UserName       string         `json:"UserName"`
	Client         string         `json:"Client"`
	DeviceName     string         `json:"DeviceName"`
	NowPlayingItem NowPlayingItem `json:"NowPlayingItem"`
	PlayState      PlayState      `json:"PlayState"`
}

// NowPlayingItem represents a media item being played
type NowPlayingItem struct {
	ID                string            `json:"Id"`
	Name              string            `json:"Name"`
	Type              string            `json:"Type"`
	MediaType         string            `json:"MediaType"`
	RunTimeTicks      int64             `json:"RunTimeTicks"`
	ProductionYear    int               `json:"ProductionYear"`
	IndexNumber       int               `json:"IndexNumber"`
	ParentIndexNumber int               `json:"ParentIndexNumber"`
	SeriesName        string            `json:"SeriesName"`
	ProviderIDs       map[string]string `json:"ProviderIds"`
}

// PlayState represents the playback state
type PlayState struct {
	PositionTicks int64 `json:"PositionTicks"`
	IsPaused      bool  `json:"IsPaused"`
	IsMuted       bool  `json:"IsMuted"`
}

func hashString(s string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return strconv.FormatUint(h.Sum64(), 16)
}

// GetSessions returns currently active sessions from Jellyfin
func (s *BaseServer) GetSessions() ([]types.MediaSession, error) {

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/Sessions", s.config.URL), nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status code %d", resp.StatusCode)
	}

	var itemSessions []Session
	if err := json.NewDecoder(resp.Body).Decode(&itemSessions); err != nil {
		return nil, err
	}

	var sessions []types.MediaSession
	for _, js := range itemSessions {
		// Skip sessions without now playing info
		if js.NowPlayingItem.ID == "" {
			continue
		}

		// Skip the sessions scrobbled by Scroblarr itself
		if js.Client == "Scroblarr" {
			s.logger.Trace().
				Str("session_id", js.ID).
				Str("user", js.UserName).
				Msg("Skipping Scroblarr's own session")
			continue
		}

		mediaType := "movie"
		if js.NowPlayingItem.Type == "Episode" {
			mediaType = "episode"
		}

		// Jellyfin/Emby uses 10000000 ticks per second
		duration := js.NowPlayingItem.RunTimeTicks / 10000
		position := js.PlayState.PositionTicks / 10000

		state := "playing"
		if js.PlayState.IsPaused {
			state = "paused"
		}

		session := types.MediaSession{
			SessionID:  js.ID,
			Title:      js.NowPlayingItem.Name,
			Type:       mediaType,
			Year:       js.NowPlayingItem.ProductionYear,
			Duration:   duration,
			ViewOffset: position,
			State:      state,
			Progress:   misc.CalculateProgress(position, duration),
			User: types.User{
				ID:       js.UserID,
				Username: js.UserName,
			},
		}

		// Extract external IDs
		if imdbID, ok := js.NowPlayingItem.ProviderIDs["Imdb"]; ok {
			session.IMDBID = imdbID
		}
		if tvdbID, ok := js.NowPlayingItem.ProviderIDs["Tvdb"]; ok {
			session.TVDBID = tvdbID
		}

		// Handle TV shows
		if mediaType == "episode" {
			session.ShowTitle = js.NowPlayingItem.SeriesName
			session.EpisodeTitle = js.NowPlayingItem.Name
			session.SeasonNum = js.NowPlayingItem.ParentIndexNumber
			session.EpisodeNum = js.NowPlayingItem.IndexNumber
		}

		sessions = append(sessions, session)
	}

	return sessions, nil
}

// GetWatchHistory returns the watch history from Jellyfin
func (s *BaseServer) GetWatchHistory() ([]types.MediaSession, error) {
	// This would need to be implemented
	return []types.MediaSession{}, nil
}

// GetServerType returns the type of this server
func (s *BaseServer) GetServerType() string {
	return "emby"
}

func (s *BaseServer) SyncHistory(session types.MediaSession) error {
	//TODO implement me
	return nil
}

func (s *BaseServer) Scrobble(session types.MediaSession, action string) error {
	// First, we need to get the Jellyfin item ID for this content
	itemId, err := s.findItem(session)
	if err != nil {
		return fmt.Errorf("failed to find Jellyfin item: %w", err)
	}

	if itemId == "" {
		return fmt.Errorf("no matching item found in Jellyfin library")
	}

	// Get the user ID if not provided
	userID, err := s.getDefaultUserID()
	if err != nil {
		return fmt.Errorf("failed to get default user ID: %w", err)
	}

	// Determine the API endpoint based on the action

	if action == "scrobble" {
		return s.markAsPlayed(itemId, userID)
	}

	positionTicks := session.ViewOffset * 10000 // Convert ms to ticks

	playbackInfo := map[string]interface{}{
		"ItemId":        itemId,
		"UserId":        userID,
		"PositionTicks": positionTicks,
		"IsPaused":      action == "pause",
		"PlaySessionId": session.SessionID,
	}

	var endpoint string

	switch action {
	case "start":
		endpoint = fmt.Sprintf("%s/Sessions/Playing/Progress", s.config.URL)
	case "pause":
		endpoint = fmt.Sprintf("%s/Sessions/Playing/Stopped", s.config.URL)
	case "stop":
		endpoint = fmt.Sprintf("%s/Sessions/Playing/Stopped", s.config.URL)
	default:
		return fmt.Errorf("unsupported action: %s", action)
	}

	jsonData, err := json.Marshal(playbackInfo)
	if err != nil {
		return fmt.Errorf("failed to serialize playback info: %w", err)
	}

	// Make the API request to update playback status
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned error %d: %s", resp.StatusCode, string(body))
	}

	s.logger.Trace().
		Str("action", action).
		Str("title", session.Title).
		Str("item", itemId).
		Msgf("Scrobbled to %s", s.name)

	return nil
}

// findItem looks up a Jellyfin item ID based on external IDs or title/year
func (s *BaseServer) findItem(session types.MediaSession) (string, error) {
	// Try to find by external ID first (more reliable)
	if session.IMDBID != "" {
		id, err := s.findByExternalID("Imdb", session.IMDBID)
		if err == nil && id != "" {
			return id, nil
		}
	}

	if session.TVDBID != "" {
		id, err := s.findByExternalID("Tvdb", session.TVDBID)
		if err == nil && id != "" {
			return id, nil
		}
	}

	// If no external IDs or lookup failed, try by title/year
	var searchQuery string
	if session.Type == "episode" && session.ShowTitle != "" {
		// For TV episodes, search by show name, season, and episode
		searchQuery = fmt.Sprintf("%s/Items?searchTerm=%s&includeItemTypes=Episode&recursive=true",
			s.config.URL, url.QueryEscape(session.Title))
	} else {
		// For movies, search by title and year
		searchQuery = fmt.Sprintf("%s/Items?searchTerm=%s&includeItemTypes=Movie&recursive=true",
			s.config.URL, url.QueryEscape(session.Title))
		if session.Year > 0 {
			searchQuery += fmt.Sprintf("&years=%d", session.Year)
		}
	}

	req, err := http.NewRequest("GET", searchQuery, nil)
	if err != nil {
		return "", err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status code %d", resp.StatusCode)
	}

	// Parse the search results
	var searchResults struct {
		Items []struct {
			ID                string `json:"Id"`
			Name              string `json:"Name"`
			ProductionYear    int    `json:"ProductionYear"`
			IndexNumber       int    `json:"IndexNumber"`
			ParentIndexNumber int    `json:"ParentIndexNumber"`
		} `json:"Items"`
		TotalRecordCount int `json:"TotalRecordCount"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchResults); err != nil {
		return "", err
	}

	// For TV episodes, find the specific episode
	if session.Type == "episode" && session.ShowTitle != "" {
		for _, item := range searchResults.Items {
			if item.ParentIndexNumber == session.SeasonNum &&
				item.IndexNumber == session.EpisodeNum {
				return item.ID, nil
			}
		}
	} else if len(searchResults.Items) > 0 {
		// For movies, just take the first match
		return searchResults.Items[0].ID, nil
	}

	return "", nil // No matches found
}

// findByExternalID looks up an item by external ID (IMDB, TVDB, etc.)
func (s *BaseServer) findByExternalID(providerName string, providerID string) (string, error) {

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/Items?ProviderIds=%s.%s&recursive=true", s.config.URL, providerName, providerID), nil)
	if err != nil {
		return "", err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status code %d", resp.StatusCode)
	}

	var results struct {
		Items []struct {
			ID string `json:"Id"`
		} `json:"Items"`
		TotalRecordCount int `json:"TotalRecordCount"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return "", err
	}

	if results.TotalRecordCount > 0 && len(results.Items) > 0 {
		return results.Items[0].ID, nil
	}

	return "", nil
}

// getDefaultUserID gets the first admin user's ID
func (s *BaseServer) getDefaultUserID() (string, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/Users", s.config.URL), nil)
	if err != nil {
		return "", err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status code %d", resp.StatusCode)
	}

	var users []struct {
		ID         string `json:"Id"`
		Name       string `json:"Name"`
		IsAdmin    bool   `json:"Policy.IsAdministrator"`
		IsDisabled bool   `json:"Policy.IsDisabled"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return "", err
	}

	// Find first enabled admin user, or first enabled user
	for _, user := range users {
		if !user.IsDisabled && user.Name == s.config.Username {
			return user.ID, nil
		}
	}

	for _, user := range users {
		if !user.IsDisabled {
			return user.ID, nil
		}
	}

	return "", fmt.Errorf("no valid users found in Jellyfin")
}

// markAsPlayed marks an item as played for a user
func (s *BaseServer) markAsPlayed(itemID, userID string) error {
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/Users/%s/PlayedItems/%s", s.config.URL, userID, itemID), nil)
	if err != nil {
		return err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (s *BaseServer) GetConfig() config.Server {
	return s.config
}

func (s *BaseServer) Connect() error {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/System/Info", s.config.URL), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", s.name, err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s API returned status code %d", s.name, resp.StatusCode)
	}
	var info struct {
		ServerVersion string `json:"Version"`
		ServerName    string `json:"ServerName"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return fmt.Errorf("failed to decode %s response: %w", s.name, err)
	}
	s.logger.Info().Str("Version", info.ServerVersion).Msgf("Connected to Emby server: %s", info.ServerName)
	return nil
}
