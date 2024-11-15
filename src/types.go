package main

type Movie struct {
	IMDB       int64 `boltholdIndex:"IMDB"`
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
