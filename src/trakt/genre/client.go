// Package genre gives functions to retrieve genre information.
package genre

import (
	"net/http"

	"github.com/jacklaaa89/trakt"
)

// client the genre client.
type client struct{ b trakt.BaseClient }

// List retrieves a list of all genres, including names and slugs.
func List(params *trakt.ListByTypeParams) *trakt.GenreIterator {
	return getC().List(params)
}

// List retrieves a list of all genres, including names and slugs.
func (c *client) List(params *trakt.ListByTypeParams) *trakt.GenreIterator {
	path := trakt.FormatURLPath("/genres/%s", params.Type)
	return &trakt.GenreIterator{
		BasicIterator: c.b.NewSimulatedIteratorWithCondition(http.MethodGet, path, params, func() error {
			switch params.Type {
			case trakt.TypeMovie, trakt.TypeShow:
				return nil
			}

			return &trakt.Error{
				HTTPStatusCode: http.StatusUnprocessableEntity,
				Body:           "invalid type: only movie / show are applicable",
				Resource:       path,
				Code:           trakt.ErrorCodeValidationError,
			}
		}),
	}
}

// getC retrieves an instance of a genre client.
func getC() *client { return &client{trakt.NewClient()} }
