package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// AllDebridClient is a minimal AllDebrid API client inspired by stremthru
type AllDebridClient struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	UserAgent  string
}

// NewAllDebridClient creates a new AllDebrid API client
func NewAllDebridClient(apiKey string) *AllDebridClient {
	return &AllDebridClient{
		BaseURL:   "https://api.alldebrid.com",
		APIKey:    apiKey,
		UserAgent: "momenarr",
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// MagnetUploadResponse represents magnet upload response
type MagnetUploadResponse struct {
	Status string `json:"status"`
	Data   struct {
		Magnets []struct {
			ID    int    `json:"id"`
			Hash  string `json:"hash"`
			Name  string `json:"name"`
			Size  int64  `json:"size"`
			Ready bool   `json:"ready"`
			Error *struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error,omitempty"`
		} `json:"magnets"`
	} `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// MagnetStatusResponse represents magnet status response
type MagnetStatusResponse struct {
	Status string `json:"status"`
	Data   struct {
		Magnet struct {
			ID         int    `json:"id"`
			Hash       string `json:"hash"`
			Name       string `json:"filename"`
			Status     string `json:"status"`
			StatusCode int    `json:"statusCode"`
			Size       int64  `json:"size"`
			Ready      bool   `json:"ready"`
			Files      []struct {
				Name string `json:"n"`
				Size int64  `json:"s"`
				Link string `json:"l"`
			} `json:"files,omitempty"`
		} `json:"magnets"`
	} `json:"data"`
}

// MagnetFilesResponse represents magnet files response
type MagnetFilesResponse struct {
	Status string `json:"status"`
	Data   struct {
		Magnets []struct {
			ID    int `json:"id"`
			Files []struct {
				Name string `json:"n"`
				Size int64  `json:"s"`
				Link string `json:"l"`
			} `json:"files"`
		} `json:"magnets"`
	} `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// LinkUnlockResponse represents link unlock response
type LinkUnlockResponse struct {
	Status string `json:"status"`
	Data   struct {
		Link     string `json:"link"`
		Filename string `json:"filename"`
		Filesize int64  `json:"filesize"`
		Delayed  int    `json:"delayed"`
	} `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// DeleteResponse represents delete response
type DeleteResponse struct {
	Status string `json:"status"`
	Data   struct {
		Message string `json:"message"`
	} `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// UploadMagnet uploads a magnet to AllDebrid
func (c *AllDebridClient) UploadMagnet(magnetURLs []string) (*MagnetUploadResponse, error) {
	form := url.Values{}
	form.Set("agent", c.UserAgent)
	form.Set("apikey", c.APIKey)
	for _, magnetURL := range magnetURLs {
		form.Add("magnets[]", magnetURL)
	}

	resp, err := c.HTTPClient.PostForm(c.BaseURL+"/v4/magnet/upload", form)
	if err != nil {
		return nil, fmt.Errorf("failed to upload magnet: %w", err)
	}
	defer resp.Body.Close()

	var result MagnetUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetMagnetStatus gets the status of a magnet by ID
func (c *AllDebridClient) GetMagnetStatus(id int) (*MagnetStatusResponse, error) {
	params := url.Values{}
	params.Set("agent", c.UserAgent)
	params.Set("apikey", c.APIKey)
	params.Set("id", strconv.Itoa(id))

	resp, err := c.HTTPClient.Get(c.BaseURL + "/v4.1/magnet/status?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("failed to get magnet status: %w", err)
	}
	defer resp.Body.Close()

	var result MagnetStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetMagnetFiles gets files for a magnet ID
func (c *AllDebridClient) GetMagnetFiles(id int) (*MagnetFilesResponse, error) {
	params := url.Values{}
	params.Set("agent", c.UserAgent)
	params.Set("apikey", c.APIKey)
	params.Set("id", strconv.Itoa(id))

	resp, err := c.HTTPClient.Get(c.BaseURL + "/v4/magnet/files?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("failed to get magnet files: %w", err)
	}
	defer resp.Body.Close()

	var result MagnetFilesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// UnlockLink unlocks a link to get direct download URL
func (c *AllDebridClient) UnlockLink(link string) (*LinkUnlockResponse, error) {
	params := url.Values{}
	params.Set("agent", c.UserAgent)
	params.Set("apikey", c.APIKey)
	params.Set("link", link)

	resp, err := c.HTTPClient.Get(c.BaseURL + "/v4/link/unlock?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("failed to unlock link: %w", err)
	}
	defer resp.Body.Close()

	var result LinkUnlockResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// DeleteMagnet deletes a magnet from AllDebrid
func (c *AllDebridClient) DeleteMagnet(id int) (*DeleteResponse, error) {
	form := url.Values{}
	form.Set("agent", c.UserAgent)
	form.Set("apikey", c.APIKey)
	form.Set("id", strconv.Itoa(id))

	resp, err := c.HTTPClient.PostForm(c.BaseURL+"/v4/magnet/delete", form)
	if err != nil {
		return nil, fmt.Errorf("failed to delete magnet: %w", err)
	}
	defer resp.Body.Close()

	var result DeleteResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}
