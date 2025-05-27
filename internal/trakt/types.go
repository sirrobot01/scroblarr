package trakt

// ScrobbleRequest represents a request to Trakt's scrobble API
type ScrobbleRequest struct {
	Movie      *Movie   `json:"movie,omitempty"`
	Episode    *Episode `json:"episode,omitempty"`
	Show       *Show    `json:"show,omitempty"`
	Progress   float64  `json:"progress"`
	AppVersion string   `json:"app_version"`
}

// Movie represents a movie in Trakt's API
type Movie struct {
	Title string            `json:"title"`
	Year  int               `json:"year,omitempty"`
	IDs   map[string]string `json:"ids"`
}

// Episode represents an episode in Trakt's API
type Episode struct {
	Title  string            `json:"title"`
	Season int               `json:"season"`
	Number int               `json:"number"`
	IDs    map[string]string `json:"ids,omitempty"`
}

// Show represents a show in Trakt's API
type Show struct {
	Title string            `json:"title"`
	IDs   map[string]string `json:"ids,omitempty"`
}
