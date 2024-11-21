// Package checkin allows user to manually signal that they are watching a media item.
//
// Checking in is a manual action used by mobile apps allowing the user to indicate what
// they are watching right now. While not as effortless as scrobbling, checkins help fill in the gaps.
// You might be watching live tv, at a friend's house, or watching a movie in theaters. You can simply
// checkin from your phone or tablet in those situations. The item will display as watching on the site,
// then automatically switch to watched status once the duration has elapsed.
package checkin

import (
	"net/http"

	"github.com/jacklaaa89/trakt"
)

// client represents a client which can be used to perform checkin requests.
type client struct{ b trakt.BaseClient }

// Start Check into a movie or episode. This should be tied to a user action to manually indicate
// they are watching something. The item will display as watching on the site, then
// automatically switch to watched status once the duration has elapsed. A unique history
// id (64-bit integer) will be returned and can be used to reference this checkin directly.
//
// If a checkin is already in progress, an error will be returned with the Code:
// ErrorCodeCheckinInProgress.
//
// All elements on the Element field are optional, but the more that is given means that
// the media object can be matched more accurately. Use the trakt ID or slug where possible
// as these always have a mapping.
//
//   - OAuth Required
func Start(params *trakt.StartCheckinParams) (*trakt.Checkin, error) {
	return getC().Start(params)
}

// Start Check into a movie or episode. This should be tied to a user action to manually indicate
// they are watching something. The item will display as watching on the site, then
// automatically switch to watched status once the duration has elapsed. A unique history
// id (64-bit integer) will be returned and can be used to reference this checkin directly.
//
// If a checkin is already in progress, an error will be returned with the Code:
// ErrorCodeCheckinInProgress.
//
// All elements on the Element field are optional, but the more that is given means that
// the media object can be matched more accurately. Use the trakt ID or slug where possible
// as these always have a mapping.
//
//   - OAuth Required
func (c *client) Start(params *trakt.StartCheckinParams) (*trakt.Checkin, error) {
	switch params.Type {
	case trakt.TypeMovie, trakt.TypeEpisode:
		break
	default:
		return nil, &trakt.Error{
			HTTPStatusCode: http.StatusUnprocessableEntity,
			Body:           "invalid type: only movie / episode are applicable",
			Resource:       "/checkin",
			Code:           trakt.ErrorCodeValidationError,
		}
	}

	ci := &trakt.Checkin{}
	p := &wrappedCheckinParams{params}
	err := c.b.Call(http.MethodPost, "/checkin", p, ci)
	return ci, err
}

// Stop removes any active check-ins, no need to provide a specific item.
//
//   - OAuth Required
func Stop(params *trakt.Params) error {
	return getC().Stop(params)
}

// Stop removes any active check-ins, no need to provide a specific item.
//
//   - OAuth Required
func (c *client) Stop(params *trakt.Params) error {
	return c.b.Call(http.MethodDelete, "/checkin", params, nil)
}

// wrappedCheckinParams provides a wrapper around checkin parameters
// which allow us to capture the status code to respond to specific errors
// related to attempting to start a checkin action.
type wrappedCheckinParams struct{ *trakt.StartCheckinParams }

// Code implements ErrorHandler interface.
func (wrappedCheckinParams) Code(statusCode int) trakt.ErrorCode {
	if statusCode == http.StatusConflict {
		return trakt.ErrorCodeCheckinInProgress
	}

	return trakt.DefaultErrorHandler.Code(statusCode)
}

// getC initialises a new checkin client with the currently defined backend.
func getC() *client { return &client{trakt.NewClient()} }
