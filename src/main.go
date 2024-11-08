package main

import (
	"encoding/xml"
	"fmt"
	"github.com/amaumene/momenarr/torbox"
	"github.com/jacklaaa89/trakt"
	"github.com/jacklaaa89/trakt/sync"
	"github.com/razsteinmetz/go-ptn"
	log "github.com/sirupsen/logrus"
	"net/http"
	"sort"
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
}

type Movies struct {
	IMDB  string
	Items []Item `xml:"item"`
}

var (
	currentMovies []Movies
)

func sortNZBsMovies(rss Rss, movie *trakt.Movie) Rss {
	sort.Slice(rss.Channel.Items, func(i, j int) bool {
		return rss.Channel.Items[i].Enclosure.Length > rss.Channel.Items[j].Enclosure.Length
	})
	returnedRss := Rss{}
	for _, item := range rss.Channel.Items {
		info, _ := ptn.Parse(item.Title)
		if int64(info.Year) == movie.Year && info.Title == movie.Title {
			if info.Quality == "BluRay" || info.Quality == "WEB-DL" {
				if info.Resolution == "1080p" || info.Resolution == "2160p" {
					returnedRss.Channel.Items = append(returnedRss.Channel.Items, item)
				}
			}
		}
	}
	return returnedRss
}

//func getNextEpisodes(showProgress *trakt.WatchedProgress, item *trakt.WatchListEntry, episodeNum int64, appConfig App) {
//	xmlResponse, err := searchTVShow(item.Show.TVDB, int(showProgress.NextEpisode.Season), int(episodeNum), appConfig)
//	if err != nil {
//		fmt.Printf("Error: %v\n", err)
//		return
//	}
//
//	var rss Rss
//	// Unmarshal the XML data into the struct
//	err = xml.Unmarshal([]byte(xmlResponse), &rss)
//	if err != nil {
//		log.WithFields(log.Fields{
//			"err": err,
//		}).Fatal("Error unmarshalling XML")
//	}
//
//	maxItem := findBiggest(rss)
//
//	exists, err := fileExists(maxItem.Title, appConfig.downloadDir)
//	if err != nil {
//		log.WithFields(log.Fields{
//			"err": err,
//		}).Fatal("Error checking file existence")
//	}
//
//	if exists {
//		log.WithFields(log.Fields{
//			"file": maxItem.Title,
//		}).Info("File already exists on disk, skipping download")
//		return
//	}
//	//uploadFileWithRetries(maxItem.Enclosure.URL, maxItem.Title)
//}

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

func findOrCreateMovie(imdbID string) *Movies {
	for i := range currentMovies {
		if currentMovies[i].IMDB == imdbID {
			return &currentMovies[i]
		}
	}
	// If not found, create a new one and append to currentMovies
	newMovie := Movies{IMDB: imdbID, Items: []Item{}}
	currentMovies = append(currentMovies, newMovie)
	return &currentMovies[len(currentMovies)-1]
}

func getNewMovies(appConfig App) {
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
		xmlResponse, err := searchMovie(item.Movie.IMDB, appConfig)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		var rss Rss
		err = xml.Unmarshal([]byte(xmlResponse), &rss)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Fatal("Error unmarshalling XML")
		}
		filteredRss := sortNZBsMovies(rss, item.Movie)
		movie := findOrCreateMovie(string(item.Movie.IMDB))
		movie.Items = append(movie.Items, filteredRss.Channel.Items...)

		fmt.Printf("Choosen file: %s", movie.Items[0].Title)

		UsenetCreateDownloadResponse, err := appConfig.TorBoxClient.CreateUsenetDownload(movie.Items[0].Enclosure.URL, movie.Items[0].Title)
		if err != nil {
			log.WithFields(log.Fields{
				"item": movie.Items[0].Title,
				"err":  err,
			}).Fatal("Error creating transfer")
		}
		fmt.Println(UsenetCreateDownloadResponse)
		//	if UsenetCreateDownloadResponse.Detail == "Found cached usenet download. Using cached download." {
		//		fmt.Printf("Usenet download ID: %d", UsenetCreateDownloadResponse.Data.UsenetDownloadID)
		//		UsenetDownload, err := appConfig.TorBoxClient.FindDownloadByID(UsenetCreateDownloadResponse.Data.UsenetDownloadID)
		//		if err != nil {
		//			log.WithFields(log.Fields{
		//				"item": UsenetDownload[0].Files[0].ShortName,
		//				"err":  err,
		//			}).Fatal("Error finding download")
		//		}
		//		err = downloadFromTorBox(UsenetDownload, appConfig)
		//		if err != nil {
		//			log.WithFields(log.Fields{
		//				"item": UsenetDownload[0].Files[0].ShortName,
		//				"err":  err,
		//			}).Fatal("Error download from torbox")
		//		}
		//	}
	}

	if err := iterator.Err(); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Fatal("Error iterating history")
	}
}

func main() {
	appConfig := setConfig()
	traktApiKey, traktClientSecret := getEnvTrakt()
	appConfig.traktToken = setUpTrakt(appConfig, traktApiKey, traktClientSecret)
	appConfig.TorBoxClient = torbox.NewTorBoxClient(getEnvTorBox())

	getNewMovies(appConfig)

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
