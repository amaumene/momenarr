package main

import (
	"github.com/amaumene/momenarr/bolthold"
	"github.com/amaumene/momenarr/sabnzbd"
	"github.com/amaumene/momenarr/trakt"
)

type App struct {
	TraktToken *trakt.Token
	Store      *bolthold.Store
	//NZBGet     *nzbget.NZBGet
	SabNZBd *sabnzbd.Client
	Config  *Config
}

type Config struct {
	DownloadDir   string
	DataDir       string
	NewsNabHost   string
	NewsNabApiKey string
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
	DownloadID string
}

type NZB struct {
	IMDB   string `boltholdIndex:"IMDB"`
	Link   string
	Length int64
	Title  string
	Failed bool
}

type Notification struct {
	Id       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Status   string `json:"status"`
	Dir      string `json:"dir"`
}
