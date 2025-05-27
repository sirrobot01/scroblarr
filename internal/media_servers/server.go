package media_servers

import (
	"fmt"
	"github.com/sirrobot01/scroblarr/internal/config"
	"github.com/sirrobot01/scroblarr/internal/media_servers/emby_jellyfin"
	"github.com/sirrobot01/scroblarr/internal/media_servers/plex"
	"github.com/sirrobot01/scroblarr/internal/types"
	"github.com/sirrobot01/scroblarr/pkg/logger"
)

// Server is the interface all media server clients must implement
type Server interface {
	GetSessions() ([]types.MediaSession, error)
	GetWatchHistory() ([]types.MediaSession, error)
	Connect() error
	GetName() string
	GetServerType() string
	Scrobble(session types.MediaSession, action string) error
	SyncHistory(session types.MediaSession) error
	GetConfig() config.Server
}

// NewServer creates a new media server client based on the configuration
func newServer(name string, config config.Server) (Server, error) {
	switch config.Type {
	case "plex":
		return plex.New(name, config)
	case "jellyfin":
		return emby_jellyfin.NewJellyfin(name, config)
	case "emby":
		return emby_jellyfin.NewEmby(name, config)
	default:
		return nil, fmt.Errorf("unsupported media server type: %s", config.Type)
	}
}

func New() (map[string]Server, error) {
	cfg := config.Get()
	_log := logger.GetDefault()
	servers := make(map[string]Server)
	for i, c := range cfg.Servers {
		if server, err := newServer(i, c); err != nil {
			_log.Error().Err(err).Msgf("Failed to connect to %s", i)
		} else {
			servers[i] = server
		}
	}

	if len(servers) == 0 {
		return nil, fmt.Errorf("no server found")
	}
	return servers, nil
}
