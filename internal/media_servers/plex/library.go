package plex

import (
	"encoding/json"
	"github.com/sirrobot01/scroblarr/internal/types"
	"net/http"
)

type librarySchema struct {
	MediaContainer struct {
		Size      int `json:"size"`
		Directory []struct {
			Key   string `json:"key"`
			Type  string `json:"type"`
			Title string `json:"title"`
		} `json:"Directory"`
	} `json:"MediaContainer"`
}

func (p *Plex) getLibraries() ([]types.Library, error) {
	libraries := make([]types.Library, 0)
	req, err := http.NewRequest("GET", p.config.URL+"/library/sections", nil)
	if err != nil {
		return libraries, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return libraries, err
	}

	var schema librarySchema
	if err := json.NewDecoder(resp.Body).Decode(&schema); err != nil {
		p.logger.Error().Err(err).Msg("Failed to decode library response")
		return libraries, err
	}

	for _, dir := range schema.MediaContainer.Directory {
		libraries = append(libraries, types.Library{
			ID:   dir.Key,
			Type: dir.Type,
			Name: dir.Title,
		})
	}

	return libraries, nil
}
