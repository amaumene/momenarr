package main

import (
	"github.com/amaumene/momenarr/torbox"
	"github.com/jacklaaa89/trakt"
	"github.com/timshannon/bolthold"
)

type App struct {
	downloadDir        string
	tempDir            string
	dataDir            string
	newsNabHost        string
	newsNabApiKey      string
	traktToken         *trakt.Token
	torBoxClient       torbox.TorBox
	torBoxMoviesFolder string
	torBoxShowsFolder  string
	store              *bolthold.Store
}

type Media struct {
	IMDB       int64 `boltholdIndex:"IMDB"`
	TVDB       int64 `boltholdIndex:"TVDB"`
	Number     int64
	Season     int64
	Title      string
	Year       int64
	OnDisk     bool
	File       string
	DownloadID int
}

type NZB struct {
	ID     int64  `boltholdIndex:"ID"`
	Link   string `boltholdIndex:"Link"`
	Length int64
	Title  string
	Failed bool
}
