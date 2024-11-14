package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/amaumene/momenarr/internal/torbox"
	"github.com/jacklaaa89/trakt"
	"github.com/jacklaaa89/trakt/show"
	"github.com/jacklaaa89/trakt/sync"
	"github.com/ostafen/clover/v2"
	"github.com/ostafen/clover/v2/document"
	"github.com/ostafen/clover/v2/query"
	log "github.com/sirupsen/logrus"
	"github.com/timshannon/bolthold"
	bolt "go.etcd.io/bbolt"
	"net/http"
	"os"
	"os/signal"
	"sort"
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
	db                 *clover.DB
}

type Downloads struct {
	ID   string
	Item []Item `xml:"item"`
}

var (
	currentDownloads []Downloads
)

func sortNZBsMovies(Feed Feed, year int64) []Item {
	// Filter the slice before sorting
	filteredItems := []Item{}
	for _, item := range Feed.Channel.Item {
		titleLower := strings.ToLower(item.Title)
		movieYearStr := fmt.Sprintf(".%d.", year)
		if strings.Contains(titleLower, "remux") || strings.Contains(titleLower, "2160p") || strings.Contains(titleLower, "1080p") {
			if strings.Contains(item.Title, movieYearStr) {
				filteredItems = append(filteredItems, item)
			}
		}
	}
	Feed.Channel.Item = filteredItems

	sort.Slice(Feed.Channel.Item, func(i, j int) bool {
		// Sort by 'remux' in title first
		iIsRemux := strings.Contains(strings.ToLower(Feed.Channel.Item[i].Title), "remux")
		jIsRemux := strings.Contains(strings.ToLower(Feed.Channel.Item[j].Title), "remux")

		if iIsRemux && !jIsRemux {
			return true
		}
		if !iIsRemux && jIsRemux {
			return false
		}
		return Feed.Channel.Item[i].Enclosure.Attributes.Length > Feed.Channel.Item[j].Enclosure.Attributes.Length
	})
	var nzbs []Item
	nzbs = append(nzbs, Feed.Channel.Item...)
	fmt.Println(nzbs)
	return nzbs
}

func sortNZBsShows(Feed Feed, show *trakt.Show) Feed {
	sort.Slice(Feed.Channel.Item, func(i, j int) bool {
		return Feed.Channel.Item[i].Enclosure.Attributes.Length > Feed.Channel.Item[j].Enclosure.Attributes.Length
	})
	sort.Slice(Feed.Channel.Item, func(i, j int) bool {
		// Sort by 'remux' in title first
		iIsRemux := strings.Contains(strings.ToLower(Feed.Channel.Item[i].Title), "remux")
		jIsRemux := strings.Contains(strings.ToLower(Feed.Channel.Item[j].Title), "remux")

		if iIsRemux && !jIsRemux {
			return true
		}
		if !iIsRemux && jIsRemux {
			return false
		}

		// Then sort by length
		return Feed.Channel.Item[i].Enclosure.Attributes.Length > Feed.Channel.Item[j].Enclosure.Attributes.Length
	})
	return Feed
}

func getNextEpisodes(showProgress *trakt.WatchedProgress, item *trakt.WatchListEntry, episodeNum int64, appConfig App) {
	fileName := fmt.Sprintf("%s S%02dE%02d", item.Show.Title, showProgress.NextEpisode.Season, episodeNum)
	if fileExists(fileName, appConfig.downloadDir) == "" {
		xmlResponse, err := searchTVShow(item.Show.TVDB, int(showProgress.NextEpisode.Season), int(episodeNum), appConfig)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		var Feed Feed
		err = xml.Unmarshal([]byte(xmlResponse), &Feed)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Fatal("Error unmarshalling XML")
		}
		filteredFeed := sortNZBsShows(Feed, item.Show)

		show := findOrCreateData(item.Show.Title + " S" + strconv.Itoa(int(showProgress.NextEpisode.Season)) + "E" + strconv.Itoa(int(episodeNum)))
		show.Item = append(show.Item, filteredFeed.Channel.Item...)
		log.WithFields(log.Fields{
			"name":    show.Item[0].Title,
			"season":  showProgress.NextEpisode.Season,
			"episode": episodeNum,
		}).Info("Going to download")
		UsenetCreateDownloadResponse, err := appConfig.TorBoxClient.CreateUsenetDownload(show.Item[0].Enclosure.Attributes.URL, show.Item[0].Title)
		if err != nil {
			log.WithFields(log.Fields{
				"show": show.Item[0].Title,
				"err":  err,
			}).Fatal("Error creating transfer")
		}
		if UsenetCreateDownloadResponse.Detail == "Found cached usenet download. Using cached download." {
			err = downloadCachedData(UsenetCreateDownloadResponse, appConfig)
			if err != nil {
				log.WithFields(log.Fields{
					"show": show.Item[0].Title,
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
	newMovie := Downloads{ID: ID, Item: []Item{}}
	currentDownloads = append(currentDownloads, newMovie)
	return &currentDownloads[len(currentDownloads)-1]
}

func syncMoviesDb(appConfig App) {
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
		existingMovie, _ := appConfig.db.FindAll(query.NewQuery("movies").Where(query.Field("IMDB").Eq(IMDB)))
		if len(existingMovie) == 0 {
			doc := document.NewDocumentOf(movie)
			_, err = appConfig.db.InsertOne("movies", doc)
			if err != nil {
				log.WithFields(log.Fields{
					"imdb_id": item.Movie.IMDB,
					"title":   item.Movie.Title,
					"year":    item.Movie.Year,
				}).Fatal("Error inserting movie into database")
			}
		}
	}

	if err := iterator.Err(); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Fatal("Error iterating history")
	}
}

// Movie represents the structure for the queried movie data.
type Movie struct {
	IMDB       int64
	Title      string
	Year       int64
	OnDisk     bool
	File       string
	DownloadID int64
}

type NZB struct {
	ID  int64
	NZB Item
}

func populateNzbsMovies(appConfig App) error {
	docs, _ := appConfig.db.FindAll(query.NewQuery("movies").Where(query.Field("OnDisk").IsFalse()))

	movie := Movie{}

	for _, doc := range docs {
		doc.Unmarshal(&movie)
		jsonResponse, err := searchMovie(movie.IMDB, appConfig)
		if err != nil {
			log.WithFields(log.Fields{"movie": movie}).Fatal("Error searching movie")
			return err
		}

		var feed Feed
		err = json.Unmarshal([]byte(jsonResponse), &feed)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Fatal("Error unmarshalling JSON")
			return err
		}

		//filteredFeed := sortNZBsMovies(feed, movie.Year)
		if len(feed.Channel.Item) > 0 {
			for _, item := range feed.Channel.Item {
				nzb := NZB{
					ID:  movie.IMDB,
					NZB: item,
				}
				doc := document.NewDocumentOf(nzb)
				_, err = appConfig.db.InsertOne("nzbs", doc)
				if err != nil {
					log.WithFields(log.Fields{
						"IMDB": nzb.ID,
						"NZB":  nzb.NZB,
					}).Fatal("Error inserting nzb into database")
				}
			}
		}
	}
	return nil
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
//			_, err = appConfig.db.Exec(`UPDATE movies SET download_id = ? WHERE imdb_id = ?`, torboxDownload.Data.UsenetDownloadID, imdbid)
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

func getNewMovies(appConfig App) error {
	docs, _ := appConfig.db.FindAll(query.NewQuery("movies").Where(query.Field("OnDisk").IsFalse()))
	for _, doc := range docs {
		movie := Movie{}
		doc.Unmarshal(&movie)
		nzbs, _ := appConfig.db.FindAll(query.NewQuery("nzbs").Where(query.Field("ID").Eq(movie.IMDB)))
		allNZBs := []NZB{}
		sort.Slice(nzbs, func(i, j int) bool {
			return nzbs[i]..Enclosure.Attributes.Length < nzbs[j].NZB.Enclosure.Attributes.Length
		})
		for _, nzb := range nzbs {
			nzb := NZB{}
			doc.Unmarshal(&nzb)
			allNZBs = append(allNZBs, nzb)
		}
		for _, nzb := range nzbs {
		
		}
		nzb := NZB{}
		nzbs[0].Unmarshal(&nzb)
		fmt.Printf("%v+", nzb)
	}
	return nil
}

func main() {
	appConfig := setConfig()
	traktApiKey, traktClientSecret := getEnvTrakt()
	appConfig.traktToken = setUpTrakt(appConfig, traktApiKey, traktClientSecret)
	appConfig.TorBoxClient = torbox.NewTorBoxClient(getEnvTorBox())
	var err error
	appConfig.db, err = clover.Open("data-db")
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Fatal("Error opening database")
	}
	appConfig.db.CreateCollection("movies")
	appConfig.db.CreateCollection("nzbs")

	// Create a channel to listen for interrupt signals (e.g., SIGINT)
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt)

	// Run a separate goroutine to handle the shutdown signal
	go func() {
		<-shutdownChan
		log.Info("Received shutdown signal, shutting down gracefully...")

		// Close the database connection
		if err := appConfig.db.Close(); err != nil {
			log.Error("Error closing database: ", err)
		}

		// Any other cleanup tasks go here

		log.Info("Server shut down successfully.")
		os.Exit(0)
	}()

	syncMoviesDb(appConfig)
	populateNzbsMovies(appConfig)
	getNewMovies(appConfig)
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
