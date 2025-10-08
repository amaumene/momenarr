package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/amaumene/momenarr/internal/domain"
	"github.com/amaumene/momenarr/internal/service"
	log "github.com/sirupsen/logrus"
)

const (
	contentTypeJSON = "application/json"
	mediaListFormat = "IMDB: %s, Title: %s, OnDisk: %t, File: %s\n"
	nzbListFormat   = "TraktID: %d, Title: %s, Link: %s, Length: %d\n"
)

type HTTPHandler struct {
	mediaRepo       domain.MediaRepository
	nzbRepo         domain.NZBRepository
	notificationSvc *service.NotificationService
	mediaSvc        *service.MediaService
	nzbSvc          *service.NZBService
	downloadSvc     *service.DownloadService
}

func NewHTTPHandler(mediaRepo domain.MediaRepository, nzbRepo domain.NZBRepository, notificationSvc *service.NotificationService, mediaSvc *service.MediaService, nzbSvc *service.NZBService, downloadSvc *service.DownloadService) *HTTPHandler {
	return &HTTPHandler{
		mediaRepo:       mediaRepo,
		nzbRepo:         nzbRepo,
		notificationSvc: notificationSvc,
		mediaSvc:        mediaSvc,
		nzbSvc:          nzbSvc,
		downloadSvc:     downloadSvc,
	}
}

func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/notify", h.handleNotify)
	mux.HandleFunc("/list", h.handleList)
	mux.HandleFunc("/nzbs", h.handleNZBs)
	mux.HandleFunc("/health", h.handleHealth)
	mux.HandleFunc("/refresh", h.handleRefresh)
}

func (h *HTTPHandler) handleNotify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	notification, err := h.parseNotification(r)
	if err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	go h.processNotificationAsync(r.Context(), notification)

	h.writeJSON(w, http.StatusOK, map[string]string{
		"message": "Processing started",
	})
}

func (h *HTTPHandler) parseNotification(r *http.Request) (*domain.Notification, error) {
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}

	var notification domain.Notification
	if err := json.Unmarshal(body, &notification); err != nil {
		return nil, fmt.Errorf("unmarshalling json: %w", err)
	}
	return &notification, nil
}

func (h *HTTPHandler) processNotificationAsync(ctx context.Context, notification *domain.Notification) {
	if err := h.notificationSvc.Process(ctx, notification); err != nil {
		log.WithField("error", err).Error("failed to process download notification")
	}
}

func (h *HTTPHandler) handleList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	medias, err := h.mediaRepo.FindWithIMDB(ctx)
	if err != nil {
		log.WithField("error", err).Error("failed to retrieve media list from database")
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	response := h.formatMediaList(medias)
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(response)); err != nil {
		log.WithField("error", err).Error("failed to write media list response")
	}
}

func (h *HTTPHandler) formatMediaList(medias []domain.Media) string {
	var builder strings.Builder
	for _, media := range medias {
		builder.WriteString(fmt.Sprintf(mediaListFormat, media.IMDB, media.Title, media.OnDisk, media.File))
	}
	return builder.String()
}

func (h *HTTPHandler) handleNZBs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nzbs, err := h.nzbRepo.FindAll(ctx)
	if err != nil {
		log.WithField("error", err).Error("failed to retrieve nzb list from database")
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	response := h.formatNZBList(nzbs)
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(response)); err != nil {
		log.WithField("error", err).Error("failed to write nzb list response")
	}
}

func (h *HTTPHandler) formatNZBList(nzbs []domain.NZB) string {
	var builder strings.Builder
	for _, nzb := range nzbs {
		builder.WriteString(fmt.Sprintf(nzbListFormat, nzb.TraktID, nzb.Title, nzb.Link, nzb.Length))
	}
	return builder.String()
}

func (h *HTTPHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h *HTTPHandler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	go h.runTasksAsync(r.Context())

	h.writeJSON(w, http.StatusOK, map[string]string{
		"message": "Refresh initiated",
	})
}

func (h *HTTPHandler) runTasksAsync(ctx context.Context) {
	if _, err := h.mediaSvc.SyncFromTrakt(ctx); err != nil {
		log.WithField("error", err).Error("failed to sync media from trakt")
	}

	if err := h.nzbSvc.PopulateNZBs(ctx); err != nil {
		log.WithField("error", err).Error("failed to populate nzb search results")
	}

	if err := h.downloadNotOnDisk(ctx); err != nil {
		log.WithField("error", err).Error("failed to download pending media")
	}
}

func (h *HTTPHandler) downloadNotOnDisk(ctx context.Context) error {
	medias, err := h.mediaRepo.FindNotOnDisk(ctx)
	if err != nil {
		return fmt.Errorf("finding media: %w", err)
	}

	for _, media := range medias {
		nzb, err := h.nzbSvc.GetNZB(ctx, media.TraktID)
		if err != nil {
			log.WithFields(log.Fields{
				"error":   err,
				"traktID": media.TraktID,
			}).Error("failed to retrieve nzb for media")
			continue
		}

		if err := h.downloadSvc.CreateDownload(ctx, media.TraktID, nzb); err != nil {
			log.WithFields(log.Fields{
				"error":   err,
				"traktID": media.TraktID,
			}).Error("failed to create download")
		}
	}
	return nil
}

func (h *HTTPHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.WithField("error", err).Error("failed to encode json response")
	}
}
