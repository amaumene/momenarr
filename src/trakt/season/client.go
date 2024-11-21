// Package season contains functions to retrieve season details.
package season

import (
	"net/http"

	"github.com/jacklaaa89/trakt"
)

// client represents a season client.
type client struct{ b trakt.BaseClient }

// Episodes returns all episodes for a specific season of a show.
//
// # Translations
//
// If you'd like to included translated episode titles and overviews in the response then
// supply a specific 2 digit country language code or ALL to retrieve all translations.
//
// Note: This returns a lot of data, so please only use this parameter if you actually need it!
//
//   - Extended Info
func Episodes(id trakt.SearchID, season int64, p *trakt.EpisodeListParams) *trakt.EpisodeWithTranslationsIterator {
	return getC().Episodes(id, season, p)
}

// Episodes returns all episodes for a specific season of a show.
//
// # Translations
//
// If you'd like to included translated episode titles and overviews in the response then
// supply a specific 2 digit country language code or ALL to retrieve all translations.
//
// Note: This returns a lot of data, so please only use this parameter if you actually need it!
//
//   - Extended Info
func (c *client) Episodes(
	id trakt.SearchID,
	season int64,
	params *trakt.EpisodeListParams,
) *trakt.EpisodeWithTranslationsIterator {

	path := trakt.FormatURLPath("/shows/%s/seasons/%s", id, season)
	return &trakt.EpisodeWithTranslationsIterator{
		BasicIterator: c.b.NewSimulatedIterator(http.MethodGet, path, params),
	}
}

// Comments returns all top level comments for a season. By default, the newest comments are returned first.
// Other sorting options include oldest, most likes, most replies, highest rated, lowest rated, most plays,
// and highest watched percentage.
//
//   - Pagination
func Comments(id trakt.SearchID, season int64, params *trakt.CommentListParams) *trakt.CommentIterator {
	return getC().Comments(id, season, params)
}

// Comments returns all top level comments for a season. By default, the newest comments are returned first.
// Other sorting options include oldest, most likes, most replies, highest rated, lowest rated, most plays,
// and highest watched percentage.
//
//   - Pagination
func (c *client) Comments(id trakt.SearchID, season int64, params *trakt.CommentListParams) *trakt.CommentIterator {
	path := trakt.FormatURLPath("shows/%s/seasons/%s/comments/%s", id, season, params.Sort)
	return &trakt.CommentIterator{Iterator: c.b.NewIterator(http.MethodGet, path, params)}
}

// Lists returns all lists that contain this season. By default, personal lists are returned sorted
// by the most popular.
//
//   - Pagination
func Lists(id trakt.SearchID, season int64, params *trakt.GetListParams) *trakt.ListIterator {
	return getC().Lists(id, season, params)
}

// Lists returns all lists that contain this season. By default, personal lists are returned sorted
// by the most popular.
//
//   - Pagination
func (c *client) Lists(id trakt.SearchID, season int64, params *trakt.GetListParams) *trakt.ListIterator {
	path := trakt.FormatURLPath(
		"/shows/%s/seasons/%s/lists/%s/%s", id, season, params.ListType, params.SortType,
	)
	return &trakt.ListIterator{Iterator: c.b.NewIterator(http.MethodGet, path, params)}
}

// People returns all cast and crew for an season.
//
// Each cast member will have a characters array and a standard person object.
//
// The crew object will be broken up into production, art, crew, costume & make-up, directing, writing, sound,
// camera, visual effects, lighting, and editing (if there are people for those crew positions). Each of those
// members will have a jobs array and a standard person object.
//
// # Guest Stars
//
// If you use the `ExtendedTypeGuestStars` extended type, it will return all guest stars that appeared in each episode
// in the requested season.
//
// Note: This returns a lot of data, so please only use this extended parameter if you actually need it!
//
//   - Extended Info
func People(id trakt.SearchID, season int64, params *trakt.ExtendedParams) (*trakt.CastAndCrew, error) {
	return getC().People(id, season, params)
}

// People returns all cast and crew for an season.
//
// Each cast member will have a characters array and a standard person object.
//
// The crew object will be broken up into production, art, crew, costume & make-up, directing, writing, sound,
// camera, visual effects, lighting, and editing (if there are people for those crew positions). Each of those
// members will have a jobs array and a standard person object.
//
// # Guest Stars
//
// If you use the `ExtendedTypeGuestStars` extended type, it will return all guest stars that appeared in each episode
// in the requested season.
//
// Note: This returns a lot of data, so please only use this extended parameter if you actually need it!
//
//   - Extended Info
func (c *client) People(id trakt.SearchID, season int64, params *trakt.ExtendedParams) (*trakt.CastAndCrew, error) {
	path := trakt.FormatURLPath("/shows/%s/seasons/%s/people", id, season)
	cc := &trakt.CastAndCrew{}
	err := c.b.Call(http.MethodGet, path, params, cc)
	return cc, err
}

// Ratings returns the rating (between 0 and 10) and distribution for a season.
func Ratings(id trakt.SearchID, season int64, params *trakt.BasicParams) (*trakt.RatingDistribution, error) {
	return getC().Ratings(id, season, params)
}

// Ratings returns the rating (between 0 and 10) and distribution for a season.
func (c *client) Ratings(id trakt.SearchID, season int64, p *trakt.BasicParams) (*trakt.RatingDistribution, error) {
	path := trakt.FormatURLPath("/shows/%s/seasons/%s/ratings", id, season)
	r := &trakt.RatingDistribution{}
	err := c.b.Call(http.MethodGet, path, p, r)
	return r, err
}

// Statistics returns lots of season stats.
func Statistics(id trakt.SearchID, season int64, params *trakt.BasicParams) (*trakt.Statistics, error) {
	return getC().Statistics(id, season, params)
}

// Statistics returns lots of season stats.
func (c *client) Statistics(id trakt.SearchID, season int64, params *trakt.BasicParams) (*trakt.Statistics, error) {
	path := trakt.FormatURLPath("/shows/%s/seasons/%s/stats", id, season)
	stats := &trakt.Statistics{}
	err := c.b.Call(http.MethodGet, path, params, stats)
	return stats, err
}

// WatchingNow returns all users watching this season right now.
//
//   - Extended Info
func WatchingNow(id trakt.SearchID, season int64, params *trakt.BasicListParams) *trakt.UserIterator {
	return getC().WatchingNow(id, season, params)
}

// WatchingNow returns all users watching this season right now.
//
//   - Extended Info
func (c *client) WatchingNow(id trakt.SearchID, season int64, params *trakt.BasicListParams) *trakt.UserIterator {
	path := trakt.FormatURLPath("/shows/%s/seasons/%s/watching", id, season)
	return &trakt.UserIterator{Iterator: c.b.NewIterator(http.MethodGet, path, params)}
}

// getC initialises a new season client with the currently defined backend configuration.
func getC() *client { return &client{trakt.NewClient()} }
