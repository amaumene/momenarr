package main

import (
	"fmt"
	"github.com/jacklaaa89/trakt"
	"io"
	"net/http"
)

type Feed struct {
	Attributes FeedAttributes `json:"@attributes"`
	Channel    Channel        `json:"channel"`
}

type FeedAttributes struct {
	Version string `json:"version"`
}

type Channel struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Link        string   `json:"link"`
	Language    string   `json:"language"`
	WebMaster   string   `json:"webMaster"`
	Category    struct{} `json:"category"`
	Response    Response `json:"response"`
	Item        []Item   `json:"item"`
}

type Response struct {
	Attributes ResponseAttributes `json:"@attributes"`
}

type ResponseAttributes struct {
	Offset string `json:"offset"`
	Total  string `json:"total"`
}

type Item struct {
	Title    string `json:"title"`
	GUID     string `json:"guid"`
	Link     string `json:"link"`
	Comments string `json:"comments"`
	PubDate  string `json:"pubDate"`
	//Category    string    `json:"category"`
	Description string    `json:"description"`
	Enclosure   Enclosure `json:"enclosure"`
	Failed      bool      `json:"failed"`
}

type Enclosure struct {
	Attributes EnclosureAttributes `json:"@attributes"`
}

type EnclosureAttributes struct {
	URL    string `json:"url"`
	Length string `json:"length"`
	Type   string `json:"type"`
}

func searchTVShow(TVDB trakt.TVDB, showSeason int, showEpisode int, appConfig App) (string, error) {
	// Construct the URL with the provided arguments
	url := fmt.Sprintf("https://%s/api?apikey=%s&t=tvsearch&tvdbid=%d&season=%d&ep=%d&o=json", appConfig.newsNabHost, appConfig.newsNabApiKey, TVDB, showSeason, showEpisode)

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

func searchMovie(IMDB int64, appConfig App) (string, error) {
	// Construct the URL with the provided arguments
	url := fmt.Sprintf("https://%s/api?apikey=%s&t=movie&imdbid=%d&o=json", appConfig.newsNabHost, appConfig.newsNabApiKey, IMDB)
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
