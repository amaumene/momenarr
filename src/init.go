package momenarr

import (
	log "github.com/sirupsen/logrus"
	"os"
)

func setConfig() App {
	var appConfig App

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

	appConfig.tempDir = os.Getenv("TEMP_DIR")
	if appConfig.tempDir == "" {
		log.WithFields(log.Fields{
			"TEMP_DIR": appConfig.tempDir,
		}).Fatal("Environment variable missing")
	}
	// Create if it doesn't exist
	createDir(appConfig.tempDir)

	// Clean
	cleanDir(appConfig.tempDir)

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

func getEnvTorBox() string {
	TorBoxApiKey := os.Getenv("TORBOX_API_KEY")

	if TorBoxApiKey == "" {
		log.WithFields(log.Fields{
			"TORBOX_API_KEY": TorBoxApiKey,
		}).Fatal("Environment variable missing")
	}
	return TorBoxApiKey
}
