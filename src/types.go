package main

import (
	"github.com/amaumene/momenarr/bolthold"
	"github.com/amaumene/momenarr/nzbget"
	"github.com/amaumene/momenarr/trakt"
)

type App struct {
	TraktToken *trakt.Token
	Store      *bolthold.Store
	NZBGet     *nzbget.NZBGet
	Config     *Config
}

type Config struct {
	DownloadDir   string
	DataDir       string
	NewsNabHost   string
	NewsNabApiKey string
}

type Media struct {
	Trakt      int64 `boltholdIndex:"Trakt"`
	IMDB       string
	Number     int64
	Season     int64
	Title      string
	Year       int64
	OnDisk     bool
	File       string
	DownloadID int64
}

type NZB struct {
	Trakt  int64 `boltholdIndex:"Trakt"`
	Link   string
	Length int64
	Title  string
	Failed bool
}

type Notification struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Status   string `json:"status"`
	Trakt    int64  `json:"trakt"`
	Dir      string `json:"dir"`
}
