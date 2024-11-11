package main

import (
	"encoding/xml"
	"fmt"
	"github.com/amaumene/momenarr/internal/torbox"
	"github.com/jacklaaa89/trakt"
	"github.com/jacklaaa89/trakt/show"
	"github.com/jacklaaa89/trakt/sync"
	"github.com/razsteinmetz/go-ptn"
	log "github.com/sirupsen/logrus"
	"net/http"
	"sort"
	"strconv"
	"time"
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

type Downloads struct {
	ID    string
	Items []Item `xml:"item"`
}

var (
	currentDownloads []Downloads
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

func sortNZBsShows(rss Rss, show *trakt.Show) Rss {
	sort.Slice(rss.Channel.Items, func(i, j int) bool {
		return rss.Channel.Items[i].Enclosure.Length > rss.Channel.Items[j].Enclosure.Length
	})
	returnedRss := Rss{}
	for _, item := range rss.Channel.Items {
		info, _ := ptn.Parse(item.Title)
		if info.Title == show.Title {
			if info.Quality == "BluRay" || info.Quality == "WEB-DL" {
				if info.Resolution == "1080p" || info.Resolution == "2160p" {
					returnedRss.Channel.Items = append(returnedRss.Channel.Items, item)
				}
			}
		}
	}
	return returnedRss
}

func getNextEpisodes(showProgress *trakt.WatchedProgress, item *trakt.WatchListEntry, episodeNum int64, appConfig App) {
	fileName := fmt.Sprintf("%s S%02dE%02d", item.Show.Title, showProgress.NextEpisode.Season, episodeNum)
	if fileExists(fileName, appConfig.downloadDir) != "" {
		xmlResponse, err := searchTVShow(item.Show.TVDB, int(showProgress.NextEpisode.Season), int(episodeNum), appConfig)
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
		filteredRss := sortNZBsShows(rss, item.Show)

		show := findOrCreateData(item.Show.Title + " S" + strconv.Itoa(int(showProgress.NextEpisode.Season)) + "E" + strconv.Itoa(int(episodeNum)))
		show.Items = append(show.Items, filteredRss.Channel.Items...)
		log.WithFields(log.Fields{
			"name":    show.Items[0].Title,
			"season":  showProgress.NextEpisode.Season,
			"episode": episodeNum,
		}).Info("Going to download")
		UsenetCreateDownloadResponse, err := appConfig.TorBoxClient.CreateUsenetDownload(show.Items[0].Enclosure.URL, show.Items[0].Title)
		if err != nil {
			log.WithFields(log.Fields{
				"show": show.Items[0].Title,
				"err":  err,
			}).Fatal("Error creating transfer")
		}
		if UsenetCreateDownloadResponse.Detail == "Found cached usenet download. Using cached download." {
			err = downloadCachedData(UsenetCreateDownloadResponse, appConfig)
			if err != nil {
				log.WithFields(log.Fields{
					"show": show.Items[0].Title,
					"err":  err,
				}).Fatal("Error downloading cached data")
			}
		}
	}
}

func getNewEpisodes(appConfig App) {
	tokenParams := trakt.ListParams{OAuth: appConfig.traktToken.AccessToken}

	watchListParams := &trakt.ListWatchListParams{
		ListParams: tokenParams,
		Type:       "show",
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

		progressParams := &trakt.ProgressParams{
			Params: trakt.Params{OAuth: appConfig.traktToken.AccessToken},
		}
		showProgress, err := show.WatchedProgress(item.Show.Trakt, progressParams)
		if err != nil {
			log.WithFields(log.Fields{
				"show": item.Show.Title,
				"err":  err,
			}).Fatal("Error getting show progress")
		}

		newEpisode := 3
		for i := 0; i < newEpisode; i++ {
			episodeNum := showProgress.NextEpisode.Number + int64(i)
			getNextEpisodes(showProgress, item, episodeNum, appConfig)
		}

	}

	if err := iterator.Err(); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Fatal("Error iterating history")
	}
}

func findOrCreateData(ID string) *Downloads {
	for i := range currentDownloads {
		if currentDownloads[i].ID == ID {
			return &currentDownloads[i]
		}
	}
	// If not found, create a new one and append to currentDownloads
	newMovie := Downloads{ID: ID, Items: []Item{}}
	currentDownloads = append(currentDownloads, newMovie)
	return &currentDownloads[len(currentDownloads)-1]
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
		if fileExists(item.Movie.Title, appConfig.downloadDir) != "" {
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
			movie := findOrCreateData(item.Movie.Title)
			movie.Items = append(movie.Items, filteredRss.Channel.Items...)

			fmt.Printf("Choosen file: %s", movie.Items[0].Title)

			UsenetCreateDownloadResponse, err := appConfig.TorBoxClient.CreateUsenetDownload(movie.Items[0].Enclosure.URL, movie.Items[0].Title)
			if err != nil {
				log.WithFields(log.Fields{
					"movie": movie.Items[0].Title,
					"err":   err,
				}).Fatal("Error creating transfer")
			}
			if UsenetCreateDownloadResponse.Detail == "Found cached usenet download. Using cached download." {
				err = downloadCachedData(UsenetCreateDownloadResponse, appConfig)
				if err != nil {
					log.WithFields(log.Fields{
						"movie": movie.Items[0].Title,
						"err":   err,
					}).Fatal("Error downloading cached data")
				}
			}
		}
	}

	if err := iterator.Err(); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Fatal("Error iterating history")
	}
}

func downloadCachedData(UsenetCreateDownloadResponse torbox.UsenetCreateDownloadResponse, appConfig App) error {
	log.WithFields(log.Fields{
		"id": UsenetCreateDownloadResponse.Data.UsenetDownloadID,
	}).Info("Found cached usenet download")
	UsenetDownload, err := appConfig.TorBoxClient.FindDownloadByID(UsenetCreateDownloadResponse.Data.UsenetDownloadID)
	if err != nil {
		return err
	}
	if UsenetDownload[0].Cached {
		log.WithFields(log.Fields{
			"name": UsenetDownload[0].Files[0].ShortName,
		}).Info("Starting download from cached data")
		err = downloadFromTorBox(UsenetDownload, appConfig)
		if err != nil {
			return err
		}
		return nil
	}
	log.WithFields(log.Fields{
		"name": UsenetDownload[0].Name,
	}).Info("Not really in cache, skipping and hoping to get a notification")
	return nil
}
func main() {
	appConfig := setConfig()
	traktApiKey, traktClientSecret := getEnvTrakt()
	appConfig.traktToken = setUpTrakt(appConfig, traktApiKey, traktClientSecret)
	appConfig.TorBoxClient = torbox.NewTorBoxClient(getEnvTorBox())

	go func() {
		for {
			cleanWatched(appConfig)
			getNewMovies(appConfig)
			getNewEpisodes(appConfig)
			time.Sleep(6 * time.Hour)
		}
	}()

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
