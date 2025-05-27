package emby_jellyfin

import (
	"fmt"
	"github.com/sirrobot01/scroblarr/internal/config"
	"github.com/sirrobot01/scroblarr/pkg/logger"
)

type Jellyfin struct {
	BaseServer
}

func NewJellyfin(name string, config config.Server) (*Jellyfin, error) {
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

	s := &Jellyfin{
		BaseServer{
			name:   name,
			config: config,
			logger: _logger,
			client: client,
		},
	}

	if err := s.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to Jellyfin: %w", err)
	}
	return s, nil
}

func (j *Jellyfin) GetServerType() string {
	return "jellyfin"
}

func (j *Jellyfin) Connect() error {
	j.logger.Info().Str("Server", j.name).Msgf("Connected to Jellyfin server: %s", j.name)
	return nil
}
