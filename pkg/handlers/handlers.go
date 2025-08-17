package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"runtime"
	"time"

	"github.com/amaumene/momenarr/pkg/models"
	"github.com/amaumene/momenarr/pkg/services"
	log "github.com/sirupsen/logrus"
)

const (
	maxRequestSize = 1 << 20
	refreshTimeout = 20 * time.Minute
)

type Handler struct {
	appService *services.AppService
}

func CreateHandler(appService *services.AppService) *Handler {
	return &Handler{
		appService: appService,
	}
}

func NewAppHandler(appService *services.AppService) http.Handler {
	return CreateHandler(appService)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer h.recoverPanic(w, r)

	h.logRequest(r)
	h.routeRequest(w, r)
}

func (h *Handler) logRequest(r *http.Request) {
	log.WithFields(log.Fields{
		"path":   r.URL.Path,
		"method": r.Method,
	}).Debug("Handling request")
}

func (h *Handler) routeRequest(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/media":
		h.handleMedia(w, r)
	case "/api/media/stats":
		h.handleMediaStats(w, r)
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

func (h *Handler) SetupRoutes() {
	routes := h.getRouteMap()
	h.registerRoutes(routes)
}

func (h *Handler) getRouteMap() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"/api/media":           h.handleMedia,
		"/api/media/stats":     h.handleMediaStats,
		"/api/download/retry":  h.handleRetryDownload,
		"/api/download/cancel": h.handleCancelDownload,
		"/api/download/status": h.handleDownloadStatus,
		"/api/refresh":         h.handleRefresh,
		"/api/cleanup/stats":   h.handleCleanupStats,
	}
}

func (h *Handler) registerRoutes(routes map[string]http.HandlerFunc) {
	for path, handler := range routes {
		http.HandleFunc(path, handler)
	}
}

type responseError struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

type responseSuccess struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (h *Handler) writeJSONResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.WithError(err).Error("failed to encode json response")
	}
}

func (h *Handler) writeErrorResponse(w http.ResponseWriter, status int, message, details string) {
	response := responseError{
		Error:   message,
		Message: details,
	}
	h.writeJSONResponse(w, status, response)
}

func (h *Handler) writeSuccessResponse(w http.ResponseWriter, message string, data interface{}) {

	if _, ok := data.(*services.MediaStats); ok {
		h.writeJSONResponse(w, http.StatusOK, data)
		return
	}

	response := responseSuccess{
		Message: message,
		Data:    data,
	}
	h.writeJSONResponse(w, http.StatusOK, response)
}


func (h *Handler) writeHTMLErrorResponse(w http.ResponseWriter, status int, message, details string) {
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


func (h *Handler) handleMedia(w http.ResponseWriter, r *http.Request) {
	if !h.validateMethod(w, r, http.MethodGet, true) {
		return
	}

	mediaList, err := h.fetchMediaList(w)
	if err != nil {
		return
	}

	stats, err := h.fetchMediaStats(w)
	if err != nil {
		return
	}

	if err := h.renderMediaPage(w, mediaList, stats, mediaPageTemplate); err != nil {
		log.WithError(err).Error("Failed to render media page")
	}
}


func (h *Handler) fetchMediaList(w http.ResponseWriter) ([]*models.Media, error) {
	mediaList, err := h.appService.GetAllMedia()
	if err != nil {
		log.WithError(err).Error("Failed to get all media")
		h.writeHTMLErrorResponse(w, http.StatusInternalServerError,
			"Failed to get media", "There was an error retrieving the media list")
		return nil, err
	}
	return mediaList, nil
}


func (h *Handler) fetchMediaStats(w http.ResponseWriter) (*services.MediaStats, error) {
	stats, err := h.appService.GetMediaStats()
	if err != nil {
		log.WithError(err).Error("Failed to get media stats")
		h.writeHTMLErrorResponse(w, http.StatusInternalServerError,
			"Failed to get stats", "There was an error retrieving media statistics")
		return nil, err
	}
	return stats, nil
}


func (h *Handler) handleMediaStats(w http.ResponseWriter, r *http.Request) {
	if !h.validateMethod(w, r, http.MethodGet, false) {
		return
	}

	stats, err := h.appService.GetMediaStats()
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get stats", err.Error())
		return
	}


	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		log.WithError(err).Error("Failed to encode stats response")
	}
}


func (h *Handler) handleRetryDownload(w http.ResponseWriter, r *http.Request) {
	if !h.validateMethod(w, r, http.MethodPost, false) {
		return
	}

	traktID, err := h.getTraktIDFromBody(w, r)
	if err != nil {
		return
	}

	if err := h.appService.RetryDownload(traktID); err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to retry download", err.Error())
		return
	}

	h.writeSuccessResponse(w, "Download retry initiated", nil)
}


func (h *Handler) handleCancelDownload(w http.ResponseWriter, r *http.Request) {
	if !h.validateMethod(w, r, http.MethodPost, false) {
		return
	}

	traktID, err := h.getTraktIDFromBody(w, r)
	if err != nil {
		return
	}

	if err := h.appService.CancelDownload(traktID); err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to cancel download", err.Error())
		return
	}

	h.writeSuccessResponse(w, "Download cancelled successfully", nil)
}


func (h *Handler) handleDownloadStatus(w http.ResponseWriter, r *http.Request) {
	if !h.validateMethod(w, r, http.MethodGet, false) {
		return
	}

	traktID, err := h.getTraktIDFromQuery(w, r)
	if err != nil {
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




func (h *Handler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if !h.validateMethod(w, r, http.MethodGet, false) {
		return
	}

	h.startAsyncRefresh()

	h.writeSuccessResponse(w, "Trakt sync and torrent search initiated", map[string]interface{}{
		"description": "Syncing latest media from Trakt, then searching for torrents and checking AllDebrid cache for media not marked as downloaded",
		"timeout":     "20 minutes",
		"steps":       getRefreshSteps(),
	})
}


func (h *Handler) handleCleanupStats(w http.ResponseWriter, r *http.Request) {
	if !h.validateMethod(w, r, http.MethodGet, false) {
		return
	}

	stats, err := h.appService.GetCleanupStats()
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get cleanup stats", err.Error())
		return
	}

	h.writeSuccessResponse(w, "Cleanup statistics retrieved", stats)
}




func (h *Handler) recoverPanic(w http.ResponseWriter, r *http.Request) {
	if rec := recover(); rec != nil {
		buf := make([]byte, 4096)
		n := runtime.Stack(buf, false)
		stackTrace := string(buf[:n])

		log.WithFields(log.Fields{
			"panic":  fmt.Sprintf("%v", rec),
			"path":   r.URL.Path,
			"method": r.Method,
		}).Error("panic recovered in http handler")

		log.Debugf("Stack trace:\n%s", stackTrace)

		h.writeErrorResponse(w, http.StatusInternalServerError, "internal server error", "an unexpected error occurred")
	}
}


func (h *Handler) validateMethod(w http.ResponseWriter, r *http.Request, expected string, isHTML bool) bool {
	if r.Method != expected {
		msg := fmt.Sprintf("Only %s requests are allowed", expected)
		if isHTML {
			h.writeHTMLErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", msg)
		} else {
			h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", msg)
		}
		return false
	}
	return true
}


func (h *Handler) getTraktIDFromQuery(w http.ResponseWriter, r *http.Request) (int64, error) {
	traktIDStr := r.URL.Query().Get("trakt_id")
	if traktIDStr == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "Missing parameter", "trakt_id parameter is required")
		return 0, fmt.Errorf("missing trakt_id")
	}

	traktID, err := validateTraktID(traktIDStr)
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid parameter", "trakt_id must be a valid positive integer")
		return 0, err
	}

	return traktID, nil
}


func (h *Handler) getTraktIDFromBody(w http.ResponseWriter, r *http.Request) (int64, error) {
	var req struct {
		TraktID int64 `json:"trakt_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON", err.Error())
		return 0, err
	}

	if req.TraktID <= 0 {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid parameter", "trakt_id must be a positive integer")
		return 0, fmt.Errorf("invalid trakt_id")
	}

	return req.TraktID, nil
}


func (h *Handler) startAsyncRefresh() {
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.WithField("panic", rec).Error("Panic recovered in refresh handler")
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
		defer cancel()

		if err := h.appService.SearchTorrentsForNotDownloaded(ctx); err != nil {
			log.WithError(err).Error("Failed to sync with Trakt and search torrents")
		} else {
			log.Info("Trakt sync and torrent search completed successfully")
		}
	}()
}


func getRefreshSteps() []string {
	return []string{
		"1. Sync movies and episodes from Trakt watchlist and favorites",
		"2. Clean up media no longer in Trakt lists",
		"3. Find media not marked as downloaded",
		"4. Search for torrents for each media item",
		"5. Check AllDebrid cache for available torrents",
		"6. Mark cached torrents as downloaded",
	}
}


func (h *Handler) renderMediaPage(w http.ResponseWriter, mediaList []*models.Media, stats *services.MediaStats, tmpl string) error {
	t, err := h.parseTemplate(w, tmpl)
	if err != nil {
		return err
	}

	data := h.prepareMediaPageData(mediaList, stats)

	w.Header().Set("Content-Type", "text/html")
	return t.Execute(w, data)
}


func (h *Handler) parseTemplate(w http.ResponseWriter, tmpl string) (*template.Template, error) {
	t, err := template.New("media").Parse(tmpl)
	if err != nil {
		h.writeHTMLErrorResponse(w, http.StatusInternalServerError,
			"Template error", "There was an error creating the page template")
		return nil, err
	}
	return t, nil
}


type mediaPageData struct {
	Media []mediaData
	Stats statsData
}


func (h *Handler) prepareMediaPageData(mediaList []*models.Media, stats *services.MediaStats) mediaPageData {
	safeMediaList := h.convertMediaList(mediaList)

	return mediaPageData{
		Media: safeMediaList,
		Stats: convertStats(stats),
	}
}


type mediaData struct {
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


type statsData struct {
	Total       int
	OnDisk      int
	NotOnDisk   int
	Movies      int
	Episodes    int
	Downloading int
}


func (h *Handler) convertMediaList(mediaList []*models.Media) []mediaData {
	safeMediaList := make([]mediaData, 0, len(mediaList))
	for _, media := range mediaList {
		safeMediaList = append(safeMediaList, convertMediaItem(media))
	}
	return safeMediaList
}


func convertMediaItem(media *models.Media) mediaData {
	return mediaData{
		Trakt:         media.Trakt,
		Title:         media.Title,
		Year:          media.Year,
		Season:        media.Season,
		Number:        media.Number,
		OnDisk:        media.OnDisk,
		File:          media.File,
		IsMovie:       media.IsMovie(),
		IsDownloading: false,
		CreatedAt:     media.CreatedAt,
		UpdatedAt:     media.UpdatedAt,
	}
}


func convertStats(stats *services.MediaStats) statsData {
	return statsData{
		Total:       stats.Total,
		OnDisk:      stats.OnDisk,
		NotOnDisk:   stats.NotOnDisk,
		Movies:      stats.Movies,
		Episodes:    stats.Episodes,
		Downloading: stats.Downloading,
	}
}
