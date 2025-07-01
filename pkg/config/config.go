package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all application configuration
type Config struct {
	// Server configuration
	Port string `json:"port" validate:"required"`
	Host string `json:"host"`

	// Storage configuration
	DownloadDir string `json:"download_dir" validate:"required"`
	DataDir     string `json:"data_dir" validate:"required"`

	// Newsnab configuration
	NewsNabHost   string `json:"newsnab_host" validate:"required,url"`
	NewsNabAPIKey string `json:"newsnab_api_key" validate:"required"`

	// Trakt configuration
	TraktAPIKey      string `json:"trakt_api_key" validate:"required"`
	TraktClientSecret string `json:"trakt_client_secret" validate:"required"`

	// NZBGet configuration
	NZBGetHost     string `json:"nzbget_host" validate:"required"`
	NZBGetPort     int    `json:"nzbget_port" validate:"required,min=1,max=65535"`
	NZBGetUsername string `json:"nzbget_username" validate:"required"`
	NZBGetPassword string `json:"nzbget_password" validate:"required"`

	// Application settings
	SyncInterval    string `json:"sync_interval"`
	BlacklistFile   string `json:"blacklist_file"`
	MaxRetries      int    `json:"max_retries"`
	RequestTimeout  int    `json:"request_timeout"`
	TestMode        bool   `json:"test_mode"` // When true, only shows selected NZBs without downloading or storing
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	config := &Config{
		Port:            getEnvOrDefault("PORT", "3000"),
		Host:            getEnvOrDefault("HOST", "0.0.0.0"),
		SyncInterval:    getEnvOrDefault("SYNC_INTERVAL", "6h"),
		BlacklistFile:   getEnvOrDefault("BLACKLIST_FILE", "blacklist.txt"),
		MaxRetries:      getEnvIntOrDefault("MAX_RETRIES", 3),
		RequestTimeout:  getEnvIntOrDefault("REQUEST_TIMEOUT", 30),
		TestMode:        getEnvBoolOrDefault("TEST_MODE", false),
	}

	// Required environment variables
	var err error
	if config.DownloadDir, err = getRequiredEnv("DOWNLOAD_DIR"); err != nil {
		return nil, err
	}
	if config.DataDir, err = getRequiredEnv("DATA_DIR"); err != nil {
		return nil, err
	}
	if config.NewsNabHost, err = getRequiredEnv("NEWSNAB_HOST"); err != nil {
		return nil, err
	}
	if config.NewsNabAPIKey, err = getRequiredEnv("NEWSNAB_API_KEY"); err != nil {
		return nil, err
	}
	if config.TraktAPIKey, err = getRequiredEnv("TRAKT_API_KEY"); err != nil {
		return nil, err
	}
	if config.TraktClientSecret, err = getRequiredEnv("TRAKT_CLIENT_SECRET"); err != nil {
		return nil, err
	}
	if config.NZBGetHost, err = getRequiredEnv("NZBGET_HOST"); err != nil {
		return nil, err
	}
	if config.NZBGetPort, err = getRequiredEnvInt("NZBGET_PORT"); err != nil {
		return nil, err
	}
	if config.NZBGetUsername, err = getRequiredEnv("NZBGET_USERNAME"); err != nil {
		return nil, err
	}
	if config.NZBGetPassword, err = getRequiredEnv("NZBGET_PASSWORD"); err != nil {
		return nil, err
	}

	return config, nil
}

// GetServerAddress returns the full server address
func (c *Config) GetServerAddress() string {
	return c.Host + ":" + c.Port
}

// GetNZBGetURL returns the full NZBGet URL
func (c *Config) GetNZBGetURL() string {
	return fmt.Sprintf("http://%s:%d", c.NZBGetHost, c.NZBGetPort)
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.DownloadDir == "" {
		return fmt.Errorf("download directory is required")
	}
	if c.DataDir == "" {
		return fmt.Errorf("data directory is required")
	}
	if c.NewsNabHost == "" || c.NewsNabAPIKey == "" {
		return fmt.Errorf("newsnab configuration is required")
	}
	if c.TraktAPIKey == "" || c.TraktClientSecret == "" {
		return fmt.Errorf("trakt configuration is required")
	}
	if c.NZBGetHost == "" || c.NZBGetPort <= 0 || c.NZBGetUsername == "" || c.NZBGetPassword == "" {
		return fmt.Errorf("nzbget configuration is required")
	}
	return nil
}

// Helper functions
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

func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
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

func getRequiredEnvInt(key string) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return 0, fmt.Errorf("required environment variable %s is not set", key)
	}
	intValue, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("environment variable %s must be a valid integer: %w", key, err)
	}
	return intValue, nil
}