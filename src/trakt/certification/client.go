// Package certification gives functions to retrieve TV Show or Movie certifications.
//
// Most TV shows and movies have a certification to indicate the content rating. Some API methods allow
// filtering by certification, this allows us to retrieve this data.
//
// Note: Only us certifications are currently returned.
package certification

import (
	"net/http"

	"github.com/jacklaaa89/trakt"
)

// client the certification client.
type client struct{ b trakt.BaseClient }

// List returns a list of all certifications, including names, slugs, and descriptions for a particular
// media type. Only TypeMovie and TypeShow are supported.
func List(params *trakt.ListByTypeParams) *trakt.CertificationIterator {
	return getC().List(params)
}

// List returns a list of all certifications, including names, slugs, and descriptions for a particular
// media type. Only TypeMovie and TypeShow are supported.
func (c *client) List(params *trakt.ListByTypeParams) *trakt.CertificationIterator {
	path := trakt.FormatURLPath("/certifications/%s", params.Type.Plural())
	return &trakt.CertificationIterator{
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

// getC retrieves an instance of a certification client.
func getC() *client { return &client{trakt.NewClient()} }
