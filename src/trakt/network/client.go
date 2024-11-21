// Package network gives functions to retrieve TV network information.
package network

import (
	"net/http"

	"github.com/jacklaaa89/trakt"
)

// client represents a network client.
type client struct{ b trakt.BaseClient }

// List retrieves a list of all TV networks, including the name.
func List(params *trakt.BasicParams) *trakt.NetworkIterator { return getC().List(params) }

// List retrieves a list of all TV networks, including the name.
func (c *client) List(params *trakt.BasicParams) *trakt.NetworkIterator {
	return &trakt.NetworkIterator{BasicIterator: c.b.NewSimulatedIterator(http.MethodGet, "/networks", params)}
}

// getC initialises a new network client with the currently defined backend configuration.
func getC() *client { return &client{trakt.NewClient()} }
