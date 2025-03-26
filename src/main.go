package main

import (
	"encoding/base64"
	"fmt"
	"github.com/amaumene/momenarr/bolthold"
	"github.com/amaumene/momenarr/nzbget"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"
)

func (app App) createDownload(Trakt int64, nzb NZB) error {
	input, err := createNZBGetInput(nzb, Trakt)
	if err != nil {
		return fmt.Errorf("creating NZBGet input: %w", err)
	}

	queue, err := app.NZBGet.ListGroups()
	if err != nil {
		return fmt.Errorf("getting NZBGet queue: %w", err)
	}
	for _, item := range queue {
		if item.NZBName == nzb.Title {
			log.WithFields(log.Fields{
				"Trakt": Trakt,
				"Title": nzb.Title,
			}).Info("NZB already in queue, skipping")
			return nil
		}
	}
	downloadID, err := app.NZBGet.Append(input)
	if err != nil || downloadID <= 0 {
		return fmt.Errorf("creating NZBGet transfer: %w", err)
	}
	err = updateMediaDownloadID(app.Store, Trakt, downloadID)
	if err != nil {
		return fmt.Errorf("updating DownloadID in database: %w", err)
	}
	logDownloadStart(Trakt, nzb.Title, downloadID)

	return nil
}

func createNZBGetInput(nzb NZB, Trakt int64) (*nzbget.AppendInput, error) {
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := httpClient.Get(nzb.Link)
	if err != nil {
		return nil, fmt.Errorf("downloading NZB file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download NZB file, status: %s", resp.Status)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading NZB file content: %w", err)
	}

	encodedContent := base64.StdEncoding.EncodeToString(content)
	return &nzbget.AppendInput{
		Filename:   nzb.Title + ".nzb",
		Content:    encodedContent,
		Category:   "momenarr",
		DupeMode:   "score",
		Parameters: []*nzbget.Parameter{{Name: "Trakt", Value: strconv.FormatInt(Trakt, 10)}},
	}, nil
}

func updateMediaDownloadID(store *bolthold.Store, Trakt int64, downloadID int64) error {
	var media Media
	if err := store.Get(Trakt, &media); err != nil {
		return fmt.Errorf("getting media from database: %w", err)
	}
	media.DownloadID = downloadID
	return store.Update(Trakt, media)
}

func logDownloadStart(Trakt int64, title string, downloadID int64) {
	log.WithFields(log.Fields{
		"Trakt":      Trakt,
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
			if err := app.removeMedia(entry.Trakt); err != nil {
				log.WithFields(log.Fields{
					"err":   err,
					"Trakt": entry.Trakt,
				}).Error("Error removing media")
			}
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
	app.NZBGet = setNZBGet()

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
