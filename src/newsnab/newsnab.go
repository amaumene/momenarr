package newsnab

import (
	"fmt"
	"io"
	"net/http"
)

func searchTVShow(TVDB int64, showSeason int, showEpisode int, newsNabHost string, newsNabApiKey string) (string, error) {
	// Construct the URL with the provided arguments
	url := fmt.Sprintf("https://%s/api?apikey=%s&t=tvsearch&tvdbid=%d&season=%d&ep=%d&o=json", newsNabHost, newsNabApiKey, TVDB, showSeason, showEpisode)

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

func SearchMovie(IMDB int64, newsNabHost string, newsNabApiKey string) (string, error) {
	// Construct the URL with the provided arguments
	url := fmt.Sprintf("https://%s/api?apikey=%s&t=movie&imdbid=%d&o=json", newsNabHost, newsNabApiKey, IMDB)
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
