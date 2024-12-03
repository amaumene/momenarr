package main

import (
	"fmt"
	"github.com/amaumene/momenarr/bolthold"
	"github.com/amaumene/momenarr/nzbget"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func (app App) createDownload(IMDB string, nzb NZB) error {
	input, err := createNZBGetInput(nzb, IMDB)
	if err != nil {
		return fmt.Errorf("creating NZBGet input: %w", err)
	}

	downloadID, err := app.NZBGet.Append(input)
	if err != nil || downloadID <= 0 {
		return fmt.Errorf("creating NZBGet transfer: %w", err)
	}

	err = updateMediaDownloadID(app.Store, IMDB, downloadID)
	if err != nil {
		return fmt.Errorf("updating DownloadID in database: %w", err)
	}

	logDownloadStart(IMDB, nzb.Title, downloadID)

	return nil
}

func createNZBGetInput(nzb NZB, IMDB string) (*nzbget.AppendInput, error) {
	return &nzbget.AppendInput{
		Filename:   nzb.Title + ".nzb",
		Content:    nzb.Link,
		Category:   "momenarr",
		DupeMode:   "score",
		Parameters: []*nzbget.Parameter{{Name: "IMDB", Value: IMDB}},
	}, nil
}

func updateMediaDownloadID(store *bolthold.Store, IMDB string, downloadID int64) error {
	var media Media
	if err := store.Get(IMDB, &media); err != nil {
		return fmt.Errorf("getting media from database: %w", err)
	}
	media.DownloadID = downloadID
	return store.Update(IMDB, media)
}

func logDownloadStart(IMDB, title string, downloadID int64) {
	log.WithFields(log.Fields{
		"IMDB":       IMDB,
		"Title":      title,
		"DownloadID": downloadID,
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
			return err
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
	nzb, err := app.getNzbFromDB(media.IMDB)
	if err != nil {
		return fmt.Errorf("getting NZB from database: %s", err)
	}
	err = app.createDownload(media.IMDB, nzb)
	if err != nil {
		return fmt.Errorf("creating or downloading cached media: %s", err)
	}
	return nil
}

func (app App) syncFromTrakt() {
	if err := app.syncMoviesFromTrakt(); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("Error syncing movies from Trakt")
	}
	if err := app.syncEpisodesFromTrakt(); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("Error syncing episodes from Trakt")
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

func startBackgroundTasks(appConfig *App) {
	for {
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
	app.NZBGet = setNZBGet()

	var err error
	app.Store, err = bolthold.Open(app.Config.DataDir+"/data.db", 0666, nil)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Fatal("Error opening database")
	}

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt)
	go handleShutdown(app, shutdownChan)

	go startBackgroundTasks(app)

	handleAPIRequests(app)
	port := "0.0.0.0:3000"
	log.Fatal(http.ListenAndServe(port, nil))
	log.WithFields(log.Fields{"port": port}).Info("Server is running")
}
