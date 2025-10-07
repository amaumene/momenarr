package main

import (
	"github.com/amaumene/momenarr/nzbget"
	log "github.com/sirupsen/logrus"
	"os"
)

const (
	dirPermissions          = 0755
	envNewsnabAPIKey        = "NEWSNAB_API_KEY"
	envNewsnabHost          = "NEWSNAB_HOST"
	envDownloadDir          = "DOWNLOAD_DIR"
	envDataDir              = "DATA_DIR"
	envTraktAPIKey          = "TRAKT_API_KEY"
	envTraktClientSecret    = "TRAKT_CLIENT_SECRET"
	envNZBGetURL            = "NZBGET_URL"
	envNZBGetUser           = "NZBGET_USER"
	envNZBGetPass           = "NZBGET_PASS"
	msgEnvVarMissing        = "Environment variable missing"
	defaultDataDir          = "."
)

func createDir(dir string) {
	if err := os.MkdirAll(dir, dirPermissions); err != nil {
		log.Fatalf("Failed to create directory %s: %v", dir, err)
	}
}

func setConfig() *Config {
	config := new(Config)
	config.NewsNabApiKey = getRequiredEnv(envNewsnabAPIKey)
	config.NewsNabHost = getRequiredEnv(envNewsnabHost)
	config.DownloadDir = getRequiredEnv(envDownloadDir)
	createDir(config.DownloadDir)
	config.DataDir = getDataDir()
	return config
}

func getRequiredEnv(key string) string {
	value := os.Getenv(key)
	if value == emptyString {
		log.WithFields(log.Fields{key: value}).Fatal(msgEnvVarMissing)
	}
	return value
}

func getDataDir() string {
	dataDir := os.Getenv(envDataDir)
	if dataDir == emptyString {
		log.WithFields(log.Fields{envDataDir: dataDir}).Warning("DATA_DIR not set, using current directory")
		return defaultDataDir
	}
	return dataDir
}

func getEnvTrakt() (string, string) {
	traktApiKey := getRequiredEnv(envTraktAPIKey)
	traktClientSecret := getRequiredEnv(envTraktClientSecret)
	return traktApiKey, traktClientSecret
}

func setNZBGet() *nzbget.NZBGet {
	config := buildNZBGetConfig()
	return nzbget.New(config)
}

func buildNZBGetConfig() *nzbget.Config {
	return &nzbget.Config{
		URL:  getRequiredEnv(envNZBGetURL),
		User: getRequiredEnv(envNZBGetUser),
		Pass: getRequiredEnv(envNZBGetPass),
	}
}
