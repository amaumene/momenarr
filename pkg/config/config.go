package config

import (
	"fmt"
	"os"
	"strconv"
)

// NewConfig holds all application configuration with AllDebrid
type NewConfig struct {
	// Server configuration
	HTTPAddr string `json:"http_addr"`

	// Storage configuration
	DataDir       string `json:"data_dir" validate:"required"`
	BlacklistFile string `json:"blacklist_file"`

	// AllDebrid configuration
	AllDebridAPIKey string `json:"alldebrid_api_key" validate:"required"`

	// Trakt configuration
	TraktAPIKey       string `json:"trakt_api_key" validate:"required"`
	TraktClientSecret string `json:"trakt_client_secret" validate:"required"`

	// Application settings
	SyncInterval   string `json:"sync_interval"`
	WatchedDays    int    `json:"watched_days"`
	MaxRetries     int    `json:"max_retries"`
	RequestTimeout int    `json:"request_timeout"`
}

// LoadNewConfig loads configuration from environment variables
func LoadNewConfig() (*NewConfig, error) {
	config := &NewConfig{
		HTTPAddr:       getEnvOrDefault("HTTP_ADDR", ":8080"),
		SyncInterval:   getEnvOrDefault("SYNC_INTERVAL", "6h"),
		BlacklistFile:  getEnvOrDefault("BLACKLIST_FILE", "blacklist.txt"),
		WatchedDays:    getEnvIntOrDefault("WATCHED_DAYS", 5),
		MaxRetries:     getEnvIntOrDefault("MAX_RETRIES", 3),
		RequestTimeout: getEnvIntOrDefault("REQUEST_TIMEOUT", 30),
	}

	// Required environment variables
	var err error
	if config.DataDir, err = getRequiredEnv("DATA_DIR"); err != nil {
		return nil, err
	}
	if config.AllDebridAPIKey, err = getRequiredEnv("ALLDEBRID_API_KEY"); err != nil {
		return nil, err
	}
	if config.TraktAPIKey, err = getRequiredEnv("TRAKT_API_KEY"); err != nil {
		return nil, err
	}
	if config.TraktClientSecret, err = getRequiredEnv("TRAKT_CLIENT_SECRET"); err != nil {
		return nil, err
	}

	// Set blacklist file path if not absolute
	if config.BlacklistFile != "" && !os.IsPathSeparator(config.BlacklistFile[0]) {
		config.BlacklistFile = fmt.Sprintf("%s/%s", config.DataDir, config.BlacklistFile)
	}

	return config, nil
}

// GetServerAddress returns the full server address
func (c *NewConfig) GetServerAddress() string {
	return c.HTTPAddr
}

// Validate validates the configuration
func (c *NewConfig) Validate() error {
	if c.DataDir == "" {
		return fmt.Errorf("data directory is required")
	}
	if c.AllDebridAPIKey == "" {
		return fmt.Errorf("AllDebrid API key is required")
	}
	if c.TraktAPIKey == "" || c.TraktClientSecret == "" {
		return fmt.Errorf("Trakt configuration is required")
	}
	return nil
}

// Helper functions (reuse from original config.go)
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getRequiredEnv(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("required environment variable %s is not set", key)
	}
	return value, nil
}
