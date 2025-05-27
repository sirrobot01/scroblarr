package emby_jellyfin

import (
	"fmt"
	"github.com/sirrobot01/scroblarr/internal/config"
	"github.com/sirrobot01/scroblarr/pkg/logger"
)

// Server implements the Server interface for Emby

type Emby struct {
	BaseServer
}

// NewEmby creates a new Emby client
func NewEmby(name string, config config.Server) (*Emby, error) {

	if config.URL == "" {
		return nil, fmt.Errorf("missing required URL")
	}

	if config.Token == "" && (config.Username == "" || config.Password == "") {
		return nil, fmt.Errorf("missing authentication information")
	}

	_logger := logger.NewLogger(name)

	client, err := getClient(config, _logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	s := &Emby{
		BaseServer{
			name:   name,
			config: config,
			logger: _logger,
			client: client,
		},
	}

	if err := s.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to Plex: %w", err)
	}
	return s, nil
}

func (e *Emby) GetServerType() string {
	return "emby"
}
