package config

import (
	"fmt"
	"os"
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

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	config := &Config{
		HTTPAddr:        getEnv("HTTP_ADDR", ":8080"),
		SyncInterval:    getEnv("SYNC_INTERVAL", "6h"),
		BlacklistFile:   getEnv("BLACKLIST_FILE", "blacklist.txt"),
		TMDBAPIKey:      getEnv("TMDB_API_KEY", ""),
		WatchedDays:     getEnvInt("WATCHED_DAYS", 5),
		MaxRetries:      getEnvInt("MAX_RETRIES", 3),
		RequestTimeout:  getEnvInt("REQUEST_TIMEOUT", 30),
	}
	
	// Required fields
	requiredFields := map[string]*string{
		"DATA_DIR":              &config.DataDir,
		"ALLDEBRID_API_KEY":     &config.AllDebridAPIKey,
		"TRAKT_API_KEY":         &config.TraktAPIKey,
		"TRAKT_CLIENT_SECRET":   &config.TraktClientSecret,
	}
	
	for env, field := range requiredFields {
		value := os.Getenv(env)
		if value == "" {
			return nil, fmt.Errorf("required env var %s not set", env)
		}
		*field = value
	}
	
	// Set blacklist file path
	if config.BlacklistFile != "" && !os.IsPathSeparator(config.BlacklistFile[0]) {
		config.BlacklistFile = fmt.Sprintf("%s/%s", config.DataDir, config.BlacklistFile)
	}
	
	return config, nil
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
		return fmt.Errorf("AllDebrid API key required")
	}
	if c.TraktAPIKey == "" || c.TraktClientSecret == "" {
		return fmt.Errorf("Trakt credentials required")
	}
	return nil
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}