package services

import (
	"context"
	"fmt"
	"time"

	"github.com/amaumene/momenarr/pkg/models"
	"github.com/amaumene/momenarr/pkg/repository"
	"github.com/amaumene/momenarr/pkg/utils"
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
	repo        repository.Repository
	token       *trakt.Token
	tmdbService *TMDBService
	rateLimiter *utils.RateLimiter
}

// NewTraktService creates a new TraktService
func NewTraktService(repo repository.Repository, token *trakt.Token) *TraktService {
	return &TraktService{
		repo:        repo,
		token:       token,
		rateLimiter: utils.TraktRateLimiter(),
	}
}

// NewTraktServiceWithTMDB creates a new TraktService with TMDB support
func NewTraktServiceWithTMDB(repo repository.Repository, token *trakt.Token, tmdbService *TMDBService) *TraktService {
	return &TraktService{
		repo:        repo,
		token:       token,
		tmdbService: tmdbService,
		rateLimiter: utils.TraktRateLimiter(),
	}
}

// UpdateToken updates the Trakt token while preserving other service configuration
func (s *TraktService) UpdateToken(token *trakt.Token) {
	s.token = token
}

// SyncFromTrakt synchronizes movies and episodes from Trakt
func (s *TraktService) SyncFromTrakt() ([]int64, error) {
	return s.SyncFromTraktWithContext(context.Background())
}

// getOriginalLanguageFromTMDB fetches original language from TMDB if service is available
func (s *TraktService) getOriginalLanguageFromTMDB(mediaType string, tmdbID int64) string {
	if s.tmdbService == nil {
		log.Debug("TMDB service not available for language lookup")
		return ""
	}

	if tmdbID == 0 {
		log.Debug("TMDB ID is 0, skipping language lookup")
		return ""
	}

	log.WithFields(log.Fields{
		"tmdb_id":    tmdbID,
		"media_type": mediaType,
	}).Info("Fetching original language from TMDB during sync")

	originalLang := s.tmdbService.GetOriginalLanguage(mediaType, tmdbID)
	if originalLang != "" {
		log.WithFields(log.Fields{
			"tmdb_id":           tmdbID,
			"media_type":        mediaType,
			"original_language": originalLang,
		}).Info("Successfully retrieved original language during sync")
	} else {
		log.WithFields(log.Fields{
			"tmdb_id":    tmdbID,
			"media_type": mediaType,
		}).Warn("Failed to retrieve original language from TMDB")
	}

	return originalLang
}

// SyncFromTraktWithContext synchronizes movies and episodes from Trakt with context support
func (s *TraktService) SyncFromTraktWithContext(ctx context.Context) ([]int64, error) {
	log.Info("Starting database sync from Trakt API")

	if err := s.checkContext(ctx); err != nil {
		return nil, err
	}

	movies, err := s.syncMoviesWithLogging(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.checkContext(ctx); err != nil {
		return nil, err
	}

	episodes, err := s.syncEpisodesWithLogging(ctx)
	if err != nil {
		return nil, err
	}

	return s.mergeAndLogResults(movies, episodes)
}

// checkContext checks if context is cancelled
func (s *TraktService) checkContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// syncMoviesWithLogging syncs movies with logging
func (s *TraktService) syncMoviesWithLogging(ctx context.Context) ([]int64, error) {
	log.Info("Syncing movies from Trakt watchlist and favorites")
	movies, err := s.syncMoviesFromTraktWithContext(ctx)
	if err != nil {
		log.WithError(err).Error("Failed to sync movies from Trakt")
		return nil, fmt.Errorf("syncing movies from Trakt: %w", err)
	}
	log.WithField("count", len(movies)).Info("Completed movie sync from Trakt")
	return movies, nil
}

// syncEpisodesWithLogging syncs episodes with logging
func (s *TraktService) syncEpisodesWithLogging(ctx context.Context) ([]int64, error) {
	log.Info("Syncing TV episodes from Trakt watchlist and favorites")
	episodes, err := s.syncEpisodesFromTraktWithContext(ctx)
	if err != nil {
		log.WithError(err).Error("Failed to sync episodes from Trakt")
		return nil, fmt.Errorf("syncing episodes from Trakt: %w", err)
	}
	log.WithField("count", len(episodes)).Info("Completed episode sync from Trakt")
	return episodes, nil
}

// mergeAndLogResults merges results and logs summary
func (s *TraktService) mergeAndLogResults(movies, episodes []int64) ([]int64, error) {
	merged := append(movies, episodes...)
	if len(merged) == 0 {
		return nil, fmt.Errorf("no media found during sync")
	}

	log.WithField("total_items", len(merged)).Info("Database sync from Trakt completed")
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
	if int64(movie.Trakt) <= 0 {
		return nil, fmt.Errorf("invalid movie data: Trakt=%d", movie.Trakt)
	}

	// Check if media already exists to preserve OnDisk status
	existing, err := s.repo.GetMedia(int64(movie.Trakt))
	if err == nil && existing != nil {
		// Update existing media but preserve OnDisk status and File path
		existing.Title = movie.Title
		existing.Year = movie.Year

		// Update original language and French title if TMDB service is available
		if existing.TMDBID > 0 && existing.OriginalLanguage == "" {
			existing.OriginalLanguage = s.getOriginalLanguageFromTMDB("movie", existing.TMDBID)
			// If original language is French, get and store French title
			if existing.OriginalLanguage == "fr" && s.tmdbService != nil {
				existing.FrenchTitle = s.tmdbService.GetFrenchTitle("movie", existing.TMDBID, existing.Title)
			}
		}

		existing.UpdatedAt = time.Now()
		return existing, nil
	}

	// Create new media entry
	tmdbID := int64(movie.MediaIDs.TMDB)
	log.WithFields(log.Fields{
		"trakt_id": movie.Trakt,
		"title":    movie.Title,
		"tmdb_id":  tmdbID,
	}).Info("Creating new movie media during sync")

	originalLanguage := s.getOriginalLanguageFromTMDB("movie", tmdbID)

	// Get French title if original language is French
	var frenchTitle string
	if originalLanguage == "fr" && s.tmdbService != nil {
		log.WithField("tmdb_id", tmdbID).Info("Fetching French title for French movie during sync")
		frenchTitle = s.tmdbService.GetFrenchTitle("movie", tmdbID, movie.Title)
		if frenchTitle != "" {
			log.WithFields(log.Fields{
				"tmdb_id": tmdbID,
				"english": movie.Title,
				"french":  frenchTitle,
			}).Info("Successfully retrieved French title during sync")
		}
	}

	return &models.Media{
		Trakt:            int64(movie.Trakt),
		TMDBID:           tmdbID,
		OriginalLanguage: originalLanguage,
		FrenchTitle:      frenchTitle,
		Title:            movie.Title,
		Year:             movie.Year,
		OnDisk:           false,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
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

		// s.rateLimiter.Wait()
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

		// s.rateLimiter.Wait()
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
		// s.rateLimiter.Wait()
		ep, err := episode.Get(show.Trakt, nextEpisode.Season, nextEpisode.Number+int64(i), nil)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"show":    show.Title,
				"season":  nextEpisode.Season,
				"episode": nextEpisode.Number + int64(i),
			}).Debug("Failed to get episode, trying next season")

			// Try next season
			// s.rateLimiter.Wait()
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
	if err := s.validateEpisode(ep); err != nil {
		return err
	}

	existing, err := s.repo.GetMedia(int64(ep.Trakt))
	if err == nil && existing != nil {
		return s.updateExistingEpisode(existing, show, ep)
	}

	return s.createNewEpisode(show, ep)
}

func (s *TraktService) validateEpisode(ep *trakt.Episode) error {
	if int64(ep.Trakt) <= 0 || ep.Number <= 0 || ep.Season <= 0 {
		return fmt.Errorf("invalid episode data: Trakt=%d, Season=%d, Number=%d",
			ep.Trakt, ep.Season, ep.Number)
	}
	return nil
}

func (s *TraktService) updateExistingEpisode(existing *models.Media, show *trakt.Show, ep *trakt.Episode) error {
	existing.Number = ep.Number
	existing.Season = ep.Season
	existing.Title = show.Title
	existing.Year = show.Year

	s.updateLanguageInfo(existing)
	existing.UpdatedAt = time.Now()

	if err := s.repo.SaveMedia(existing); err != nil {
		return fmt.Errorf("updating episode %d: %w", ep.Trakt, err)
	}
	return nil
}

func (s *TraktService) updateLanguageInfo(media *models.Media) {
	if media.TMDBID > 0 && media.OriginalLanguage == "" {
		media.OriginalLanguage = s.getOriginalLanguageFromTMDB("show", media.TMDBID)
		if media.OriginalLanguage == "fr" && s.tmdbService != nil {
			media.FrenchTitle = s.tmdbService.GetFrenchTitle("show", media.TMDBID, media.Title)
		}
	}
}

func (s *TraktService) createNewEpisode(show *trakt.Show, ep *trakt.Episode) error {
	tmdbID := int64(show.MediaIDs.TMDB)
	s.logNewEpisodeCreation(show, ep, tmdbID)

	originalLanguage := s.getOriginalLanguageFromTMDB("show", tmdbID)
	frenchTitle := s.getFrenchTitleIfNeeded(originalLanguage, tmdbID, show.Title)

	media := &models.Media{
		Trakt:            int64(ep.Trakt),
		TMDBID:           tmdbID,
		OriginalLanguage: originalLanguage,
		FrenchTitle:      frenchTitle,
		Number:           ep.Number,
		Season:           ep.Season,
		Title:            show.Title,
		Year:             show.Year,
		OnDisk:           false,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := s.repo.SaveMedia(media); err != nil {
		if err.Error() != duplicateKeyError {
			return fmt.Errorf("saving episode to database: %w", err)
		}
	}
	return nil
}

func (s *TraktService) logNewEpisodeCreation(show *trakt.Show, ep *trakt.Episode, tmdbID int64) {
	log.WithFields(log.Fields{
		"trakt_id": ep.Trakt,
		"show":     show.Title,
		"tmdb_id":  tmdbID,
		"season":   ep.Season,
		"episode":  ep.Number,
	}).Info("Creating new episode media during sync")
}

func (s *TraktService) getFrenchTitleIfNeeded(originalLanguage string, tmdbID int64, title string) string {
	if originalLanguage != "fr" || s.tmdbService == nil {
		return ""
	}

	log.WithField("tmdb_id", tmdbID).Info("Fetching French title for French show during sync")
	frenchTitle := s.tmdbService.GetFrenchTitle("show", tmdbID, title)

	if frenchTitle != "" {
		log.WithFields(log.Fields{
			"tmdb_id": tmdbID,
			"english": title,
			"french":  frenchTitle,
		}).Info("Successfully retrieved French title during sync")
	}
	return frenchTitle
}
