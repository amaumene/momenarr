package models

import (
	"strconv"
	"time"
)

// Notification represents a download notification from NZBGet
type Notification struct {
	Name     string `json:"name" validate:"required"`
	Category string `json:"category" validate:"required"`
	Status   string `json:"status" validate:"required"`
	Trakt    string `json:"trakt" validate:"required"`
	Dir      string `json:"dir" validate:"required"`
}

// NotificationStatus represents the status of a download
type NotificationStatus string

const (
	StatusSuccess NotificationStatus = "SUCCESS"
	StatusFailure NotificationStatus = "FAILURE"
	StatusWarning NotificationStatus = "WARNING"
)

// GetTraktID returns the Trakt ID as int64
func (n *Notification) GetTraktID() (int64, error) {
	return strconv.ParseInt(n.Trakt, 10, 64)
}

// IsSuccess returns true if the notification indicates success
func (n *Notification) IsSuccess() bool {
	return NotificationStatus(n.Status) == StatusSuccess
}

// IsFailure returns true if the notification indicates failure
func (n *Notification) IsFailure() bool {
	return NotificationStatus(n.Status) == StatusFailure
}

// ProcessedNotification represents a processed notification with metadata
type ProcessedNotification struct {
	*Notification
	TraktID     int64     `json:"trakt_id"`
	ProcessedAt time.Time `json:"processed_at"`
}