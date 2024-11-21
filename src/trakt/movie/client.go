// Package movie contains functions to retrieve movie details
// either listing the most popular ones or a single one by ID.
//
// It also has functions to retrieve translations, release
// types and name aliases for movies.
package movie

import (
	"net/http"

	"github.com/jacklaaa89/trakt"
)

// client represents a movie client.
type client struct{ b trakt.BaseClient }

// Trending returns all movies being watched right now. Movies with the most users are returned first.
//
//   - Pagination
//   - Filters
//   - Extended Info
func Trending(params *trakt.FilterListParams) *trakt.TrendingMovieIterator {
	return getC().Trending(params)
}

// Trending returns all movies being watched right now. Movies with the most users are returned first.
//
//   - Pagination
//   - Filters
//   - Extended Info
func (c *client) Trending(params *trakt.FilterListParams) *trakt.TrendingMovieIterator {
	return &trakt.TrendingMovieIterator{Iterator: c.b.NewIterator(http.MethodGet, "/movies/trending", params)}
}

// Popular returns the most popular movies. Popularity is calculated using the rating
// percentage and the number of ratings.
//
//   - Pagination
//   - Filters
//   - Extended Info
func Popular(params *trakt.FilterListParams) *trakt.MovieIterator {
	return getC().Popular(params)
}

// Popular returns the most popular movies. Popularity is calculated using the rating
// percentage and the number of ratings.
//
//   - Pagination
//   - Filters
//   - Extended Info
func (c *client) Popular(params *trakt.FilterListParams) *trakt.MovieIterator {
	return &trakt.MovieIterator{Iterator: c.b.NewIterator(http.MethodGet, "/movies/popular", params)}
}

// Played returns the most played (a single user can watch multiple times) movies in the specified time
// period.
// All stats are relative to the specific time period. The time period is defaulted to WEEKLY if not supplied.
//
//   - Pagination
//   - Filters
//   - Extended Info
func Played(params *trakt.TimePeriodListParams) *trakt.MovieWithStatisticsIterator {
	return getC().Played(params)
}

// Played returns the most played (a single user can watch multiple times) movies in the specified time
// period.
// All stats are relative to the specific time period. The time period is defaulted to WEEKLY if not supplied.
//
//   - Pagination
//   - Filters
//   - Extended Info
func (c *client) Played(params *trakt.TimePeriodListParams) *trakt.MovieWithStatisticsIterator {
	return c.newTimePeriodIterator("played", params)
}

// Watched returns the most watched (unique users) movies in the specified time period,
// defaulting to WEEKLY. All stats are relative to the specific time period.
//
//   - Pagination
//   - Filters
//   - Extended Info
func Watched(params *trakt.TimePeriodListParams) *trakt.MovieWithStatisticsIterator {
	return getC().Watched(params)
}

// Watched returns the most watched (unique users) movies in the specified time period,
// defaulting to WEEKLY. All stats are relative to the specific time period.
//
//   - Pagination
//   - Filters
//   - Extended Info
func (c *client) Watched(params *trakt.TimePeriodListParams) *trakt.MovieWithStatisticsIterator {
	return c.newTimePeriodIterator("watched", params)
}

// Collected returns the most collected (unique users) movies in the specified time period,
// defaulting to weekly. All stats are relative to the specific time period.
//
//   - Pagination
//   - Filters
//   - Extended Info
func Collected(params *trakt.TimePeriodListParams) *trakt.MovieWithStatisticsIterator {
	return getC().Collected(params)
}

// Collected returns the most collected (unique users) movies in the specified time period,
// defaulting to weekly. All stats are relative to the specific time period.
//
//   - Pagination
//   - Filters
//   - Extended Info
func (c *client) Collected(params *trakt.TimePeriodListParams) *trakt.MovieWithStatisticsIterator {
	return c.newTimePeriodIterator("collected", params)
}

// Anticipated returns the most anticipated movies based on the number of lists a movie appears on.
//
//   - Pagination
//   - Filters
//   - Extended Info
func Anticipated(params *trakt.FilterListParams) *trakt.AnticipatedMovieIterator {
	return getC().Anticipated(params)
}

// Anticipated returns the most anticipated movies based on the number of lists a movie appears on.
//
//   - Pagination
//   - Filters
//   - Extended Info
func (c *client) Anticipated(params *trakt.FilterListParams) *trakt.AnticipatedMovieIterator {
	return &trakt.AnticipatedMovieIterator{Iterator: c.b.NewIterator(http.MethodGet, "/movies/anticipated", params)}
}

// BoxOffice returns the top 10 grossing movies in the U.S. box office last weekend. Updated every Monday morning.
//
//   - Extended Info
func BoxOffice(params *trakt.BoxOfficeListParams) *trakt.BoxOfficeMovieIterator {
	return getC().BoxOffice(params)
}

// BoxOffice returns the top 10 grossing movies in the U.S. box office last weekend. Updated every Monday morning.
//
//   - Extended Info
func (c *client) BoxOffice(params *trakt.BoxOfficeListParams) *trakt.BoxOfficeMovieIterator {
	return &trakt.BoxOfficeMovieIterator{
		BasicIterator: c.b.NewSimulatedIterator(http.MethodGet, "/movies/boxoffice", params),
	}
}

// RecentlyUpdated returns all movies updated since the specified UTC date. We recommended storing the date
// you can be efficient using this method moving forward.
//
// By default, 10 results are returned. You can send a limit to get up to 100 results per page.
//
//   - Pagination
//   - Extended Info
func RecentlyUpdated(params *trakt.RecentlyUpdatedListParams) *trakt.RecentlyUpdatedMovieIterator {
	return getC().RecentlyUpdated(params)
}

// RecentlyUpdated returns all movies updated since the specified UTC date. We recommended storing the date
// you can be efficient using this method moving forward.
//
// By default, 10 results are returned. You can send a limit to get up to 100 results per page.
//
//   - Pagination
//   - Extended Info
func (c *client) RecentlyUpdated(params *trakt.RecentlyUpdatedListParams) *trakt.RecentlyUpdatedMovieIterator {
	path := trakt.FormatURLPath("/movies/updates/%s", params.StartDate.Format(`2006-01-02`))
	return &trakt.RecentlyUpdatedMovieIterator{Iterator: c.b.NewIterator(http.MethodGet, path, params)}
}

// Get returns a single movie's details.
//
//   - Extended Info
func Get(id trakt.SearchID, params *trakt.ExtendedParams) (*trakt.Movie, error) {
	return getC().Get(id, params)
}

// Get returns a single movie's details.
//
//   - Extended Info
func (c *client) Get(id trakt.SearchID, params *trakt.ExtendedParams) (*trakt.Movie, error) {
	path := trakt.FormatURLPath("/movies/%s", id)
	mov := &trakt.Movie{}
	err := c.b.Call(http.MethodGet, path, params, mov)
	return mov, err
}

// Aliases returns all title aliases for a movie. Includes country where name is different.
func Aliases(id trakt.SearchID, params *trakt.BasicParams) *trakt.AliasIterator {
	return getC().Aliases(id, params)
}

// Aliases returns all title aliases for a movie. Includes country where name is different.
func (c *client) Aliases(id trakt.SearchID, params *trakt.BasicParams) *trakt.AliasIterator {
	path := trakt.FormatURLPath("movies/%s/aliases", id)
	return &trakt.AliasIterator{BasicIterator: c.b.NewSimulatedIterator(http.MethodGet, path, params)}
}

// Releases returns all releases for a movie including country, certification, release date, release type,
// and note.
//
// The release type can be set to unknown, premiere, limited, theatrical, digital, physical, or tv.
// The note might have optional info such as the film festival name for a premiere release or
// Blu-ray specs for a physical release. This info is pulled from TMDB.
func Releases(id trakt.SearchID, params *trakt.ReleaseListParams) *trakt.ReleaseIterator {
	return getC().Releases(id, params)
}

// Releases returns all releases for a movie including country, certification, release date, release type,
// and note.
//
// The release type can be set to unknown, premiere, limited, theatrical, digital, physical, or tv.
// The note might have optional info such as the film festival name for a premiere release or
// Blu-ray specs for a physical release. This info is pulled from TMDB.
func (c *client) Releases(id trakt.SearchID, params *trakt.ReleaseListParams) *trakt.ReleaseIterator {
	path := trakt.FormatURLPath("movies/%s/releases/%s", id, params.Country)
	return &trakt.ReleaseIterator{BasicIterator: c.b.NewSimulatedIterator(http.MethodGet, path, params)}
}

// Translations returns all translations for a movie, including language and translated values for
// title, tagline and overview.
func Translations(id trakt.SearchID, params *trakt.TranslationListParams) *trakt.TranslationIterator {
	return getC().Translations(id, params)
}

// Translations returns all translations for a movie, including language and translated values for
// title, tagline and overview.
func (c *client) Translations(id trakt.SearchID, params *trakt.TranslationListParams) *trakt.TranslationIterator {
	path := trakt.FormatURLPath("movies/%s/translations/%s", id, params.Language)
	return &trakt.TranslationIterator{BasicIterator: c.b.NewSimulatedIterator(http.MethodGet, path, params)}
}

// Comments returns all top level comments for a movie.
//
// By default, the newest comments are returned first. Other sorting options include oldest, most likes,
// most replies, highest rated, lowest rated, and most plays.
//
//   - Pagination
func Comments(id trakt.SearchID, params *trakt.CommentListParams) *trakt.CommentIterator {
	return getC().Comments(id, params)
}

// Comments returns all top level comments for a movie.
//
// By default, the newest comments are returned first. Other sorting options include oldest, most likes,
// most replies, highest rated, lowest rated, and most plays.
//
//   - Pagination
func (c *client) Comments(id trakt.SearchID, params *trakt.CommentListParams) *trakt.CommentIterator {
	path := trakt.FormatURLPath("movies/%s/comments/%s", id, params.Sort)
	return &trakt.CommentIterator{Iterator: c.b.NewIterator(http.MethodGet, path, params)}
}

// WatchingNow returns all users watching this movie right now.
//
//   - Extended Info
func WatchingNow(id trakt.SearchID, params *trakt.BasicListParams) *trakt.UserIterator {
	return getC().WatchingNow(id, params)
}

// WatchingNow returns all users watching this movie right now.
//
//   - Extended Info
func (c *client) WatchingNow(id trakt.SearchID, params *trakt.BasicListParams) *trakt.UserIterator {
	path := trakt.FormatURLPath("movies/%s/watching", id)
	return &trakt.UserIterator{Iterator: c.b.NewIterator(http.MethodGet, path, params)}
}

// Related returns related and similar movies.
//
//   - Pagination
//   - Extended Info
func Related(id trakt.SearchID, params *trakt.ExtendedListParams) *trakt.MovieIterator {
	return getC().Related(id, params)
}

// Related returns related and similar movies.
//
//   - Pagination
//   - Extended Info
func (c *client) Related(id trakt.SearchID, params *trakt.ExtendedListParams) *trakt.MovieIterator {
	path := trakt.FormatURLPath("movies/%s/related", id)
	return &trakt.MovieIterator{Iterator: c.b.NewIterator(http.MethodGet, path, params)}
}

// Ratings returns the rating (between 0 and 10) and distribution for a movie.
func Ratings(id trakt.SearchID, params *trakt.BasicParams) (*trakt.RatingDistribution, error) {
	return getC().Ratings(id, params)
}

// Ratings returns the rating (between 0 and 10) and distribution for a movie.
func (c *client) Ratings(id trakt.SearchID, params *trakt.BasicParams) (*trakt.RatingDistribution, error) {
	path := trakt.FormatURLPath("/movies/%s/ratings", id)
	stats := &trakt.RatingDistribution{}
	err := c.b.Call(http.MethodGet, path, params, stats)
	return stats, err
}

// Statistics returns lots of movie stats.
func Statistics(id trakt.SearchID, params *trakt.BasicParams) (*trakt.Statistics, error) {
	return getC().Statistics(id, params)
}

// Statistics returns lots of movie stats.
func (c *client) Statistics(id trakt.SearchID, params *trakt.BasicParams) (*trakt.Statistics, error) {
	path := trakt.FormatURLPath("/movies/%s/stats", id)
	stats := &trakt.Statistics{}
	err := c.b.Call(http.MethodGet, path, params, stats)
	return stats, err
}

// Lists returns all lists that contain this movie. By default, personal lists are returned sorted
// by the most popular.
//
//   - Pagination
func Lists(id trakt.SearchID, params *trakt.GetListParams) *trakt.ListIterator {
	return getC().Lists(id, params)
}

// Lists returns all lists that contain this movie. By default, personal lists are returned sorted
// by the most popular.
//
//   - Pagination
func (c *client) Lists(id trakt.SearchID, params *trakt.GetListParams) *trakt.ListIterator {
	path := trakt.FormatURLPath("movies/%s/lists/%s/%s", id, params.ListType, params.SortType)
	return &trakt.ListIterator{Iterator: c.b.NewIterator(http.MethodGet, path, params)}
}

// People returns all cast and crew for a movie.
//
// Each cast member will have a characters array and a standard person object.
//
// The crew object will be broken up into production, art, crew, costume & make-up, directing, writing, sound,
// camera, visual effects, lighting, and editing (if there are people for those crew positions). Each of those
// members will have a jobs array and a standard person object.
//
// Note: This returns a lot of data, so please only use this extended parameter if you actually need it!
//
//   - Extended Info
func People(id trakt.SearchID, params *trakt.ExtendedParams) (*trakt.CastAndCrew, error) {
	return getC().People(id, params)
}

// People returns all cast and crew for a movie.
//
// Each cast member will have a characters array and a standard person object.
//
// The crew object will be broken up into production, art, crew, costume & make-up, directing, writing, sound,
// camera, visual effects, lighting, and editing (if there are people for those crew positions). Each of those
// members will have a jobs array and a standard person object.
//
// Note: This returns a lot of data, so please only use this extended parameter if you actually need it!
//
//   - Extended Info
func (c *client) People(id trakt.SearchID, params *trakt.ExtendedParams) (*trakt.CastAndCrew, error) {
	path := trakt.FormatURLPath("/movies/%s/people", id)
	cc := &trakt.CastAndCrew{}
	err := c.b.Call(http.MethodGet, path, params, cc)
	return cc, err
}

// newTimePeriodIterator generates an iterator for a list of movies based on an action and a time period.
// the time period defaults to WEEKLY if not supplied.
func (c *client) newTimePeriodIterator(act string, p *trakt.TimePeriodListParams) *trakt.MovieWithStatisticsIterator {
	var period = trakt.TimePeriodWeekly
	if p.Period != "" {
		period = p.Period
	}
	path := trakt.FormatURLPath("/movies/%s/%s", act, period)
	return &trakt.MovieWithStatisticsIterator{Iterator: c.b.NewIterator(http.MethodGet, path, p)}
}

// getC initialises a new movie client with the currently defined backend configuration.
func getC() *client { return &client{trakt.NewClient()} }
