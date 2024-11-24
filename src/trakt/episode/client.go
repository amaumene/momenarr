// Package episode contains functions to retrieve episode details.
package episode

import (
	"net/http"

	"github.com/amaumene/momenarr/trakt"
)

// client is a representation of a episode client, capable of
// retrieving information on specific show episodes.
type client struct{ b trakt.BaseClient }

// Get returns a single episode's details. All date and times are in UTC and were calculated using the
// episode's air_date and show's country and air_time.
//
// Note: If the first_aired is unknown, it will be set to null.
//
//   - Extended Info
func Get(id trakt.SearchID, season, episode int64, params *trakt.ExtendedParams) (*trakt.Episode, error) {
	return getC().Get(id, season, episode, params)
}

// Get returns a single episode's details. All date and times are in UTC and were calculated using the
// episode's air_date and show's country and air_time.
//
// Note: If the first_aired is unknown, it will be set to null.
//
//   - Extended Info
func (c *client) Get(id trakt.SearchID, season, episode int64, params *trakt.ExtendedParams) (*trakt.Episode, error) {
	path := trakt.FormatURLPath("/shows/%s/seasons/%s/episodes/%s", id, season, episode)
	ep := &trakt.Episode{}
	err := c.b.Call(http.MethodGet, path, params, ep)
	return ep, err
}

// Translations returns all translations for an episode, including language and translated
// values for title and overview.
func Translations(
	id trakt.SearchID,
	season, episode int64,
	params *trakt.TranslationListParams,
) *trakt.TranslationIterator {

	return getC().Translations(id, season, episode, params)
}

// Translations returns all translations for an episode, including language and translated
// values for title and overview.
func (c *client) Translations(
	id trakt.SearchID,
	season, episode int64,
	params *trakt.TranslationListParams,
) *trakt.TranslationIterator {

	path := trakt.FormatURLPath(
		"/shows/%s/seasons/%s/episodes/%s/translations/%s", id, season, episode, params.Language,
	)
	return &trakt.TranslationIterator{BasicIterator: c.b.NewSimulatedIterator(http.MethodGet, path, params)}
}

// Comments Returns all top level comments for an episode. By default, the newest comments are returned first.
// Other sorting options include oldest, most likes, most replies, highest rated, lowest rated, and most plays.
//
//   - Pagination
func Comments(id trakt.SearchID, season, episode int64, params *trakt.CommentListParams) *trakt.CommentIterator {
	return getC().Comments(id, season, episode, params)
}

// Comments Returns all top level comments for an episode. By default, the newest comments are returned first.
// Other sorting options include oldest, most likes, most replies, highest rated, lowest rated, and most plays.
//
//   - Pagination
func (c *client) Comments(
	id trakt.SearchID,
	season, episode int64,
	params *trakt.CommentListParams,
) *trakt.CommentIterator {

	path := trakt.FormatURLPath(
		"shows/%s/seasons/%s/episodes/%s/comments/%s",
		id, season, episode, params.Sort,
	)
	return &trakt.CommentIterator{Iterator: c.b.NewIterator(http.MethodGet, path, params)}
}

// Lists returns all lists that contain this episode. By default, personal lists are returned
// sorted by the most popular.
//
//   - Pagination
func Lists(id trakt.SearchID, season, episode int64, params *trakt.GetListParams) *trakt.ListIterator {
	return getC().Lists(id, season, episode, params)
}

// Lists returns all lists that contain this episode. By default, personal lists are returned
// sorted by the most popular.
//
//   - Pagination
func (c *client) Lists(id trakt.SearchID, season, episode int64, params *trakt.GetListParams) *trakt.ListIterator {
	path := trakt.FormatURLPath(
		"/shows/%s/seasons/%s/episodes/%s/lists/%s/%s",
		id, season, episode, params.ListType, params.SortType,
	)
	return &trakt.ListIterator{Iterator: c.b.NewIterator(http.MethodGet, path, params)}
}

// People returns all cast and crew for an episode.
//
// Each cast member will have a characters array and a standard person object.
//
// The crew object will be broken up into production, art, crew, costume & make-up, directing, writing, sound,
// camera, visual effects, lighting, and editing (if there are people for those crew positions). Each of those
// members will have a jobs array and a standard person object.
//
// # Guest Stars
//
// If you use the "ExtendedTypeGuestStars" extended type, it will return all guest stars that appeared in the episode.
//
// Note: This returns a lot of data, so please only use this extended parameter if you actually need it!
//
//   - Extended Info
func People(id trakt.SearchID, season, episode int64, params *trakt.ExtendedParams) (*trakt.CastAndCrew, error) {
	return getC().People(id, season, episode, params)
}

// People returns all cast and crew for an episode.
//
// Each cast member will have a characters array and a standard person object.
//
// The crew object will be broken up into production, art, crew, costume & make-up, directing, writing, sound,
// camera, visual effects, lighting, and editing (if there are people for those crew positions). Each of those
// members will have a jobs array and a standard person object.
//
// # Guest Stars
//
// If you use the "ExtendedTypeGuestStars" extended type, it will return all guest stars that appeared in the episode.
//
// Note: This returns a lot of data, so please only use this extended parameter if you actually need it!
//
//   - Extended Info
func (c *client) People(
	id trakt.SearchID,
	season, episode int64,
	params *trakt.ExtendedParams,
) (*trakt.CastAndCrew, error) {

	path := trakt.FormatURLPath("/shows/%s/seasons/%s/episodes/%s/people", id, season, episode)
	cc := &trakt.CastAndCrew{}
	err := c.b.Call(http.MethodGet, path, params, cc)
	return cc, err
}

// Ratings returns the rating (between 0 and 10) and distribution for an episode.
func Ratings(id trakt.SearchID, season, episode int64, params *trakt.BasicParams) (*trakt.RatingDistribution, error) {
	return getC().Ratings(id, season, episode, params)
}

// Ratings returns the rating (between 0 and 10) and distribution for an episode.
func (c *client) Ratings(
	id trakt.SearchID,
	season, episode int64,
	params *trakt.BasicParams,
) (*trakt.RatingDistribution, error) {

	path := trakt.FormatURLPath("/shows/%s/seasons/%s/episodes/%s/ratings", id, season, episode)
	r := &trakt.RatingDistribution{}
	err := c.b.Call(http.MethodGet, path, params, r)
	return r, err
}

// Statistics returns lots of episode stats.
func Statistics(id trakt.SearchID, season, episode int64, params *trakt.BasicParams) (*trakt.Statistics, error) {
	return getC().Statistics(id, season, episode, params)
}

// Statistics returns lots of episode stats.
func (c *client) Statistics(
	id trakt.SearchID,
	season, episode int64,
	params *trakt.BasicParams,
) (*trakt.Statistics, error) {

	path := trakt.FormatURLPath("/shows/%s/seasons/%s/episodes/%s/stats", id, season, episode)
	stats := &trakt.Statistics{}
	err := c.b.Call(http.MethodGet, path, params, stats)
	return stats, err
}

// WatchingNow returns all users watching this episode right now.
//
//   - Extended Info
func WatchingNow(id trakt.SearchID, season, episode int64, params *trakt.BasicListParams) *trakt.UserIterator {
	return getC().WatchingNow(id, season, episode, params)
}

// WatchingNow returns all users watching this episode right now.
//
//   - Extended Info
func (c *client) WatchingNow(
	id trakt.SearchID,
	season, episode int64,
	params *trakt.BasicListParams,
) *trakt.UserIterator {

	path := trakt.FormatURLPath("/shows/%s/seasons/%s/episodes/%s/watching", id, season, episode)
	return &trakt.UserIterator{Iterator: c.b.NewSimulatedIterator(http.MethodGet, path, params)}
}

// getC initialises a new episode client with the current backend configuration.
func getC() *client { return &client{trakt.NewClient()} }
