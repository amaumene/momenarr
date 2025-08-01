package services

import (
	"time"

	"github.com/amaumene/momenarr/pkg/models"
)

// AllDebridInterface defines the interface for AllDebrid services
type AllDebridInterface interface {
	IsTorrentCached(hash string) (bool, int64, error)
	UploadTorrent(result *models.TorrentSearchResult) (int64, error)
	WaitForTorrentReady(allDebridID int64, timeout time.Duration) error
	DownloadFile(allDebridID int64, torrentResult *models.TorrentSearchResult, media *models.Media) error
	DeleteMagnet(allDebridID int64) error
	GetMagnetStatus(allDebridID int64) (string, error)
}
