package main

import (
	"log"
	"os"
)

func setConfig() App {
	var appConfig App

	appConfig.newsNabApiKey = os.Getenv("NEWSNAB_API_KEY")
	if appConfig.newsNabApiKey == "" {
		log.Fatalf("NEWSNAB_API_KEY empty. Example: 12345678901234567890123456789012")
	}

	appConfig.newsNabHost = os.Getenv("NEWSNAB_HOST")
	if appConfig.newsNabHost == "" {
		log.Fatalf("NEWSNAB_HOST empty. Example: nzbs.com, no need for https://")
	}

	appConfig.downloadDir = os.Getenv("DOWNLOAD_DIR")
	if appConfig.downloadDir == "" {
		log.Fatal("DOWNLOAD_DIR must be set in environment variables")
	}
	// Create if it doesn't exist
	createDir(appConfig.downloadDir)

	appConfig.tempDir = os.Getenv("TEMP_DIR")
	if appConfig.tempDir == "" {
		log.Fatal("TEMP_DIR environment variable is not set")
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
		log.Fatalf("TRAKT_API_KEY and TRAKT_CLIENT_SECRET must be set in environment variables")
	}
	return traktApiKey, traktClientSecret
}

func getEnvTorBox() string {
	TorBoxApiKey := os.Getenv("TORBOX_API_KEY")

	if TorBoxApiKey == "" {
		log.Fatalf("TORBOX_API_KEY must be set in environment variables")
	}
	return TorBoxApiKey
}
