package main

import (
	"fmt"
	"github.com/jacklaaa89/trakt"
	"io/ioutil"
	"net/http"
)

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
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %v", err)
	}

	return string(body), nil
}
