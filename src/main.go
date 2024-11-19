package main

import (
	"encoding/json"
	"fmt"
	"github.com/amaumene/momenarr/newsnab"
	"github.com/amaumene/momenarr/torbox"
	log "github.com/sirupsen/logrus"
	"github.com/timshannon/bolthold"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"
)

func (appConfig *App) searchNZB(media Media) newsnab.Feed {
	var feed newsnab.Feed
	if media.Number > 0 && media.Season > 0 {
		jsonResponse, err := newsnab.SearchTVShow(media.TVDB, media.Season, media.Number, appConfig.newsNabHost, appConfig.newsNabApiKey)
		if err != nil {
			log.WithFields(log.Fields{"IMDB": media.IMDB}).Error("Searching NZB for media")
		}
		err = json.Unmarshal([]byte(jsonResponse), &feed)
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("Unmarshalling JSON NZB media")
		}
	} else {
		jsonResponse, err := newsnab.SearchMovie(media.IMDB, appConfig.newsNabHost, appConfig.newsNabApiKey)
		if err != nil {
			log.WithFields(log.Fields{"IMDB": media.IMDB}).Error("Searching NZB for media")
		}
		err = json.Unmarshal([]byte(jsonResponse), &feed)
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("Unmarshalling JSON NZB media")
		}
	}
	return feed
}

func (appConfig *App) insertNZBItems(media Media, items []newsnab.Item) {
	for _, item := range items {
		length, err := strconv.ParseInt(item.Enclosure.Attributes.Length, 10, 64)
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("Converting NZB media Length to int64")
			continue
		}

		nzb := NZB{
			IMDB:   media.IMDB,
			Link:   item.Link,
			Length: length,
			Title:  item.Title,
		}

		err = appConfig.store.Insert(strings.TrimPrefix(item.GUID, "https://nzbs.in/details/"), nzb)
		if err != nil && err.Error() != "This Key already exists in this bolthold for this type" {
			log.WithFields(log.Fields{"err": err}).Error("Inserting NZB media into database")
		}
	}
}

func (appConfig *App) populateNZB() error {
	var medias []Media
	err := appConfig.store.Find(&medias, bolthold.Where("OnDisk").Eq(false).SortBy("IMDB"))
	if err != nil {
		return err
	}

	for _, media := range medias {
		feed := appConfig.searchNZB(media)
		if len(feed.Channel.Item) > 0 {
			appConfig.insertNZBItems(media, feed.Channel.Item)
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
			log.WithFields(log.Fields{"err": err}).Error("request NZB from database")
			continue
		}
		err = appConfig.createOrDownloadCachedMedia(media.IMDB, nzb)
		if err != nil {
			return fmt.Errorf("creating or downloading cached media: %s", err)
		}
	}
	return nil
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

func startBackgroundTasks(appConfig *App) {
	for {
		if err := appConfig.syncMoviesDbFromTrakt(); err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("Error syncing movies from Trakt")
		}
		if err := appConfig.getNewEpisodes(); err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("Error syncing episodes from Trakt")
		}
		appConfig.populateNZB()
		appConfig.downloadNotOnDisk()
		appConfig.cleanWatched()
		time.Sleep(6 * time.Hour)
	}
}

func handleAPIRequests(appConfig *App) {
	http.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		handlePostData(w, r, *appConfig)
	})
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	http.HandleFunc("/refresh", func(w http.ResponseWriter, r *http.Request) {
		go func() {
			appConfig.syncMoviesDbFromTrakt()
			appConfig.getNewEpisodes()
			appConfig.populateNZB()
			appConfig.downloadNotOnDisk()
			appConfig.cleanWatched()
		}()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Refresh initiated"))
	})
}

func main() {
	appConfig := setConfig()
	traktApiKey, traktClientSecret := getEnvTrakt()
	appConfig.traktToken = setUpTrakt(appConfig, traktApiKey, traktClientSecret)
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
