// Package country gives functions to retrieve country information.
package country

import (
	"net/http"

	"github.com/jacklaaa89/trakt"
)

// client the country client.
type client struct{ b trakt.BaseClient }

// List retrieves a list of all countries, including names and codes. Only TypeMovie and TypeShow are supported.
func List(params *trakt.ListByTypeParams) *trakt.CountryIterator {
	return getC().List(params)
}

// List retrieves a list of all countries, including names and codes. Only TypeMovie and TypeShow are supported.
func (c *client) List(params *trakt.ListByTypeParams) *trakt.CountryIterator {
	path := trakt.FormatURLPath("/countries/%s", params.Type.Plural())
	return &trakt.CountryIterator{
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

// getC retrieves an instance of a country client.
func getC() *client { return &client{trakt.NewClient()} }
