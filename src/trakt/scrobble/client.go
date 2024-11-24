// Package scrobble provides an automatic way to track what a user is watching.
//
// Scrobbling is an automatic way to track what a user is watching in a media center.
// The media center should send events that correspond to starting, pausing, and stopping
// (or finishing) watching a movie or episode.
package scrobble

import (
	"net/http"

	"github.com/amaumene/momenarr/trakt"
)

// client represents a client which can be used to perform scrobble requests.
type client struct{ b trakt.BaseClient }

// Start use this method when the video initially starts playing or is un-paused. This will remove
// any playback progress if it exists.
//
// Note: A watching status will auto expire after the remaining runtime has elapsed.
// There is no need to call this method again while continuing to watch the same item.
//
//   - OAuth Required
func Start(params *trakt.ScrobbleParams) (*trakt.Scrobble, error) {
	return getC().Start(params)
}

// Start use this method when the video initially starts playing or is un-paused. This will remove
// any playback progress if it exists.
//
// Note: A watching status will auto expire after the remaining runtime has elapsed.
// There is no need to call this method again while continuing to watch the same item.
//
//   - OAuth Required
func (c *client) Start(params *trakt.ScrobbleParams) (*trakt.Scrobble, error) {
	s := &trakt.Scrobble{}
	err := c.b.Call(http.MethodPost, "/scrobble/start", params, s)
	return s, err
}

// Pause use this method when the video is paused. The playback progress will be saved
// and "sync.Playbacks" can be used to resume the video from this exact position.
// Unpause a video by calling the "Start method again.
//
//   - OAuth Required
func Pause(params *trakt.ScrobbleParams) (*trakt.Scrobble, error) {
	return getC().Pause(params)
}

// Pause use this method when the video is paused. The playback progress will be saved
// and "sync.Playbacks" can be used to retrieve the playback position to resume the
// video in the exact position.
//
//   - OAuth Required
func (c *client) Pause(params *trakt.ScrobbleParams) (*trakt.Scrobble, error) {
	s := &trakt.Scrobble{}
	err := c.b.Call(http.MethodPost, "/scrobble/pause", params, s)
	return s, err
}

// Stop use this method when the video is stopped or finishes playing on its own.
// If the progress is above 80%, the video will be scrobbled and the action will be
// set to scrobble.
//
// A unique history id (64-bit integer) will be returned and can be used to reference
// this scrobble directly.
//
// If the progress is less than 80%, it will be treated as a pause and the action will
// be set to pause. The playback progress will be saved and "sync.Playbacks" can be used to retrieve the
// playback position to resume the video in the exact position.
//
// Note: If you prefer to use a threshold higher than 80%, you should use "Pause" yourself so
// it doesn't create duplicate scrobbles.
func Stop(params *trakt.ScrobbleParams) (*trakt.Scrobble, error) {
	return getC().Stop(params)
}

// Stop use this method when the video is stopped or finishes playing on its own.
// If the progress is above 80%, the video will be scrobbled and the action will be
// set to scrobble.
//
// A unique history id (64-bit integer) will be returned and can be used to reference
// this scrobble directly.
//
// If the progress is less than 80%, it will be treated as a pause and the action will
// be set to pause. The playback progress will be saved and "sync.Playbacks" can be used to retrieve the
// playback position to resume the video in the exact position.
//
// Note: If you prefer to use a threshold higher than 80%, you should use "Pause" yourself so
// it doesn't create duplicate scrobbles.
func (c *client) Stop(params *trakt.ScrobbleParams) (*trakt.Scrobble, error) {
	s := &trakt.Scrobble{}
	err := c.b.Call(http.MethodPost, "/scrobble/stop", params, s)
	return s, err
}

// getC initialises a new scrobble client with the currently defined backend.
func getC() *client { return &client{trakt.NewClient()} }
