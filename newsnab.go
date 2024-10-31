package main

import "encoding/xml"

type Rss struct {
	XMLName xml.Name `xml:"rss"`
	Channel Channel  `xml:"channel"`
}

type Channel struct {
	Title       string   `xml:"title"`
	Description string   `xml:"description"`
	Link        string   `xml:"link"`
	Language    string   `xml:"language"`
	WebMaster   string   `xml:"webMaster"`
	Category    string   `xml:"category"`
	Response    Response `xml:"response"`
	Items       []Item   `xml:"item"`
}

type Response struct {
	Offset int `xml:"offset,attr"`
	Total  int `xml:"total,attr"`
}

type Item struct {
	Title       string    `xml:"title"`
	Guid        string    `xml:"guid"`
	Link        string    `xml:"link"`
	Comments    string    `xml:"comments"`
	PubDate     string    `xml:"pubDate"`
	Category    string    `xml:"category"`
	Description string    `xml:"description"`
	Enclosure   Enclosure `xml:"enclosure"`
}

type Enclosure struct {
	URL    string `xml:"url,attr"`
	Length string `xml:"length,attr"`
	Type   string `xml:"type,attr"`
}
