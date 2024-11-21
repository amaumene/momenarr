// Package language gives functions to retrieve language information.
package language

import (
	"net/http"

	"github.com/jacklaaa89/trakt"
)

// client the language client.
type client struct{ b trakt.BaseClient }

// List retrieves a list of all languages, including names and codes.
func List(params *trakt.ListByTypeParams) *trakt.LanguageIterator {
	return getC().List(params)
}

// List retrieves a list of all languages, including names and codes.
func (c *client) List(params *trakt.ListByTypeParams) *trakt.LanguageIterator {
	path := trakt.FormatURLPath("/languages/%s", params.Type)
	return &trakt.LanguageIterator{
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

// getC retrieves an instance of a language client.
func getC() *client { return &client{trakt.NewClient()} }
