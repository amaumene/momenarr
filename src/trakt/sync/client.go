package sync

import (
	"net/http"

	"github.com/jacklaaa89/trakt"
)

// client represents a sync client which gives us access to functions to sync
// trakt with one or more media centres.
type client struct{ b trakt.BaseClient }

// LastActivities returns all of the dates of the latest activity for a user.
//
// This method is a useful first step in the syncing process. We recommended caching these dates locally,
// then you can compare to know exactly what data has changed recently. This can greatly optimize your syncs
// so you don't pull down a ton of data only to see nothing has actually changed.
//
//   - OAuth Required
func LastActivities(params *trakt.Params) (*trakt.LastActivity, error) {
	return getC().LastActivities(params)
}

// LastActivities returns all of the dates of the latest activity for a user.
//
// This method is a useful first step in the syncing process. We recommended caching these dates locally,
// then you can compare to know exactly what data has changed recently. This can greatly optimize your syncs
// so you don't pull down a ton of data only to see nothing has actually changed.
//
//   - OAuth Required
func (c *client) LastActivities(params *trakt.Params) (*trakt.LastActivity, error) {
	l := &trakt.LastActivity{}
	err := c.b.Call(http.MethodGet, "/sync/last_activities", params, l)
	return l, err
}

// Playbacks returns a list of stored paused playbacks.
//
// Whenever a scrobble is paused, the playback progress is saved. Use this progress to sync up
// playback across different media centers or apps. For example, you can start watching a movie in a
// media center, stop it, then resume on your tablet from the same spot. Each item will have the progress
// percentage between 0 and 100.You can optionally specify a type to only get movies or episodes.
//
// By default, all results will be returned. However, you can send a limit if you only need a few recent
// results for something like an "on deck" feature.
//
//   - OAuth Required
func Playbacks(params *trakt.ListPlaybackParams) *trakt.PlaybackIterator {
	return getC().Playbacks(params)
}

// Playbacks returns a list of stored paused playbacks.
//
// Whenever a scrobble is paused, the playback progress is saved. Use this progress to sync up
// playback across different media centers or apps. For example, you can start watching a movie in a
// media center, stop it, then resume on your tablet from the same spot. Each item will have the progress
// percentage between 0 and 100.You can optionally specify a type to only get movies or episodes.
//
// By default, all results will be returned. However, you can send a limit if you only need a few recent
// results for something like an "on deck" feature.
//
//   - OAuth Required
func (c *client) Playbacks(params *trakt.ListPlaybackParams) *trakt.PlaybackIterator {
	path := trakt.FormatURLPath("/sync/playback/%s", params.Type)
	return &trakt.PlaybackIterator{BasicIterator: c.b.NewSimulatedIterator(http.MethodGet, path, params)}
}

// RemovePlaybacks removes a playback item from a user's playback progress list.
// an error with the code "ErrorCodeNotFound" will be returned if the playback was not found.
//
//   - OAuth Required
func RemovePlayback(id int64, params *trakt.RemovePlaybackParams) error {
	return getC().RemovePlayback(id, params)
}

// RemovePlaybacks removes a playback item from a user's playback progress list.
// an error with the code "ErrorCodeNotFound" will be returned if the playback was not found.
//
//   - OAuth Required
func (c *client) RemovePlayback(id int64, params *trakt.RemovePlaybackParams) error {
	path := trakt.FormatURLPath("/sync/playback/%s", params.Type)
	return c.b.Call(http.MethodDelete, path, params, nil)
}

// Collection returns all collected items in a user's collection. A collected item indicates availability to watch
// digitally or on physical media.
//
// Each movie object contains CollectedAt and UpdatedAt times. Since users can set custom dates when they
// collected movies, it is possible for CollectedAt to be in the past. We also include UpdatedAt to help
// sync Trakt data with your app. Cache this timestamp locally and only re-process the movie if
// you see a newer timestamp.
//
// Each show object contains LastCollected and LastUpdated times. Since users can set custom dates when they
// collected episodes, it is possible for LastCollected to be in the past. We also include LastUpdated to help
// sync Trakt data with your app. Cache this timestamp locally and only re-process the show if you see a
// newer timestamp.
//
// # Metadata
//
// If you set Extended to "ExtendedTypeMetadata", it will return the additional metadata. It will be nil if the
// metadata isn't set for an item.
//
//   - OAuth Required
//   - Extended Info
func Collection(params *trakt.ListCollectionParams) trakt.CollectionIterator {
	return getC().Collection(params)
}

// Collection returns all collected items in a user's collection. A collected item indicates availability to watch
// digitally or on physical media.
//
// Each movie object contains CollectedAt and UpdatedAt times. Since users can set custom dates when they
// collected movies, it is possible for CollectedAt to be in the past. We also include UpdatedAt to help
// sync Trakt data with your app. Cache this timestamp locally and only re-process the movie if
// you see a newer timestamp.
//
// Each show object contains LastCollected and LastUpdated times. Since users can set custom dates when they
// collected episodes, it is possible for LastCollected to be in the past. We also include LastUpdated to help
// sync Trakt data with your app. Cache this timestamp locally and only re-process the show if you see a
// newer timestamp.
//
// # Metadata
//
// If you set Extended to "ExtendedTypeMetadata", it will return the additional metadata. It will be nil if the
// metadata isn't set for an item.
//
//   - OAuth Required
//   - Extended Info
func (c *client) Collection(params *trakt.ListCollectionParams) trakt.CollectionIterator {
	if params.Type == trakt.TypeMovie {
		return c.movieCollection(params)
	}

	return c.showCollection(params)
}

// AddToCollection adds items to a user's collection. Accepts shows, seasons, episodes and movies. If only a show
// is passed, all episodes for the show will be collected. If seasons are specified, all episodes in those seasons
// will be collected.
//
// Set CollectedAt on any media entry to mark items as collected in the past. You can also send additional
// metadata about the media itself to have a very accurate collection. Showcase what is available to watch
// from your epic HD DVD collection down to your downloaded iTunes movies.
//
// Note: You can resend items already in your collection and they will be updated with any new values.
// This includes the CollectedAt value and any other metadata.
//
//   - OAuth Required
func AddToCollection(params *trakt.AddToCollectionParams) (*trakt.AddToCollectionResult, error) {
	return getC().AddToCollection(params)
}

// AddToCollection adds items to a user's collection. Accepts shows, seasons, episodes and movies. If only a show
// is passed, all episodes for the show will be collected. If seasons are specified, all episodes in those seasons
// will be collected.
//
// Set CollectedAt on any media entry to mark items as collected in the past. You can also send additional
// metadata about the media itself to have a very accurate collection. Showcase what is available to watch
// from your epic HD DVD collection down to your downloaded iTunes movies.
//
// Note: You can resend items already in your collection and they will be updated with any new values.
// This includes the CollectedAt value and any other metadata.
//
//   - OAuth Required
func (c *client) AddToCollection(params *trakt.AddToCollectionParams) (*trakt.AddToCollectionResult, error) {
	rcv := &trakt.AddToCollectionResult{}
	err := c.b.Call(http.MethodPost, "/sync/collection", params, &rcv)
	return rcv, err
}

// RemoveFromCollection removes one or more items from a user's collection.
//
//   - OAuth Required
func RemoveFromCollection(params *trakt.RemoveFromCollectionParams) (*trakt.RemoveFromCollectionResult, error) {
	return getC().RemoveFromCollection(params)
}

// RemoveFromCollection removes one or more items from a user's collection.
//
//   - OAuth Required
func (c *client) RemoveFromCollection(params *trakt.RemoveFromCollectionParams) (*trakt.RemoveFromCollectionResult, error) {
	rcv := &trakt.RemoveFromCollectionResult{}
	err := c.b.Call(http.MethodPost, "/sync/collection/remove", params, &rcv)
	return rcv, err
}

// Watched returns all movies or shows a user has watched sorted by most plays. If type is set to shows
// and you set Extended to "ExtendedTypeNoSeasons" it won't return season or episode info.
//
// Each movie and show object contains LastWatched and LastUpdated times. Since users can set custom dates
// when they watched movies and episodes, it is possible for LastWatched to be in the past. We also include
// LastUpdated to help sync Trakt data with your app. Cache this timestamp locally and only re-process
// the movies and shows if you see a newer timestamp.
//
// Each show object contains a ResetAt timestamp. If not null, this is when the user started re-watching the show.
// Your app can adjust the progress by ignoring episodes with a LastWatched prior to the ResetAt.
//
//   - OAuth Required
//   - Extended Info
func Watched(params *trakt.ListCollectionParams) trakt.WatchedIterator { return getC().Watched(params) }

// Watched returns all movies or shows a user has watched sorted by most plays. If type is set to shows
// and you set Extended to "ExtendedTypeNoSeasons" it won't return season or episode info.
//
// Each movie and show object contains LastWatched and LastUpdated times. Since users can set custom dates
// when they watched movies and episodes, it is possible for LastWatched to be in the past. We also include
// LastUpdated to help sync Trakt data with your app. Cache this timestamp locally and only re-process
// the movies and shows if you see a newer timestamp.
//
// Each show object contains a ResetAt timestamp. If not null, this is when the user started re-watching the show.
// Your app can adjust the progress by ignoring episodes with a LastWatched prior to the ResetAt.
//
//   - OAuth Required
//   - Extended Info
func (c *client) Watched(params *trakt.ListCollectionParams) trakt.WatchedIterator {
	if params.Type == trakt.TypeMovie {
		return c.watchedMovies(params)
	}

	return c.watchedShows(params)
}

// History returns movies and episodes that a user has watched, sorted by most recent. You can optionally limit
// the type to movies or episodes. The id (64-bit integer) in each history item uniquely identifies the
// event and can be used to remove individual events by using the RemoveFromHistory method. The action will be set to
// scrobble, checkin, or watch.
//
// Specify a type and trakt id to limit the history for just that item. If the id is valid, but there is no history,
// an empty array will be returned.
//
//   - OAuth Required
//   - Pagination
//   - Extended Info
func History(params *trakt.ListHistoryParams) *trakt.HistoryIterator {
	return getC().History(params)
}

// History returns movies and episodes that a user has watched, sorted by most recent. You can optionally limit
// the type to movies or episodes. The id (64-bit integer) in each history item uniquely identifies the
// event and can be used to remove individual events by using the RemoveFromHistory method. The action will be set to
// scrobble, checkin, or watch.
//
// Specify a type and trakt id to limit the history for just that item. If the id is valid, but there is no history,
// an empty array will be returned.
//
//   - OAuth Required
//   - Pagination
//   - Extended Info
func (c *client) History(params *trakt.ListHistoryParams) *trakt.HistoryIterator {
	path := trakt.FormatURLPath("/sync/history/%s/%s", params.Type, params.ID)
	return &trakt.HistoryIterator{Iterator: c.b.NewIterator(http.MethodGet, path, params)}
}

// AddToHistory adds items to a user's watch history. Accepts shows, seasons, episodes and movies.
// If only a show is passed, all episodes for the show will be added. If seasons are specified, only episodes
// in those seasons will be added.
//
// Supply WatchedAt to mark items as watched in the past. This is useful for syncing past
// watches from a media center.
//
//   - OAuth Required
func AddToHistory(params *trakt.AddToHistoryParams) (*trakt.AddToHistoryResult, error) {
	return getC().AddToHistory(params)
}

// AddToHistory adds items to a user's watch history. Accepts shows, seasons, episodes and movies.
// If only a show is passed, all episodes for the show will be added. If seasons are specified, only episodes
// in those seasons will be added.
//
// Supply WatchedAt to mark items as watched in the past. This is useful for syncing past
// watches from a media center.
//
//   - OAuth Required
func (c *client) AddToHistory(params *trakt.AddToHistoryParams) (*trakt.AddToHistoryResult, error) {
	rcv := &trakt.AddToHistoryResult{}
	err := c.b.Call(http.MethodPost, "/sync/history", params, &rcv)
	return rcv, err
}

// RemoveFromHistory removes items from a user's watch history including all watches, scrobbles, and checkins.
// Accepts shows, seasons, episodes and movies. If only a show is passed, all episodes for the show will be removed.
// If seasons are specified, only episodes in those seasons will be removed.
//
// You can also send a list of raw history ids (64-bit integers) to delete single plays from the watched history.
// The "History" method will return an individual id (64-bit integer) for each history item.
//
//   - OAuth Required
func RemoveFromHistory(params *trakt.RemoveFromHistoryParams) (*trakt.RemoveFromHistoryResult, error) {
	return getC().RemoveFromHistory(params)
}

// RemoveFromHistory removes items from a user's watch history including all watches, scrobbles, and checkins.
// Accepts shows, seasons, episodes and movies. If only a show is passed, all episodes for the show will be removed.
// If seasons are specified, only episodes in those seasons will be removed.
//
// You can also send a list of raw history ids (64-bit integers) to delete single plays from the watched history.
// The "History" method will return an individual id (64-bit integer) for each history item.
//
//   - OAuth Required
func (c *client) RemoveFromHistory(params *trakt.RemoveFromHistoryParams) (*trakt.RemoveFromHistoryResult, error) {
	rcv := &trakt.RemoveFromHistoryResult{}
	err := c.b.Call(http.MethodPost, "/sync/history/remove", params, &rcv)
	return rcv, err
}

// Ratings returns a user's ratings filtered by type. You can optionally filter for a specific rating
// between 1 and 10. Send a comma separated string for rating if you need multiple ratings.
//
//   - OAuth Required
//   - Pagination
//   - Extended Info
func Ratings(params *trakt.ListRatingParams) *trakt.RatingIterator {
	return getC().Ratings(params)
}

// Ratings returns a user's ratings filtered by type. You can optionally filter for a specific rating
// between 1 and 10. Send a comma separated string for rating if you need multiple ratings.
//
//   - OAuth Required
//   - Pagination
//   - Extended Info
func (c *client) Ratings(params *trakt.ListRatingParams) *trakt.RatingIterator {
	path := trakt.FormatURLPath("/sync/ratings/%s/%s", params.Type.Plural(), params.Ratings)
	return &trakt.RatingIterator{Iterator: c.b.NewIterator(http.MethodGet, path, params)}
}

// AddRatings rates one or more items. Accepts shows, seasons, episodes and movies. If only a show is passed,
// only the show itself will be rated. If seasons are specified, all of those seasons will be rated.
// Send a RatedAt time to mark items as rated in the past. This is useful for syncing ratings from a media center.
//
//   - OAuth Required
func AddRatings(params *trakt.AddRatingsParams) (*trakt.AddRatingsResult, error) {
	return getC().AddRatings(params)
}

// AddRatings rates one or more items. Accepts shows, seasons, episodes and movies. If only a show is passed,
// only the show itself will be rated. If seasons are specified, all of those seasons will be rated.
// Send a RatedAt time to mark items as rated in the past. This is useful for syncing ratings from a media center.
//
//   - OAuth Required
func (c *client) AddRatings(params *trakt.AddRatingsParams) (*trakt.AddRatingsResult, error) {
	rcv := &trakt.AddRatingsResult{}
	err := c.b.Call(http.MethodPost, "/sync/ratings", params, &rcv)
	return rcv, err
}

// RemoveRatings removes ratings for one or more items.
//
//   - OAuth Required
func RemoveRatings(params *trakt.RemoveRatingsParams) (*trakt.RemoveRatingsResult, error) {
	return getC().RemoveRatings(params)
}

// RemoveRatings removes ratings for one or more items.
//
//   - OAuth Required
func (c *client) RemoveRatings(params *trakt.RemoveRatingsParams) (*trakt.RemoveRatingsResult, error) {
	rcv := &trakt.RemoveRatingsResult{}
	err := c.b.Call(http.MethodPost, "/sync/ratings/remove", params, &rcv)
	return rcv, err
}

// WatchList returns all items in a user's watchlist filtered by type.
//
// # Sorting
//
// By default, all list items are sorted by rank asc. You can call the "Applied" function on the iterator to
// indicate how the results are actually being sorted.
// You can call the "Preferred" function on the iterator to retrieve the user's sort preference. Use these to
// perform a custom sort on the watchlist in your app for more advanced sort abilities we can't do in the API.
//
//   - OAuth Required
//   - Pagination
//   - Extended Info
func WatchList(params *trakt.ListWatchListParams) *trakt.WatchListEntryIterator {
	return getC().WatchList(params)
}

// WatchList returns all items in a user's watchlist filtered by type.
//
// # Sorting
//
// By default, all list items are sorted by rank asc. You can call the "Applied" function on the iterator to
// indicate how the results are actually being sorted.
// You can call the "Preferred" function on the iterator to retrieve the user's sort preference. Use these to
// perform a custom sort on the watchlist in your app for more advanced sort abilities we can't do in the API.
//
//   - OAuth Required
//   - Pagination
//   - Extended Info
func (c *client) WatchList(params *trakt.ListWatchListParams) *trakt.WatchListEntryIterator {
	path := trakt.FormatURLPath("/sync/watchlist/%s/%s", params.Type.Plural(), params.Sort)
	return &trakt.WatchListEntryIterator{Iterator: c.b.NewIterator(http.MethodGet, path, params)}
}

// AddToWatchList adds one of more items to a user's watchlist. Accepts shows, seasons, episodes and movies.
// If only a show is passed, only the show itself will be added. If seasons are specified, all of those
// seasons will be added.
//
//   - OAuth Required
func AddToWatchList(params *trakt.AddToWatchListParams) (*trakt.AddToWatchListResult, error) {
	return getC().AddToWatchList(params)
}

// AddToWatchList adds one of more items to a user's watchlist. Accepts shows, seasons, episodes and movies.
// If only a show is passed, only the show itself will be added. If seasons are specified, all of those
// seasons will be added.
//
//   - OAuth Required
func (c *client) AddToWatchList(params *trakt.AddToWatchListParams) (*trakt.AddToWatchListResult, error) {
	rcv := &trakt.AddToWatchListResult{}
	err := c.b.Call(http.MethodPost, "/sync/watchlist", params, &rcv)
	return rcv, err
}

// RemoveFromWatchList removes one or more items from a user's watchlist.
//
//   - OAuth Required
func RemoveFromWatchList(params *trakt.RemoveFromWatchListParams) (*trakt.RemoveFromWatchListResult, error) {
	return getC().RemoveFromWatchList(params)
}

// RemoveFromWatchList removes one or more items from a user's watchlist.
//
//   - OAuth Required
func (c *client) RemoveFromWatchList(params *trakt.RemoveFromWatchListParams) (*trakt.RemoveFromWatchListResult, error) {
	rcv := &trakt.RemoveFromWatchListResult{}
	err := c.b.Call(http.MethodPost, "/sync/watchlist/remove", params, &rcv)
	return rcv, err
}

// movieCollection generates an iterator for collected movies.
func (c *client) movieCollection(params *trakt.ListCollectionParams) *collection {
	return c.newCollectionIterator(trakt.TypeMovie, params)
}

// showCollection generates an iterator for collected shows.
func (c *client) showCollection(params *trakt.ListCollectionParams) *collection {
	return c.newCollectionIterator(trakt.TypeShow, params)
}

// newWatchedIterator generates an iterator for either watched shows or movies
// based on the type.
func (c *client) newCollectionIterator(t trakt.Type, p *trakt.ListCollectionParams) *collection {
	path := trakt.FormatURLPath("/sync/collection/%s", t.Plural())
	return &collection{
		genericIterator: genericIterator{
			BasicIterator: c.b.NewSimulatedIteratorWithCondition(
				http.MethodGet, path, p, func() error {
					return compareType(path, p.Type, t)
				},
			),
			typ: p.Type,
		},
	}
}

// watchedMovies generates an iterator for watched movies.
func (c *client) watchedMovies(params *trakt.ListWatchedParams) *watched {
	return c.newWatchedIterator(trakt.TypeMovie, params)
}

// watchedShows generates an iterator for watched shows.
func (c *client) watchedShows(params *trakt.ListWatchedParams) *watched {
	return c.newWatchedIterator(trakt.TypeShow, params)
}

// newWatchedIterator generates an iterator for either watched shows or movies
// based on the type.
func (c *client) newWatchedIterator(t trakt.Type, p *trakt.ListWatchedParams) *watched {
	path := trakt.FormatURLPath("/sync/watched/%s", t.Plural())
	return &watched{
		genericIterator: genericIterator{
			BasicIterator: c.b.NewSimulatedIteratorWithCondition(
				http.MethodGet, path, p, func() error {
					return compareType(path, p.Type, t)
				},
			),
			typ: p.Type,
		},
	}
}

// genericIterator this is a generic iterator for listing
// both watched and collected movies and shows.
type genericIterator struct {
	trakt.BasicIterator

	// typ the type of object which this iterator represents
	// can either be show or movie.
	typ trakt.Type
}

// Type implements both WatchedIterator and CollectionIterator interfaces.
// returns the type so the user knows which entry to use.
func (g *genericIterator) Type() trakt.Type { return g.typ }

// collection an implementation of a CollectionIterator
// uses the internal iterator defined on the genericIterator
// to attempt to scan and cast to either a collected show or movie
// based on the type.
type collection struct{ genericIterator }

// Show implements CollectionIterator interface.
func (c *collection) Show() (*trakt.CollectedShow, error) {
	rcv := &trakt.CollectedShow{}
	return rcv, c.Scan(rcv)
}

// Movie implements CollectionIterator interface.
func (c *collection) Movie() (*trakt.CollectedMovie, error) {
	rcv := &trakt.CollectedMovie{}
	return rcv, c.Scan(rcv)
}

// watched an implementation of a WatchedIterator
// uses the internal iterator defined on the genericIterator
// to attempt to scan and cast to either a watched show or movie
// based on the type.
type watched struct{ genericIterator }

// Show implements WatchedIterator interface.
func (c *watched) Show() (*trakt.WatchedShow, error) {
	rcv := &trakt.WatchedShow{}
	return rcv, c.Scan(rcv)
}

// Movie implements WatchedIterator interface.
func (c *watched) Movie() (*trakt.WatchedMovie, error) {
	rcv := &trakt.WatchedMovie{}
	return rcv, c.Scan(rcv)
}

// compareType helper function to compare to types to see if they are equal.
// the path is required to generate the standard error signature if the types do not
// match.
func compareType(path string, a, b trakt.Type) error {
	if a == b {
		return nil
	}

	return &trakt.Error{
		HTTPStatusCode: http.StatusUnprocessableEntity,
		Body:           "invalid type: only movie / show are applicable",
		Resource:       path,
		Code:           trakt.ErrorCodeValidationError,
	}
}

// getC initialises a new sync client using the currently configured backend.
func getC() *client { return &client{trakt.NewClient()} }
