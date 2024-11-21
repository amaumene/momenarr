// Package search allows us to perform textual based queries or ID lookups.
//
// Queries will search text fields like the title and overview.
// ID lookups are helpful if you have an external ID and want to
// get the Trakt ID and info.
//
// These methods can search for movies, shows, episodes, people, and lists.
package search

import (
	"net/http"

	"github.com/jacklaaa89/trakt"
)

// client represents a client which can be used to perform search requests.
type client struct{ b trakt.BaseClient }

// wrappedSearchQuery this is only required because there seems to be
// a weird bug with the "query" package in which it only runs the custom
// encoder on sub-fields and not the top level interface originally supplied to
// the "query.Values" func.
// see: "github.com/google/go-querystring/query".reflectValues to see what happens.
type wrappedSearchQuery struct{ *trakt.SearchQueryParams }

// TextQuery searches all text fields that a media object contains (i.e. title, overview, etc). Results
// are ordered by the most relevant score. Specify the type of results by sending a single value
// or a comma delimited string for multiple types.
//
// # Search Fields
//
// By default, all text fields are used to search for the query. You can optionally specify the fields
// parameter with one or more search fields. Each type has specific fields that can be specified. This can
// be useful if you want to support more strict searches (i.e. title only).
//
//   - movie   - title | tagline     | overview | people       | translations | aliases
//   - show    - title | overview    | people   | translations | aliases
//   - episode - title | overview    |
//   - person  - name  | biography   |
//   - list    - name  | description |
//
// # Special Characters
//
// Our search engine (Solr) gives the following characters special meaning when they appear in a query:
//   - - && || ! ( ) { } [ ] ^ " ~ * ? : /
//
// To interpret any of these characters literally (and not as a special character), precede the character
// with a backslash \ character.
//
//   - Pagination
//   - Filters
//   - Extended Info
func TextQuery(params *trakt.SearchQueryParams) *trakt.SearchResultIterator {
	return getC().TextQuery(params)
}

// TextQuery searches all text fields that a media object contains (i.e. title, overview, etc). Results
// are ordered by the most relevant score. Specify the type of results by sending a single value
// or a comma delimited string for multiple types.
//
// # Search Fields
//
// By default, all text fields are used to search for the query. You can optionally specify the fields
// parameter with one or more search fields. Each type has specific fields that can be specified. This can
// be useful if you want to support more strict searches (i.e. title only).
//
//   - movie   - title | tagline     | overview | people       | translations | aliases
//   - show    - title | overview    | people   | translations | aliases
//   - episode - title | overview    |
//   - person  - name  | biography   |
//   - list    - name  | description |
//
// # Special Characters
//
// Our search engine (Solr) gives the following characters special meaning when they appear in a query:
//   - - && || ! ( ) { } [ ] ^ " ~ * ? : /
//
// To interpret any of these characters literally (and not as a special character), precede the character
// with a backslash \ character.
//
//   - Pagination
//   - Filters
//   - Extended Info
func (c *client) TextQuery(params *trakt.SearchQueryParams) *trakt.SearchResultIterator {
	path := trakt.FormatURLPath("/search/%s", params.Type)
	return &trakt.SearchResultIterator{Iterator: c.b.NewIterator(http.MethodGet, path, wrappedSearchQuery{params})}
}

// IDLookup attempts to lookup items by their Trakt, IMDB, TMDB, or TVDB ID. If you use the search url
// without a type it might return multiple items if the id_type is not globally unique. Specify the type
// of results by sending one or more Types.
//
//   - Pagination
//   - Extended Info
func IDLookup(id trakt.SearchID, params *trakt.IDLookupParams) *trakt.SearchResultIterator {
	return getC().IDLookup(id, params)
}

// IDLookup attempts to lookup items by their Trakt, IMDB, TMDB, or TVDB ID. If you use the search url
// without a type it might return multiple items if the id_type is not globally unique. Specify the type
// of results by sending one or more Types.
//
//   - Pagination
//   - Extended Info
func (c *client) IDLookup(id trakt.SearchID, params *trakt.IDLookupParams) *trakt.SearchResultIterator {
	path := trakt.FormatURLPath(trakt.IDPath(id), id)
	return &trakt.SearchResultIterator{
		Iterator: c.b.NewIteratorWithCondition(http.MethodGet, path, params, func() error {
			switch id.(type) {
			case trakt.Slug, *trakt.Slug:
				return &trakt.Error{
					HTTPStatusCode: http.StatusUnprocessableEntity,
					Body:           "invalid type supplied for ID lookup, only IMDB|ID|TVDB|TMDB allowed",
					Resource:       path,
					Code:           trakt.ErrorCodeValidationError,
				}
			}
			return nil
		}),
	}
}

// getC initialises a new search client with the currently defined backend.
func getC() *client { return &client{trakt.NewClient()} }
