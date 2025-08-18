package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port string `json:"port" validate:"required"`
	Host string `json:"host"`

	DownloadDir string `json:"download_dir" validate:"required"`
	DataDir     string `json:"data_dir" validate:"required"`

	NewsNabHost   string `json:"newsnab_host" validate:"required,url"`
	NewsNabAPIKey string `json:"newsnab_api_key" validate:"required"`

	TraktAPIKey       string `json:"trakt_api_key" validate:"required"`
	TraktClientSecret string `json:"trakt_client_secret" validate:"required"`

	PremiumizeAPIKey string `json:"premiumize_api_key" validate:"required"`

	SyncInterval   string `json:"sync_interval"`
	BlacklistFile  string `json:"blacklist_file"`
	MaxRetries     int    `json:"max_retries"`
	RequestTimeout int    `json:"request_timeout"`
}

func LoadConfig() (*Config, error) {
	config := &Config{
		Port:           getEnvOrDefault("PORT", "3000"),
		Host:           getEnvOrDefault("HOST", "0.0.0.0"),
		SyncInterval:   getEnvOrDefault("SYNC_INTERVAL", "6h"),
		BlacklistFile:  getEnvOrDefault("BLACKLIST_FILE", "blacklist.txt"),
		MaxRetries:     getEnvIntOrDefault("MAX_RETRIES", 3),
		RequestTimeout: getEnvIntOrDefault("REQUEST_TIMEOUT", 30),
	}

	return loadRequiredConfig(config)
}

func loadRequiredConfig(config *Config) (*Config, error) {
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
	if config.PremiumizeAPIKey, err = getRequiredEnv("PREMIUMIZE_API_KEY"); err != nil {
		return nil, err
	}

	return config, nil
}

func (c *Config) GetServerAddress() string {
	return c.Host + ":" + c.Port
}

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
	if c.PremiumizeAPIKey == "" {
		return fmt.Errorf("premiumize API key is required")
	}
	return nil
}

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

