package types

import (
	"fmt"
	"sync"
)

func GetHistoryKey(session MediaSession) string {
	return fmt.Sprintf("%s-%s-%s", session.Type, session.ShowTitle, session.EpisodeTitle)
}

type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type Library struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // "movie", "show", "music", etc.
}

// MediaSession represents a media playback session
type MediaSession struct {
	SessionID    string  `json:"session_id"`
	Title        string  `json:"title"`
	Year         int     `json:"year"`
	Type         string  `json:"type"`  // "movie" or "episode"
	State        string  `json:"state"` // "playing", "paused", "stopped"
	Progress     float64 `json:"progress"`
	Duration     int64   `json:"duration"`
	ViewOffset   int64   `json:"view_offset"`
	IMDBID       string  `json:"imdb_id"`
	TVDBID       string  `json:"tvdb_id"`
	SeasonNum    int     `json:"season_num"`
	EpisodeNum   int     `json:"episode_num"`
	ShowTitle    string  `json:"show_title"`
	EpisodeTitle string  `json:"episode_title"`
	ViewedAt     int64   `json:"viewed_at"`
	User         User    `json:"user"` // User who is watching the session
	Source       string  `json:"source"`
	LibraryID    string  `json:"library_id"`
	LibraryName  string  `json:"library_name"`
	LibraryType  string  `json:"library_type"` // "movie", "show", "music", etc.
}

// MediaSessionHistory is a map of session type and title to MediaSession
type MediaSessionHistory struct {
	sessions map[string]MediaSession
	lock     sync.RWMutex
}

func NewMediaSessionHistory() *MediaSessionHistory {
	return &MediaSessionHistory{
		sessions: make(map[string]MediaSession),
		lock:     sync.RWMutex{},
	}
}

func (h *MediaSessionHistory) Get(key string) (MediaSession, bool) {
	h.lock.RLock()
	defer h.lock.RUnlock()
	session, exists := h.sessions[key]
	return session, exists
}

func (h *MediaSessionHistory) Set(session MediaSession) {
	h.lock.Lock()
	defer h.lock.Unlock()
	key := GetHistoryKey(session)
	h.sessions[key] = session
}
func (h *MediaSessionHistory) Delete(key string) {
	h.lock.Lock()
	defer h.lock.Unlock()
	delete(h.sessions, key)
}

func (h *MediaSessionHistory) GetAll() []MediaSession {
	h.lock.RLock()
	defer h.lock.RUnlock()
	sessions := make([]MediaSession, 0, len(h.sessions))
	for _, session := range h.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

func (h *MediaSessionHistory) Clear() {
	h.lock.Lock()
	defer h.lock.Unlock()
	h.sessions = make(map[string]MediaSession)
}

func (h *MediaSessionHistory) SetMany(sessions []MediaSession) {
	h.lock.Lock()
	defer h.lock.Unlock()
	for _, session := range sessions {
		key := GetHistoryKey(session)
		h.sessions[key] = session
	}
}
