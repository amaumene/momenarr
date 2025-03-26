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
	DownloadID string
}

type NZB struct {
	Trakt  int64 `boltholdIndex:"Trakt"`
	Link   string
	Length int64
	Title  string
	Failed bool
}

type Failure struct {
	Type    string `json:"type"`
	Title   string `json:"title"`
	Message string `json:"message"`
}

type Success struct {
	Name     string `json:"name"`
	Id       string `json:"id"`
	Category string `json:"category"`
	Dir      string `json:"dir"`
	Status   string `json:"status"`
}
