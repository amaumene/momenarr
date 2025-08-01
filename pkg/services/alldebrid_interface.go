package services

import (
	"time"

	"github.com/amaumene/momenarr/pkg/models"
)

// AllDebridInterface defines the interface for AllDebrid services.
type AllDebridInterface interface {
	// IsTorrentCached checks if a torrent is cached by its hash.
	IsTorrentCached(hash string) (bool, int64, error)
	// UploadTorrent uploads a torrent to AllDebrid.
	UploadTorrent(result *models.TorrentSearchResult) (int64, error)
	// WaitForTorrentReady waits for a torrent to be ready for download.
	WaitForTorrentReady(allDebridID int64, timeout time.Duration) error
	// DownloadFile downloads a file from AllDebrid.
	DownloadFile(allDebridID int64, torrentResult *models.TorrentSearchResult, media *models.Media) error
	// DeleteMagnet deletes a magnet from AllDebrid.
	DeleteMagnet(allDebridID int64) error
	// GetMagnetStatus retrieves the status of a magnet.
	GetMagnetStatus(allDebridID int64) (string, error)
}
