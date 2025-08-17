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

type TraktService struct {
	repo        repository.Repository
	token       *trakt.Token
	tmdbService *TMDBService
	rateLimiter *utils.RateLimiter
}

func NewTraktService(repo repository.Repository, token *trakt.Token) *TraktService {
	return &TraktService{
		repo:        repo,
		token:       token,
		rateLimiter: utils.TraktRateLimiter(),
	}
}

func NewTraktServiceWithTMDB(repo repository.Repository, token *trakt.Token, tmdbService *TMDBService) *TraktService {
	return &TraktService{
		repo:        repo,
		token:       token,
		tmdbService: tmdbService,
		rateLimiter: utils.TraktRateLimiter(),
	}
}

func (s *TraktService) UpdateToken(token *trakt.Token) {
	s.token = token
}

func (s *TraktService) SyncFromTrakt() ([]int64, error) {
	return s.SyncFromTraktWithContext(context.Background())
}

func (s *TraktService) getOriginalLanguageFromTMDB(mediaType string, tmdbID int64) string {
	if !s.canFetchLanguage(tmdbID) {
		return ""
	}

	s.logLanguageFetchStart(mediaType, tmdbID)
	originalLang := s.tmdbService.GetOriginalLanguage(mediaType, tmdbID)
	s.logLanguageFetchResult(mediaType, tmdbID, originalLang)

	return originalLang
}

func (s *TraktService) canFetchLanguage(tmdbID int64) bool {
	if s.tmdbService == nil {
		log.Debug("TMDB service not available for language lookup")
		return false
	}
	if tmdbID == 0 {
		log.Debug("TMDB ID is 0, skipping language lookup")
		return false
	}
	return true
}

func (s *TraktService) logLanguageFetchStart(mediaType string, tmdbID int64) {
	log.WithFields(log.Fields{
		"tmdb_id":    tmdbID,
		"media_type": mediaType,
	}).Info("Fetching original language from TMDB during sync")
}

func (s *TraktService) logLanguageFetchResult(mediaType string, tmdbID int64, originalLang string) {
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
}

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

func (s *TraktService) checkContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

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

func (s *TraktService) syncMoviesFromTrakt() ([]int64, error) {
	return s.syncMoviesFromTraktWithContext(context.Background())
}

func (s *TraktService) syncMoviesFromTraktWithContext(ctx context.Context) ([]int64, error) {
	watchlist, err := s.syncMoviesFromWatchlist()
	if err != nil {
		return nil, fmt.Errorf("syncing movies from watchlist: %w", err)
	}

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

func (s *TraktService) syncMoviesFromWatchlist() ([]int64, error) {
	tokenParams := trakt.ListParams{OAuth: s.token.AccessToken}
	watchListParams := &trakt.ListWatchListParams{
		ListParams: tokenParams,
		Type:       trakt.TypeMovie,
	}

	iterator := sync.WatchList(watchListParams)
	return s.processMovieIterator(iterator, "watchlist")
}

func (s *TraktService) syncMoviesFromFavorites() ([]int64, error) {
	tokenParams := trakt.ListParams{OAuth: s.token.AccessToken}
	params := &trakt.ListFavoritesParams{
		ListParams: tokenParams,
		Type:       trakt.TypeMovie,
	}

	iterator := sync.Favorites(params)
	return s.processMovieIterator(iterator, "favorites")
}

func (s *TraktService) processMovieIterator(iterator interface{}, source string) ([]int64, error) {
	next, err := s.getIteratorFunctions(iterator)
	if next == nil {
		return nil, fmt.Errorf("unsupported iterator type: %T", iterator)
	}

	movieIDs, mediaBatch := s.iterateAndProcessMovies(iterator, next, source)
	s.saveFinalMovieBatch(mediaBatch, source)

	if iterErr := err(); iterErr != nil {
		return nil, fmt.Errorf("iterating movie %s: %w", source, iterErr)
	}

	return movieIDs, nil
}

func (s *TraktService) getIteratorFunctions(iterator interface{}) (func() bool, func() error) {
	switch it := iterator.(type) {
	case *trakt.WatchListEntryIterator:
		return it.Next, it.Err
	case *trakt.FavoritesEntryIterator:
		return it.Next, it.Err
	default:
		return nil, nil
	}
}

func (s *TraktService) iterateAndProcessMovies(iterator interface{}, next func() bool, source string) ([]int64, []*models.Media) {
	var movieIDs []int64
	var mediaBatch []*models.Media
	const batchSize = 200

	for next() {
		movieID, media := s.processNextMovie(iterator, source)
		if media == nil {
			continue
		}

		mediaBatch = append(mediaBatch, media)
		movieIDs = append(movieIDs, movieID)

		if len(mediaBatch) >= batchSize {
			s.saveMovieBatch(mediaBatch, source)
			mediaBatch = nil
		}
	}

	return movieIDs, mediaBatch
}

func (s *TraktService) processNextMovie(iterator interface{}, source string) (int64, *models.Media) {
	movie, err := s.extractMovieFromIterator(iterator)
	if err != nil {
		log.WithError(err).Errorf("Failed to scan movie item from %s", source)
		return 0, nil
	}

	media, createErr := s.createMovieMedia(movie)
	if createErr != nil {
		log.WithError(createErr).WithField("movie", movie.Title).Errorf("Failed to create movie media from %s", source)
		return 0, nil
	}

	return int64(movie.Trakt), media
}

func (s *TraktService) extractMovieFromIterator(iterator interface{}) (*trakt.Movie, error) {
	switch it := iterator.(type) {
	case *trakt.WatchListEntryIterator:
		item, err := it.Entry()
		if err != nil {
			return nil, err
		}
		return item.Movie, nil
	case *trakt.FavoritesEntryIterator:
		item, err := it.Entry()
		if err != nil {
			return nil, err
		}
		return item.Movie, nil
	default:
		return nil, fmt.Errorf("unsupported iterator type")
	}
}

func (s *TraktService) saveMovieBatch(mediaBatch []*models.Media, source string) {
	if saveErr := s.repo.SaveMediaBatch(mediaBatch); saveErr != nil {
		log.WithError(saveErr).Errorf("Failed to save movie batch from %s", source)
	}
}

func (s *TraktService) saveFinalMovieBatch(mediaBatch []*models.Media, source string) {
	if len(mediaBatch) > 0 {
		if saveErr := s.repo.SaveMediaBatch(mediaBatch); saveErr != nil {
			log.WithError(saveErr).Errorf("Failed to save final movie batch from %s", source)
		}
	}
}

func (s *TraktService) createMovieMedia(movie *trakt.Movie) (*models.Media, error) {
	if int64(movie.Trakt) <= 0 {
		return nil, fmt.Errorf("invalid movie data: Trakt=%d", movie.Trakt)
	}

	existing, err := s.repo.GetMedia(int64(movie.Trakt))
	if err == nil && existing != nil {
		return s.updateExistingMovie(existing, movie)
	}

	return s.createNewMovie(movie)
}

func (s *TraktService) updateExistingMovie(existing *models.Media, movie *trakt.Movie) (*models.Media, error) {
	existing.Title = movie.Title
	existing.Year = movie.Year

	if existing.TMDBID > 0 && existing.OriginalLanguage == "" {
		existing.OriginalLanguage = s.getOriginalLanguageFromTMDB("movie", existing.TMDBID)
		if existing.OriginalLanguage == "fr" && s.tmdbService != nil {
			existing.FrenchTitle = s.tmdbService.GetFrenchTitle("movie", existing.TMDBID, existing.Title)
		}
	}

	existing.UpdatedAt = time.Now()
	return existing, nil
}

func (s *TraktService) createNewMovie(movie *trakt.Movie) (*models.Media, error) {
	tmdbID := int64(movie.MediaIDs.TMDB)
	s.logNewMovieCreation(movie, tmdbID)

	originalLanguage := s.getOriginalLanguageFromTMDB("movie", tmdbID)
	frenchTitle := s.getMovieFrenchTitle(originalLanguage, tmdbID, movie.Title)

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

func (s *TraktService) logNewMovieCreation(movie *trakt.Movie, tmdbID int64) {
	log.WithFields(log.Fields{
		"trakt_id": movie.Trakt,
		"title":    movie.Title,
		"tmdb_id":  tmdbID,
	}).Info("Creating new movie media during sync")
}

func (s *TraktService) getMovieFrenchTitle(originalLanguage string, tmdbID int64, title string) string {
	if originalLanguage != "fr" || s.tmdbService == nil {
		return ""
	}

	log.WithField("tmdb_id", tmdbID).Info("Fetching French title for French movie during sync")
	frenchTitle := s.tmdbService.GetFrenchTitle("movie", tmdbID, title)

	if frenchTitle != "" {
		log.WithFields(log.Fields{
			"tmdb_id": tmdbID,
			"english": title,
			"french":  frenchTitle,
		}).Info("Successfully retrieved French title during sync")
	}
	return frenchTitle
}

func (s *TraktService) syncEpisodesFromTrakt() ([]int64, error) {
	return s.syncEpisodesFromTraktWithContext(context.Background())
}

func (s *TraktService) syncEpisodesFromTraktWithContext(ctx context.Context) ([]int64, error) {
	watchlist, err := s.syncEpisodesFromWatchlist()
	if err != nil {
		return nil, fmt.Errorf("syncing episodes from watchlist: %w", err)
	}

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

func (s *TraktService) syncEpisodesFromWatchlist() ([]int64, error) {
	iterator := s.createWatchlistIterator()
	episodeIDs := s.processWatchlistIterator(iterator)

	if err := iterator.Err(); err != nil {
		return nil, fmt.Errorf("iterating episode watchlist: %w", err)
	}

	return episodeIDs, nil
}

func (s *TraktService) createWatchlistIterator() *trakt.WatchListEntryIterator {
	tokenParams := trakt.ListParams{OAuth: s.token.AccessToken}
	watchListParams := &trakt.ListWatchListParams{
		ListParams: tokenParams,
		Type:       trakt.TypeShow,
	}
	return sync.WatchList(watchListParams)
}

func (s *TraktService) processWatchlistIterator(iterator *trakt.WatchListEntryIterator) []int64 {
	var episodeIDs []int64

	for iterator.Next() {
		item, err := iterator.Entry()
		if err != nil {
			log.WithError(err).Error("Failed to scan episode item from watchlist")
			continue
		}

		epID := s.processWatchlistShow(item.Show)
		if epID > 0 {
			episodeIDs = append(episodeIDs, epID)
		}
	}

	return episodeIDs
}

func (s *TraktService) processWatchlistShow(showItem *trakt.Show) int64 {
	progressParams := &trakt.ProgressParams{
		Params: trakt.Params{OAuth: s.token.AccessToken},
	}

	showProgress, err := show.WatchedProgress(showItem.Trakt, progressParams)
	if err != nil {
		log.WithError(err).WithField("show", showItem.Title).Error("Failed to get show progress")
		return 0
	}

	if showProgress.NextEpisode == nil {
		return 0
	}

	if err := s.insertEpisodeToDB(showItem, showProgress.NextEpisode); err != nil {
		log.WithError(err).WithField("episode", showProgress.NextEpisode.Title).Error("Failed to insert episode into database")
		return 0
	}

	return int64(showProgress.NextEpisode.Trakt)
}

func (s *TraktService) syncEpisodesFromFavorites() ([]int64, error) {
	iterator := s.createFavoritesIterator()
	episodeIDs := s.processFavoritesIterator(iterator)

	if err := iterator.Err(); err != nil {
		return nil, fmt.Errorf("iterating episode favorites: %w", err)
	}

	return episodeIDs, nil
}

func (s *TraktService) createFavoritesIterator() *trakt.FavoritesEntryIterator {
	tokenParams := trakt.ListParams{OAuth: s.token.AccessToken}
	params := &trakt.ListFavoritesParams{
		ListParams: tokenParams,
		Type:       trakt.TypeShow,
	}
	return sync.Favorites(params)
}

func (s *TraktService) processFavoritesIterator(iterator *trakt.FavoritesEntryIterator) []int64 {
	var episodeIDs []int64

	for iterator.Next() {
		item, err := iterator.Entry()
		if err != nil {
			log.WithError(err).Error("Failed to scan episode item from favorites")
			continue
		}

		ids := s.processFavoriteShow(item.Show)
		episodeIDs = append(episodeIDs, ids...)
	}

	return episodeIDs
}

func (s *TraktService) processFavoriteShow(showItem *trakt.Show) []int64 {
	progressParams := &trakt.ProgressParams{
		Params: trakt.Params{OAuth: s.token.AccessToken},
	}

	showProgress, err := show.WatchedProgress(showItem.Trakt, progressParams)
	if err != nil {
		log.WithError(err).WithField("show", showItem.Title).Error("Failed to get show progress")
		return []int64{}
	}

	if showProgress.NextEpisode == nil {
		return []int64{}
	}

	ids, err := s.getNextEpisodes(showItem, showProgress.NextEpisode)
	if err != nil {
		log.WithError(err).WithField("show", showItem.Title).Error("Failed to get next episodes")
		return []int64{}
	}

	return ids
}

func (s *TraktService) getNextEpisodes(showItem *trakt.Show, nextEpisode *trakt.Episode) ([]int64, error) {
	var episodeIDs []int64

	for i := 0; i < maxEpisodesPerShow; i++ {
		ep := s.fetchEpisode(showItem, nextEpisode, i)
		if ep == nil {
			break
		}

		if err := s.insertEpisodeToDB(showItem, ep); err != nil {
			log.WithError(err).WithField("episode", ep.Title).Error("Failed to insert episode into database")
			continue
		}

		episodeIDs = append(episodeIDs, int64(ep.Trakt))
	}

	return episodeIDs, nil
}

func (s *TraktService) fetchEpisode(showItem *trakt.Show, nextEpisode *trakt.Episode, offset int) *trakt.Episode {
	ep, err := episode.Get(showItem.Trakt, nextEpisode.Season, nextEpisode.Number+int64(offset), nil)
	if err == nil {
		return ep
	}

	s.logEpisodeFetchError(showItem, nextEpisode, offset, err)
	return s.tryNextSeasonFirstEpisode(showItem, nextEpisode)
}

func (s *TraktService) logEpisodeFetchError(showItem *trakt.Show, nextEpisode *trakt.Episode, offset int, err error) {
	log.WithError(err).WithFields(log.Fields{
		"show":    showItem.Title,
		"season":  nextEpisode.Season,
		"episode": nextEpisode.Number + int64(offset),
	}).Debug("Failed to get episode, trying next season")
}

func (s *TraktService) tryNextSeasonFirstEpisode(showItem *trakt.Show, nextEpisode *trakt.Episode) *trakt.Episode {
	ep, err := episode.Get(showItem.Trakt, nextEpisode.Season+1, 1, nil)
	if err != nil {
		log.WithError(err).WithField("show", showItem.Title).Debug("No more episodes available")
		return nil
	}
	return ep
}

func (s *TraktService) insertEpisodeToDB(showItem *trakt.Show, ep *trakt.Episode) error {
	if err := s.validateEpisode(ep); err != nil {
		return err
	}

	existing, err := s.repo.GetMedia(int64(ep.Trakt))
	if err == nil && existing != nil {
		return s.updateExistingEpisode(existing, showItem, ep)
	}

	return s.createNewEpisode(showItem, ep)
}

func (s *TraktService) validateEpisode(ep *trakt.Episode) error {
	if int64(ep.Trakt) <= 0 || ep.Number <= 0 || ep.Season <= 0 {
		return fmt.Errorf("invalid episode data: Trakt=%d, Season=%d, Number=%d",
			ep.Trakt, ep.Season, ep.Number)
	}
	return nil
}

func (s *TraktService) updateExistingEpisode(existing *models.Media, showItem *trakt.Show, ep *trakt.Episode) error {
	existing.Number = ep.Number
	existing.Season = ep.Season
	existing.Title = showItem.Title
	existing.Year = showItem.Year

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

func (s *TraktService) createNewEpisode(showItem *trakt.Show, ep *trakt.Episode) error {
	tmdbID := int64(showItem.MediaIDs.TMDB)
	s.logNewEpisodeCreation(showItem, ep, tmdbID)

	media := s.buildEpisodeMedia(showItem, ep, tmdbID)
	return s.saveEpisodeMedia(media)
}

func (s *TraktService) buildEpisodeMedia(showItem *trakt.Show, ep *trakt.Episode, tmdbID int64) *models.Media {
	originalLanguage := s.getOriginalLanguageFromTMDB("show", tmdbID)
	frenchTitle := s.getFrenchTitleIfNeeded(originalLanguage, tmdbID, showItem.Title)

	return &models.Media{
		Trakt:            int64(ep.Trakt),
		TMDBID:           tmdbID,
		OriginalLanguage: originalLanguage,
		FrenchTitle:      frenchTitle,
		Number:           ep.Number,
		Season:           ep.Season,
		Title:            showItem.Title,
		Year:             showItem.Year,
		OnDisk:           false,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
}

func (s *TraktService) saveEpisodeMedia(media *models.Media) error {
	if err := s.repo.SaveMedia(media); err != nil {
		if err.Error() != duplicateKeyError {
			return fmt.Errorf("saving episode to database: %w", err)
		}
	}
	return nil
}

func (s *TraktService) logNewEpisodeCreation(showItem *trakt.Show, ep *trakt.Episode, tmdbID int64) {
	log.WithFields(log.Fields{
		"trakt_id": ep.Trakt,
		"show":     showItem.Title,
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
