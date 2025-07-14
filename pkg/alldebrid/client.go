package alldebrid

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func findJsonEnd(s string) int {
	if len(s) == 0 || s[0] != '{' {
		return -1
	}

	depth := 0
	inString := false
	escaped := false

	for i, r := range s {
		if escaped {
			escaped = false
			continue
		}

		if r == '\\' && inString {
			escaped = true
			continue
		}

		if r == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		if r == '{' {
			depth++
		} else if r == '}' {
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}

	return -1
}

func parseJSONResponse(body []byte, result interface{}) error {
	cleanBody := strings.TrimSpace(string(body))

	if strings.Contains(cleanBody, "}{") {
		parts := strings.Split(cleanBody, "}{")
		if len(parts) >= 2 {
			cleanBody = "{" + parts[len(parts)-1]
		}
	}

	if err := json.Unmarshal([]byte(cleanBody), result); err != nil {
		if jsonStart := strings.Index(cleanBody, "{"); jsonStart >= 0 {
			jsonEnd := findJsonEnd(cleanBody[jsonStart:])
			if jsonEnd > 0 {
				jsonOnly := cleanBody[jsonStart : jsonStart+jsonEnd]
				return json.Unmarshal([]byte(jsonOnly), result)
			}
		}
		return err
	}
	return nil
}

// Client represents an AllDebrid API client
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient creates a new AllDebrid API client
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://api.alldebrid.com/v4",
	}
}

// MagnetUploadResponse represents the response from magnet upload endpoint
type MagnetUploadResponse struct {
	Status string `json:"status"`
	Data   struct {
		Magnets []struct {
			ID    int64         `json:"id"`
			Hash  string        `json:"hash"`
			Name  string        `json:"name"`
			Size  int64         `json:"size"`
			Ready bool          `json:"ready"`
			Files []interface{} `json:"files"`
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

// LinkUnlockResponse represents the response from link unlock endpoint
type LinkUnlockResponse struct {
	Status string `json:"status"`
	Data   struct {
		Link     string        `json:"link"`
		Host     string        `json:"host"`
		Filename string        `json:"filename"`
		Filesize int64         `json:"filesize"`
		ID       string        `json:"id"`
		Streams  []interface{} `json:"streams"`
	} `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// MagnetFilesResponse represents the response from magnet files endpoint
type MagnetFilesResponse struct {
	Status string `json:"status"`
	Data   struct {
		Magnets []struct {
			ID    int64  `json:"id"`
			Hash  string `json:"hash"`
			Name  string `json:"name"`
			Size  int64  `json:"size"`
			Ready bool   `json:"ready"`
			Links []struct {
				Link     string `json:"link"`
				Filename string `json:"filename"`
				Size     int64  `json:"size"`
			} `json:"links"`
		} `json:"magnets"`
	} `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// MagnetStatusResponse represents the response from magnet status endpoint
type MagnetStatusResponse struct {
	Status string `json:"status"`
	Data   struct {
		Magnets []struct {
			ID         int64  `json:"id"`
			Hash       string `json:"hash"`
			Name       string `json:"name"`
			Status     string `json:"status"`
			StatusCode int    `json:"statusCode"`
			Size       int64  `json:"size"`
			UploadDate int64  `json:"uploadDate"`
			Ready      bool   `json:"ready"`
			Links      []struct {
				Link     string `json:"link"`
				Filename string `json:"filename"`
				Size     int64  `json:"size"`
			} `json:"links"`
		} `json:"magnets"`
	} `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// MagnetDeleteResponse represents the response from magnet delete endpoint
type MagnetDeleteResponse struct {
	Status string `json:"status"`
	Data   struct {
		Message string `json:"message"`
	} `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// UploadMagnet uploads a magnet link to AllDebrid
func (c *Client) UploadMagnet(apiKey string, magnetURLs []string) (*MagnetUploadResponse, error) {
	endpoint := fmt.Sprintf("%s/magnet/upload", c.baseURL)

	formData := url.Values{}
	formData.Set("agent", "stremio")
	formData.Set("apikey", apiKey)

	// Add all magnet URLs
	for _, magnetURL := range magnetURLs {
		formData.Add("magnets[]", magnetURL)
	}

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for HTTP error status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AllDebrid API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result MagnetUploadResponse
	if err := parseJSONResponse(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Check for API-level errors
	if result.Status != "success" && result.Error != nil {
		return nil, fmt.Errorf("AllDebrid API error: %s - %s", result.Error.Code, result.Error.Message)
	}

	return &result, nil
}

// UnlockLink unlocks a link to get direct download URL
func (c *Client) UnlockLink(apiKey, link string) (*LinkUnlockResponse, error) {
	endpoint := fmt.Sprintf("%s/link/unlock", c.baseURL)

	params := url.Values{}
	params.Set("agent", "stremio")
	params.Set("apikey", apiKey)
	params.Set("link", link)

	fullURL := fmt.Sprintf("%s?%s", endpoint, params.Encode())

	resp, err := c.httpClient.Get(fullURL)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var result LinkUnlockResponse
	if err := parseJSONResponse(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode unlock response: %w", err)
	}

	return &result, nil
}

// GetMagnetFiles gets files for a magnet ID
func (c *Client) GetMagnetFiles(apiKey, magnetID string) (*MagnetFilesResponse, error) {
	endpoint := fmt.Sprintf("%s/magnet/files", c.baseURL)

	params := url.Values{}
	params.Set("agent", "stremio")
	params.Set("apikey", apiKey)
	params.Set("id", magnetID)

	fullURL := fmt.Sprintf("%s?%s", endpoint, params.Encode())

	resp, err := c.httpClient.Get(fullURL)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var result MagnetFilesResponse
	if err := parseJSONResponse(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode files response: %w", err)
	}

	return &result, nil
}

// CheckMagnetCache checks if magnets are cached by their hashes
func (c *Client) CheckMagnetCache(apiKey string, hashes []string) (*MagnetStatusResponse, error) {
	endpoint := fmt.Sprintf("%s/magnet/status", c.baseURL)

	formData := url.Values{}
	formData.Set("agent", "stremio")
	formData.Set("apikey", apiKey)

	// Add magnet hashes
	for _, hash := range hashes {
		formData.Add("magnets[]", hash)
	}

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var result MagnetStatusResponse
	if err := parseJSONResponse(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode cache check response: %w", err)
	}

	return &result, nil
}

// GetMagnetStatus gets the status of magnets by IDs
func (c *Client) GetMagnetStatus(apiKey string, magnetIDs []int64) (*MagnetStatusResponse, error) {
	endpoint := fmt.Sprintf("%s/magnet/status", c.baseURL)

	params := url.Values{}
	params.Set("agent", "stremio")
	params.Set("apikey", apiKey)

	// Add magnet IDs
	for _, id := range magnetIDs {
		params.Add("id[]", fmt.Sprintf("%d", id))
	}

	fullURL := fmt.Sprintf("%s?%s", endpoint, params.Encode())

	resp, err := c.httpClient.Get(fullURL)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var result MagnetStatusResponse
	if err := parseJSONResponse(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode status response: %w", err)
	}

	return &result, nil
}

// DeleteMagnet deletes a magnet from AllDebrid
func (c *Client) DeleteMagnet(apiKey string, magnetID int64) (*MagnetDeleteResponse, error) {
	endpoint := fmt.Sprintf("%s/magnet/delete", c.baseURL)

	formData := url.Values{}
	formData.Set("agent", "stremio")
	formData.Set("apikey", apiKey)
	formData.Set("id", fmt.Sprintf("%d", magnetID))

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var result MagnetDeleteResponse
	if err := parseJSONResponse(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode delete response: %w", err)
	}

	return &result, nil
}
