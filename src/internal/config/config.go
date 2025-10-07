package config

import (
	"fmt"
	"os"
	"time"
)

const (
	defaultDataDir              = "."
	defaultTaskInterval         = 6 * time.Hour
	defaultHTTPTimeout          = 30 * time.Second
	defaultRetryCount           = 3
	defaultRetryDelay           = 10 * time.Second
	defaultHistoryLookbackDays  = 5
	defaultNextEpisodesCount    = 3
	defaultServerPort           = "0.0.0.0:3000"
	defaultNZBCategory          = "momenarr"
	defaultNZBDupeMode          = "score"
	defaultDirPermissions       = 0755
	defaultDBFilePermissions    = 0666
	defaultBlacklistPermissions = 0644
)

type Config struct {
	DownloadDir          string
	DataDir              string
	NewsNabHost          string
	NewsNabAPIKey        string
	TraktAPIKey          string
	TraktClientSecret    string
	NZBGetURL            string
	NZBGetUser           string
	NZBGetPass           string
	ServerPort           string
	TaskInterval         time.Duration
	HTTPTimeout          time.Duration
	RetryCount           int
	RetryDelay           time.Duration
	HistoryLookbackDays  int
	NextEpisodesCount    int
	NZBCategory          string
	NZBDupeMode          string
	DirPermissions       os.FileMode
	DBFilePermissions    os.FileMode
	BlacklistPermissions os.FileMode
}

func Load() (*Config, error) {
	cfg := &Config{
		DataDir:              getEnvOrDefault("DATA_DIR", defaultDataDir),
		ServerPort:           getEnvOrDefault("SERVER_PORT", defaultServerPort),
		TaskInterval:         defaultTaskInterval,
		HTTPTimeout:          defaultHTTPTimeout,
		RetryCount:           defaultRetryCount,
		RetryDelay:           defaultRetryDelay,
		HistoryLookbackDays:  defaultHistoryLookbackDays,
		NextEpisodesCount:    defaultNextEpisodesCount,
		NZBCategory:          defaultNZBCategory,
		NZBDupeMode:          defaultNZBDupeMode,
		DirPermissions:       defaultDirPermissions,
		DBFilePermissions:    defaultDBFilePermissions,
		BlacklistPermissions: defaultBlacklistPermissions,
	}

	if err := cfg.loadRequired(); err != nil {
		return nil, err
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) loadRequired() error {
	required := map[string]*string{
		"NEWSNAB_API_KEY":     &c.NewsNabAPIKey,
		"NEWSNAB_HOST":        &c.NewsNabHost,
		"DOWNLOAD_DIR":        &c.DownloadDir,
		"TRAKT_API_KEY":       &c.TraktAPIKey,
		"TRAKT_CLIENT_SECRET": &c.TraktClientSecret,
		"NZBGET_URL":          &c.NZBGetURL,
		"NZBGET_USER":         &c.NZBGetUser,
		"NZBGET_PASS":         &c.NZBGetPass,
	}

	for key, ptr := range required {
		value := os.Getenv(key)
		if value == "" {
			return fmt.Errorf("required environment variable missing: %s", key)
		}
		*ptr = value
	}
	return nil
}

func (c *Config) validate() error {
	if err := createDirIfNotExists(c.DownloadDir, c.DirPermissions); err != nil {
		return fmt.Errorf("creating download directory: %w", err)
	}
	return nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func createDirIfNotExists(dir string, perm os.FileMode) error {
	if err := os.MkdirAll(dir, perm); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}
	return nil
}

func (c *Config) DBPath() string {
	return c.DataDir + "/data.db"
}

func (c *Config) TokenPath() string {
	return c.DataDir + "/token.json"
}

func (c *Config) BlacklistPath() string {
	return c.DataDir + "/blacklist.txt"
}
