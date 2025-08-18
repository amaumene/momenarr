package models

import "time"

// Media represents a media item (movie or TV episode) in the system
type Media struct {
	Trakt        int64     `json:"trakt" boltholdIndex:"Trakt" validate:"required"`
	IMDB         string    `json:"imdb,omitempty"`
	TMDBID       int64     `json:"tmdb_id,omitempty"`
	ShowTMDBID   int64     `json:"show_tmdb_id,omitempty"`
	ShowTitle    string    `json:"show_title,omitempty"`
	Number       int64     `json:"number,omitempty"`
	Season       int64     `json:"season,omitempty"`
	Title        string    `json:"title" validate:"required"`
	Year         int64     `json:"year,omitempty"`
	OnDisk       bool      `json:"on_disk"`
	File         string    `json:"file,omitempty"`
	TransferID   string    `json:"transfer_id,omitempty"`
	DownloadID   int64     `json:"download_id,omitempty"`
	IsSeasonPack bool      `json:"is_season_pack,omitempty"`
	SeasonPackID int64     `json:"season_pack_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (m *Media) IsEpisode() bool {
	return m.Season > 0 && m.Number > 0
}

func (m *Media) IsMovie() bool {
	return !m.IsEpisode()
}

// MediaType represents the type of media
type MediaType string

const (
	MediaTypeMovie   MediaType = "movie"
	MediaTypeEpisode MediaType = "episode"
)

func (m *Media) GetType() MediaType {
	if m.IsEpisode() {
		return MediaTypeEpisode
	}
	return MediaTypeMovie
}
