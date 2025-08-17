package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	HTTPAddr string `json:"http_addr"`

	DataDir       string `json:"data_dir" validate:"required"`
	BlacklistFile string `json:"blacklist_file"`

	AllDebridAPIKey   string `json:"alldebrid_api_key" validate:"required"`
	TraktAPIKey       string `json:"trakt_api_key" validate:"required"`
	TraktClientSecret string `json:"trakt_client_secret" validate:"required"`
	TMDBAPIKey        string `json:"tmdb_api_key"`

	SyncInterval   string `json:"sync_interval"`
	WatchedDays    int    `json:"watched_days"`
	MaxRetries     int    `json:"max_retries"`
	RequestTimeout int    `json:"request_timeout"`
}

const (
	defaultHTTPAddr       = ":8080"
	defaultSyncInterval   = "6h"
	defaultBlacklistFile  = "blacklist.txt"
	defaultWatchedDays    = 5
	defaultMaxRetries     = 3
	defaultRequestTimeout = 30
)

func LoadConfig() (*Config, error) {
	cfg := createConfigWithDefaults()

	if err := loadRequiredFields(cfg); err != nil {
		return nil, err
	}

	cfg.normalizeBlacklistPath()

	return cfg, nil
}

func createConfigWithDefaults() *Config {
	return &Config{
		HTTPAddr:       getEnv("HTTP_ADDR", defaultHTTPAddr),
		SyncInterval:   getEnv("SYNC_INTERVAL", defaultSyncInterval),
		BlacklistFile:  getEnv("BLACKLIST_FILE", defaultBlacklistFile),
		TMDBAPIKey:     getEnv("TMDB_API_KEY", ""),
		WatchedDays:    getEnvInt("WATCHED_DAYS", defaultWatchedDays),
		MaxRetries:     getEnvInt("MAX_RETRIES", defaultMaxRetries),
		RequestTimeout: getEnvInt("REQUEST_TIMEOUT", defaultRequestTimeout),
	}
}

func (c *Config) GetServerAddress() string {
	return c.HTTPAddr
}

func (c *Config) Validate() error {
	if c.DataDir == "" {
		return fmt.Errorf("data directory required")
	}
	if c.AllDebridAPIKey == "" {
		return fmt.Errorf("AllDebrid API key required")
	}
	if c.TraktAPIKey == "" {
		return fmt.Errorf("Trakt API key required")
	}
	if c.TraktClientSecret == "" {
		return fmt.Errorf("Trakt client secret required")
	}
	return nil
}

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

func (c *Config) normalizeBlacklistPath() {
	if c.BlacklistFile == "" {
		return
	}

	if !filepath.IsAbs(c.BlacklistFile) {
		c.BlacklistFile = filepath.Join(c.DataDir, c.BlacklistFile)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

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
