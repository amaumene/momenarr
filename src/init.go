package main

import (
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
)

func createDir(dir string) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalf("Failed to create directory %s: %v", dir, err)
	}
}

func cleanDir(tempDir string) {
	files, err := os.ReadDir(tempDir)
	if err != nil {
		log.Fatalf("Failed to read temp directory: %v", err)
	}

	for _, file := range files {
		if err := os.RemoveAll(filepath.Join(tempDir, file.Name())); err != nil {
			log.Printf("Failed to remove file %s: %v", file.Name(), err)
		}
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

func getEnvTorBox() string {
	TorBoxApiKey := os.Getenv("TORBOX_API_KEY")

	if TorBoxApiKey == "" {
		log.WithFields(log.Fields{
			"TORBOX_API_KEY": TorBoxApiKey,
		}).Fatal("Environment variable missing")
	}
	return TorBoxApiKey
}
