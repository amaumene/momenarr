package newsnab

import (
	"fmt"
	"io"
	"net/http"
)

func SearchTVShow(TVDB int64, showSeason int64, showEpisode int64, newsNabHost string, newsNabApiKey string) (string, error) {
	// Construct the URL with the provided arguments
	url := fmt.Sprintf("https://%s/api?apikey=%s&t=tvsearch&tvdbid=%d&season=%d&ep=%d", newsNabHost, newsNabApiKey, TVDB, showSeason, showEpisode)
	// Make the HTTP GET request
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("making request: %v", err)
	}
	defer resp.Body.Close()

	// Check if the request was successful
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("did not receive a 200 OK status, received %d", resp.StatusCode)
	}

	// Read the body of the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response body: %v", err)
	}

	return string(body), nil
}

func SearchMovie(IMDB string, newsNabHost string, newsNabApiKey string) (string, error) {
	if len(IMDB) > 2 {
		IMDB = IMDB[2:]
	} else {
		return "", fmt.Errorf("invalid IMDB ID")
	}
	// Construct the URL with the provided arguments
	url := fmt.Sprintf("https://%s/api?apikey=%s&t=movie&imdbid=%s", newsNabHost, newsNabApiKey, IMDB)
	fmt.Println(url)
	// Make the HTTP GET request
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("making request: %v", err)
	}
	defer resp.Body.Close()

	// Check if the request was successful
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("did not receive a 200 OK status, received %d", resp.StatusCode)
	}

	// Read the body of the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response body: %v", err)
	}

	return string(body), nil
}
