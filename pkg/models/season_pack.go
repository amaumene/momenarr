package models

import "time"

type SeasonPack struct {
	ID            int64     `json:"id" boltholdKey:"ID"`
	ShowTMDBID    int64     `json:"show_tmdb_id" boltholdIndex:"ShowTMDBID"`
	ShowTitle     string    `json:"show_title"`
	Season        int64     `json:"season"`
	TotalEpisodes int       `json:"total_episodes"`
	TransferID    string    `json:"transfer_id"`
	Episodes      []int64   `json:"episodes"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type SeasonWatchStatus struct {
	ShowTMDBID      int64     `json:"show_tmdb_id"`
	ShowTitle       string    `json:"show_title"`
	Season          int64     `json:"season"`
	TotalEpisodes   int       `json:"total_episodes"`
	WatchedEpisodes int       `json:"watched_episodes"`
	WatchedList     []int64   `json:"watched_list"`
	IsComplete      bool      `json:"is_complete"`
	SeasonPackID    int64     `json:"season_pack_id,omitempty"`
	LastWatchedAt   time.Time `json:"last_watched_at"`
}