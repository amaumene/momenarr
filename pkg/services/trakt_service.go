package services

import (
	"context"
	"fmt"
	"net/http"
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
	repo             repository.Repository
	token            *trakt.Token
	translationCache map[string]string // Cache for translations
}

// NewTraktService creates a new TraktService
func NewTraktService(repo repository.Repository, token *trakt.Token) *TraktService {
	return &TraktService{
		repo:             repo,
		token:            token,
		translationCache: make(map[string]string),
	}
}

// SyncFromTrakt synchronizes movies and episodes from Trakt
func (s *TraktService) SyncFromTrakt() ([]int64, error) {
	return s.SyncFromTraktWithContext(context.Background())
}

// SyncFromTraktWithContext synchronizes movies and episodes from Trakt with context support
func (s *TraktService) SyncFromTraktWithContext(ctx context.Context) ([]int64, error) {
	// Check context before starting
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	movies, err := s.syncMoviesFromTraktWithContext(ctx)
	if err != nil {
		log.WithError(err).Error("Failed to sync movies from Trakt")
		return nil, fmt.Errorf("syncing movies from Trakt: %w", err)
	}

	// Check context between operations
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	episodes, err := s.syncEpisodesFromTraktWithContext(ctx)
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
	return s.syncMoviesFromTraktWithContext(context.Background())
}

// syncMoviesFromTraktWithContext syncs movies from both watchlist and favorites with context
func (s *TraktService) syncMoviesFromTraktWithContext(ctx context.Context) ([]int64, error) {
	watchlist, err := s.syncMoviesFromWatchlist()
	if err != nil {
		return nil, fmt.Errorf("syncing movies from watchlist: %w", err)
	}

	// Check context between operations
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
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

// syncMoviesFromWatchlist syncs movies from Trakt watchlist using batch operations
func (s *TraktService) syncMoviesFromWatchlist() ([]int64, error) {
	tokenParams := trakt.ListParams{OAuth: s.token.AccessToken}
	watchListParams := &trakt.ListWatchListParams{
		ListParams: tokenParams,
		Type:       trakt.TypeMovie,
	}

	iterator := sync.WatchList(watchListParams)
	return s.processMovieIterator(iterator, "watchlist")
}

// syncMoviesFromFavorites syncs movies from Trakt favorites using batch operations
func (s *TraktService) syncMoviesFromFavorites() ([]int64, error) {
	tokenParams := trakt.ListParams{OAuth: s.token.AccessToken}
	params := &trakt.ListFavoritesParams{
		ListParams: tokenParams,
		Type:       trakt.TypeMovie,
	}

	iterator := sync.Favorites(params)
	return s.processMovieIterator(iterator, "favorites")
}

// processMovieIterator processes a movie iterator and saves movies in batches
func (s *TraktService) processMovieIterator(iterator interface{}, source string) ([]int64, error) {
	var movieIDs []int64
	var mediaBatch []*models.Media
	const batchSize = 200

	// Type assertion for different iterator types
	var next func() bool
	var err func() error

	switch it := iterator.(type) {
	case *trakt.WatchListEntryIterator:
		next = it.Next
		err = it.Err
	case *trakt.FavoritesEntryIterator:
		next = it.Next
		err = it.Err
	default:
		return nil, fmt.Errorf("unsupported iterator type: %T", iterator)
	}

	for next() {
		var movie *trakt.Movie
		var scanErr error

		// Get the movie from the appropriate iterator type
		switch it := iterator.(type) {
		case *trakt.WatchListEntryIterator:
			item, err := it.Entry()
			if err != nil {
				scanErr = err
			} else {
				movie = item.Movie
			}
		case *trakt.FavoritesEntryIterator:
			item, err := it.Entry()
			if err != nil {
				scanErr = err
			} else {
				movie = item.Movie
			}
		}

		if scanErr != nil {
			log.WithError(scanErr).Errorf("Failed to scan movie item from %s", source)
			continue
		}

		media, createErr := s.createMovieMedia(movie)
		if createErr != nil {
			log.WithError(createErr).WithField("movie", movie.Title).Errorf("Failed to create movie media from %s", source)
			continue
		}

		mediaBatch = append(mediaBatch, media)
		movieIDs = append(movieIDs, int64(movie.Trakt))

		// Save batch when it reaches batch size
		if len(mediaBatch) >= batchSize {
			if saveErr := s.repo.SaveMediaBatch(mediaBatch); saveErr != nil {
				log.WithError(saveErr).Errorf("Failed to save movie batch from %s", source)
			}
			mediaBatch = nil
		}
	}

	// Save remaining items in batch
	if len(mediaBatch) > 0 {
		if saveErr := s.repo.SaveMediaBatch(mediaBatch); saveErr != nil {
			log.WithError(saveErr).Errorf("Failed to save final movie batch from %s", source)
		}
	}

	if iterErr := err(); iterErr != nil {
		return nil, fmt.Errorf("iterating movie %s: %w", source, iterErr)
	}

	return movieIDs, nil
}

// createMovieMedia creates a media object from a Trakt movie without saving it
func (s *TraktService) createMovieMedia(movie *trakt.Movie) (*models.Media, error) {
	if int64(movie.Trakt) <= 0 || len(movie.IMDB) == 0 {
		return nil, fmt.Errorf("invalid movie data: Trakt=%d, IMDB=%s", movie.Trakt, movie.IMDB)
	}

	// Check if media already exists to preserve OnDisk status
	existing, err := s.repo.GetMedia(int64(movie.Trakt))
	if err == nil && existing != nil {
		// Update existing media but preserve OnDisk status and File path
		existing.IMDB = string(movie.IMDB)
		existing.TraktSlug = string(movie.Slug) // Update slug
		existing.Title = movie.Title
		existing.Year = movie.Year
		existing.UpdatedAt = time.Now()
		return existing, nil
	}

	// Create new media entry
	return &models.Media{
		Trakt:     int64(movie.Trakt),
		TraktSlug: string(movie.Slug), // Store slug for API calls
		IMDB:      string(movie.IMDB),
		Title:     movie.Title,
		Year:      movie.Year,
		OnDisk:    false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

// syncEpisodesFromTrakt syncs episodes from both watchlist and favorites
func (s *TraktService) syncEpisodesFromTrakt() ([]int64, error) {
	return s.syncEpisodesFromTraktWithContext(context.Background())
}

// syncEpisodesFromTraktWithContext syncs episodes from both watchlist and favorites with context
func (s *TraktService) syncEpisodesFromTraktWithContext(ctx context.Context) ([]int64, error) {
	watchlist, err := s.syncEpisodesFromWatchlist()
	if err != nil {
		return nil, fmt.Errorf("syncing episodes from watchlist: %w", err)
	}

	// Check context between operations
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
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
		Type:       trakt.TypeShow,
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
		Type:       trakt.TypeShow,
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

	// Check if media already exists to preserve OnDisk status
	existing, err := s.repo.GetMedia(int64(ep.Trakt))
	if err == nil && existing != nil {
		// Update existing media but preserve OnDisk status and File path
		existing.Number = ep.Number
		existing.Season = ep.Season
		existing.IMDB = string(show.IMDB)
		existing.TraktSlug = string(show.Slug) // Update show slug
		existing.Title = show.Title            // Use show title for torrent searches
		existing.Year = show.Year
		existing.UpdatedAt = time.Now()

		if err := s.repo.SaveMedia(existing); err != nil {
			return fmt.Errorf("updating episode %d: %w", ep.Trakt, err)
		}
		return nil
	}

	// Create new media entry
	media := &models.Media{
		Trakt:     int64(ep.Trakt),
		TraktSlug: string(show.Slug), // Store show slug for API calls
		Number:    ep.Number,
		Season:    ep.Season,
		IMDB:      string(show.IMDB),
		Title:     show.Title, // Use show title for torrent searches
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

// GetFrenchTitle gets the French title for a movie or show using Trakt API
func (s *TraktService) GetFrenchTitle(englishTitle string, mediaType string, traktSlug string) string {

	cacheKey := fmt.Sprintf("%s_%s_fr", mediaType, traktSlug)
	if cached, ok := s.translationCache[cacheKey]; ok {
		log.WithFields(log.Fields{
			"cache_key":    cacheKey,
			"cached_title": cached,
		}).Debug("Using cached French title")
		return cached
	}

	log.WithFields(log.Fields{
		"english_title": englishTitle,
		"media_type":    mediaType,
		"trakt_slug":    traktSlug,
	}).Debug("Looking up French title via Trakt API")

	var frenchTitle string
	if mediaType == "movie" {
		frenchTitle = s.getMovieTranslation(traktSlug, "fr")
	} else if mediaType == "series" || mediaType == "show" {
		frenchTitle = s.getShowTranslation(traktSlug, "fr")
	}

	log.WithFields(log.Fields{
		"english_title":   englishTitle,
		"french_title":    frenchTitle,
		"trakt_slug":      traktSlug,
		"media_type":      mediaType,
		"has_translation": frenchTitle != "",
	}).Debug("Trakt API translation result")

	if frenchTitle != "" && frenchTitle != englishTitle {
		s.translationCache[cacheKey] = frenchTitle
		log.WithFields(log.Fields{
			"english_title": englishTitle,
			"french_title":  frenchTitle,
			"trakt_slug":    traktSlug,
		}).Info("Found French translation for title")
		return frenchTitle
	}

	s.translationCache[cacheKey] = englishTitle
	return englishTitle
}

// getMovieTranslation fetches movie translation from Trakt API
func (s *TraktService) getMovieTranslation(traktSlug string, language string) string {
	params := &trakt.TranslationListParams{
		Language: language,
	}

	client := trakt.NewClient()
	path := trakt.FormatURLPath("movies/%s/translations/%s", trakt.Slug(traktSlug), language)
	iterator := &trakt.TranslationIterator{BasicIterator: client.NewSimulatedIterator(http.MethodGet, path, params)}

	if iterator.Next() {
		translation, err := iterator.Translation()
		if err != nil {
			log.WithFields(log.Fields{
				"trakt_slug": traktSlug,
				"language":   language,
				"error":      err,
			}).Error("Error getting movie translation")
			return ""
		}

		if translation != nil && translation.Title != "" {
			log.WithFields(log.Fields{
				"trakt_slug":       traktSlug,
				"language":         language,
				"translated_title": translation.Title,
			}).Debug("Successfully got movie translation")
			return translation.Title
		}
	} else {
		log.WithFields(log.Fields{
			"trakt_slug": traktSlug,
			"language":   language,
		}).Debug("No translation found for movie")
	}

	return ""
}

// getShowTranslation fetches show translation from Trakt API
func (s *TraktService) getShowTranslation(traktSlug string, language string) string {
	params := &trakt.TranslationListParams{
		Language: language,
	}

	log.WithFields(log.Fields{
		"trakt_slug": traktSlug,
		"language":   language,
	}).Debug("Fetching show translation from Trakt API")

	iterator := show.Translations(trakt.Slug(traktSlug), params)

	// Check if iterator has an error
	if iterator.Err() != nil {
		log.WithFields(log.Fields{
			"trakt_slug": traktSlug,
			"language":   language,
			"error":      iterator.Err(),
		}).Error("Error creating show translation iterator")
		return ""
	}

	if iterator.Next() {
		translation, err := iterator.Translation()
		if err != nil {
			log.WithFields(log.Fields{
				"trakt_slug": traktSlug,
				"language":   language,
				"error":      err,
			}).Error("Error getting show translation")
			return ""
		}

		if translation != nil && translation.Title != "" {
			log.WithFields(log.Fields{
				"trakt_slug":       traktSlug,
				"language":         language,
				"translated_title": translation.Title,
			}).Info("Successfully got show translation")
			return translation.Title
		}
	} else {
		// Check if there was an error during iteration
		if iterator.Err() != nil {
			log.WithFields(log.Fields{
				"trakt_slug": traktSlug,
				"language":   language,
				"error":      iterator.Err(),
			}).Error("Error iterating show translations")
		} else {
			log.WithFields(log.Fields{
				"trakt_slug": traktSlug,
				"language":   language,
			}).Debug("No translation found for show")
		}
	}

	return ""
}
