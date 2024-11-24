package main

import (
	"github.com/amaumene/momenarr/bolthold"
	"github.com/amaumene/momenarr/nzbget"
	"github.com/amaumene/momenarr/trakt"
)

type App struct {
	downloadDir   string
	tempDir       string
	dataDir       string
	newsNabHost   string
	newsNabApiKey string
	traktToken    *trakt.Token
	store         *bolthold.Store
	nzbget        *nzbget.NZBGet
}

type Media struct {
	IMDB       string `boltholdIndex:"IMDB"`
	TVDB       int64
	Number     int64
	Season     int64
	Title      string
	Year       int64
	OnDisk     bool
	File       string
	DownloadID int64
}

type NZB struct {
	IMDB   string `boltholdIndex:"IMDB"`
	Link   string
	Length int64
	Title  string
	Failed bool
}

type Notification struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Status   string `json:"status"`
	IMDB     string `json:"imdb"`
	Dir      string `json:"dir"`
}
