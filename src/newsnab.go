package main

import (
	"encoding/xml"
	"fmt"
	"github.com/jacklaaa89/trakt"
	"io"
	"net/http"
	"strings"
)

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

func searchTVShow(TVDB trakt.TVDB, showSeason int, showEpisode int) (string, error) {
	// Construct the URL with the provided arguments
	url := fmt.Sprintf("https://%s/api?apikey=%s&t=tvsearch&tvdbid=%d&season=%d&ep=%d", newsNabHost, newsNabApiKey, TVDB, showSeason, showEpisode)

	// Make the HTTP GET request
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	// Check if the request was successful
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error: did not receive a 200 OK status, received %d", resp.StatusCode)
	}

	// Read the body of the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %v", err)
	}

	return string(body), nil
}

func searchMovie(IMDB trakt.IMDB) (string, error) {
	imdbID := strings.TrimPrefix(string(IMDB), "tt")
	// Construct the URL with the provided arguments
	url := fmt.Sprintf("https://%s/api?apikey=%s&t=movie&imdbid=%s", newsNabHost, newsNabApiKey, imdbID)
	fmt.Println(url)

	// Make the HTTP GET request
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	// Check if the request was successful
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error: did not receive a 200 OK status, received %d", resp.StatusCode)
	}

	// Read the body of the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %v", err)
	}

	return string(body), nil
}
