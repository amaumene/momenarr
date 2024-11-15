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
)

func (appConfig *App) searchNZB(episode Media) newsnab.Feed {
	var feed newsnab.Feed
	if episode.Number > 0 && episode.Season > 0 {
		jsonResponse, err := newsnab.SearchTVShow(episode.TVDB, episode.Season, episode.Number, appConfig.newsNabHost, appConfig.newsNabApiKey)
		if err != nil {
			log.WithFields(log.Fields{
				"IMDB": episode.IMDB,
			}).Error("Searching NZB for episode")
		}
		err = json.Unmarshal([]byte(jsonResponse), &feed)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("Unmarshalling JSON NZB episode")
		}
	} else {
		jsonResponse, err := newsnab.SearchMovie(episode.IMDB, appConfig.newsNabHost, appConfig.newsNabApiKey)
		if err != nil {
			log.WithFields(log.Fields{
				"IMDB": episode.IMDB,
			}).Error("Searching NZB for episode")
		}
		err = json.Unmarshal([]byte(jsonResponse), &feed)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("Unmarshalling JSON NZB episode")
		}
	}
	return feed
}

func (appConfig *App) populateNzb() {
	episodes := []Media{}
	_ = appConfig.store.Find(&episodes, bolthold.Where("OnDisk").Eq(false).SortBy("IMDB"))
	for _, episode := range episodes {
		feed := appConfig.searchNZB(episode)
		if len(feed.Channel.Item) > 0 {
			for _, item := range feed.Channel.Item {
				length, err := strconv.ParseInt(item.Enclosure.Attributes.Length, 10, 64)
				if err != nil {
					log.WithFields(log.Fields{
						"err": err,
					}).Error("Converting NZB episode Length to int64")
				}
				nzb := NZB{
					ID:     episode.IMDB,
					Link:   item.Link,
					Length: length,
					Title:  item.Title,
				}
				err = appConfig.store.Insert(strings.TrimPrefix(item.GUID, "https://nzbs.in/details/"), nzb)
				if err != nil && err.Error() != "This Key already exists in this bolthold for this type" {
					log.WithFields(log.Fields{
						"err": err,
					}).Error("Inserting NZB episode into database")
				}
			}
		}
	}
}

func (appConfig *App) createOrDownloadCachedMedia(IMDB int64, nzb NZB) error {
	torboxDownload, err := appConfig.torBoxClient.CreateUsenetDownload(nzb.Link, nzb.Title)
	if err != nil {
		log.WithFields(log.Fields{
			"title":  nzb.Title,
			"detail": torboxDownload.Detail,
			"err":    err,
		}).Error("Creating TorBox transfer")
	}
	if torboxDownload.Success {
		err = appConfig.store.UpdateMatching(&Media{}, bolthold.Where("IMDB").Eq(IMDB).Index("IMDB"), func(record interface{}) error {
			update, ok := record.(*Media) // record will always be a pointer
			if !ok {
				return fmt.Errorf("Record isn't the correct type!  Wanted Episode, got %T", record)
			}
			update.DownloadID = torboxDownload.Data.UsenetDownloadID
			return nil
		})
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("Request NZB from database")
		}
		log.WithFields(log.Fields{
			"IMDB":  IMDB,
			"Title": nzb.Title,
		}).Info("Download started successfully")
	}
	if torboxDownload.Detail == "Found cached usenet download. Using cached download." {
		err = appConfig.downloadCachedData(torboxDownload)
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
	var episodes []Media
	_ = appConfig.store.Find(&episodes, bolthold.Where("OnDisk").Eq(false))
	for _, episode := range episodes {
		nzb, err := appConfig.getNzbFromDB(episode.IMDB)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("Request NZB from database")
		} else {
			appConfig.createOrDownloadCachedMedia(episode.IMDB, nzb)
		}
	}
}

func (appConfig *App) getNzbFromDB(ID int64) (NZB, error) {
	var nzb []NZB
	err := appConfig.store.Find(&nzb, bolthold.Where("ID").Eq(ID).And("Title").
		RegExp(regexp.MustCompile("(?i)remux")).
		And("Failed").Eq(false).
		SortBy("Length").Reverse().Limit(1).Index("ID"))
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("Request NZB from database")
	}
	if len(nzb) == 0 {
		err = appConfig.store.Find(&nzb, bolthold.Where("ID").Eq(ID).And("Title").
			RegExp(regexp.MustCompile("(?i)web-dl")).
			And("Failed").Eq(false).
			SortBy("Length").Reverse().Limit(1).Index("ID"))
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("Request NZB from database")
		}
	}
	if len(nzb) > 0 {
		return nzb[0], nil
	}
	return NZB{}, fmt.Errorf("No NZB found for %d", ID)
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

	// Create a channel to listen for interrupt signals (e.g., SIGINT)
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt)

	// Run a separate goroutine to handle the shutdown signal
	go func() {
		<-shutdownChan
		log.Info("Received shutdown signal, shutting down gracefully...")

		// Close the database connection
		if err := appConfig.store.Close(); err != nil {
			log.Error("Error closing database: ", err)
		}

		// Any other cleanup tasks go here

		log.Info("Server shut down successfully.")
		os.Exit(0)
	}()
	appConfig.syncMoviesDbFromTrakt()
	appConfig.getNewEpisodes()

	appConfig.populateNzb()

	appConfig.downloadNotOnDisk()

	//go func() {
	//	for {
	//		//cleanWatched(appConfig)
	//		getNewMovies(appConfig)
	//		//getNewEpisodes(appConfig)
	//		time.Sleep(6 * time.Hour)
	//	}
	//}()

	http.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		handlePostData(w, r, appConfig)
	})
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	port := "0.0.0.0:3000"
	fmt.Printf("Server is running on port %s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
