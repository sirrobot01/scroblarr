package scrobble

import (
	"context"
	"github.com/rs/zerolog"
	"github.com/sirrobot01/scroblarr/internal/config"
	"github.com/sirrobot01/scroblarr/internal/media_servers"
	"github.com/sirrobot01/scroblarr/internal/trakt"
	"github.com/sirrobot01/scroblarr/internal/types"
	"github.com/sirrobot01/scroblarr/pkg/logger"
	"sync"
	"time"
)

type Sync struct {
	source   media_servers.Server
	targets  []media_servers.Server
	trakt    *trakt.Client
	interval time.Duration
	logger   zerolog.Logger
	sessions *types.MediaSessionHistory
}

type Scrobble struct {
	syncs     map[string]*Sync
	syncsLock sync.Mutex
	logger    zerolog.Logger
}

func New(servers map[string]media_servers.Server) (*Scrobble, error) {
	cfg := config.Get()
	_logger := logger.NewLogger("scrobble")
	traktClient := trakt.New()

	syncs := make(map[string]*Sync)
	for _, s := range cfg.Sync {
		skipTrakt := true
		source, ok := servers[s.Source]
		if !ok {
			_logger.Info().Msgf("Source server %s not found, skipping sync", s.Source)
			continue
		}
		targets := make([]media_servers.Server, 0)
		for _, t := range s.Targets {
			if t == s.Source {
				_logger.Info().Msgf("Skipping sync to self (%s) for %s", s.Source, s.Name)
				continue
			}
			if t == "trakt" {
				skipTrakt = false
				if traktClient == nil {
					_logger.Info().Msgf("Trakt client is not initialized, skipping sync for %s", s.Name)
					skipTrakt = true
					continue
				}
				continue
			}

			target, ok := servers[t]
			if !ok {
				_logger.Info().Msgf("Target server %s not found, skipping sync for %s", t, s.Name)
				continue
			}
			targets = append(targets, target)
		}
		_interval := cfg.Interval
		if s.Interval != nil {
			_interval = *s.Interval
		}

		interval, err := time.ParseDuration(_interval)
		if err != nil {
			interval = 30 * time.Second
		}

		syn := &Sync{
			source:   source,
			sessions: types.NewMediaSessionHistory(),
			targets:  targets,
			interval: interval,
			logger:   _logger.With().Str("Sync", s.Name).Str("Source", source.GetName()).Logger(),
		}
		if !skipTrakt {
			syn.trakt = traktClient
			syn.logger.Info().Msg("Trakt sync enabled")
		} else {
			syn.logger.Info().Msg("Trakt sync disabled")
		}
		syncs[s.Name] = syn
	}

	s := &Scrobble{
		syncs:  syncs,
		logger: _logger,
	}
	return s, nil
}

func (s *Sync) scrobble(ctx context.Context) error {
	s.logger.Info().Msg("starting scrobble")

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info().Msg("context cancelled, stopping scrobble")
			return nil
		case <-ticker.C:
			activeSessions, err := s.source.GetSessions() // Get Active Sessions
			if err != nil {
				s.logger.Error().Err(err).Msgf("Error getting sessions")
				continue
			}
			totalActiveSessions := len(activeSessions)
			if totalActiveSessions > 0 {
				s.logger.Debug().Msgf("Found %d active sessions", totalActiveSessions)
			}
			s.sync(activeSessions)
		}
	}
}

func (s *Sync) sync(activeSessions []types.MediaSession) {
	// Set active sessions in the history
	s.sessions.SetMany(activeSessions)

	sessions := s.sessions.GetAll()

	// If active sessions are empty, mark all existing sessions as stopped
	if len(activeSessions) == 0 {
		for _, session := range sessions {
			session.State = "stopped"
			s.sessions.Set(session)
		}
	}

	for _, session := range sessions {
		if s.trakt != nil {
			s.syncToTrakt(session)
		}

		for _, target := range s.targets {
			action := getAction(session)
			if action == "stop" && session.Progress > 90 {
				session.Progress = 100 // Set progress to 100% for completed items
			}
			if err := target.Scrobble(session, action); err != nil {
				s.logger.Error().Err(err).Msgf("Error scrobbling from %s", target.GetName())
			} else {
				s.logger.Trace().Msgf("[%s] Scrobbled %s: %s at %.2f%%", target.GetName(), action, session.Title, session.Progress)
			}
		}
	}

}

func (s *Sync) syncToTrakt(session types.MediaSession) {
	action := getAction(session)
	if action == "stop" && session.Progress > 90 {
		session.Progress = 100 // Set progress to 100% for completed items
	}

	if err := s.trakt.Scrobble(session, action); err != nil {
		s.logger.Error().Err(err).Msgf("Error syncing to Trakt")
	} else {
		s.logger.Trace().Msgf("Synced to Trakt: %s %s at %.2f%%", action, session.Title, session.Progress)
	}
}

func (s *Scrobble) Scrobble(ctx context.Context) {
	s.logger.Info().Msg("Starting scrobble process")

	for _, syn := range s.syncs {
		go func(s *Sync) {
			if err := s.scrobble(ctx); err != nil {
				s.logger.Error().Err(err).Msgf("Error in scrobble")
			}
		}(syn)
	}
}

func getAction(session types.MediaSession) string {
	var action string
	switch session.State {
	case "playing":
		action = "start"
	case "paused":
		action = "pause"
	case "stopped":
		if session.Progress > 90 {
			action = "stop"
		} else {
			action = "pause"
		}
	default:
		action = "start"
	}
	return action
}

//func (s *Scrobble) SyncHistory() {
//	s.logger.Info().Msgf("Starting full history sync")
//
//	newHistory := make(types.MediaSessionHistory)
//
//	for name, c := range s.sources {
//		s.logger.Debug().Msgf("Syncing history from %s", name)
//		history, err := c.GetWatchHistory()
//		if err != nil {
//			s.logger.Info().Msgf("Error getting watch history from %s: %v", name, err)
//			continue
//		}
//
//		for _, item := range history {
//			key := historyKey(item)
//			if _, ok := s.history[key]; ok {
//				// Skip if we already have this item
//				continue
//			}
//			// Set progress to 100% for completed items
//			item.Progress = 100
//			item.Source = name
//			newHistory[key] = item
//		}
//	}
//	s.historyMutex.Lock()
//	s.history = newHistory
//	s.historyMutex.Unlock()
//
//	items := make([]types.MediaSession, 0, len(newHistory))
//	for _, item := range newHistory {
//		items = append(items, item)
//	}
//	// Sort by ViewedAt
//	sort.Slice(items, func(i, j int) bool {
//		return items[i].ViewedAt > items[j].ViewedAt
//	})
//
//	for _, item := range items {
//		s.sourcesLock.RLock()
//		cl, ok := s.sources[item.Source]
//		s.sourcesLock.RUnlock()
//		if !ok {
//			s.logger.Info().Msgf("Client %s not found", item.Source)
//			continue
//		}
//		// If the client is configured to skip Trakt sync, skip it
//
//		if !cl.GetConfig().SkipTraktSync {
//			s.syncToTrakt(item)
//		}
//
//		for _, c := range s.getDestinations(cl) {
//			if err := c.Scrobble(item, "stop"); err != nil {
//				s.logger.Debug().Msgf("Error syncing history from %s: %v", c.GetServerType(), err)
//			} else {
//				s.logger.Trace().Msgf("Synced %s with %s", c.GetServerType(), item.Title)
//			}
//		}
//
//	}
//	go s.saveHistory()
//
//	s.logger.Info().Msgf("Finished full history sync")
//}

func (s *Scrobble) Stop() {
	s.logger.Info().Msg("Stopping scrobbling process")
}

//func (s *Scrobble) loadHistory() {
//	s.logger.Info().Msg("Loading history")
//	cfg := config.Get()
//	path := filepath.Join(cfg.Path, "history.json")
//	if _, err := os.Stat(path); os.IsNotExist(err) {
//		s.history = make(types.MediaSessionHistory)
//		return
//	}
//	data, err := os.ReadFile(path)
//	if err != nil {
//		s.logger.Info().Msgf("Error reading history file: %v", err)
//		s.history = make(types.MediaSessionHistory)
//		return
//	}
//	var history types.MediaSessionHistory
//	if err := json.Unmarshal(data, &history); err != nil {
//		s.logger.Info().Msgf("Error parsing history file: %v", err)
//	}
//
//	s.logger.Info().Msgf("Loaded %d media history", len(history))
//	s.history = history
//}
//
//func (s *Scrobble) saveHistory() {
//	cfg := config.Get()
//	path := filepath.Join(cfg.Path, "history.json")
//	s.historyMutex.RLock()
//	data, err := json.Marshal(s.history)
//	s.historyMutex.RUnlock()
//	if err != nil {
//		s.logger.Info().Msgf("Error saving history file: %v", err)
//	}
//	if err := os.WriteFile(path, data, 0644); err != nil {
//		s.logger.Info().Msgf("Error saving history file: %v", err)
//	}
//}
