// Package config handles application configuration management
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Config holds all application configuration
type Config struct {
	// Server
	HTTPAddr string `json:"http_addr"`

	// Storage
	DataDir       string `json:"data_dir" validate:"required"`
	BlacklistFile string `json:"blacklist_file"`

	// Services
	AllDebridAPIKey   string `json:"alldebrid_api_key" validate:"required"`
	TraktAPIKey       string `json:"trakt_api_key" validate:"required"`
	TraktClientSecret string `json:"trakt_client_secret" validate:"required"`
	TMDBAPIKey        string `json:"tmdb_api_key"`

	// Settings
	SyncInterval   string `json:"sync_interval"`
	WatchedDays    int    `json:"watched_days"`
	MaxRetries     int    `json:"max_retries"`
	RequestTimeout int    `json:"request_timeout"`
}

// Default configuration values
const (
	defaultHTTPAddr       = ":8080"
	defaultSyncInterval   = "6h"
	defaultBlacklistFile  = "blacklist.txt"
	defaultWatchedDays    = 5
	defaultMaxRetries     = 3
	defaultRequestTimeout = 30
)

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	cfg := &Config{
		HTTPAddr:       getEnv("HTTP_ADDR", defaultHTTPAddr),
		SyncInterval:   getEnv("SYNC_INTERVAL", defaultSyncInterval),
		BlacklistFile:  getEnv("BLACKLIST_FILE", defaultBlacklistFile),
		TMDBAPIKey:     getEnv("TMDB_API_KEY", ""),
		WatchedDays:    getEnvInt("WATCHED_DAYS", defaultWatchedDays),
		MaxRetries:     getEnvInt("MAX_RETRIES", defaultMaxRetries),
		RequestTimeout: getEnvInt("REQUEST_TIMEOUT", defaultRequestTimeout),
	}

	if err := loadRequiredFields(cfg); err != nil {
		return nil, err
	}

	cfg.normalizeBlacklistPath()

	return cfg, nil
}

// GetServerAddress returns the HTTP server address
func (c *Config) GetServerAddress() string {
	return c.HTTPAddr
}

// Validate checks if all required fields are set
func (c *Config) Validate() error {
	if c.DataDir == "" {
		return fmt.Errorf("data directory required")
	}
	if c.AllDebridAPIKey == "" {
		return fmt.Errorf("allDebrid API key required")
	}
	if c.TraktAPIKey == "" || c.TraktClientSecret == "" {
		return fmt.Errorf("trakt credentials required")
	}
	return nil
}

// loadRequiredFields loads all required environment variables
func loadRequiredFields(cfg *Config) error {
	requiredFields := map[string]*string{
		"DATA_DIR":            &cfg.DataDir,
		"ALLDEBRID_API_KEY":   &cfg.AllDebridAPIKey,
		"TRAKT_API_KEY":       &cfg.TraktAPIKey,
		"TRAKT_CLIENT_SECRET": &cfg.TraktClientSecret,
	}

	for env, field := range requiredFields {
		value := os.Getenv(env)
		if value == "" {
			return fmt.Errorf("required env var %s not set", env)
		}
		*field = value
	}

	return nil
}

// normalizeBlacklistPath ensures blacklist file has absolute path
func (c *Config) normalizeBlacklistPath() {
	if c.BlacklistFile == "" {
		return
	}

	if !filepath.IsAbs(c.BlacklistFile) {
		c.BlacklistFile = filepath.Join(c.DataDir, c.BlacklistFile)
	}
}

// getEnv retrieves environment variable with default fallback
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt retrieves integer environment variable with default fallback
func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return intValue
}
