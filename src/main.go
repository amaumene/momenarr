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
	TorBoxClient       torbox.TorBox
	TorBoxMoviesFolder string
	TorBoxShowsFolder  string
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
//		UsenetCreateDownloadResponse, err := appConfig.TorBoxClient.CreateUsenetDownload(show.Item[0].Enclosure.Attributes.URL, show.Item[0].Title)
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
			}).Fatal("Error scanning item")
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
			}).Fatal("Error inserting movie into database")
		}
	}
	if err := iterator.Err(); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Fatal("Error iterating history")
	}
}

func (appConfig *App) populateNzbForMovies() {
	var movies []Movie
	_ = appConfig.store.Find(&movies, bolthold.Where("OnDisk").Eq(false).SortBy("IMDB"))
	for _, movie := range movies {
		jsonResponse, err := newsnab.searchMovie(movie.IMDB, appConfig.newsNabHost, appConfig.newsNabApiKey)
		if err != nil {
			log.WithFields(log.Fields{"movie": movie.Title}).Fatal("Error searching movie")
			return
		}

		var feed newsnab.Feed
		err = json.Unmarshal([]byte(jsonResponse), &feed)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Fatal("Error unmarshalling JSON")
			return
		}

		if len(feed.Channel.Item) > 0 {
			for _, item := range feed.Channel.Item {
				length, err := strconv.ParseInt(item.Enclosure.Attributes.Length, 10, 64)
				if err != nil {
					log.WithFields(log.Fields{
						"err": err,
					}).Fatal("Error converting Length to int64")
					return
				}
				nzb := NZB{
					ID:     movie.IMDB,
					Link:   feed.Channel.Link,
					Length: length,
					Title:  item.Title,
				}
				err = appConfig.store.Insert(item.GUID, nzb)
				if err != nil {
					log.WithFields(log.Fields{
						"err": err,
					}).Fatal("Error inserting movie into database")
				}
			}
		}
	}
}

func (appConfig *App) getNzbForMovie() {
	var movies []Movie
	_ = appConfig.store.Find(&movies, bolthold.Where("OnDisk").Eq(false))
	for _, movie := range movies {
		var result_nzbs []NZB
		err := appConfig.store.Find(&result_nzbs, bolthold.Where("ID").
			Eq(movie.IMDB).And("Title").RegExp(regexp.MustCompile("(?i)remux|1080p|2160p")).
			SortBy("Length").Reverse().Limit(1).Index("ID"))
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("Request bolthold")
		}
		for _, i := range result_nzbs {
			log.WithFields(log.Fields{
				"title":  i.Title,
				"length": i.Length,
			}).Info("Found NZB")
		}
	}
}

type Movie struct {
	IMDB       int64 `boltholdIndex:"IMDB"`
	Title      string
	Year       int64
	OnDisk     bool
	File       string
	DownloadID int64
}

type NZB struct {
	ID     int64 `boltholdIndex:"ID"`
	Link   string
	Length int64
	Title  string
}

//	func createOrDownloadCached(appConfig App, imdbid int, link string, title string) error {
//		torboxDownload, err := appConfig.TorBoxClient.CreateUsenetDownload(link, title)
//		if err != nil {
//			log.WithFields(log.Fields{
//				"movie": title,
//				"err":   err,
//			}).Fatal("Error creating transfer")
//		}
//		fmt.Printf("%+v", torboxDownload)
//		if torboxDownload.Success {
//			_, err = appConfig.store.Exec(`UPDATE movies SET download_id = ? WHERE imdb_id = ?`, torboxDownload.Data.UsenetDownloadID, imdbid)
//			if err != nil {
//				return err
//			}
//			log.WithFields(log.Fields{
//				"imdb_id": imdbid,
//				"title":   title,
//			}).Info("Download started successfully")
//		}
//		if torboxDownload.Detail == "Found cached usenet download. Using cached download." {
//			err = downloadCachedData(torboxDownload, appConfig)
//			if err != nil {
//				log.WithFields(log.Fields{
//					"movie": title,
//					"err":   err,
//				}).Fatal("Error downloading cached data")
//			}
//		}
//		return nil
//	}

//func getNewMovies(appConfig App) error {
//	docs, _ := appConfig.db.FindAll(query.NewQuery("movies").Where(query.Field("OnDisk").IsFalse()))
//	for _, doc := range docs {
//		movie := Movie{}
//		doc.Unmarshal(&movie)
//		start := time.Now()
//		nzbs, _ := appConfig.db.FindAll(query.NewQuery("nzbs").Where(query.Field("ID").Eq(movie.IMDB).And(query.Field("Title").Like("\"(?i)remux|1080p|2160p\""))).Sort(query.SortOption{"Length", -1}).Limit(1))
//		fmt.Printf("clover request took %s\n", time.Since(start))
//		var result_nzbs []NZB
//		for _, i := range nzbs {
//			nzb := NZB{}
//			i.Unmarshal(&nzb)
//			log.WithFields(log.Fields{
//				"title":  nzb.Title,
//				"length": nzb.Length,
//			}).Info("Found nzb")
//		}
//
//	}
//	return nil
//}

func main() {
	appConfig := setConfig()
	traktApiKey, traktClientSecret := getEnvTrakt()
	appConfig.traktToken = setUpTrakt(appConfig, traktApiKey, traktClientSecret)
	appConfig.TorBoxClient = torbox.NewTorBoxClient(getEnvTorBox())

	var err error
	os.Remove("data-store")
	appConfig.store, err = bolthold.Open("data-store", 0666, nil)
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
