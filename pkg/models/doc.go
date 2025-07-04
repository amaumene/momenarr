// Package models defines the core data structures used throughout the Momenarr application.
//
// It includes:
//   - Media: Represents movies and TV episodes tracked from Trakt
//   - NZB: Represents downloadable NZB files from Usenet indexers
//   - Notification: Represents download status notifications from NZBGet
//
// All models include appropriate serialization tags for database storage
// and JSON API responses.
package models