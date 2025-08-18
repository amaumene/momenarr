package premiumize

import (
	"errors"
	"time"
)

var (
	ErrAPIKeyNotSet     = errors.New("premiumize API key not set")
	ErrTransferNotFound = errors.New("transfer not found")
)

type TransferStatus string

const (
	TransferStatusQueued      TransferStatus = "queued"
	TransferStatusDownloading TransferStatus = "downloading"
	TransferStatusSeeding     TransferStatus = "seeding"
	TransferStatusFinished    TransferStatus = "finished"
	TransferStatusError       TransferStatus = "error"
	TransferStatusTimeout     TransferStatus = "timeout"
)

type BaseResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type Transfer struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Status   TransferStatus `json:"status"`
	Progress float64        `json:"progress"`
	Source   string         `json:"src,omitempty"`
	FolderID string         `json:"folder_id,omitempty"`
	FileID   string         `json:"file_id,omitempty"`
	Size     int64          `json:"size,omitempty"`
	Created  time.Time      `json:"created,omitempty"`
}

type TransferCreateResponse struct {
	BaseResponse
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type TransferListResponse struct {
	BaseResponse
	Transfers []Transfer `json:"transfers"`
}

type Folder struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	ParentID string    `json:"parent_id,omitempty"`
	Created  time.Time `json:"created_at,omitempty"`
}

type File struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	Link       string    `json:"link,omitempty"`
	StreamLink string    `json:"stream_link,omitempty"`
	Created    time.Time `json:"created_at,omitempty"`
	MimeType   string    `json:"mime_type,omitempty"`
}

type FolderListResponse struct {
	BaseResponse
	Content struct {
		Items   []File   `json:"items,omitempty"`
		Folders []Folder `json:"folders,omitempty"`
	} `json:"content"`
	Name     string `json:"name"`
	ParentID string `json:"parent_id,omitempty"`
}

func (s TransferStatus) IsComplete() bool {
	return s == TransferStatusFinished || s == TransferStatusSeeding
}

func (s TransferStatus) IsFailed() bool {
	return s == TransferStatusError || s == TransferStatusTimeout
}

func (s TransferStatus) IsActive() bool {
	return s == TransferStatusDownloading || s == TransferStatusQueued
}