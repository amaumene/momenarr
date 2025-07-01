// Package show contains functions to retrieve show details.
package show

import (
	"net/http"

	"github.com/amaumene/momenarr/trakt"
)

// client represents a show client which can retrieve details about shows.
type client struct{ b trakt.BaseClient }

// Trending returns all shows being watched right now. Shows with the most users are returned first.
//
//   - Pagination
//   - Filters
//   - Extended Info
func Trending(params *trakt.FilterListParams) *trakt.TrendingShowIterator {
	return getC().Trending(params)
}

// Trending returns all shows being watched right now. Shows with the most users are returned first.
//
//   - Pagination
//   - Filters
//   - Extended Info
func (c *client) Trending(params *trakt.FilterListParams) *trakt.TrendingShowIterator {
	return &trakt.TrendingShowIterator{Iterator: c.b.NewIterator(http.MethodGet, "/shows/trending", params)}
}

// Popular returns the most popular shows. Popularity is calculated using the rating percentage and
// the number of ratings.
//
//   - Pagination
//   - Filters
//   - Extended Info
func Popular(params *trakt.FilterListParams) *trakt.ShowIterator {
	return getC().Popular(params)
}

// Popular returns the most popular shows. Popularity is calculated using the rating percentage and
// the number of ratings.
//
//   - Pagination
//   - Filters
//   - Extended Info
func (c *client) Popular(params *trakt.FilterListParams) *trakt.ShowIterator {
	return &trakt.ShowIterator{Iterator: c.b.NewIterator(http.MethodGet, "/shows/popular", params)}
}

// Played returns the most played (a single user can watch multiple episodes multiple times) shows in the
// specified time period, defaulting to weekly. All stats are relative to the specific time period.
//
//   - Pagination
//   - Filters
//   - Extended Info
func Played(params *trakt.TimePeriodListParams) *trakt.ShowWithStatisticsIterator {
	return getC().Played(params)
}

// Played returns the most played (a single user can watch multiple episodes multiple times) shows in the
// specified time period, defaulting to weekly. All stats are relative to the specific time period.
//
//   - Pagination
//   - Filters
//   - Extended Info
func (c *client) Played(params *trakt.TimePeriodListParams) *trakt.ShowWithStatisticsIterator {
	return c.newTimePeriodIterator("played", params)
}

// Watched returns the most watched (unique users) shows in the specified time period, defaulting to weekly.
// All stats are relative to the specific time period.
//
//   - Pagination
//   - Filters
//   - Extended Info
func Watched(params *trakt.TimePeriodListParams) *trakt.ShowWithStatisticsIterator {
	return getC().Watched(params)
}

// Watched returns the most watched (unique users) shows in the specified time period, defaulting to weekly.
// All stats are relative to the specific time period.
//
//   - Pagination
//   - Filters
//   - Extended Info
func (c *client) Watched(params *trakt.TimePeriodListParams) *trakt.ShowWithStatisticsIterator {
	return c.newTimePeriodIterator("watched", params)
}

// Collected returns the most collected (unique users) shows in the specified time period, defaulting to
// weekly. All stats are relative to the specific time period.
//
//   - Pagination
//   - Filters
//   - Extended Info
func Collected(params *trakt.TimePeriodListParams) *trakt.ShowWithStatisticsIterator {
	return getC().Collected(params)
}

// Collected returns the most collected (unique users) shows in the specified time period, defaulting to
// weekly. All stats are relative to the specific time period.
//
//   - Pagination
//   - Filters
//   - Extended Info
func (c *client) Collected(params *trakt.TimePeriodListParams) *trakt.ShowWithStatisticsIterator {
	return c.newTimePeriodIterator("collected", params)
}

// Anticipated returns the most anticipated shows based on the number of lists a show appears on.
//
//   - Pagination
//   - Filters
//   - Extended Info
func Anticipated(params *trakt.FilterListParams) *trakt.AnticipatedShowIterator {
	return getC().Anticipated(params)
}

// Anticipated returns the most anticipated shows based on the number of lists a show appears on.
//
//   - Pagination
//   - Filters
//   - Extended Info
func (c *client) Anticipated(params *trakt.FilterListParams) *trakt.AnticipatedShowIterator {
	return &trakt.AnticipatedShowIterator{Iterator: c.b.NewIterator(http.MethodGet, "/shows/anticipated", params)}
}

// RecentlyUpdated Returns all shows updated since the specified UTC date. We recommended storing the date
// you can be efficient using this method moving forward. By default, 10 results are returned. You can send
// a limit to get up to 100 results per page.
//
//   - Pagination
//   - Extended Info
func RecentlyUpdated(params *trakt.RecentlyUpdatedListParams) *trakt.RecentlyUpdatedShowIterator {
	return getC().RecentlyUpdated(params)
}

// RecentlyUpdated Returns all shows updated since the specified UTC date. We recommended storing the date
// you can be efficient using this method moving forward. By default, 10 results are returned. You can send
// a limit to get up to 100 results per page.
//
//   - Pagination
//   - Extended Info
func (c *client) RecentlyUpdated(params *trakt.RecentlyUpdatedListParams) *trakt.RecentlyUpdatedShowIterator {
	path := trakt.FormatURLPath("/shows/updates/%s", params.StartDate.Format(`2006-01-02`))
	return &trakt.RecentlyUpdatedShowIterator{Iterator: c.b.NewIterator(http.MethodGet, path, params)}
}

// Get returns a single shows's details. If you request extended info, the airs object is relative to
// the show's country. You can use the day, time, and timezone to construct your own date then convert
// it to whatever timezone your user is in.
//
//   - Extended Info
func Get(id trakt.SearchID, params *trakt.ExtendedParams) (*trakt.Show, error) {
	return getC().Get(id, params)
}

// Get returns a single shows's details. If you request extended info, the airs object is relative to
// the show's country. You can use the day, time, and timezone to construct your own date then convert
// it to whatever timezone your user is in.
//
//   - Extended Info
func (c *client) Get(id trakt.SearchID, params *trakt.ExtendedParams) (*trakt.Show, error) {
	path := trakt.FormatURLPath("/shows/%s", id)
	mov := &trakt.Show{}
	err := c.b.Call(http.MethodGet, path, params, mov)
	return mov, err
}

// Aliases returns all title aliases for a show. Includes country where name is different.
func Aliases(id trakt.SearchID, params *trakt.BasicParams) *trakt.AliasIterator {
	return getC().Aliases(id, params)
}

// Aliases returns all title aliases for a show. Includes country where name is different.
func (c *client) Aliases(id trakt.SearchID, params *trakt.BasicParams) *trakt.AliasIterator {
	path := trakt.FormatURLPath("shows/%s/aliases", id)
	return &trakt.AliasIterator{BasicIterator: c.b.NewSimulatedIterator(http.MethodGet, path, params)}
}

// Certifications returns all content certifications for a show, including the country.
func Certifications(id trakt.SearchID, params *trakt.BasicParams) *trakt.CertificationIterator {
	return getC().Certifications(id, params)
}

// Certifications returns all content certifications for a show, including the country.
func (c *client) Certifications(id trakt.SearchID, params *trakt.BasicParams) *trakt.CertificationIterator {
	path := trakt.FormatURLPath("shows/%s/certifications", id)
	return &trakt.CertificationIterator{BasicIterator: c.b.NewSimulatedIterator(http.MethodGet, path, params)}
}

// Translations returns all translations for a show, including language and translated values for title and overview.
func Translations(id trakt.SearchID, params *trakt.TranslationListParams) *trakt.TranslationIterator {
	return getC().Translations(id, params)
}

// Translations returns all translations for a show, including language and translated values for title and overview.
func (c *client) Translations(id trakt.SearchID, params *trakt.TranslationListParams) *trakt.TranslationIterator {
	path := trakt.FormatURLPath("shows/%s/translations/%s", id, params.Language)
	return &trakt.TranslationIterator{BasicIterator: c.b.NewSimulatedIterator(http.MethodGet, path, params)}
}

// Comments returns all top level comments for a show. By default, the newest comments are returned first.
// Other sorting options include oldest, most likes, most replies, highest rated, lowest rated, most plays,
// and highest watched percentage.
//
//   - Pagination
func Comments(id trakt.SearchID, params *trakt.CommentListParams) *trakt.CommentIterator {
	return getC().Comments(id, params)
}

// Comments returns all top level comments for a show. By default, the newest comments are returned first.
// Other sorting options include oldest, most likes, most replies, highest rated, lowest rated, most plays,
// and highest watched percentage.
//
//   - Pagination
func (c *client) Comments(id trakt.SearchID, params *trakt.CommentListParams) *trakt.CommentIterator {
	path := trakt.FormatURLPath("shows/%s/comments/%s", id, params.Sort)
	return &trakt.CommentIterator{Iterator: c.b.NewIterator(http.MethodGet, path, params)}
}

// Lists Returns all lists that contain this show. By default, personal lists are returned sorted by the
// most popular.
//
//   - Pagination
func Lists(id trakt.SearchID, params *trakt.GetListParams) *trakt.ListIterator {
	return getC().Lists(id, params)
}

// Lists Returns all lists that contain this show. By default, personal lists are returned sorted by the
// most popular.
//
//   - Pagination
func (c *client) Lists(id trakt.SearchID, params *trakt.GetListParams) *trakt.ListIterator {
	path := trakt.FormatURLPath("shows/%s/lists/%s/%s", id, params.ListType, params.SortType)
	return &trakt.ListIterator{Iterator: c.b.NewIterator(http.MethodGet, path, params)}
}

// CollectionProgress returns collection progress for a show including details on all aired seasons and episodes.
// The "NextEpisode" will be the next episode the user should collect, if there are no upcoming episodes it will be
// set nil.
//
// By default, any hidden seasons will be removed from the response and stats. To include these and adjust the
// completion stats, set the "Hidden" flag to true.
//
// By default, specials will be excluded from the response. Set the "Specials" flag to true to include season 0
// and adjust the stats accordingly. If you'd like to include specials, but not adjust the stats, set
// "CountSpecials" to false.
//
// By default, the "LastEpisode" and "NextEpisode" are calculated using the last aired episode the user has
// collected, even if they've collected older episodes more recently. To use their last collected episode for
// these calculations, set "LastActivity" to collected.
//
// Note: Only aired episodes are used to calculate progress. Episodes in the future or without an air
// date are ignored.
//
//   - OAuth Required
func CollectionProgress(id trakt.SearchID, params *trakt.ProgressParams) (*trakt.CollectedProgress, error) {
	return getC().CollectionProgress(id, params)
}

// CollectionProgress returns collection progress for a show including details on all aired seasons and episodes.
// The "NextEpisode" will be the next episode the user should collect, if there are no upcoming episodes it will be
// set nil.
//
// By default, any hidden seasons will be removed from the response and stats. To include these and adjust the
// completion stats, set the "Hidden" flag to true.
//
// By default, specials will be excluded from the response. Set the "Specials" flag to true to include season 0
// and adjust the stats accordingly. If you'd like to include specials, but not adjust the stats, set
// "CountSpecials" to false.
//
// By default, the "LastEpisode" and "NextEpisode" are calculated using the last aired episode the user has
// collected, even if they've collected older episodes more recently. To use their last collected episode for
// these calculations, set "LastActivity" to collected.
//
// Note: Only aired episodes are used to calculate progress. Episodes in the future or without an air
// date are ignored.
//
//   - OAuth Required
func (c *client) CollectionProgress(id trakt.SearchID, params *trakt.ProgressParams) (*trakt.CollectedProgress, error) {
	path := trakt.FormatURLPath("/shows/%s/progress/collection", id)
	cc := &trakt.CollectedProgress{}
	err := c.b.Call(http.MethodGet, path, params, cc)
	return cc, err
}

// WatchedProgress returns watched progress for a show including details on all aired seasons and episodes.
// The "NextEpisode" will be the next episode the user should watch, if there are no upcoming episodes it
// will be nil. If not empty, the "ResetAt" date is when the user started re-watching the show. Your app can
// adjust the progress by ignoring episodes with a "WatchedAt" prior to the "ResetAt".
//
// By default, any hidden seasons will be removed from the response and stats. To include these and adjust the
// completion stats, set the "Hidden" flag to true.
//
// By default, specials will be excluded from the response. Set the "Specials" flag to true to include season 0
// and adjust the stats accordingly. If you'd like to include specials, but not adjust the stats, set
// "CountSpecials" to false.
//
// By default, the "LastEpisode" and "NextEpisode" are calculated using the last aired episode the user has
// collected, even if they've collected older episodes more recently. To use their last collected episode for
// these calculations, set "LastActivity" to collected.
//
// Note: Only aired episodes are used to calculate progress. Episodes in the future or without an air
// date are ignored.
//
//   - OAuth Required
func WatchedProgress(id trakt.SearchID, params *trakt.ProgressParams) (*trakt.WatchedProgress, error) {
	return getC().WatchedProgress(id, params)
}

// WatchedProgress returns watched progress for a show including details on all aired seasons and episodes.
// The "NextEpisode" will be the next episode the user should watch, if there are no upcoming episodes it
// will be nil. If not empty, the "ResetAt" date is when the user started re-watching the show. Your app can
// adjust the progress by ignoring episodes with a "WatchedAt" prior to the "ResetAt".
//
// By default, any hidden seasons will be removed from the response and stats. To include these and adjust the
// completion stats, set the "Hidden" flag to true.
//
// By default, specials will be excluded from the response. Set the "Specials" flag to true to include season 0
// and adjust the stats accordingly. If you'd like to include specials, but not adjust the stats, set
// "CountSpecials" to false.
//
// By default, the "LastEpisode" and "NextEpisode" are calculated using the last aired episode the user has
// collected, even if they've collected older episodes more recently. To use their last collected episode for
// these calculations, set "LastActivity" to collected.
//
// Note: Only aired episodes are used to calculate progress. Episodes in the future or without an air
// date are ignored.
//
//   - OAuth Required
func (c *client) WatchedProgress(id trakt.SearchID, params *trakt.ProgressParams) (*trakt.WatchedProgress, error) {
	path := trakt.FormatURLPath("/shows/%s/progress/watched", id)
	cc := &trakt.WatchedProgress{}
	err := c.b.Call(http.MethodGet, path, params, cc)
	return cc, err
}

// People returns all cast and crew for a show.
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
// in the requested show.
//
// Note: This returns a lot of data, so please only use this extended parameter if you actually need it!
//
//   - Extended Info
func People(id trakt.SearchID, params *trakt.ExtendedParams) (*trakt.CastAndCrew, error) {
	return getC().People(id, params)
}

// People returns all cast and crew for a show.
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
// in the requested show.
//
// Note: This returns a lot of data, so please only use this extended parameter if you actually need it!
//
//   - Extended Info
func (c *client) People(id trakt.SearchID, params *trakt.ExtendedParams) (*trakt.CastAndCrew, error) {
	path := trakt.FormatURLPath("/shows/%s/people", id)
	cc := &trakt.CastAndCrew{}
	err := c.b.Call(http.MethodGet, path, params, cc)
	return cc, err
}

// Ratings returns the rating (between 0 and 10) and distribution for a show.
func Ratings(id trakt.SearchID, params *trakt.BasicParams) (*trakt.RatingDistribution, error) {
	return getC().Ratings(id, params)
}

// Ratings returns the rating (between 0 and 10) and distribution for a show.
func (c *client) Ratings(id trakt.SearchID, params *trakt.BasicParams) (*trakt.RatingDistribution, error) {
	path := trakt.FormatURLPath("/shows/%s/ratings", id)
	stats := &trakt.RatingDistribution{}
	err := c.b.Call(http.MethodGet, path, params, stats)
	return stats, err
}

// Related returns related and similar shows.
//
//   - Pagination
//   - Extended Info
func Related(id trakt.SearchID, params *trakt.ExtendedListParams) *trakt.ShowIterator {
	return getC().Related(id, params)
}

// Related returns related and similar shows.
//
//   - Pagination
//   - Extended Info
func (c *client) Related(id trakt.SearchID, params *trakt.ExtendedListParams) *trakt.ShowIterator {
	path := trakt.FormatURLPath("shows/%s/related", id)
	return &trakt.ShowIterator{Iterator: c.b.NewIterator(http.MethodGet, path, params)}
}

// Statistics returns lots of show stats.
func Statistics(id trakt.SearchID, params *trakt.BasicParams) (*trakt.Statistics, error) {
	return getC().Statistics(id, params)
}

// Statistics returns lots of show stats.
func (c *client) Statistics(id trakt.SearchID, params *trakt.BasicParams) (*trakt.Statistics, error) {
	path := trakt.FormatURLPath("/shows/%s/stats", id)
	stats := &trakt.Statistics{}
	err := c.b.Call(http.MethodGet, path, params, stats)
	return stats, err
}

// WatchingNow returns all users watching this show right now.
//
//   - Extended Info
func WatchingNow(id trakt.SearchID, params *trakt.BasicListParams) *trakt.UserIterator {
	return getC().WatchingNow(id, params)
}

// WatchingNow returns all users watching this show right now.
//
//   - Extended Info
func (c *client) WatchingNow(id trakt.SearchID, params *trakt.BasicListParams) *trakt.UserIterator {
	path := trakt.FormatURLPath("shows/%s/watching", id)
	return &trakt.UserIterator{Iterator: c.b.NewIterator(http.MethodGet, path, params)}
}

// NextEpisode returns the next scheduled to air episode. If no episode is found,
// no error will be returned, but the episode will also be nil.
//
//   - Extended Info
func NextEpisode(id trakt.SearchID, params *trakt.ExtendedParams) (*trakt.Episode, error) {
	return getC().NextEpisode(id, params)
}

// NextEpisode returns the next scheduled to air episode. If no episode is found,
// no error will be returned, but the episode will also be nil.
//
//   - Extended Info
func (c *client) NextEpisode(id trakt.SearchID, params *trakt.ExtendedParams) (*trakt.Episode, error) {
	ep := &trakt.Episode{}
	path := trakt.FormatURLPath("shows/%s/next_episode", id)
	err := c.b.Call(http.MethodGet, path, params, ep)
	return handleNoEpisodeFound(ep, err)
}

// NextEpisode returns the most recently aired episode. If no episode is found,
// no error will be returned, but the episode will also be nil.
//
//   - Extended Info
func LastEpisode(id trakt.SearchID, params *trakt.ExtendedParams) (*trakt.Episode, error) {
	return getC().LastEpisode(id, params)
}

// NextEpisode returns the most recently aired episode. If no episode is found,
// no error will be returned, but the episode will also be nil.
//
//   - Extended Info
func (c *client) LastEpisode(id trakt.SearchID, params *trakt.ExtendedParams) (*trakt.Episode, error) {
	ep := &trakt.Episode{}
	path := trakt.FormatURLPath("shows/%s/last_episode", id)
	err := c.b.Call(http.MethodGet, path, params, ep)
	return handleNoEpisodeFound(ep, err)
}

// Seasons returns all seasons for a show including the number of episodes in each season.
//
// # Episodes
//
// If Extended is set to "ExtendedTypeEpisodes", it will return all episodes for all seasons.
//
// Note: This returns a lot of data, so please only use this extended parameter if you actually need it!
//
//   - Extended Info
func Seasons(id trakt.SearchID, params *trakt.ExtendedListParams) *trakt.SeasonWithEpisodesIterator {
	return getC().Seasons(id, params)
}

// Seasons returns all seasons for a show including the number of episodes in each season.
//
// # Episodes
//
// If Extended is set to "ExtendedTypeEpisodes", it will return all episodes for all seasons.
//
// Note: This returns a lot of data, so please only use this extended parameter if you actually need it!
//
//   - Extended Info
func (c *client) Seasons(id trakt.SearchID, params *trakt.ExtendedListParams) *trakt.SeasonWithEpisodesIterator {
	path := trakt.FormatURLPath("shows/%s/seasons", id)
	return &trakt.SeasonWithEpisodesIterator{BasicIterator: c.b.NewSimulatedIterator(http.MethodGet, path, params)}
}

// newTimePeriodIterator generates a new show iterator with the defined action and params.
// it will set the default time period to weekly if not supplied, this emulates the APIs default.
func (c *client) newTimePeriodIterator(action string, p *trakt.TimePeriodListParams) *trakt.ShowWithStatisticsIterator {
	var period = trakt.TimePeriodWeekly
	if p.Period != "" {
		period = p.Period
	}
	path := trakt.FormatURLPath("/shows/%s/%s", action, period)
	return &trakt.ShowWithStatisticsIterator{Iterator: c.b.NewIterator(http.MethodGet, path, p)}
}

// handleNoEpisodeFound helper function to handle if the response was a success but no episode was found
// in any request to trakt the BARE minimum we should receive in all types of request are a trakt UUID
// if we had no error and an episode with no ID then we can assume that the request was a success
// but no episode was found.
func handleNoEpisodeFound(e *trakt.Episode, err error) (*trakt.Episode, error) {
	if err != nil {
		return nil, err
	}

	if e == nil {
		return nil, nil
	}

	if e.Trakt <= 0 {
		return nil, nil
	}

	return e, nil
}

// getC initialises a new show client with the current backend configuration.
func getC() *client { return &client{trakt.NewClient()} }
