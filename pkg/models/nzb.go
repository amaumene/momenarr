package models

import (
	"crypto/md5"
	"fmt"
	"time"
)

// NZB represents an NZB download link and metadata
type NZB struct {
	ID        string    `json:"id" boltholdKey:"ID"`
	Trakt     int64     `json:"trakt" boltholdIndex:"Trakt" validate:"required"`
	Link      string    `json:"link" validate:"required,url"`
	Length    int64     `json:"length" validate:"min=0"`
	Title     string    `json:"title" validate:"required"`
	Failed    bool      `json:"failed"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (n *NZB) GenerateID() {
	hash := md5.Sum([]byte(fmt.Sprintf("%d_%s", n.Trakt, n.Link)))
	n.ID = fmt.Sprintf("%x", hash)
}

func (n *NZB) IsValid() bool {
	return !n.Failed && n.Link != "" && n.Title != ""
}

func (n *NZB) MarkFailed() {
	n.Failed = true
	n.UpdatedAt = time.Now()
}
