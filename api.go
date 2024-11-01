package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

type APIResponse struct {
	Success bool        `json:"success"`
	Error   interface{} `json:"error"`
	Detail  string      `json:"detail"`
	Data    []APIData   `json:"data"`
}

type APIData struct {
	ID               int         `json:"id"`
	Name             string      `json:"name"`
	CreatedAt        string      `json:"created_at"`
	UpdatedAt        string      `json:"updated_at"`
	AuthID           string      `json:"auth_id"`
	Hash             string      `json:"hash"`
	DownloadState    string      `json:"download_state"`
	DownloadSpeed    int         `json:"download_speed"`
	OriginalURL      interface{} `json:"original_url"`
	Eta              int         `json:"eta"`
	Progress         float64     `json:"progress"`
	Size             int64       `json:"size"`
	DownloadID       string      `json:"download_id"`
	Files            []APIFile   `json:"files"`
	Active           bool        `json:"active"`
	Cached           bool        `json:"cached"`
	DownloadPresent  bool        `json:"download_present"`
	DownloadFinished bool        `json:"download_finished"`
	ExpiresAt        string      `json:"expires_at"`
}

type APIFile struct {
	ID           int    `json:"id"`
	Md5          string `json:"md5"`
	Hash         string `json:"hash"`
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	S3Path       string `json:"s3_path"`
	MimeType     string `json:"mimetype"`
	ShortName    string `json:"short_name"`
	AbsolutePath string `json:"absolute_path"`
}

type DownloadResponse struct {
	Success bool        `json:"success"`
	Error   interface{} `json:"error"`
	Detail  string      `json:"detail"`
	Data    string      `json:"data"`
}

var serverResponse struct {
	Success bool   `json:"success"`
	Detail  string `json:"detail"`
	Data    struct {
		Hash             string `json:"hash"`
		UsenetDownloadID int    `json:"usenetdownload_id"`
		AuthID           string `json:"auth_id"`
	} `json:"data"`
}

func performGetRequest(url, token string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create API request: %v", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform API request: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read API response: %v", err)
	}

	return respBody, nil
}

func findMatchingItemByName(apiResponse APIResponse, extractedString string) (int, APIFile, error) {
	for _, item := range apiResponse.Data {
		if item.Name == extractedString {
			for _, file := range item.Files {
				if strings.HasPrefix(file.MimeType, "video/") && !strings.Contains(file.ShortName, "sample") {
					return item.ID, file, nil
				}
			}
		}
	}
	return 0, APIFile{}, fmt.Errorf("no matching item found for name %s", extractedString)
}

func findMatchingItemID(apiResponse APIResponse, itemID int) (int, APIFile, error) {
	for _, item := range apiResponse.Data {
		if item.ID == itemID {
			for _, file := range item.Files {
				if strings.HasPrefix(file.MimeType, "video/") && !strings.Contains(file.ShortName, "sample") {
					return item.ID, file, nil
				}
			}
		}
	}
	return 0, APIFile{}, fmt.Errorf("no matching item found for ID %d", itemID)
}

func requestDownload(itemID int, file APIFile, token string) error {
	url := fmt.Sprintf("%s?token=%s&usenet_id=%d&file_id=%d&zip=false", requestDLURL, token, itemID, file.ID)

	respBody, err := performGetRequest(url, token)
	if err != nil {
		return err
	}

	var downloadResponse DownloadResponse
	err = json.Unmarshal(respBody, &downloadResponse)
	if err != nil {
		return fmt.Errorf("failed to parse download API response: %v", err)
	}

	if !downloadResponse.Success {
		return fmt.Errorf("failed to request download")
	}

	fmt.Printf("Download requested successfully for %s\n", file.Name)
	err = downloadFile(downloadResponse.Data, file)
	if err != nil {
		return fmt.Errorf("failed to download file: %v", err)
	}
	return deleteFile(itemID, token)
}

func deleteFile(itemID int, token string) error {
	data := map[string]interface{}{
		"usenet_id": itemID,
		"operation": "delete",
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %v", err)
	}

	req, err := http.NewRequest("POST", controlUsenetURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to perform request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete file, status: %s", resp.Status)
	}
	fmt.Printf("File deleted successfully\n")
	return nil
}

func uploadFileWithRetries(link string, title string) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		err = uploadFile(link, title)
		if err == nil {
			return nil
		}
		log.Printf("Upload failed: %v. Retrying in %v...\n", err, retryDelay)
		time.Sleep(retryDelay)
	}
	return fmt.Errorf("failed to upload file after %d attempts: %v", maxRetries, err)
}

func uploadFile(link string, title string) error {

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add the name field without the extension
	err := writer.WriteField("name", title)
	if err != nil {
		return fmt.Errorf("failed to write name field: %v", err)
	}

	err = writer.WriteField("link", link)
	if err != nil {
		return fmt.Errorf("failed to write link field: %v", err)
	}

	err = writer.Close()
	if err != nil {
		return fmt.Errorf("failed to close writer: %v", err)
	}

	req, err := http.NewRequest("POST", createUsenetDLURL, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", torboxApiKey))

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload file: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to upload file, status: %s", resp.Status)
	}

	// Read and print the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	err = json.Unmarshal(respBody, &serverResponse)
	if err != nil {
		return fmt.Errorf("failed to parse response body: %v", err)
	}

	if serverResponse.Success != true {
		return fmt.Errorf("failed to upload file: %s", serverResponse.Detail)
	}
	fmt.Println("File uploaded successfully:", serverResponse.Detail)

	if serverResponse.Detail == "Found cached usenet download. Using cached download." {
		fmt.Println("Found cached usenet download. Using cached download.")
		respBody, err := performGetRequest(apiURL, torboxApiKey)
		if err != nil {
			return fmt.Errorf("failed to perform API request: %v", err)
		}

		var apiResponse APIResponse
		err = json.Unmarshal(respBody, &apiResponse)
		if err != nil {
			return fmt.Errorf("failed to parse API response: %v", err)
		}
		itemID, file, err := findMatchingItemID(apiResponse, serverResponse.Data.UsenetDownloadID)
		if err != nil {
			return fmt.Errorf("failed to find matching item: %v", err)
		}

		err = requestDownload(itemID, file, torboxApiKey)
		if err != nil {
			return fmt.Errorf("failed to request download: %v", err)
		}
	}
	return nil
}
