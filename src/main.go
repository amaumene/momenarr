package main

import (
	"fmt"
	"github.com/amaumene/momenarr/torbox"
	log "github.com/sirupsen/logrus"
	"github.com/timshannon/bolthold"
	"net/http"
	"os"
	"os/signal"
	"regexp"
)

//func (appConfig *App) getNextEpisodes(showProgress *trakt.WatchedProgress, item *trakt.WatchListEntry, episodeNum int64) {
//	fileName := fmt.Sprintf("%s S%02dE%02d", item.Show.Title, showProgress.NextEpisode.Season, episodeNum)
//	if fileExists(fileName, appConfig.downloadDir) == "" {
//		xmlResponse, err := newsnab.searchTVShow(item.Show.TVDB, int(showProgress.NextEpisode.Season), int(episodeNum), appConfig)
//		if err != nil {
//			fmt.Printf("Error: %v\n", err)
//			return
//		}
//
//		var Feed newsnab.Feed
//		err = xml.Unmarshal([]byte(xmlResponse), &Feed)
//		if err != nil {
//			log.WithFields(log.Fields{
//				"err": err,
//			}).Fatal("Error unmarshalling XML")
//		}
//		filteredFeed := sortNZBsShows(Feed, item.Show)
//
//		show := findOrCreateData(item.Show.Title + " S" + strconv.Itoa(int(showProgress.NextEpisode.Season)) + "E" + strconv.Itoa(int(episodeNum)))
//		show.Item = append(show.Item, filteredFeed.Channel.Item...)
//		log.WithFields(log.Fields{
//			"name":    show.Item[0].Title,
//			"season":  showProgress.NextEpisode.Season,
//			"episode": episodeNum,
//		}).Info("Going to download")
//		UsenetCreateDownloadResponse, err := appConfig.torBoxClient.CreateUsenetDownload(show.Item[0].Enclosure.Attributes.URL, show.Item[0].Title)
//		if err != nil {
//			log.WithFields(log.Fields{
//				"show": show.Item[0].Title,
//				"err":  err,
//			}).Fatal("Error creating transfer")
//		}
//		if UsenetCreateDownloadResponse.Detail == "Found cached usenet download. Using cached download." {
//			err = downloadCachedData(UsenetCreateDownloadResponse, appConfig)
//			if err != nil {
//				log.WithFields(log.Fields{
//					"show": show.Item[0].Title,
//					"err":  err,
//				}).Fatal("Error downloading cached data")
//			}
//		}
//	}
//}

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
	appConfig.populateNzbForMovies()
	appConfig.downloadMovieNotOnDisk()
	appConfig.getNewEpisodes()
	appConfig.populateNzbForEpisodes()
	appConfig.downloadEpisodeNotOnDisk()

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
