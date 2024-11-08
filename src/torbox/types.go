package torbox

import "time"

type UsenetDownloadFile struct {
	ID           int    `json:"id"`
	MD5          string `json:"md5"`
	Hash         string `json:"hash"`
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	Zipped       bool   `json:"zipped"`
	S3Path       string `json:"s3_path"`
	Infected     bool   `json:"infected"`
	MimeType     string `json:"mimetype"`
	ShortName    string `json:"short_name"`
	AbsolutePath string `json:"absolute_path"`
}

type UsenetDownload struct {
	ID               int                  `json:"id"`
	CreatedAt        string               `json:"created_at"`
	UpdatedAt        string               `json:"updated_at"`
	AuthID           string               `json:"auth_id"`
	Name             string               `json:"name"`
	Hash             string               `json:"hash"`
	DownloadState    string               `json:"download_state"`
	DownloadSpeed    int                  `json:"download_speed"`
	OriginalURL      string               `json:"original_url"`
	ETA              int                  `json:"eta"`
	Progress         float64              `json:"progress"`
	Size             int64                `json:"size"`
	DownloadID       string               `json:"download_id"`
	Files            []UsenetDownloadFile `json:"files"`
	Active           bool                 `json:"active"`
	Cached           bool                 `json:"cached"`
	DownloadPresent  bool                 `json:"download_present"`
	DownloadFinished bool                 `json:"download_finished"`
	ExpiresAt        *string              `json:"expires_at"`
}

type UsenetListResponse struct {
	Success bool             `json:"success"`
	Error   *string          `json:"error"`
	Detail  string           `json:"detail"`
	Data    []UsenetDownload `json:"data"`
}

type UsenetRequestDLResponse struct {
	Success bool    `json:"success"`
	Error   *string `json:"error"`
	Detail  string  `json:"detail"`
	Data    string  `json:"data"`
}

type UsenetCreateDownloadData struct {
	Hash             string `json:"hash"`
	UsenetDownloadID int    `json:"usenetdownload_id"`
	AuthID           string `json:"auth_id"`
}

type UsenetCreateDownloadResponse struct {
	Success bool                     `json:"success"`
	Error   *string                  `json:"error"`
	Detail  string                   `json:"detail"`
	Data    UsenetCreateDownloadData `json:"data"`
}

type UsenetCacheStatusData struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	Hash string `json:"hash"`
}

type UsenetCacheStatusResponse struct {
	Success bool                             `json:"success"`
	Error   *string                          `json:"error"`
	Detail  string                           `json:"detail"`
	Data    map[string]UsenetCacheStatusData `json:"data"`
}

type UsenetOperationRequest struct {
	UsenetID  *int   `json:"usenet_id,omitempty"`
	Operation string `json:"operation"`
	All       bool   `json:"all,omitempty"`
}

type UsenetOperationResponse struct {
	Success bool    `json:"success"`
	Error   *string `json:"error"`
	Detail  string  `json:"detail"`
	Data    bool    `json:"data,omitempty"`
}

type Notification struct {
	Type      string           `json:"type"`
	Timestamp time.Time        `json:"timestamp"`
	Data      NotificationData `json:"data"`
}

type NotificationData struct {
	Title   string `json:"title"`
	Message string `json:"message"`
}
