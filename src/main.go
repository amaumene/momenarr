package main

import (
	"fmt"
	"github.com/amaumene/momenarr/torbox"
	log "github.com/sirupsen/logrus"
	"github.com/timshannon/bolthold"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func (appConfig *App) downloadCachedData(UsenetCreateDownloadResponse torbox.UsenetCreateDownloadResponse, IMDB string) error {
	log.WithFields(log.Fields{
		"id": UsenetCreateDownloadResponse.Data.UsenetDownloadID,
	}).Info("Found cached usenet download")

	UsenetDownload, err := appConfig.torBoxClient.FindDownloadByID(UsenetCreateDownloadResponse.Data.UsenetDownloadID)
	if err != nil {
		log.WithFields(log.Fields{
			"id": UsenetCreateDownloadResponse.Data.UsenetDownloadID,
		}).Error("Link not found")
		return appConfig.downloadCachedData(UsenetCreateDownloadResponse, IMDB)
	}

	if UsenetDownload[0].Cached {
		log.WithFields(log.Fields{
			"name": UsenetDownload[0].Name,
		}).Info("Starting download from cached data")

		go func() {
			if err := appConfig.downloadFromTorBox(UsenetDownload, IMDB); err != nil {
				log.WithFields(log.Fields{
					"name":  UsenetDownload[0].Name,
					"error": err,
				}).Error("Failed to download from TorBox")
			} else {
				log.WithFields(log.Fields{
					"name": UsenetDownload[0].Name,
				}).Info("Download from TorBox complete")
			}
		}()
		return nil
	}
	log.WithFields(log.Fields{
		"name": UsenetDownload[0].Name,
	}).Info("Not really in cache, skipping and hoping to get a notification")
	return nil
}

func (appConfig *App) createOrDownloadCachedMedia(IMDB string, nzb NZB) error {
	torboxDownload, err := appConfig.torBoxClient.CreateUsenetDownload(nzb.Link, nzb.Title)
	if err != nil {
		return fmt.Errorf("creating TorBox transfer: %s", err)
	}
	if torboxDownload.Success {
		var media Media
		if err = appConfig.store.Get(IMDB, &media); err != nil {
			return fmt.Errorf("get media from database: %s", err)
		}
		media.DownloadID = torboxDownload.Data.UsenetDownloadID
		if err = appConfig.store.Update(IMDB, &media); err != nil {
			return fmt.Errorf("update DownloadID on database: %s", err)
		}
		log.WithFields(log.Fields{
			"IMDB":  IMDB,
			"Title": nzb.Title,
		}).Info("Download started successfully")
	}
	if torboxDownload.Detail == "Found cached usenet download. Using cached download." {
		if err = appConfig.downloadCachedData(torboxDownload, IMDB); err != nil {
			return fmt.Errorf("downloading cached data: %s", err)
		}
	}
	return nil
}

func (appConfig *App) downloadNotOnDisk() error {
	var medias []Media
	err := appConfig.store.Find(&medias, bolthold.Where("OnDisk").Eq(false))
	if err != nil {
		return fmt.Errorf("finding media not on disk: %s", err)
	}

	for _, media := range medias {
		nzb, err := appConfig.getNzbFromDB(media.IMDB)
		if err != nil {
			return fmt.Errorf("getting NZB from database: %s", err)
		}
		err = appConfig.createOrDownloadCachedMedia(media.IMDB, nzb)
		if err != nil {
			return fmt.Errorf("creating or downloading cached media: %s", err)
		}
	}
	return nil
}

func (appConfig *App) syncFromTrakt() {
	if err := appConfig.syncMoviesFromTrakt(); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("Error syncing movies from Trakt")
	}
	if err := appConfig.syncEpisodesFromTrakt(); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("Error syncing episodes from Trakt")
	}
}

func (appConfig *App) runTasks() {
	appConfig.syncFromTrakt()
	if err := appConfig.populateNZB(); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("populating NZB")
	}
	if err := appConfig.downloadNotOnDisk(); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("downloading on disk")
	}
	err := appConfig.cleanWatched()
	if err != nil {
		log.Error("Error cleaning watched: %v", err)
	}
}

func startBackgroundTasks(appConfig *App) {
	for {
		appConfig.runTasks()
		time.Sleep(6 * time.Hour)
	}
}

func handleShutdown(appConfig *App, shutdownChan chan os.Signal) {
	<-shutdownChan
	log.Info("Received shutdown signal, shutting down gracefully...")
	if err := appConfig.store.Close(); err != nil {
		log.Error("Error closing database: ", err)
	}
	log.Info("Server shut down successfully.")
	os.Exit(0)
}

func main() {
	appConfig := setConfig()
	traktApiKey, traktClientSecret := getEnvTrakt()
	appConfig.traktToken = appConfig.setUpTrakt(traktApiKey, traktClientSecret)
	appConfig.torBoxClient = torbox.NewTorBoxClient(getEnvTorBox())
	log.SetOutput(os.Stdout)

	var err error
	appConfig.store, err = bolthold.Open(appConfig.dataDir+"/data.db", 0666, nil)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Fatal("Error opening database")
	}

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt)
	go handleShutdown(appConfig, shutdownChan)

	go startBackgroundTasks(appConfig)

	handleAPIRequests(appConfig)

	port := "0.0.0.0:3000"
	fmt.Printf("Server is running on port %s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
