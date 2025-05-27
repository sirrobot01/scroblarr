package plex

import (
	"encoding/json"
	"fmt"
	"github.com/sirrobot01/scroblarr/internal/types"
	"net/http"
	"net/url"
)

// getMediaType converts a media type string to a Plex-specific media type code
func getMediaType(mediaType string) string {
	switch mediaType {
	case "movie":
		return "1"
	case "show":
		return "2"
	case "episode":
		return "3"
	case "music":
		return "4"
	default:
		return "0" // Default to 0 for unknown types
	}
}

func (p *Plex) search(session types.MediaSession) ([]types.MediaSession, error) {
	mediaType := getMediaType(session.Type)
	if mediaType == "0" {
		return nil, fmt.Errorf("unsupported media type: %s", session.Type)
	}

	_url := fmt.Sprintf("%s/library/all", p.config.URL)

	// Add query parameters for search
	query := url.Values{}
	query.Add("type", mediaType)
	query.Add("title", session.Title)
	if session.Year > 0 {
		query.Add("year", fmt.Sprintf("%d", session.Year))
	}
	_url += "?" + query.Encode()
	req, err := http.NewRequest("GET", _url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("plex API returned status code %d", resp.StatusCode)
	}

	var container Session
	if err := json.NewDecoder(resp.Body).Decode(&container); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	sessions := p.plexItemsToMediaSessions(container.MediaContainer.Metadata)
	return sessions, nil
}
