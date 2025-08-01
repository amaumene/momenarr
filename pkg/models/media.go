// Package models defines data structures used throughout the momenarr application.
package models

import "time"

// Media represents a media item (movie or TV episode) in the system.
type Media struct {
	Trakt            int64     `json:"trakt" boltholdIndex:"Trakt" validate:"required"`
	TMDBID           int64     `json:"tmdb_id,omitempty"`
	OriginalLanguage string    `json:"original_language,omitempty"`
	FrenchTitle      string    `json:"french_title,omitempty"`
	Number           int64     `json:"number,omitempty"`
	Season           int64     `json:"season,omitempty"`
	Title            string    `json:"title" validate:"required"`
	Year             int64     `json:"year,omitempty"`
	OnDisk           bool      `json:"on_disk"`
	File             string    `json:"file,omitempty"`
	DownloadID       int64     `json:"download_id,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// IsEpisode checks if the media is a TV episode.
func (m *Media) IsEpisode() bool {
	return m.Season > 0 && m.Number > 0
}

// IsMovie checks if the media is a movie.
func (m *Media) IsMovie() bool {
	return !m.IsEpisode()
}

// MediaType represents the type of media.
type MediaType string

const (
	MediaTypeMovie   MediaType = "movie"
	MediaTypeEpisode MediaType = "episode"
)

// GetType returns the media type (movie or episode).
func (m *Media) GetType() MediaType {
	if m.IsEpisode() {
		return MediaTypeEpisode
	}
	return MediaTypeMovie
}
