// Package recommendation provides functions to retrieve and configure movie and show recommendations.
//
// Recommendations are based on the watched history for a user and their friends. There are other factors
// that go into the algorithm as well to further personalize what gets recommended.
package recommendation

import (
	"net/http"

	"github.com/amaumene/momenarr/trakt"
)

// client represents a recommendation client.
type client struct{ b trakt.BaseClient }

// Movies returns personalized movie recommendations for a user. By default, 10 results are returned.
// You can send a limit to get up to 100 results per page. Set "IgnoreCollected" to true
// to filter out movies the user has already collected.
//
//   - OAuth Required
//   - Extended Info
func Movies(params *trakt.RecommendationListParams) *trakt.MovieIterator {
	return getC().Movies(params)
}

// Movies returns personalized movie recommendations for a user. By default, 10 results are returned.
// You can send a limit to get up to 100 results per page. Set "IgnoreCollected" to true
// to filter out movies the user has already collected.
//
//   - OAuth Required
//   - Extended Info
func (c *client) Movies(params *trakt.RecommendationListParams) *trakt.MovieIterator {
	return &trakt.MovieIterator{Iterator: c.b.NewIterator(http.MethodGet, "/recommendations/movies", params)}
}

// Shows returns personalized show recommendations for a user. By default, 10 results are returned.
// You can send a limit to get up to 100 results per page. Set "IgnoreCollected" to true
// to filter out movies the user has already collected.
//
//   - OAuth Required
//   - Extended Info
func Shows(params *trakt.RecommendationListParams) *trakt.ShowIterator {
	return getC().Shows(params)
}

// Shows returns personalized show recommendations for a user. By default, 10 results are returned.
// You can send a limit to get up to 100 results per page. Set "IgnoreCollected" to true
// to filter out movies the user has already collected.
//
//   - OAuth Required
//   - Extended Info
func (c *client) Shows(params *trakt.RecommendationListParams) *trakt.ShowIterator {
	return &trakt.ShowIterator{Iterator: c.b.NewIterator(http.MethodGet, "/recommendations/shows", params)}
}

// HideShow hides a show from getting recommended anymore.
//
//   - OAuth Required
func HideShow(id trakt.SearchID, params *trakt.Params) error {
	return getC().HideShow(id, params)
}

// HideShow hides a show from getting recommended anymore.
//
//   - OAuth Required
func (c *client) HideShow(id trakt.SearchID, params *trakt.Params) error {
	return c.b.Call(http.MethodDelete, trakt.FormatURLPath("/recommendations/shows/%s", id), params, nil)
}

// HideMovie hides a movie from getting recommended anymore.
//
//   - OAuth Required
func HideMovie(id trakt.SearchID, params *trakt.Params) error {
	return getC().HideMovie(id, params)
}

// HideMovie hides a movie from getting recommended anymore.
//
//   - OAuth Required
func (c *client) HideMovie(id trakt.SearchID, params *trakt.Params) error {
	return c.b.Call(http.MethodDelete, trakt.FormatURLPath("/recommendations/movies/%s", id), params, nil)
}

// getC initialises a new recommendation client with the currently defined backend configuration.
func getC() *client { return &client{trakt.NewClient()} }
