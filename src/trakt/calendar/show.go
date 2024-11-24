package calendar

import (
	"net/http"
	"strconv"
	"time"

	"github.com/amaumene/momenarr/trakt"
)

// scope defines the scope or context a function should be applied to.
type scope string

const (
	// scopeAuthenticated represents retrieving data for the currently
	// authenticated user via OAuth.
	scopeAuthenticated = "my"
	// scopeAll represents retrieving data on everything.
	scopeAll = "all"

	// timeFormat the time format used when constructing the URL for calendar
	// based queries.
	timeFormat = "2006-01-02"
)

// client the calendar client used to make requests.
type client struct{ b trakt.BaseClient }

// MyShows returns all shows airing during the time period specified for the authenticated user.
//
//   - OAuth Required
//   - Extended Info
//   - Filters
func MyShows(p *trakt.CalendarParams) *trakt.CalendarShowIterator { return getC().MyShows(p) }

// MyShows returns all shows airing during the time period specified for the authenticated user.
//
//   - OAuth Required
//   - Extended Info
//   - Filters
func (c *client) MyShows(params *trakt.CalendarParams) *trakt.CalendarShowIterator {
	return c.shows(scopeAuthenticated, &wrappedCalendarParams{params})
}

// MyNewShows returns all new show premieres (season 1, episode 1) airing during the time period specified
// for the authenticated user.
//
//   - OAuth Required
//   - Extended Info
//   - Filters
func MyNewShows(p *trakt.CalendarParams) *trakt.CalendarShowIterator { return getC().MyNewShows(p) }

// MyNewShows returns all new show premieres (season 1, episode 1) airing during the time period specified
// for the authenticated user.
//
//   - OAuth Required
//   - Extended Info
//   - Filters
func (c *client) MyNewShows(params *trakt.CalendarParams) *trakt.CalendarShowIterator {
	return c.newShows(scopeAuthenticated, &wrappedCalendarParams{params})
}

// MySeasonPremieres returns all show premieres (any season, episode 1) airing during the time period
// specified for the authenticated user.
//
//   - OAuth Required
//   - Extended Info
//   - Filters
func MySeasonPremieres(p *trakt.CalendarParams) *trakt.CalendarShowIterator {
	return getC().MySeasonPremieres(p)
}

// MySeasonPremieres returns all show premieres (any season, episode 1) airing during the time period
// specified for the authenticated user.
//
//   - OAuth Required
//   - Extended Info
//   - Filters
func (c *client) MySeasonPremieres(params *trakt.CalendarParams) *trakt.CalendarShowIterator {
	return c.seasonPremieres(scopeAuthenticated, &wrappedCalendarParams{params})
}

// Shows returns all shows airing during the time period specified.
//
//   - Extended Info
//   - Filters
func Shows(p *trakt.BasicCalendarParams) *trakt.CalendarShowIterator { return getC().Shows(p) }

// Shows returns all shows airing during the time period specified.
//
//   - Extended Info
//   - Filters
func (c *client) Shows(params *trakt.BasicCalendarParams) *trakt.CalendarShowIterator {
	return c.shows(scopeAll, &wrappedBasicCalendarParams{params})
}

// NewShows returns all new show premieres (season 1, episode 1) airing during the time period specified.
//
//   - Extended Info
//   - Filters
func NewShows(params *trakt.BasicCalendarParams) *trakt.CalendarShowIterator {
	return getC().NewShows(params)
}

// NewShows returns all new show premieres (season 1, episode 1) airing during the time period specified.
//
//   - Extended Info
//   - Filters
func (c *client) NewShows(params *trakt.BasicCalendarParams) *trakt.CalendarShowIterator {
	return c.newShows(scopeAll, &wrappedBasicCalendarParams{params})
}

// SeasonPremieres returns all show premieres (any season, episode 1) airing during the time period
// specified.
//
//   - Extended Info
//   - Filters
func SeasonPremieres(params *trakt.BasicCalendarParams) *trakt.CalendarShowIterator {
	return getC().SeasonPremieres(params)
}

// SeasonPremieres returns all show premieres (any season, episode 1) airing during the time period
// specified.
//
//   - Extended Info
//   - Filters
func (c *client) SeasonPremieres(params *trakt.BasicCalendarParams) *trakt.CalendarShowIterator {
	return c.seasonPremieres(scopeAll, &wrappedBasicCalendarParams{params})
}

// shows helper function which generates an iterator for a list of shows under the supplied scope.
func (c *client) shows(scope scope, params calendarParams) *trakt.CalendarShowIterator {
	return c.generateShowIterator(trakt.FormatURLPath("/calendars/%s/shows", scope), params)
}

// newShows helper function which generates an iterator for a list of new shows under the supplied scope.
func (c *client) newShows(scope scope, params calendarParams) *trakt.CalendarShowIterator {
	return c.generateShowIterator(trakt.FormatURLPath("/calendars/%s/shows/new", scope), params)
}

// seasonPremieres helper function which generates an iterator for a list of season premieres
// under the supplied scope.
func (c *client) seasonPremieres(scope scope, params calendarParams) *trakt.CalendarShowIterator {
	return c.generateShowIterator(trakt.FormatURLPath("/calendars/%s/shows/premieres", scope), params)
}

// generateShowIterator generates an iterator to retrieve calender shows for the supplied
// path and params.
func (c *client) generateShowIterator(path string, params calendarParams) *trakt.CalendarShowIterator {
	return &trakt.CalendarShowIterator{Iterator: c.b.NewIterator(http.MethodGet, formatPath(path, params), params.elem())}
}

// formatPath formats the arguments from the supplied parameters into the path, setting
// default where required.
func formatPath(path string, c calendarParams) string {
	var days = c.days()
	if days == 0 {
		days = 7
	}

	date := c.startDate().Format(timeFormat)
	return trakt.FormatURLPath(path+"/%s/%s", date, strconv.Itoa(days))
}

// calendarParams a generic interface for a set of params designed to
// retrieve calendar information.
// This is required because slightly different parameter structs are required
// based on the scope requested. i.e authenticated scope functions require
// an OAuth Token, where as the all scope functions don't.
// this allows us to format a request irregardless of this difference.
type calendarParams interface {
	// ensure we inherit list params so we can use it internally.
	trakt.ListParamsContainer

	// startDate retrieves the start date to use in the URL path.
	startDate() time.Time
	// days retrieves the number of days to use the URL path.
	days() int
	// elem retrieves the internal parameter element.
	// this is required because of the nuances of URL marshalling in that
	// it encodes the interface name as part of the URL which is a expected
	// part of the library but an annoyance in the way we intend to use it.
	elem() trakt.ListParamsContainer
}

// wrappedCalendarParams a wrapped implementation of calendarParams
// for a param set which requires an OAuth token.
type wrappedCalendarParams struct{ *trakt.CalendarParams }

// startDate implements the calendarParams interface.
func (w *wrappedCalendarParams) startDate() time.Time { return w.StartDate }

// days implements the calendarParams interface.
func (w *wrappedCalendarParams) days() int { return int(w.Days) }

// elem implements the calendarParams interface.
func (w *wrappedCalendarParams) elem() trakt.ListParamsContainer { return w.CalendarParams }

// wrappedBasicCalendarParams a wrapped implementation of calendarParams
// for a param set which does not require an OAuth token.
type wrappedBasicCalendarParams struct{ *trakt.BasicCalendarParams }

// startDate implements the calendarParams interface.
func (w *wrappedBasicCalendarParams) startDate() time.Time { return w.StartDate }

// days implements the calendarParams interface.
func (w *wrappedBasicCalendarParams) days() int { return int(w.Days) }

// elem implements the calendarParams interface.
func (w *wrappedBasicCalendarParams) elem() trakt.ListParamsContainer { return w.BasicCalendarParams }

func getC() *client { return &client{trakt.NewClient()} }
