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
	"regexp"
	"strconv"
	"strings"
	"time"
)

func (appConfig *App) searchNZB(media Media) newsnab.Feed {
	var feed newsnab.Feed
	if media.Number > 0 && media.Season > 0 {
		jsonResponse, err := newsnab.SearchTVShow(media.TVDB, media.Season, media.Number, appConfig.newsNabHost, appConfig.newsNabApiKey)
		if err != nil {
			log.WithFields(log.Fields{
				"IMDB": media.IMDB,
			}).Error("Searching NZB for media")
		}
		err = json.Unmarshal([]byte(jsonResponse), &feed)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("Unmarshalling JSON NZB media")
		}
	} else {
		jsonResponse, err := newsnab.SearchMovie(media.IMDB, appConfig.newsNabHost, appConfig.newsNabApiKey)
		if err != nil {
			log.WithFields(log.Fields{
				"IMDB": media.IMDB,
			}).Error("Searching NZB for media")
		}
		err = json.Unmarshal([]byte(jsonResponse), &feed)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("Unmarshalling JSON NZB media")
		}
	}
	return feed
}

func (appConfig *App) populateNzb() {
	medias := []Media{}
	_ = appConfig.store.Find(&medias, bolthold.Where("OnDisk").Eq(false).SortBy("IMDB"))
	for _, media := range medias {
		feed := appConfig.searchNZB(media)
		if len(feed.Channel.Item) > 0 {
			for _, item := range feed.Channel.Item {
				length, err := strconv.ParseInt(item.Enclosure.Attributes.Length, 10, 64)
				if err != nil {
					log.WithFields(log.Fields{
						"err": err,
					}).Error("Converting NZB media Length to int64")
				}
				nzb := NZB{
					IMDB:   media.IMDB,
					Link:   item.Link,
					Length: length,
					Title:  item.Title,
				}
				err = appConfig.store.Insert(strings.TrimPrefix(item.GUID, "https://nzbs.in/details/"), nzb)
				if err != nil && err.Error() != "This Key already exists in this bolthold for this type" {
					log.WithFields(log.Fields{
						"err": err,
					}).Error("Inserting NZB media into database")
				}
			}
		}
	}
}

func (appConfig *App) createOrDownloadCachedMedia(IMDB string, nzb NZB) error {
	torboxDownload, err := appConfig.torBoxClient.CreateUsenetDownload(nzb.Link, nzb.Title)
	if err != nil {
		log.WithFields(log.Fields{
			"title":  nzb.Title,
			"detail": torboxDownload.Detail,
			"err":    err,
		}).Error("Creating TorBox transfer")
	}
	if torboxDownload.Success {
		var media Media
		err = appConfig.store.Get(IMDB, &media)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("Failed to get media from database")
		}
		media.DownloadID = torboxDownload.Data.UsenetDownloadID
		err = appConfig.store.Update(IMDB, &media)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("Update media downloadID on database")
		}
		log.WithFields(log.Fields{
			"IMDB":  IMDB,
			"Title": nzb.Title,
		}).Info("Download started successfully")
	}
	if torboxDownload.Detail == "Found cached usenet download. Using cached download." {
		err = appConfig.downloadCachedData(torboxDownload, IMDB)
		if err != nil {
			log.WithFields(log.Fields{
				"movie": nzb.Title,
				"err":   err,
			}).Fatal("Error downloading cached data")
		}
	}
	return nil
}

func (appConfig *App) downloadNotOnDisk() {
	var medias []Media
	_ = appConfig.store.Find(&medias, bolthold.Where("OnDisk").Eq(false))
	for _, media := range medias {
		nzb, err := appConfig.getNzbFromDB(media.IMDB)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("Request NZB from database")
		} else {
			appConfig.createOrDownloadCachedMedia(media.IMDB, nzb)
		}
	}
}

func (appConfig *App) getNzbFromDB(IMDB string) (NZB, error) {
	var nzb []NZB
	err := appConfig.store.Find(&nzb, bolthold.Where("IMDB").Eq(IMDB).And("Title").
		RegExp(regexp.MustCompile("(?i)remux")).
		And("Failed").Eq(false).
		SortBy("Length").Reverse().Limit(1).Index("IMDB"))
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("Request NZB from database")
	}
	if len(nzb) == 0 {
		err = appConfig.store.Find(&nzb, bolthold.Where("IMDB").Eq(IMDB).And("Title").
			RegExp(regexp.MustCompile("(?i)web-dl")).
			And("Failed").Eq(false).
			SortBy("Length").Reverse().Limit(1).Index("IMDB"))
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("Request NZB from database")
		}
	}
	if len(nzb) > 0 {
		return nzb[0], nil
	}
	return NZB{}, fmt.Errorf("No NZB found for %d", IMDB)
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
	go func() {
		<-shutdownChan
		log.Info("Received shutdown signal, shutting down gracefully...")
		if err := appConfig.store.Close(); err != nil {
			log.Error("Error closing database: ", err)
		}
		log.Info("Server shut down successfully.")
		os.Exit(0)
	}()

	go func() {
		for {
			appConfig.syncMoviesDbFromTrakt()
			appConfig.getNewEpisodes()
			appConfig.populateNzb()
			appConfig.downloadNotOnDisk()
			appConfig.cleanWatched()
			time.Sleep(6 * time.Hour)
		}
	}()

	http.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		handlePostData(w, r, appConfig)
	})
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/refresh", func(w http.ResponseWriter, r *http.Request) {
		go func() {
			appConfig.syncMoviesDbFromTrakt()
			appConfig.getNewEpisodes()
			appConfig.populateNzb()
			appConfig.downloadNotOnDisk()
			appConfig.cleanWatched()
		}()

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Refresh initiated"))
	})

	port := "0.0.0.0:3000"
	fmt.Printf("Server is running on port %s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
