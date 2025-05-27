package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3" // You'll need to add this dependency
)

type ClientType string

var (
	instance   *Config
	once       sync.Once
	configPath string     = "config.yaml" // Changed file extension
	Plex       ClientType = "plex"
	Jellyfin   ClientType = "jellyfin"
	Emby       ClientType = "emby"
	Tautulli   ClientType = "tautulli"
)

type Server struct {
	Type     ClientType `yaml:"type,omitempty" json:"type,omitempty"` // Changed from json to yaml tags
	URL      string     `yaml:"url,omitempty" json:"url,omitempty"`
	Token    string     `yaml:"token,omitempty" json:"token,omitempty"`
	Username string     `yaml:"username,omitempty" json:"username,omitempty"`
	Password string     `yaml:"password,omitempty" json:"password,omitempty"`
}

type Trakt struct {
	Enabled      bool    `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Interval     *string `yaml:"interval,omitempty" json:"interval,omitempty"`
	AccessToken  string  `yaml:"access_token,omitempty" json:"access_token,omitempty"`
	RefreshToken string  `yaml:"refresh_token,omitempty" json:"refresh_token,omitempty"`
	ExpiresIn    int     `yaml:"expires_in,omitempty" json:"expires_in,omitempty"`
	TokenType    string  `yaml:"token_type,omitempty" json:"token_type,omitempty"`
}

type Sync struct {
	Name     string   `yaml:"name,omitempty" json:"name,omitempty"`       // Name of the sync destination
	Source   string   `yaml:"source,omitempty" json:"source,omitempty"`   // Source server name
	Targets  []string `yaml:"targets,omitempty" json:"targets,omitempty"` // List of target server names
	Interval *string  `yaml:"interval,omitempty" json:"interval,omitempty"`
}

type Config struct {
	Servers      map[string]Server `yaml:"servers,omitempty" json:"servers,omitempty"`
	Trakt        *Trakt            `yaml:"-" json:"-"` // Trakt configuration, loaded separately
	TraktEnabled bool              `yaml:"-" json:"-"` // Indicates if Trakt is enabled
	TraktDetails struct {
		ClientID     string `yaml:"client_id,omitempty" json:"client_id,omitempty"`
		ClientSecret string `yaml:"client_secret,omitempty" json:"client_secret,omitempty"`
	} `yaml:"trakt,omitempty" json:"trakt,omitempty"` // Trakt details, if enabled
	Interval string `yaml:"interval,omitempty" json:"interval,omitempty"`
	Sync     []Sync `yaml:"sync,omitempty" json:"sync,omitempty"` // List of sync configurations
	Path     string `yaml:"-" json:"-"`
	LogLevel string `yaml:"log_level,omitempty" json:"log_level,omitempty"`
	Port     int    `yaml:"port,omitempty" json:"port,omitempty"`
}

func (c *Config) GetTraktClientID() string {
	return ""
}

func SetConfigPath(path string) {
	configPath = path
}

func Get() *Config {
	once.Do(func() {
		instance = &Config{}
		if err := instance.loadConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "configuration Error: %v\n", err)
			os.Exit(1)
		}
	})
	return instance
}

func (c *Config) configFilePath() string {
	return filepath.Join(c.Path, "config.yaml") // Changed file name
}

func (c *Config) loadConfig() error {
	if configPath == "" {
		return fmt.Errorf("config path not set")
	}
	c.Path = configPath
	path := c.configFilePath()

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := c.Create(); err != nil {
			return fmt.Errorf("error creating config file: %w", err)
		}
		return nil
	}

	// Read config file
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	if err := yaml.Unmarshal(data, &c); err != nil { // Changed from json to yaml
		return fmt.Errorf("error parsing config file: %w", err)
	}

	if c.Interval == "" {
		c.Interval = "10s"
	}
	if c.Port == 0 {
		c.Port = 8080
	}

	c.TraktEnabled = false

	// load trakt clients
	trakt, err := c.loadTrakt()
	if trakt != nil && err == nil {
		c.TraktEnabled = true
		c.Trakt = trakt
	}

	// Validate required fields
	if err := c.Validate(); err != nil {
		return err
	}

	return nil
}

func (c *Config) Create() error {
	defaultClients := map[string]Server{
		"plex": {
			Type:  Plex,
			URL:   "http://localhost:32400",
			Token: "your-plex-token",
		},
	}
	c.Servers = defaultClients
	c.Interval = "10s"
	c.Path = configPath
	c.LogLevel = "info"
	c.Port = 8080

	if err := c.Save(); err != nil {
		return err
	}
	return nil
}

func (c *Config) Validate() error {
	if len(c.Servers) == 0 {
		return errors.New("no servers configured")
	}

	// Validate each client
	for name, server := range c.Servers {
		if server.URL == "" {
			return fmt.Errorf("server %s URL is required", name)
		}
		if server.Type == "" {
			return fmt.Errorf("server %s type is required", name)
		}
		if server.Type != Plex && server.Type != Jellyfin && server.Type != Emby {
			return fmt.Errorf("server %s has an invalid type: %s", name, server.Type)
		}
	}

	// Validate Sync config
	for _, _sync := range c.Sync {
		if _sync.Name == "" {
			return errors.New("sync name is required")
		}
		if _sync.Source == "" {
			return fmt.Errorf("sync %s source is required", _sync.Name)
		}
		//if len(_sync.Targets) == 0 {
		//	return fmt.Errorf("sync %s targets are required", _sync.Name)
		//}
		for _, target := range _sync.Targets {
			if target == "" {
				return fmt.Errorf("sync %s has an empty target", _sync.Name)
			}
		}
		if _sync.Interval != nil && *_sync.Interval == "0" {
			return fmt.Errorf("sync %s interval cannot be zero", _sync.Name)
		}
	}

	return nil
}

func (c *Config) Save() error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("error encoding config: %w", err)
	}
	if err := os.WriteFile(c.configFilePath(), data, 0644); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}
	// Update the instance
	instance = c
	return nil
}

func (c *Config) GetInterval() time.Duration {
	if c.Interval == "0" {
		return 0
	}
	d, err := time.ParseDuration(c.Interval)
	if err != nil {
		return 30 * time.Second
	}
	return d
}

func (c *Config) SaveTrakt() error {
	if c.Trakt == nil {
		return errors.New("trakt config not loaded")
	}
	data, err := json.MarshalIndent(c.Trakt, "", "  ")
	if err != nil {
		return fmt.Errorf("error encoding trakt config: %w", err)
	}
	if err := os.WriteFile(filepath.Join(c.Path, "trakt.json"), data, 0644); err != nil {
		return fmt.Errorf("error writing trakt config file: %w", err)
	}
	return nil
}

func (c *Config) loadTrakt() (*Trakt, error) {
	if _, err := os.Stat(filepath.Join(c.Path, "trakt.json")); os.IsNotExist(err) {
		return nil, nil
	}
	data, err := os.ReadFile(filepath.Join(c.Path, "trakt.json"))
	if err != nil {
		return nil, fmt.Errorf("error reading trakt config file: %w", err)
	}
	var trakt *Trakt
	if err := json.Unmarshal(data, &trakt); err != nil {
		return nil, fmt.Errorf("error parsing trakt config file: %w", err)
	}
	return trakt, nil
}

func (c *Config) RefreshTrakt() error {
	// Make request to Trakt API to get device code
	payload := map[string]string{
		"client_id": "4ee97aae28ec4797b76a7c97d2655286e3c113124028339c9c08d9ab12a2f81a",
	}
	jsonPayload, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", "https://api.trakt.tv/oauth/device/code", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	_, err = client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to contact Trakt API: %w", err)
	}
	if err := c.SaveTrakt(); err != nil {
		return fmt.Errorf("error saving trakt config: %w", err)
	}

	return nil
}
