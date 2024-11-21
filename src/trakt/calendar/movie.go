package calendar

import (
	"net/http"

	"github.com/jacklaaa89/trakt"
)

// mediaType represents the available media types
// for movies which can be queried by the calendar client.
type mediaType string

const (
	// mediaTypeMovie represents the movie media type.
	// returns all movie results on the calendar.
	mediaTypeMovie mediaType = "movies"
	// mediaTypeDVD represents the dvd media type.
	// limits the calendar search to DVD releases.
	mediaTypeDVD mediaType = "dvds"
)

// MyMovies returns all movies with a release date during the time period specified.
//
// - OAuth Required
// - Extended Info
// - Filters
func MyMovies(p *trakt.CalendarParams) *trakt.CalendarMovieIterator { return getC().MyMovies(p) }

// MyMovies returns all movies with a release date during the time period specified.
//
// - OAuth Required
// - Extended Info
// - Filters
func (c *client) MyMovies(params *trakt.CalendarParams) *trakt.CalendarMovieIterator {
	return c.movies(scopeAuthenticated, mediaTypeMovie, &wrappedCalendarParams{params})
}

// MyDVDs returns all movies with a DVD release date during the time period specified.
//
// - OAuth Required
// - Extended Info
// - Filters
func MyDVDs(p *trakt.CalendarParams) *trakt.CalendarMovieIterator { return getC().MyDVDs(p) }

// MyDVDs returns all movies with a DVD release date during the time period specified.
//
// - OAuth Required
// - Extended Info
// - Filters
func (c *client) MyDVDs(params *trakt.CalendarParams) *trakt.CalendarMovieIterator {
	return c.movies(scopeAuthenticated, mediaTypeDVD, &wrappedCalendarParams{params})
}

// Movies returns all movies with a release date during the time period specified.
//
// - Extended Info
// - Filters
func Movies(p *trakt.BasicCalendarParams) *trakt.CalendarMovieIterator { return getC().Movies(p) }

// Movies returns all movies with a release date during the time period specified.
//
// - Extended Info
// - Filters
func (c *client) Movies(params *trakt.BasicCalendarParams) *trakt.CalendarMovieIterator {
	return c.movies(scopeAll, mediaTypeMovie, &wrappedBasicCalendarParams{params})
}

// DVDs returns all movies with a DVD release date during the time period specified.
//
// - Extended Info
// - Filters
func DVDs(p *trakt.BasicCalendarParams) *trakt.CalendarMovieIterator { return getC().DVDs(p) }

// DVDs returns all movies with a DVD release date during the time period specified.
//
// - Extended Info
// - Filters
func (c *client) DVDs(params *trakt.BasicCalendarParams) *trakt.CalendarMovieIterator {
	return c.movies(scopeAll, mediaTypeDVD, &wrappedBasicCalendarParams{params})
}

// movies helper function which generates an iterator for a set of movies based on the scope level and
// media type.
func (c *client) movies(scope scope, mediaType mediaType, params calendarParams) *trakt.CalendarMovieIterator {
	return c.generateMovieIterator(trakt.FormatURLPath("/calendars/%s/%s", scope, mediaType), params)
}

// generateMovieIterator generates an iterator for movies based on the path and params provided.
func (c *client) generateMovieIterator(path string, p calendarParams) *trakt.CalendarMovieIterator {
	return &trakt.CalendarMovieIterator{Iterator: c.b.NewIterator(http.MethodGet, formatPath(path, p), p.elem())}
}
