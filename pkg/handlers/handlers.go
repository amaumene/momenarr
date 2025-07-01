package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/amaumene/momenarr/pkg/models"
	"github.com/amaumene/momenarr/pkg/services"
	log "github.com/sirupsen/logrus"
)

// Handler contains all HTTP handlers
type Handler struct {
	appService *services.AppService
}

func NewHandler(appService *services.AppService) *Handler {
	return &Handler{
		appService: appService,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/notify":
		h.handleNotify(w, r)
	case "/api/media":
		h.handleMedia(w, r)
	case "/api/media/stats":
		h.handleMediaStats(w, r)
	case "/api/cleanup/stats":
		h.handleCleanupStats(w, r)
	case "/api/download/retry":
		h.handleRetryDownload(w, r)
	case "/api/download/cancel":
		h.handleCancelDownload(w, r)
	case "/api/download/status":
		h.handleDownloadStatus(w, r)
	case "/api/nzb/list":
		h.handleNZBList(w, r)
	case "/api/media/status":
		h.handleMediaStatus(w, r)
	case "/api/refresh":
		h.handleRefresh(w, r)
	case "/health":
		h.handleHealth(w, r)
	default:
		h.writeErrorResponse(w, http.StatusNotFound, "Not found", "The requested endpoint does not exist")
	}
}

func (h *Handler) SetupRoutes() {
	http.HandleFunc("/api/notify", h.handleNotify)
	http.HandleFunc("/api/media", h.handleMedia)
	http.HandleFunc("/api/media/stats", h.handleMediaStats)
	http.HandleFunc("/api/cleanup/stats", h.handleCleanupStats)
	http.HandleFunc("/api/download/retry", h.handleRetryDownload)
	http.HandleFunc("/api/download/cancel", h.handleCancelDownload)
	http.HandleFunc("/api/download/status", h.handleDownloadStatus)
	http.HandleFunc("/api/nzb/list", h.handleNZBList)
	http.HandleFunc("/api/media/status", h.handleMediaStatus)
	http.HandleFunc("/api/refresh", h.handleRefresh)
	http.HandleFunc("/health", h.handleHealth)
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

func (h *Handler) writeJSONResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.WithError(err).Error("Failed to encode JSON response")
	}
}

func (h *Handler) writeErrorResponse(w http.ResponseWriter, status int, message, details string) {
	response := ResponseError{
		Error:   message,
		Message: details,
	}
	h.writeJSONResponse(w, status, response)
}

func (h *Handler) writeSuccessResponse(w http.ResponseWriter, message string, data interface{}) {
	response := ResponseSuccess{
		Message: message,
		Data:    data,
	}
	h.writeJSONResponse(w, http.StatusOK, response)
}

// handleNotify handles download notifications from NZBGet
func (h *Handler) handleNotify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only POST requests are allowed")
		return
	}

	var notification models.Notification
	if err := json.NewDecoder(r.Body).Decode(&notification); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON", fmt.Sprintf("Failed to parse request body: %v", err))
		return
	}
	defer r.Body.Close()

	// Process notification asynchronously
	go func() {
		if err := h.appService.ProcessNotification(&notification); err != nil {
			log.WithError(err).WithFields(log.Fields{
				"name":     notification.Name,
				"category": notification.Category,
				"status":   notification.Status,
			}).Error("Failed to process notification")
		}
	}()

	h.writeSuccessResponse(w, "Notification received and processing started", nil)
}

// handleMedia handles media listing requests
func (h *Handler) handleMedia(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only GET requests are allowed")
		return
	}

	// For now, return media stats instead of full listing
	// A full media listing would require extending the repository interface
	stats, err := h.appService.GetMediaStats()
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get media", err.Error())
		return
	}

	h.writeSuccessResponse(w, "Media statistics retrieved successfully", stats)
}

// handleMediaStats handles media statistics requests
func (h *Handler) handleMediaStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only GET requests are allowed")
		return
	}

	stats, err := h.appService.GetMediaStats()
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get media stats", err.Error())
		return
	}

	h.writeSuccessResponse(w, "Media statistics retrieved successfully", stats)
}

// handleCleanupStats handles cleanup statistics requests
func (h *Handler) handleCleanupStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only GET requests are allowed")
		return
	}

	stats, err := h.appService.GetCleanupStats()
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get cleanup stats", err.Error())
		return
	}

	h.writeSuccessResponse(w, "Cleanup statistics retrieved successfully", stats)
}

// handleRetryDownload handles download retry requests
func (h *Handler) handleRetryDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only POST requests are allowed")
		return
	}

	traktIDStr := r.URL.Query().Get("trakt_id")
	if traktIDStr == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "Missing parameter", "trakt_id parameter is required")
		return
	}

	traktID, err := strconv.ParseInt(traktIDStr, 10, 64)
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid parameter", "trakt_id must be a valid integer")
		return
	}

	if err := h.appService.RetryFailedDownload(traktID); err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to retry download", err.Error())
		return
	}

	h.writeSuccessResponse(w, "Download retry initiated", map[string]int64{"trakt_id": traktID})
}

// handleCancelDownload handles download cancellation requests
func (h *Handler) handleCancelDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only POST requests are allowed")
		return
	}

	downloadIDStr := r.URL.Query().Get("download_id")
	if downloadIDStr == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "Missing parameter", "download_id parameter is required")
		return
	}

	downloadID, err := strconv.ParseInt(downloadIDStr, 10, 64)
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid parameter", "download_id must be a valid integer")
		return
	}

	if err := h.appService.CancelDownload(downloadID); err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to cancel download", err.Error())
		return
	}

	h.writeSuccessResponse(w, "Download canceled", map[string]int64{"download_id": downloadID})
}

// handleDownloadStatus handles download status requests
func (h *Handler) handleDownloadStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only GET requests are allowed")
		return
	}

	downloadIDStr := r.URL.Query().Get("download_id")
	if downloadIDStr == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "Missing parameter", "download_id parameter is required")
		return
	}

	downloadID, err := strconv.ParseInt(downloadIDStr, 10, 64)
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid parameter", "download_id must be a valid integer")
		return
	}

	status, err := h.appService.GetDownloadStatus(downloadID)
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get download status", err.Error())
		return
	}

	data := map[string]interface{}{
		"download_id": downloadID,
		"status":      status,
	}

	h.writeSuccessResponse(w, "Download status retrieved", data)
}

// handleRefresh handles manual refresh requests
func (h *Handler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	// Run tasks asynchronously
	go func() {
		if err := h.appService.RunTasks(); err != nil {
			log.WithError(err).Error("Failed to run refresh tasks")
		}
	}()

	h.writeSuccessResponse(w, "Refresh initiated", nil)
}

// handleHealth handles health check requests
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only GET requests are allowed")
		return
	}

	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   "1.0.0",
	}

	h.writeSuccessResponse(w, "Service is healthy", health)
}

// handleNZBList handles NZB listing requests for a specific Trakt ID
func (h *Handler) handleNZBList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only GET requests are allowed")
		return
	}

	traktIDStr := r.URL.Query().Get("trakt_id")
	if traktIDStr == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "Missing parameter", "trakt_id parameter is required")
		return
	}

	traktID, err := strconv.ParseInt(traktIDStr, 10, 64)
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid parameter", "trakt_id must be a valid integer")
		return
	}

	nzbs, err := h.appService.GetNZBsByTraktID(traktID)
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get NZBs", err.Error())
		return
	}

	data := map[string]interface{}{
		"trakt_id": traktID,
		"count":    len(nzbs),
		"nzbs":     nzbs,
	}

	h.writeSuccessResponse(w, "NZBs retrieved successfully", data)
}

// handleMediaStatus handles media status listing requests
func (h *Handler) handleMediaStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only GET requests are allowed")
		return
	}

	mediaStatus, err := h.appService.GetMediaStatus()
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get media status", err.Error())
		return
	}

	data := map[string]interface{}{
		"total_items": len(mediaStatus),
		"media":       mediaStatus,
	}

	h.writeSuccessResponse(w, "Media status retrieved successfully", data)
}
