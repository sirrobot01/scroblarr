package scroblarr

import (
	"context"
	"fmt"
	"github.com/sirrobot01/scroblarr/internal/config"
	"github.com/sirrobot01/scroblarr/internal/media_servers"
	"github.com/sirrobot01/scroblarr/internal/scrobble"
	"github.com/sirrobot01/scroblarr/pkg/logger"
	"github.com/sirrobot01/scroblarr/web"
	"os/signal"
	"sync"
	"syscall"
)

func Start(ctx context.Context) error {
	// Initialize Trakt client
	cfg := config.Get()
	var wg sync.WaitGroup
	errChan := make(chan error)
	_log := logger.GetDefault()
	// Initialize media servers
	servers, err := media_servers.New()
	//traktClient, err := trakt.GetClient()

	if err != nil {
		return fmt.Errorf("error creating media server clients: %v", err)
	}

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	webServer := web.New()

	// Create a new scrobble instance

	interval := cfg.GetInterval()
	if interval != 0 {
		scrobbler, err := scrobble.New(servers)
		if err != nil {
			return fmt.Errorf("error creating scrobbler: %v", err)
		}

		//wg.Add(1)
		//go func() {
		//	scrobbler.SyncHistory()
		//}()

		wg.Add(1)
		go func() {
			scrobbler.Scrobble(ctx)
		}()
	} else {
		_log.Info().Msg("Scrobbling disabled")
	}

	wg.Add(1)
	go func() {
		if err := webServer.Start(ctx); err != nil {
			_log.Info().Msgf("Web server error: %v", err)
		}
	}()

	go func() {
		wg.Wait()
		close(errChan)
	}()

	// Wait for context cancellation or completion or error
	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
