package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/amaumene/momenarr/pkg/services"
	log "github.com/sirupsen/logrus"
)

const (
	// MaxRequestSize is the maximum allowed request body size (1MB)
	MaxRequestSize = 1 << 20
)

// NewHandler contains all HTTP handlers for the new torrent/AllDebrid version
type NewHandler struct {
	appService *services.NewAppService
}

func CreateNewHandler(appService *services.NewAppService) *NewHandler {
	return &NewHandler{
		appService: appService,
	}
}

func NewAppHandler(appService *services.NewAppService) http.Handler {
	return CreateNewHandler(appService)
}

func (h *NewHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Add panic recovery
	defer func() {
		if rec := recover(); rec != nil {
			log.WithFields(log.Fields{
				"panic":  rec,
				"path":   r.URL.Path,
				"method": r.Method,
			}).Error("panic recovered in http handler")
			h.writeErrorResponse(w, http.StatusInternalServerError, "internal server error", "an unexpected error occurred")
		}
	}()

	switch r.URL.Path {
	case "/api/media":
		h.handleMedia(w, r)
	case "/api/media/stats":
		h.handleMediaStats(w, r)
	case "/api/torrents/list":
		h.handleTorrentList(w, r)
	case "/api/download/retry":
		h.handleRetryDownload(w, r)
	case "/api/download/cancel":
		h.handleCancelDownload(w, r)
	case "/api/download/status":
		h.handleDownloadStatus(w, r)
	case "/api/refresh":
		h.handleRefresh(w, r)
	case "/api/cleanup/stats":
		h.handleCleanupStats(w, r)
	default:
		h.writeErrorResponse(w, http.StatusNotFound, "not found", "the requested endpoint does not exist")
	}
}

func (h *NewHandler) SetupRoutes() {
	http.HandleFunc("/api/media", h.handleMedia)
	http.HandleFunc("/api/media/stats", h.handleMediaStats)
	http.HandleFunc("/api/torrents/list", h.handleTorrentList)
	http.HandleFunc("/api/download/retry", h.handleRetryDownload)
	http.HandleFunc("/api/download/cancel", h.handleCancelDownload)
	http.HandleFunc("/api/download/status", h.handleDownloadStatus)
	http.HandleFunc("/api/refresh", h.handleRefresh)
	http.HandleFunc("/api/cleanup/stats", h.handleCleanupStats)
}

// ResponseError represents an error response
type ResponseError struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// ResponseSuccess represents a success response
type ResponseSuccess struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (h *NewHandler) writeJSONResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.WithError(err).Error("failed to encode json response")
	}
}

func (h *NewHandler) writeErrorResponse(w http.ResponseWriter, status int, message, details string) {
	response := ResponseError{
		Error:   message,
		Message: details,
	}
	h.writeJSONResponse(w, status, response)
}

func (h *NewHandler) writeSuccessResponse(w http.ResponseWriter, message string, data interface{}) {
	response := ResponseSuccess{
		Message: message,
		Data:    data,
	}
	h.writeJSONResponse(w, http.StatusOK, response)
}

// writeHTMLErrorResponse writes an HTML error response instead of JSON
func (h *NewHandler) writeHTMLErrorResponse(w http.ResponseWriter, status int, message, details string) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(status)

	errorHTML := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>Error - Momenarr</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .error { background-color: #f8d7da; color: #721c24; padding: 20px; border-radius: 5px; }
    </style>
</head>
<body>
    <div class="error">
        <h2>%s</h2>
        <p>%s</p>
    </div>
</body>
</html>`, message, details)

	fmt.Fprint(w, errorHTML)
}

// handleMedia handles media listing requests and returns an HTML page
func (h *NewHandler) handleMedia(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeHTMLErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed", "only GET requests are allowed")
		return
	}

	// Get all media with their status
	mediaList, err := h.appService.GetAllMedia()
	if err != nil {
		log.WithError(err).Error("Failed to get all media")
		h.writeHTMLErrorResponse(w, http.StatusInternalServerError, "Failed to get media", "There was an error retrieving the media list")
		return
	}

	// Get statistics
	stats, err := h.appService.GetMediaStats()
	if err != nil {
		log.WithError(err).Error("Failed to get media stats")
		h.writeHTMLErrorResponse(w, http.StatusInternalServerError, "Failed to get stats", "There was an error retrieving media statistics")
		return
	}

	// Create HTML template
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <title>Momenarr - Media List</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            margin: 20px;
            background-color: #f5f5f5;
        }
        h1 {
            color: #333;
        }
        table {
            width: 100%;
            border-collapse: collapse;
            background-color: white;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        th {
            background-color: #4CAF50;
            color: white;
            padding: 12px;
            text-align: left;
            position: sticky;
            top: 0;
        }
        td {
            padding: 10px;
            border-bottom: 1px solid #ddd;
        }
        tr:hover {
            background-color: #f5f5f5;
        }
        .status-on-disk {
            color: green;
            font-weight: bold;
        }
        .status-not-on-disk {
            color: orange;
            font-weight: bold;
        }
        .status-downloading {
            color: blue;
            font-weight: bold;
        }
        .type-movie {
            background-color: #e3f2fd;
        }
        .type-episode {
            background-color: #f3e5f5;
        }
        .stats {
            margin-bottom: 20px;
            padding: 15px;
            background-color: white;
            border-radius: 5px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .file-path {
            font-family: monospace;
            font-size: 0.9em;
            color: #555;
        }
        .powered-by {
            margin-top: 20px;
            text-align: center;
            color: #666;
        }
    </style>
</head>
<body>
    <h1>Momenarr Media Library</h1>
    
    <div class="stats">
        <h2>Statistics</h2>
        <p>Total Media: {{.Stats.Total}} | On Disk: {{.Stats.OnDisk}} | Not on Disk: {{.Stats.NotOnDisk}} | Downloading: {{.Stats.Downloading}}</p>
        <p>Movies: {{.Stats.Movies}} | Episodes: {{.Stats.Episodes}}</p>
    </div>

    <table>
        <thead>
            <tr>
                <th>Trakt ID</th>
                <th>Type</th>
                <th>Title</th>
                <th>Year</th>
                <th>Season/Episode</th>
                <th>Status</th>
                <th>File Path</th>
                <th>Created</th>
                <th>Updated</th>
            </tr>
        </thead>
        <tbody>
            {{range .Media}}
            <tr class="{{if .IsMovie}}type-movie{{else}}type-episode{{end}}">
                <td>{{.Trakt}}</td>
                <td>{{if .IsMovie}}Movie{{else}}Episode{{end}}</td>
                <td>{{.Title}}</td>
                <td>{{.Year}}</td>
                <td>{{if not .IsMovie}}S{{printf "%02d" .Season}}E{{printf "%02d" .Number}}{{else}}-{{end}}</td>
                <td>
                    {{if .OnDisk}}
                        <span class="status-on-disk">On Disk</span>
                    {{else if .IsDownloading}}
                        <span class="status-downloading">Downloading</span>
                    {{else}}
                        <span class="status-not-on-disk">Not on Disk</span>
                    {{end}}
                </td>
                <td class="file-path">{{if .File}}{{.File}}{{else}}-{{end}}</td>
                <td>{{.CreatedAt.Format "2006-01-02 15:04"}}</td>
                <td>{{.UpdatedAt.Format "2006-01-02 15:04"}}</td>
            </tr>
            {{end}}
        </tbody>
    </table>
    
    <div class="powered-by">
        <p>Powered by Momenarr with AllDebrid</p>
    </div>
</body>
</html>
`

	// Parse template
	t, err := template.New("media").Parse(tmpl)
	if err != nil {
		log.WithError(err).Error("Failed to parse template")
		h.writeHTMLErrorResponse(w, http.StatusInternalServerError, "Template error", "There was an error creating the page template")
		return
	}

	// Prepare safe data for template
	type SafeMediaData struct {
		Trakt         int64
		Title         string
		Year          int64
		Season        int64
		Number        int64
		OnDisk        bool
		File          string
		IsMovie       bool
		IsDownloading bool
		CreatedAt     time.Time
		UpdatedAt     time.Time
	}

	type SafeStats struct {
		Total       int
		OnDisk      int
		NotOnDisk   int
		Movies      int
		Episodes    int
		Downloading int
	}

	var safeMediaList []SafeMediaData
	for _, media := range mediaList {
		// Check if downloading by looking for active torrents
		isDownloading := false
		if !media.OnDisk {
			torrents, _ := h.appService.GetTorrentsByTraktID(media.Trakt)
			for _, torrent := range torrents {
				if torrent.AllDebridID > 0 && !torrent.Failed {
					isDownloading = true
					break
				}
			}
		}

		safeMediaList = append(safeMediaList, SafeMediaData{
			Trakt:         media.Trakt,
			Title:         media.Title,
			Year:          media.Year,
			Season:        media.Season,
			Number:        media.Number,
			OnDisk:        media.OnDisk,
			File:          media.File,
			IsMovie:       media.IsMovie(),
			IsDownloading: isDownloading,
			CreatedAt:     media.CreatedAt,
			UpdatedAt:     media.UpdatedAt,
		})
	}

	data := struct {
		Media []SafeMediaData
		Stats SafeStats
	}{
		Media: safeMediaList,
		Stats: SafeStats{
			Total:       stats.Total,
			OnDisk:      stats.OnDisk,
			NotOnDisk:   stats.NotOnDisk,
			Movies:      stats.Movies,
			Episodes:    stats.Episodes,
			Downloading: stats.Downloading,
		},
	}

	// Set content type and execute template
	w.Header().Set("Content-Type", "text/html")
	if err := t.Execute(w, data); err != nil {
		log.WithError(err).Error("Failed to execute template")
		// Don't try to write another response here as headers are already sent
	}
}

// handleMediaStats returns JSON media statistics
func (h *NewHandler) handleMediaStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only GET requests are allowed")
		return
	}

	stats, err := h.appService.GetMediaStats()
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get stats", err.Error())
		return
	}

	h.writeSuccessResponse(w, "Statistics retrieved successfully", stats)
}

// handleTorrentList handles torrent listing requests for a specific Trakt ID
func (h *NewHandler) handleTorrentList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only GET requests are allowed")
		return
	}

	traktIDStr := r.URL.Query().Get("trakt_id")
	if traktIDStr == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "Missing parameter", "trakt_id parameter is required")
		return
	}

	traktID, err := validateTraktID(traktIDStr)
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid parameter", "trakt_id must be a valid positive integer")
		return
	}

	torrents, err := h.appService.GetTorrentsByTraktID(traktID)
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get torrents", err.Error())
		return
	}

	data := map[string]interface{}{
		"trakt_id": traktID,
		"count":    len(torrents),
		"torrents": torrents,
	}

	h.writeSuccessResponse(w, "Torrents retrieved successfully", data)
}

// handleRetryDownload handles download retry requests
func (h *NewHandler) handleRetryDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only POST requests are allowed")
		return
	}

	var req struct {
		TraktID int64 `json:"trakt_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON", err.Error())
		return
	}

	if req.TraktID <= 0 {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid parameter", "trakt_id must be a positive integer")
		return
	}

	if err := h.appService.RetryDownload(req.TraktID); err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to retry download", err.Error())
		return
	}

	h.writeSuccessResponse(w, "Download retry initiated", nil)
}

// handleCancelDownload handles download cancellation requests
func (h *NewHandler) handleCancelDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only POST requests are allowed")
		return
	}

	var req struct {
		TraktID int64 `json:"trakt_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON", err.Error())
		return
	}

	if req.TraktID <= 0 {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid parameter", "trakt_id must be a positive integer")
		return
	}

	if err := h.appService.CancelDownload(req.TraktID); err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to cancel download", err.Error())
		return
	}

	h.writeSuccessResponse(w, "Download cancelled successfully", nil)
}

// handleDownloadStatus gets the status of a download
func (h *NewHandler) handleDownloadStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only GET requests are allowed")
		return
	}

	traktIDStr := r.URL.Query().Get("trakt_id")
	if traktIDStr == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "Missing parameter", "trakt_id parameter is required")
		return
	}

	traktID, err := validateTraktID(traktIDStr)
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid parameter", "trakt_id must be a valid positive integer")
		return
	}

	status, err := h.appService.GetDownloadStatus(traktID)
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get download status", err.Error())
		return
	}

	data := map[string]interface{}{
		"trakt_id": traktID,
		"status":   status,
	}

	h.writeSuccessResponse(w, "Download status retrieved", data)
}

// handleRefresh handles manual refresh requests - syncs with Trakt and searches for torrents
// GET /api/refresh - Syncs media from Trakt and searches for torrents for media not marked as downloaded
// This will sync the latest media from Trakt, then search multiple torrent providers and check AllDebrid cache
func (h *NewHandler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only GET requests are allowed")
		return
	}

	// Run torrent search asynchronously with panic recovery
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.WithField("panic", rec).Error("Panic recovered in refresh handler")
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()

		if err := h.appService.SearchTorrentsForNotDownloaded(ctx); err != nil {
			log.WithError(err).Error("Failed to sync with Trakt and search torrents for media not on disk")
		} else {
			log.Info("Trakt sync and torrent search for media not on disk completed successfully")
		}
	}()

	h.writeSuccessResponse(w, "Trakt sync and torrent search initiated", map[string]interface{}{
		"description": "Syncing latest media from Trakt, then searching for torrents and checking AllDebrid cache for media not marked as downloaded",
		"timeout":     "20 minutes",
		"steps": []string{
			"1. Sync movies and episodes from Trakt watchlist and favorites",
			"2. Clean up media no longer in Trakt lists",
			"3. Find media not marked as downloaded",
			"4. Search torrent providers (YGG, APIBay) for each media item",
			"5. Check AllDebrid cache for available torrents",
			"6. Mark cached torrents as downloaded",
		},
	})
}

// handleCleanupStats returns cleanup statistics
func (h *NewHandler) handleCleanupStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only GET requests are allowed")
		return
	}

	stats, err := h.appService.GetCleanupStats()
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get cleanup stats", err.Error())
		return
	}

	h.writeSuccessResponse(w, "Cleanup statistics retrieved", stats)
}
