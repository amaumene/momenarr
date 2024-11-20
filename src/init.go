package main

import (
	log "github.com/sirupsen/logrus"
	"golift.io/nzbget"
	"os"
)

func createDir(dir string) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalf("Failed to create directory %s: %v", dir, err)
	}
}

func setConfig() *App {
	appConfig := new(App)
	appConfig.newsNabApiKey = os.Getenv("NEWSNAB_API_KEY")
	if appConfig.newsNabApiKey == "" {
		log.WithFields(log.Fields{
			"NEWSNAB_API_KEY": appConfig.newsNabApiKey,
		}).Fatal("Environment variable missing")
	}

	appConfig.newsNabHost = os.Getenv("NEWSNAB_HOST")
	if appConfig.newsNabHost == "" {
		log.WithFields(log.Fields{
			"NEWSNAB_HOST": appConfig.newsNabHost,
		}).Fatal("Environment variable missing")
	}

	appConfig.downloadDir = os.Getenv("DOWNLOAD_DIR")
	if appConfig.downloadDir == "" {
		log.WithFields(log.Fields{
			"DOWNLOAD_DIR": appConfig.downloadDir,
		}).Fatal("Environment variable missing")
	}
	// Create if it doesn't exist
	createDir(appConfig.downloadDir)

	appConfig.dataDir = os.Getenv("DATA_DIR")
	if appConfig.dataDir == "" {
		log.WithFields(log.Fields{
			"DATA_DIR": appConfig.dataDir,
		}).Warning("DATA_DIR not set, using current directory")
		appConfig.dataDir = "."
	}
	return appConfig
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
