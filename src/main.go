package main

import (
	"context"
	"fmt"
	"github.com/amaumene/momenarr/bolthold"
	"github.com/amaumene/momenarr/sabnzbd"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func (app App) createDownload(Trakt int64, nzb NZB) error {
	var media Media
	if err := app.Store.Get(Trakt, &media); err != nil {
		return fmt.Errorf("getting media from database: %s", err)
	}
	if media.DownloadID == "" {
		ctx := context.Background()
		response, err := app.SabNZBd.AddFromUrl(ctx, sabnzbd.AddNzbRequest{Url: nzb.Link, Category: "momenarr"})
		if err != nil {
			return fmt.Errorf("creating NZB transfer: %s", err)
		}

		err = updateMediaDownloadID(app.Store, Trakt, response.NzoIDs)
		if err != nil {
			return fmt.Errorf("updating DownloadID in database: %s", err)
		}
		logDownloadStart(Trakt, nzb.Title, response.NzoIDs)
	}
	return nil
}

func updateMediaDownloadID(store *bolthold.Store, Trakt int64, downloadID []string) error {
	var media Media
	if err := store.Get(Trakt, &media); err != nil {
		return fmt.Errorf("getting media from database: %w", err)
	}
	media.DownloadID = downloadID[0]
	return store.Update(Trakt, media)
}

func logDownloadStart(Trakt int64, title string, downloadID []string) {
	log.WithFields(log.Fields{
		"TraktID":    Trakt,
		"Title":      title,
		"DownloadID": downloadID[0],
	}).Info("Download started successfully")
}

func (app App) downloadNotOnDisk() error {
	medias, err := findMediasNotOnDisk(app.Store)
	if err != nil {
		return err
	}
	for _, media := range medias {
		err = app.processMediaDownload(media)
		if err != nil {
			log.WithFields(log.Fields{
				"err":   err,
				"media": media.Trakt,
				"title": media.Title,
			}).Error("No NZB found for media")
		}
	}
	return nil
}

func findMediasNotOnDisk(store *bolthold.Store) ([]Media, error) {
	var medias []Media
	err := store.Find(&medias, bolthold.Where("OnDisk").Eq(false))
	if err != nil {
		return medias, fmt.Errorf("finding media not on disk: %s", err)
	}
	return medias, err
}

func (app App) processMediaDownload(media Media) error {
	nzb, err := app.getNzbFromDB(media.Trakt)
	if err != nil {
		return fmt.Errorf("getting NZB from database: %s", err)
	}
	err = app.createDownload(media.Trakt, nzb)
	if err != nil {
		return fmt.Errorf("creating or downloading cached media: %s", err)
	}
	return nil
}

func (app App) syncFromTrakt() {
	err, movies := app.syncMoviesFromTrakt()
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("Error syncing movies from Trakt")
	}
	err, episodes := app.syncEpisodesFromTrakt()
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("Error syncing episodes from Trakt")
	}
	merged := append(movies, episodes...)
	if len(merged) >= 1 {
		var existingEntries []Media
		err = app.Store.Find(&existingEntries, bolthold.Where("Trakt").Not().ContainsAny(merged...))
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("retrieving existing media entries from database")
		}
		for _, entry := range existingEntries {
			app.removeMedia(entry.Trakt)
		}
	}
}

func (app App) runTasks() {
	app.syncFromTrakt()
	if err := app.populateNZB(); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("populating NZB")
	}
	if err := app.downloadNotOnDisk(); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("downloading on disk")
	}
	err := app.cleanWatched()
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("cleaning watched")
	}
	log.Info("Tasks ran successfully")
}

func startBackgroundTasks(appConfig *App, traktClientSecret string) {
	for {
		appConfig.TraktToken = appConfig.refreshTraktToken(traktClientSecret)
		appConfig.runTasks()
		time.Sleep(6 * time.Hour)
	}
}

func handleShutdown(appConfig *App, shutdownChan chan os.Signal) {
	<-shutdownChan
	log.Info("Received shutdown signal, shutting down gracefully...")
	if err := appConfig.Store.Close(); err != nil {
		log.Error("Error closing database: ", err)
	}
	log.Info("Server shut down successfully.")
	os.Exit(0)
}

func main() {
	log.SetOutput(os.Stdout)
	app := new(App)
	app.Config = setConfig()
	traktApiKey, traktClientSecret := getEnvTrakt()
	app.TraktToken = app.setUpTrakt(traktApiKey, traktClientSecret)
	app.SabNZBd = setSabNZBd()

	var err error
	app.Store, err = bolthold.Open(app.Config.DataDir+"/data.db", 0666, nil)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Fatal("Error opening database")
	}

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt)
	go handleShutdown(app, shutdownChan)

	go startBackgroundTasks(app, traktClientSecret)

	handleAPIRequests(app)
	port := "0.0.0.0:3000"
	log.Fatal(http.ListenAndServe(port, nil))
	log.WithFields(log.Fields{"port": port}).Info("Server is running")
}
