// Package person gives function to retrieve people from trakt.
//
// This includes retrieving credits for both movies and shows.
package person

import (
	"net/http"

	"github.com/amaumene/momenarr/trakt"
)

// client represents a person client.
type client struct{ b trakt.BaseClient }

// Get returns a single person's details.
//
//   - Extended Info
func Get(id trakt.SearchID, p *trakt.ExtendedParams) (*trakt.Person, error) { return getC().Get(id, p) }

// Get returns a single person's details.
//
//   - Extended Info
func (c *client) Get(id trakt.SearchID, params *trakt.ExtendedParams) (*trakt.Person, error) {
	p := &trakt.Person{}
	path := trakt.FormatURLPath("/people/%s", id)
	err := c.b.Call(http.MethodGet, path, params, p)
	return p, err
}

// MovieCredits returns all movies where this person is in the cast or crew.
//
// Each cast object will have a characters array and a standard movie object.
//
// The crew object will be broken up into production, art, crew, costume & make-up, directing, writing,
// sound, camera, visual effects, lighting, and editing (if there are people for those crew positions).
// Each of those members will have a jobs array and a standard movie object.
//
//   - Extended Info
func MovieCredits(id trakt.SearchID, params *trakt.ExtendedParams) (*trakt.Credits, error) {
	return getC().MovieCredits(id, params)
}

// MovieCredits returns all movies where this person is in the cast or crew.
//
// Each cast object will have a characters array and a standard movie object.
//
// The crew object will be broken up into production, art, crew, costume & make-up, directing, writing,
// sound, camera, visual effects, lighting, and editing (if there are people for those crew positions).
// Each of those members will have a jobs array and a standard movie object.
//
//   - Extended Info
func (c *client) MovieCredits(id trakt.SearchID, params *trakt.ExtendedParams) (*trakt.Credits, error) {
	cr := &trakt.Credits{}
	path := trakt.FormatURLPath("/people/%s/movies", id)
	err := c.b.Call(http.MethodGet, path, params, cr)
	return cr, err
}

// ShowCredits returns all shows where this person is in the cast or crew.
//
// Each cast object will have a characters array and a standard show object.
//
// The crew object will be broken up into production, art, crew, costume & make-up, directing, writing,
// sound, camera, visual effects, lighting, and editing (if there are people for those crew positions).
// Each of those members will have a jobs array and a standard movie object.
//
//   - Extended Info
func ShowCredits(id trakt.SearchID, params *trakt.ExtendedParams) (*trakt.Credits, error) {
	return getC().ShowCredits(id, params)
}

// ShowCredits returns all shows where this person is in the cast or crew.
//
// Each cast object will have a characters array and a standard show object.
//
// The crew object will be broken up into production, art, crew, costume & make-up, directing, writing,
// sound, camera, visual effects, lighting, and editing (if there are people for those crew positions).
// Each of those members will have a jobs array and a standard movie object.
//
//   - Extended Info
func (c *client) ShowCredits(id trakt.SearchID, params *trakt.ExtendedParams) (*trakt.Credits, error) {
	cr := &trakt.Credits{}
	path := trakt.FormatURLPath("/people/%s/shows", id)
	err := c.b.Call(http.MethodGet, path, params, cr)
	return cr, err
}

// Lists returns all lists that contain this person.
//
// By default, personal lists are returned sorted by the most popular.
//
//   - Pagination
func Lists(id trakt.SearchID, params *trakt.GetListParams) *trakt.ListIterator {
	return getC().Lists(id, params)
}

// Lists returns all lists that contain this person.
//
// By default, personal lists are returned sorted by the most popular.
//
//   - Pagination
func (c *client) Lists(id trakt.SearchID, params *trakt.GetListParams) *trakt.ListIterator {
	path := trakt.FormatURLPath("people/%s/lists/%s/%s", id, params.ListType, params.SortType)
	return &trakt.ListIterator{Iterator: c.b.NewIterator(http.MethodGet, path, params)}
}

// getC initialises a new person client with the currently defined backend configuration.
func getC() *client { return &client{trakt.NewClient()} }
