package service

import (
	"context"
	"fmt"

	"github.com/amaumene/momenarr/internal/config"
	"github.com/amaumene/momenarr/internal/domain"
	"github.com/amaumene/momenarr/trakt"
	"github.com/amaumene/momenarr/trakt/episode"
	"github.com/amaumene/momenarr/trakt/show"
	"github.com/amaumene/momenarr/trakt/sync"
	log "github.com/sirupsen/logrus"
)

const (
	typeMovie  = "movie"
	typeMovies = "movies"
	typeShow   = "show"
	typeShows  = "shows"
)

type MediaService struct {
	cfg       *config.Config
	mediaRepo domain.MediaRepository
	token     *trakt.Token
}

func NewMediaService(cfg *config.Config, mediaRepo domain.MediaRepository, token *trakt.Token) *MediaService {
	return &MediaService{
		cfg:       cfg,
		mediaRepo: mediaRepo,
		token:     token,
	}
}

func (s *MediaService) SyncFromTrakt(ctx context.Context) ([]int64, error) {
	movies, err := s.syncMovies(ctx)
	if err != nil {
		log.WithFields(log.Fields{
			"operation": "sync_movies",
			"error":     err,
		}).Error("failed to sync movies from trakt, continuing")
	}

	episodes, err := s.syncEpisodes(ctx)
	if err != nil {
		log.WithFields(log.Fields{
			"operation": "sync_episodes",
			"error":     err,
		}).Error("failed to sync episodes from trakt, continuing")
	}

	merged := append(movies, episodes...)
	if len(merged) > 0 {
		if err := s.removeStaleMedia(ctx, merged); err != nil {
			log.WithFields(log.Fields{
				"operation": "remove_stale",
				"error":     err,
			}).Error("failed to remove stale media, continuing")
		}
	}

	return merged, nil
}

func (s *MediaService) syncMovies(ctx context.Context) ([]int64, error) {
	watchlist, err := s.syncMoviesFromWatchlist(ctx)
	if err != nil {
		return nil, fmt.Errorf("syncing watchlist: %w", err)
	}

	favorites, err := s.syncMoviesFromFavorites(ctx)
	if err != nil {
		return nil, fmt.Errorf("syncing favorites: %w", err)
	}

	merged := append(watchlist, favorites...)
	if len(merged) == 0 {
		return nil, domain.ErrNoMoviesFound
	}
	return merged, nil
}

func (s *MediaService) syncMoviesFromWatchlist(ctx context.Context) ([]int64, error) {
	params := s.buildMovieWatchlistParams()
	iterator := sync.WatchList(params)

	return s.collectMovies(ctx, iterator)
}

func (s *MediaService) buildMovieWatchlistParams() *trakt.ListWatchListParams {
	return &trakt.ListWatchListParams{
		ListParams: trakt.ListParams{OAuth: s.token.AccessToken},
		Type:       typeMovie,
	}
}

func (s *MediaService) collectMovies(ctx context.Context, iterator *trakt.WatchListEntryIterator) ([]int64, error) {
	var movieIDs []int64
	for iterator.Next() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		item, err := iterator.Entry()
		if err != nil {
			log.WithField("error", err).Error("failed to scan watchlist movie entry")
			continue
		}

		if err := s.insertMovie(ctx, item.Movie); err != nil {
			log.WithField("error", err).Error("failed to insert watchlist movie")
			continue
		}
		movieIDs = append(movieIDs, int64(item.Movie.Trakt))
	}

	return movieIDs, iterator.Err()
}

func (s *MediaService) insertMovie(ctx context.Context, movie *trakt.Movie) error {
	if !isValidMovie(movie) {
		return nil
	}

	media := buildMediaFromMovie(movie)
	err := s.mediaRepo.Insert(ctx, media.TraktID, media)
	if err != nil && err != domain.ErrDuplicateKey {
		return fmt.Errorf("inserting movie: %w", err)
	}
	return nil
}

func isValidMovie(movie *trakt.Movie) bool {
	return int64(movie.Trakt) > 0 && len(movie.IMDB) > 0
}

func buildMediaFromMovie(movie *trakt.Movie) *domain.Media {
	return &domain.Media{
		TraktID: int64(movie.Trakt),
		IMDB:    string(movie.IMDB),
		Title:   movie.Title,
		Year:    movie.Year,
		OnDisk:  false,
	}
}

func (s *MediaService) syncMoviesFromFavorites(ctx context.Context) ([]int64, error) {
	params := &trakt.ListFavoritesParams{
		ListParams: trakt.ListParams{OAuth: s.token.AccessToken},
		Type:       typeMovies,
	}
	iterator := sync.Favorites(params)

	return s.collectMoviesFromFavorites(ctx, iterator)
}

func (s *MediaService) collectMoviesFromFavorites(ctx context.Context, iterator *trakt.FavoritesEntryIterator) ([]int64, error) {
	var movieIDs []int64
	for iterator.Next() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		item, err := iterator.Entry()
		if err != nil {
			log.WithField("error", err).Error("failed to scan favorites movie entry")
			continue
		}

		if err := s.insertMovie(ctx, item.Movie); err != nil {
			log.WithField("error", err).Error("failed to insert favorites movie")
			continue
		}
		movieIDs = append(movieIDs, int64(item.Movie.Trakt))
	}

	return movieIDs, iterator.Err()
}

func (s *MediaService) syncEpisodes(ctx context.Context) ([]int64, error) {
	watchlist, err := s.syncEpisodesFromWatchlist(ctx)
	if err != nil {
		return nil, fmt.Errorf("syncing watchlist: %w", err)
	}

	favorites, err := s.syncEpisodesFromFavorites(ctx)
	if err != nil {
		return nil, fmt.Errorf("syncing favorites: %w", err)
	}

	merged := append(watchlist, favorites...)
	if len(merged) == 0 {
		return nil, domain.ErrNoEpisodesFound
	}
	return merged, nil
}

func (s *MediaService) syncEpisodesFromWatchlist(ctx context.Context) ([]int64, error) {
	params := &trakt.ListWatchListParams{
		ListParams: trakt.ListParams{OAuth: s.token.AccessToken},
		Type:       typeShow,
	}
	iterator := sync.WatchList(params)

	return s.collectEpisodesFromWatchlist(ctx, iterator)
}

func (s *MediaService) collectEpisodesFromWatchlist(ctx context.Context, iterator *trakt.WatchListEntryIterator) ([]int64, error) {
	var episodeIDs []int64
	for iterator.Next() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		item, err := iterator.Entry()
		if err != nil {
			log.WithField("error", err).Error("failed to scan watchlist episode entry")
			continue
		}

		nextEp := s.getNextEpisode(item.Show)
		if nextEp != nil {
			if err := s.insertEpisode(ctx, item.Show, nextEp); err != nil {
				log.WithField("error", err).Error("failed to insert watchlist episode")
				continue
			}
			episodeIDs = append(episodeIDs, int64(nextEp.Trakt))
		}
	}

	return episodeIDs, iterator.Err()
}

func (s *MediaService) getNextEpisode(sh *trakt.Show) *trakt.Episode {
	params := &trakt.ProgressParams{
		Params: trakt.Params{OAuth: s.token.AccessToken},
	}
	progress, err := show.WatchedProgress(sh.Trakt, params)
	if err != nil {
		log.WithField("error", err).Error("failed to fetch show watch progress")
		return nil
	}
	return progress.NextEpisode
}

func (s *MediaService) syncEpisodesFromFavorites(ctx context.Context) ([]int64, error) {
	params := &trakt.ListFavoritesParams{
		ListParams: trakt.ListParams{OAuth: s.token.AccessToken},
		Type:       typeShows,
	}
	iterator := sync.Favorites(params)

	return s.collectEpisodesFromFavorites(ctx, iterator)
}

func (s *MediaService) collectEpisodesFromFavorites(ctx context.Context, iterator *trakt.FavoritesEntryIterator) ([]int64, error) {
	var episodeIDs []int64
	for iterator.Next() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		item, err := iterator.Entry()
		if err != nil {
			log.WithField("error", err).Error("failed to scan favorites episode entry")
			continue
		}

		ids := s.fetchNextEpisodes(ctx, item.Show)
		episodeIDs = append(episodeIDs, ids...)
	}

	return episodeIDs, iterator.Err()
}

func (s *MediaService) fetchNextEpisodes(ctx context.Context, sh *trakt.Show) []int64 {
	nextEp := s.getNextEpisode(sh)
	if nextEp == nil {
		return nil
	}

	var episodeIDs []int64
	for i := 0; i < s.cfg.NextEpisodesCount; i++ {
		if err := ctx.Err(); err != nil {
			return episodeIDs
		}

		ep := s.fetchEpisodeWithFallback(sh.Trakt, nextEp.Season, nextEp.Number+int64(i))
		if ep == nil {
			break
		}

		if err := s.insertEpisode(ctx, sh, ep); err != nil {
			log.WithField("error", err).Error("failed to insert favorites episode")
			continue
		}
		episodeIDs = append(episodeIDs, int64(ep.Trakt))
	}
	return episodeIDs
}

func (s *MediaService) fetchEpisodeWithFallback(showID trakt.SearchID, season, number int64) *trakt.Episode {
	ep, err := episode.Get(showID, season, number, nil)
	if err == nil {
		return ep
	}

	ep, err = episode.Get(showID, season+1, 1, nil)
	if err != nil {
		log.WithField("error", err).Info("no more episodes available for show")
		return nil
	}
	return ep
}

func (s *MediaService) insertEpisode(ctx context.Context, sh *trakt.Show, ep *trakt.Episode) error {
	if !isValidEpisode(sh, ep) {
		return nil
	}

	media := buildMediaFromEpisode(sh, ep)
	err := s.mediaRepo.Insert(ctx, media.TraktID, media)
	if err != nil && err != domain.ErrDuplicateKey {
		return fmt.Errorf("inserting episode: %w", err)
	}
	return nil
}

func isValidEpisode(sh *trakt.Show, ep *trakt.Episode) bool {
	return int64(ep.Trakt) > 0 && len(sh.IMDB) > 0 && ep.Number > 0 && ep.Season > 0
}

func buildMediaFromEpisode(sh *trakt.Show, ep *trakt.Episode) *domain.Media {
	return &domain.Media{
		TraktID: int64(ep.Trakt),
		Number:  ep.Number,
		Season:  ep.Season,
		IMDB:    string(sh.IMDB),
		Title:   ep.Title,
		Year:    sh.Year,
		OnDisk:  false,
	}
}

func (s *MediaService) removeStaleMedia(ctx context.Context, validIDs []int64) error {
	staleMedia, err := s.mediaRepo.FindNotInList(ctx, validIDs)
	if err != nil {
		return fmt.Errorf("finding stale media: %w", err)
	}

	for _, media := range staleMedia {
		if err := s.mediaRepo.Delete(ctx, media.TraktID); err != nil {
			log.WithFields(log.Fields{
				"traktID": media.TraktID,
				"error":   err,
			}).Error("failed to delete stale media from database")
		}
	}
	return nil
}
