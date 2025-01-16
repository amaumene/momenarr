package main

import (
	"github.com/amaumene/momenarr/bolthold"
	"github.com/amaumene/momenarr/sabnzbd"
	"github.com/amaumene/momenarr/trakt"
)

type App struct {
	TraktToken *trakt.Token
	Store      *bolthold.Store
	SabNZBd    *sabnzbd.Client
	Config     *Config
}

type Config struct {
	DownloadDir   string
	DataDir       string
	NewsNabHost   string
	NewsNabApiKey string
}

type Media struct {
	IMDB       string `boltholdIndex:"IMDB"`
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
