package services

import (
	"fmt"
	"time"

	"github.com/amaumene/momenarr/pkg/models"
	"github.com/amaumene/momenarr/pkg/repository"
	"github.com/amaumene/momenarr/trakt"
	"github.com/amaumene/momenarr/trakt/episode"
	"github.com/amaumene/momenarr/trakt/show"
	"github.com/amaumene/momenarr/trakt/sync"
	log "github.com/sirupsen/logrus"
)

const (
	maxEpisodesPerShow = 3
	duplicateKeyError  = "This Key already exists in this bolthold for this type"
)

// TraktService handles Trakt API operations
type TraktService struct {
	repo  repository.Repository
	token *trakt.Token
}

// NewTraktService creates a new TraktService
func NewTraktService(repo repository.Repository, token *trakt.Token) *TraktService {
	return &TraktService{
		repo:  repo,
		token: token,
	}
}

// SyncFromTrakt synchronizes movies and episodes from Trakt
func (s *TraktService) SyncFromTrakt() ([]int64, error) {
	movies, err := s.syncMoviesFromTrakt()
	if err != nil {
		log.WithError(err).Error("Failed to sync movies from Trakt")
		return nil, fmt.Errorf("syncing movies from Trakt: %w", err)
	}

	episodes, err := s.syncEpisodesFromTrakt()
	if err != nil {
		log.WithError(err).Error("Failed to sync episodes from Trakt")
		return nil, fmt.Errorf("syncing episodes from Trakt: %w", err)
	}

	merged := append(movies, episodes...)
	if len(merged) == 0 {
		return nil, fmt.Errorf("no media found during sync")
	}

	log.WithFields(log.Fields{
		"movies":   len(movies),
		"episodes": len(episodes),
		"total":    len(merged),
	}).Info("Successfully synced media from Trakt")

	return merged, nil
}

// syncMoviesFromTrakt syncs movies from both watchlist and favorites
func (s *TraktService) syncMoviesFromTrakt() ([]int64, error) {
	watchlist, err := s.syncMoviesFromWatchlist()
	if err != nil {
		return nil, fmt.Errorf("syncing movies from watchlist: %w", err)
	}

	favorites, err := s.syncMoviesFromFavorites()
	if err != nil {
		return nil, fmt.Errorf("syncing movies from favorites: %w", err)
	}

	merged := append(watchlist, favorites...)
	if len(merged) == 0 {
		return nil, fmt.Errorf("no movies found")
	}

	return merged, nil
}

// syncMoviesFromWatchlist syncs movies from Trakt watchlist
func (s *TraktService) syncMoviesFromWatchlist() ([]int64, error) {
	tokenParams := trakt.ListParams{OAuth: s.token.AccessToken}
	watchListParams := &trakt.ListWatchListParams{
		ListParams: tokenParams,
		Type:       "movie",
	}
	
	iterator := sync.WatchList(watchListParams)
	var movieIDs []int64

	for iterator.Next() {
		item, err := iterator.Entry()
		if err != nil {
			log.WithError(err).Error("Failed to scan movie item from watchlist")
			continue
		}

		if err := s.insertMovieToDB(item.Movie); err != nil {
			log.WithError(err).WithField("movie", item.Movie.Title).Error("Failed to insert movie into database")
			continue
		}

		movieIDs = append(movieIDs, int64(item.Movie.Trakt))
	}

	if err := iterator.Err(); err != nil {
		return nil, fmt.Errorf("iterating movie watchlist: %w", err)
	}

	return movieIDs, nil
}

// syncMoviesFromFavorites syncs movies from Trakt favorites
func (s *TraktService) syncMoviesFromFavorites() ([]int64, error) {
	tokenParams := trakt.ListParams{OAuth: s.token.AccessToken}
	params := &trakt.ListFavoritesParams{
		ListParams: tokenParams,
		Type:       "movies",
	}
	
	iterator := sync.Favorites(params)
	var movieIDs []int64

	for iterator.Next() {
		item, err := iterator.Entry()
		if err != nil {
			log.WithError(err).Error("Failed to scan movie item from favorites")
			continue
		}

		if err := s.insertMovieToDB(item.Movie); err != nil {
			log.WithError(err).WithField("movie", item.Movie.Title).Error("Failed to insert movie into database")
			continue
		}

		movieIDs = append(movieIDs, int64(item.Movie.Trakt))
	}

	if err := iterator.Err(); err != nil {
		return nil, fmt.Errorf("iterating movie favorites: %w", err)
	}

	return movieIDs, nil
}

// insertMovieToDB inserts a movie into the database
func (s *TraktService) insertMovieToDB(movie *trakt.Movie) error {
	if int64(movie.Trakt) <= 0 || len(movie.IMDB) == 0 {
		return fmt.Errorf("invalid movie data: Trakt=%d, IMDB=%s", movie.Trakt, movie.IMDB)
	}

	media := &models.Media{
		Trakt:     int64(movie.Trakt),
		IMDB:      string(movie.IMDB),
		Title:     movie.Title,
		Year:      movie.Year,
		OnDisk:    false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repo.SaveMedia(media); err != nil {
		// Handle duplicate key error gracefully
		if err.Error() != duplicateKeyError {
			return fmt.Errorf("saving movie to database: %w", err)
		}
	}

	return nil
}

// syncEpisodesFromTrakt syncs episodes from both watchlist and favorites
func (s *TraktService) syncEpisodesFromTrakt() ([]int64, error) {
	watchlist, err := s.syncEpisodesFromWatchlist()
	if err != nil {
		return nil, fmt.Errorf("syncing episodes from watchlist: %w", err)
	}

	favorites, err := s.syncEpisodesFromFavorites()
	if err != nil {
		return nil, fmt.Errorf("syncing episodes from favorites: %w", err)
	}

	merged := append(watchlist, favorites...)
	if len(merged) == 0 {
		return nil, fmt.Errorf("no episodes found")
	}

	return merged, nil
}

// syncEpisodesFromWatchlist syncs episodes from Trakt watchlist
func (s *TraktService) syncEpisodesFromWatchlist() ([]int64, error) {
	tokenParams := trakt.ListParams{OAuth: s.token.AccessToken}
	watchListParams := &trakt.ListWatchListParams{
		ListParams: tokenParams,
		Type:       "show",
	}
	
	iterator := sync.WatchList(watchListParams)
	var episodeIDs []int64

	for iterator.Next() {
		item, err := iterator.Entry()
		if err != nil {
			log.WithError(err).Error("Failed to scan episode item from watchlist")
			continue
		}

		progressParams := &trakt.ProgressParams{
			Params: trakt.Params{OAuth: s.token.AccessToken},
		}
		
		showProgress, err := show.WatchedProgress(item.Show.Trakt, progressParams)
		if err != nil {
			log.WithError(err).WithField("show", item.Show.Title).Error("Failed to get show progress")
			continue
		}

		if showProgress.NextEpisode != nil {
			if err := s.insertEpisodeToDB(item.Show, showProgress.NextEpisode); err != nil {
				log.WithError(err).WithField("episode", showProgress.NextEpisode.Title).Error("Failed to insert episode into database")
				continue
			}
			episodeIDs = append(episodeIDs, int64(showProgress.NextEpisode.Trakt))
		}
	}

	if err := iterator.Err(); err != nil {
		return nil, fmt.Errorf("iterating episode watchlist: %w", err)
	}

	return episodeIDs, nil
}

// syncEpisodesFromFavorites syncs episodes from Trakt favorites
func (s *TraktService) syncEpisodesFromFavorites() ([]int64, error) {
	tokenParams := trakt.ListParams{OAuth: s.token.AccessToken}
	params := &trakt.ListFavoritesParams{
		ListParams: tokenParams,
		Type:       "shows",
	}
	
	iterator := sync.Favorites(params)
	var episodeIDs []int64

	for iterator.Next() {
		item, err := iterator.Entry()
		if err != nil {
			log.WithError(err).Error("Failed to scan episode item from favorites")
			continue
		}

		progressParams := &trakt.ProgressParams{
			Params: trakt.Params{OAuth: s.token.AccessToken},
		}
		
		showProgress, err := show.WatchedProgress(item.Show.Trakt, progressParams)
		if err != nil {
			log.WithError(err).WithField("show", item.Show.Title).Error("Failed to get show progress")
			continue
		}

		if showProgress.NextEpisode != nil {
			ids, err := s.getNextEpisodes(item.Show, showProgress.NextEpisode)
			if err != nil {
				log.WithError(err).WithField("show", item.Show.Title).Error("Failed to get next episodes")
				continue
			}
			episodeIDs = append(episodeIDs, ids...)
		}
	}

	if err := iterator.Err(); err != nil {
		return nil, fmt.Errorf("iterating episode favorites: %w", err)
	}

	return episodeIDs, nil
}

// getNextEpisodes gets the next episodes for a show
func (s *TraktService) getNextEpisodes(show *trakt.Show, nextEpisode *trakt.Episode) ([]int64, error) {
	var episodeIDs []int64

	for i := 0; i < maxEpisodesPerShow; i++ {
		ep, err := episode.Get(show.Trakt, nextEpisode.Season, nextEpisode.Number+int64(i), nil)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"show":    show.Title,
				"season":  nextEpisode.Season,
				"episode": nextEpisode.Number + int64(i),
			}).Debug("Failed to get episode, trying next season")

			// Try next season
			ep, err = episode.Get(show.Trakt, nextEpisode.Season+1, 1, nil)
			if err != nil {
				log.WithError(err).WithField("show", show.Title).Debug("No more episodes available")
				break
			}
		}

		if err := s.insertEpisodeToDB(show, ep); err != nil {
			log.WithError(err).WithField("episode", ep.Title).Error("Failed to insert episode into database")
			continue
		}

		episodeIDs = append(episodeIDs, int64(ep.Trakt))
	}

	return episodeIDs, nil
}

// insertEpisodeToDB inserts an episode into the database
func (s *TraktService) insertEpisodeToDB(show *trakt.Show, ep *trakt.Episode) error {
	if int64(ep.Trakt) <= 0 || len(show.IMDB) == 0 || ep.Number <= 0 || ep.Season <= 0 {
		return fmt.Errorf("invalid episode data: Trakt=%d, IMDB=%s, Season=%d, Number=%d", 
			ep.Trakt, show.IMDB, ep.Season, ep.Number)
	}

	media := &models.Media{
		Trakt:     int64(ep.Trakt),
		Number:    ep.Number,
		Season:    ep.Season,
		IMDB:      string(show.IMDB),
		Title:     ep.Title,
		Year:      show.Year,
		OnDisk:    false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repo.SaveMedia(media); err != nil {
		// Handle duplicate key error gracefully
		if err.Error() != duplicateKeyError {
			return fmt.Errorf("saving episode to database: %w", err)
		}
	}

	return nil
}