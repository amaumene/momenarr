package main

import (
	"github.com/amaumene/momenarr/nzbget"
	log "github.com/sirupsen/logrus"
	"os"
)

func createDir(dir string) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalf("Failed to create directory %s: %v", dir, err)
	}
}

func setConfig() *Config {
	config := new(Config)
	config.NewsNabApiKey = os.Getenv("NEWSNAB_API_KEY")
	if config.NewsNabApiKey == "" {
		log.WithFields(log.Fields{
			"NEWSNAB_API_KEY": config.NewsNabApiKey,
		}).Fatal("Environment variable missing")
	}

	config.NewsNabHost = os.Getenv("NEWSNAB_HOST")
	if config.NewsNabHost == "" {
		log.WithFields(log.Fields{
			"NEWSNAB_HOST": config.NewsNabHost,
		}).Fatal("Environment variable missing")
	}

	config.DownloadDir = os.Getenv("DOWNLOAD_DIR")
	if config.DownloadDir == "" {
		log.WithFields(log.Fields{
			"DOWNLOAD_DIR": config.DownloadDir,
		}).Fatal("Environment variable missing")
	}
	// Create if it doesn't exist
	createDir(config.DownloadDir)

	config.DataDir = os.Getenv("DATA_DIR")
	if config.DataDir == "" {
		log.WithFields(log.Fields{
			"DATA_DIR": config.DataDir,
		}).Warning("DATA_DIR not set, using current directory")
		config.DataDir = "."
	}
	return config
}

func getEnvTrakt() (string, string) {
	traktApiKey := os.Getenv("TRAKT_API_KEY")
	traktClientSecret := os.Getenv("TRAKT_CLIENT_SECRET")

	if traktApiKey == "" || traktClientSecret == "" {
		log.WithFields(log.Fields{
			"TRAKT_API_KEY":       traktApiKey,
			"TRAKT_CLIENT_SECRET": traktClientSecret,
		}).Fatal("Environment variable missing")
	}
	return traktApiKey, traktClientSecret
}

func setNZBGet() *nzbget.NZBGet {
	nzbgetURL := os.Getenv("NZBGET_URL")
	if nzbgetURL == "" {
		log.WithFields(log.Fields{
			"NZBGET_URL": nzbgetURL,
		}).Fatal("Environment variable missing")
	}
	nzbgetUser := os.Getenv("NZBGET_USER")
	if nzbgetUser == "" {
		log.WithFields(log.Fields{
			"NZBGET_USER": nzbgetUser,
		}).Fatal("Environment variable missing")
	}
	nzbgetPass := os.Getenv("NZBGET_PASS")
	if nzbgetPass == "" {
		log.WithFields(log.Fields{
			"NZBGET_PASS": nzbgetPass,
		}).Fatal("Environment variable missing")
	}
	nzbget := nzbget.New(&nzbget.Config{
		URL:  nzbgetURL,
		User: nzbgetUser,
		Pass: nzbgetPass,
	})
	return nzbget
}
