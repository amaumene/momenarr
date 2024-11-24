package main

import (
	"github.com/amaumene/momenarr/nzbget"
	"github.com/amaumene/momenarr/trakt"
	"github.com/timshannon/bolthold"
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
	TVDB       int64  `boltholdIndex:"TVDB"`
	Number     int64
	Season     int64
	Title      string
	Year       int64
	OnDisk     bool
	File       string
	downloadID int64
}

type NZB struct {
	IMDB   string `boltholdIndex:"IMDB"`
	Link   string `boltholdIndex:"Link"`
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
