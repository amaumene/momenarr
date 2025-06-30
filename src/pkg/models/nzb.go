package models

import "time"

// NZB represents an NZB download link and metadata
type NZB struct {
	Trakt     int64     `json:"trakt" boltholdIndex:"Trakt" validate:"required"`
	Link      string    `json:"link" validate:"required,url"`
	Length    int64     `json:"length" validate:"min=0"`
	Title     string    `json:"title" validate:"required"`
	Failed    bool      `json:"failed"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IsValid returns true if the NZB is valid for download
func (n *NZB) IsValid() bool {
	return !n.Failed && n.Link != "" && n.Title != ""
}

// MarkFailed marks the NZB as failed
func (n *NZB) MarkFailed() {
	n.Failed = true
	n.UpdatedAt = time.Now()
}