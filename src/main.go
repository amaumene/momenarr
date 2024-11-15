package main

import (
	"encoding/json"
	"fmt"
	"github.com/amaumene/momenarr/newsnab"
	"github.com/amaumene/momenarr/torbox"
	"github.com/jacklaaa89/trakt"
	"github.com/jacklaaa89/trakt/sync"
	log "github.com/sirupsen/logrus"
	"github.com/timshannon/bolthold"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
)

type App struct {
	downloadDir        string
	tempDir            string
	newsNabHost        string
	newsNabApiKey      string
	traktToken         *trakt.Token
	torBoxClient       torbox.TorBox
	torBoxMoviesFolder string
	torBoxShowsFolder  string
	store              *bolthold.Store
}

//func getNextEpisodes(showProgress *trakt.WatchedProgress, item *trakt.WatchListEntry, episodeNum int64, appConfig App) {
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
//
//func getNewEpisodes(appConfig App) {
//	tokenParams := trakt.ListParams{OAuth: appConfig.traktToken.AccessToken}
//
//	watchListParams := &trakt.ListWatchListParams{
//		ListParams: tokenParams,
//		Type:       "show",
//	}
//	iterator := sync.WatchList(watchListParams)
//
//	for iterator.Next() {
//		item, err := iterator.Entry()
//		if err != nil {
//			log.WithFields(log.Fields{
//				"item": item,
//				"err":  err,
//			}).Fatal("Error scanning item")
//		}
//
//		progressParams := &trakt.ProgressParams{
//			Params: trakt.Params{OAuth: appConfig.traktToken.AccessToken},
//		}
//		showProgress, err := show.WatchedProgress(item.Show.Trakt, progressParams)
//		if err != nil {
//			log.WithFields(log.Fields{
//				"show": item.Show.Title,
//				"err":  err,
//			}).Fatal("Error getting show progress")
//		}
//
//		newEpisode := 3
//		for i := 0; i < newEpisode; i++ {
//			episodeNum := showProgress.NextEpisode.Number + int64(i)
//			getNextEpisodes(showProgress, item, episodeNum, appConfig)
//		}
//
//	}
//
//	if err := iterator.Err(); err != nil {
//		log.WithFields(log.Fields{
//			"err": err,
//		}).Fatal("Error iterating history")
//	}
//}

func (appConfig *App) syncMoviesDbFromTrakt() {
	tokenParams := trakt.ListParams{OAuth: appConfig.traktToken.AccessToken}

	watchListParams := &trakt.ListWatchListParams{
		ListParams: tokenParams,
		Type:       "movie",
	}
	iterator := sync.WatchList(watchListParams)

	for iterator.Next() {
		item, err := iterator.Entry()
		if err != nil {
			log.WithFields(log.Fields{
				"item": item,
				"err":  err,
			}).Error("Scanning movie history")
		}
		IMDB, _ := strconv.ParseInt(strings.TrimPrefix(string(item.Movie.IMDB), "tt"), 10, 64)
		movie := Movie{
			IMDB:       IMDB,
			Title:      item.Movie.Title,
			Year:       item.Movie.Year,
			OnDisk:     false,
			File:       "",
			DownloadID: 0,
		}
		err = appConfig.store.Insert(IMDB, movie)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("Inserting movie into database")
		}
	}
	if err := iterator.Err(); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("Iterating movie history")
	}
}

func (appConfig *App) populateNzbForMovies() {
	movies := []Movie{}
	_ = appConfig.store.Find(&movies, bolthold.Where("OnDisk").Eq(false).SortBy("IMDB"))
	for _, movie := range movies {
		jsonResponse, err := newsnab.SearchMovie(movie.IMDB, appConfig.newsNabHost, appConfig.newsNabApiKey)
		if err != nil {
			log.WithFields(log.Fields{
				"movie": movie.Title,
			}).Error("Searching NZB for movie")
		}

		var feed newsnab.Feed
		err = json.Unmarshal([]byte(jsonResponse), &feed)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("Unmarshalling JSON NZB movie")
		}
		if len(feed.Channel.Item) > 0 {
			for _, item := range feed.Channel.Item {
				length, err := strconv.ParseInt(item.Enclosure.Attributes.Length, 10, 64)
				if err != nil {
					log.WithFields(log.Fields{
						"err": err,
					}).Error("Converting NZB movie Length to int64")
				}
				nzb := NZB{
					ID:     movie.IMDB,
					Link:   item.Link,
					Length: length,
					Title:  item.Title,
				}
				err = appConfig.store.Insert(strings.TrimPrefix(item.GUID, "https://nzbs.in/details/"), nzb)
				if err != nil {
					log.WithFields(log.Fields{
						"err": err,
					}).Error("Inserting NZB movie into database")
				}
			}
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

func (appConfig *App) downloadMovieNotOnDisk() {
	var movies []Movie
	_ = appConfig.store.Find(&movies, bolthold.Where("OnDisk").Eq(false))
	for _, movie := range movies {
		nzb, err := appConfig.getNzbFromDB(movie.IMDB)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("Request NZB from database")
		} else {
			appConfig.createOrDownloadCached(movie.IMDB, nzb)
		}
	}
}

func (appConfig *App) createOrDownloadCached(IMDB int64, nzb NZB) error {
	torboxDownload, err := appConfig.torBoxClient.CreateUsenetDownload(nzb.Link, nzb.Title)
	if err != nil {
		log.WithFields(log.Fields{
			"title":  nzb.Title,
			"detail": torboxDownload.Detail,
			"err":    err,
		}).Error("Creating TorBox transfer")
	}
	if torboxDownload.Success {
		err = appConfig.store.UpdateMatching(&Movie{}, bolthold.Where("IMDB").Eq(IMDB).Index("IMDB"), func(record interface{}) error {
			update, ok := record.(*Movie) // record will always be a pointer
			if !ok {
				return fmt.Errorf("Record isn't the correct type!  Wanted Movie, got %T", record)
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

func main() {
	appConfig := setConfig()
	traktApiKey, traktClientSecret := getEnvTrakt()
	appConfig.traktToken = setUpTrakt(appConfig, traktApiKey, traktClientSecret)
	appConfig.torBoxClient = torbox.NewTorBoxClient(getEnvTorBox())
	log.SetOutput(os.Stdout)

	var err error
	os.Remove("data.db")
	appConfig.store, err = bolthold.Open("data.db", 0666, nil)
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
