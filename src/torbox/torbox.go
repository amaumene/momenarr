package torbox

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

type TorBox struct {
	APIKey string
}

func NewTorBoxClient(APIKey string) TorBox {
	return TorBox{APIKey: APIKey}
}

func (pm *TorBox) createTorBoxURL(urlPath string) (url.URL, error) {
	u, err := url.Parse("https://api.torbox.app/v1/api")
	if err != nil {
		return *u, err
	}
	u.Path = path.Join(u.Path, urlPath)
	q := u.Query()
	//q.Set("apikey", pm.APIKey)
	u.RawQuery = q.Encode()
	return *u, nil
}

func (pm *TorBox) createTorBoxURLrequestDL(urlPath string, usenetId int, fileId int) (url.URL, error) {
	u, err := url.Parse("https://api.torbox.app/v1/api")
	if err != nil {
		return *u, err
	}
	u.Path = path.Join(u.Path, urlPath)
	q := u.Query()
	q.Set("token", pm.APIKey)
	q.Set("usenet_id", strconv.Itoa(usenetId))
	q.Set("file_id", strconv.Itoa(fileId))
	u.RawQuery = q.Encode()
	return *u, nil
}

var (
	ErrAPIKeyNotSet = fmt.Errorf("TorBox API key not set")
)

func (pm *TorBox) CreateUsenetDownload(link string, name string) (UsenetCreateDownloadResponse, error) {
	UsenetCreateDownloadResponse := UsenetCreateDownloadResponse{}
	if pm.APIKey == "" {
		return UsenetCreateDownloadResponse, ErrAPIKeyNotSet
	}

	url, err := pm.createTorBoxURL("/usenet/createusenetdownload")
	if err != nil {
		return UsenetCreateDownloadResponse, err
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add the name field without the extension
	err = writer.WriteField("name", name)
	if err != nil {
		return UsenetCreateDownloadResponse, fmt.Errorf("failed to write name field: %v", err)
	}

	err = writer.WriteField("link", link)
	if err != nil {
		return UsenetCreateDownloadResponse, fmt.Errorf("failed to write link field: %v", err)
	}

	err = writer.Close()
	if err != nil {
		return UsenetCreateDownloadResponse, fmt.Errorf("failed to close writer: %v", err)
	}

	req, err := http.NewRequest("POST", url.String(), body)
	if err != nil {
		return UsenetCreateDownloadResponse, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", pm.APIKey))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return UsenetCreateDownloadResponse, fmt.Errorf("failed to upload file: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return UsenetCreateDownloadResponse, fmt.Errorf("failed to upload file, status: %s", resp.Status)
	}

	// Read and print the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return UsenetCreateDownloadResponse, fmt.Errorf("failed to read response body: %v", err)
	}

	err = json.Unmarshal(respBody, &UsenetCreateDownloadResponse)
	if err != nil {
		return UsenetCreateDownloadResponse, fmt.Errorf("failed to parse response body: %v", err)
	}

	if UsenetCreateDownloadResponse.Success != true {
		return UsenetCreateDownloadResponse, fmt.Errorf("failed to upload file: %s", UsenetCreateDownloadResponse.Detail)
	}

	return UsenetCreateDownloadResponse, nil
}

func (pm *TorBox) ControlUsenetDownload(id int, operation string) error {
	if pm.APIKey == "" {
		return ErrAPIKeyNotSet
	}

	url, err := pm.createTorBoxURL("/usenet/controlusenetdownload")
	if err != nil {
		return err
	}

	// Create a map with the data to send
	data := map[string]interface{}{
		"usenet_id": id,
		"operation": operation,
	}

	// Marshal the map into a JSON byte slice
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	req, err := http.NewRequest("POST", url.String(), bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", pm.APIKey))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload file: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to control download, status: %s, response: %s", resp.Status, string(respBody))
	}
	return nil
}

func (pm *TorBox) ListUsenetDownloads() ([]UsenetDownload, error) {
	if pm.APIKey == "" {
		return nil, ErrAPIKeyNotSet
	}

	url, err := pm.createTorBoxURL("/usenet/mylist")
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", pm.APIKey))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to request usenet downloads, status: %s", resp.Status)
	}

	// Read and print the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	UsenetListResponse := UsenetListResponse{}
	err = json.Unmarshal(respBody, &UsenetListResponse)
	if err != nil {
		return nil, err
	}
	if UsenetListResponse.Success != true {
		return nil, fmt.Errorf("failed to parse usenet downloads: %s", UsenetListResponse.Detail)
	}

	return UsenetListResponse.Data, nil

}

func (pm *TorBox) FindDownloadByName(name string) ([]UsenetDownload, error) {
	downloads, err := pm.ListUsenetDownloads()
	if err != nil {
		return nil, fmt.Errorf("failed to list Usenet downloads: %v", err)
	}

	// Filter downloads by name
	for _, download := range downloads {
		if strings.ToLower(download.Name) == strings.ToLower(name) {

			return []UsenetDownload{download}, nil
		}
	}
	return nil, fmt.Errorf("could not find the file in downloads: %v", err)
}

func (pm *TorBox) FindDownloadByID(ID int) ([]UsenetDownload, error) {
	downloads, err := pm.ListUsenetDownloads()
	if err != nil {
		return nil, fmt.Errorf("failed to list Usenet downloads: %v", err)
	}

	// Filter downloads by name
	for _, download := range downloads {
		if download.ID == ID {
			return []UsenetDownload{download}, nil
		}
	}
	return nil, fmt.Errorf("could not find the file in downloads: %v", err)
}

func (pm *TorBox) RequestUsenetDownloadLink(downloadFile []UsenetDownload) (string, error) {
	if pm.APIKey == "" {
		return "", ErrAPIKeyNotSet
	}
	url, err := pm.createTorBoxURLrequestDL("/usenet/requestdl", downloadFile[0].ID, downloadFile[0].Files[0].ID)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to request usenet downloads, status: %s", resp.Status)
	}

	// Read and print the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	UsenetRequestDLResponse := UsenetRequestDLResponse{}
	err = json.Unmarshal(respBody, &UsenetRequestDLResponse)
	if err != nil {
		return "", err
	}
	if UsenetRequestDLResponse.Success != true {
		return "", fmt.Errorf("failed to parse usenet downloads: %s", UsenetRequestDLResponse.Detail)
	}

	return UsenetRequestDLResponse.Data, nil
}
