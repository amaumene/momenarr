package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/amaumene/momenarr/pkg/models"
	"github.com/amaumene/momenarr/pkg/services"
	log "github.com/sirupsen/logrus"
)

const (
	// MaxRequestSize is the maximum allowed request body size (1MB)
	MaxRequestSize = 1 << 20
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
	// Add panic recovery
	defer func() {
		if rec := recover(); rec != nil {
			log.WithFields(log.Fields{
				"panic":  rec,
				"path":   r.URL.Path,
				"method": r.Method,
			}).Error("Panic recovered in HTTP handler")
			h.writeErrorResponse(w, http.StatusInternalServerError, "Internal server error", "An unexpected error occurred")
		}
	}()

	switch r.URL.Path {
	case "/api/notify":
		h.handleNotify(w, r)
	case "/api/media":
		h.handleMedia(w, r)
	case "/api/nzb/list":
		h.handleNZBList(w, r)
	case "/api/refresh":
		h.handleRefresh(w, r)
	default:
		h.writeErrorResponse(w, http.StatusNotFound, "Not found", "The requested endpoint does not exist")
	}
}

func (h *Handler) SetupRoutes() {
	http.HandleFunc("/api/notify", h.handleNotify)
	http.HandleFunc("/api/media", h.handleMedia)
	http.HandleFunc("/api/nzb/list", h.handleNZBList)
	http.HandleFunc("/api/refresh", h.handleRefresh)
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

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestSize)
	defer r.Body.Close()

	var notification models.Notification
	if err := json.NewDecoder(r.Body).Decode(&notification); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON", fmt.Sprintf("Failed to parse request body: %v", err))
		return
	}

	log.WithFields(log.Fields{
		"name":     notification.Name,
		"category": notification.Category,
		"status":   notification.Status,
		"trakt":    notification.Trakt,
		"dir":      notification.Dir,
	}).Info("Received notification from NZBGet")

	// Process notification asynchronously with panic recovery
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.WithFields(log.Fields{
					"panic":    rec,
					"name":     notification.Name,
					"category": notification.Category,
					"status":   notification.Status,
					"trakt":    notification.Trakt,
				}).Error("Panic recovered in notification processor")
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		if err := h.appService.ProcessNotificationWithContext(ctx, &notification); err != nil {
			log.WithError(err).WithFields(log.Fields{
				"name":     notification.Name,
				"category": notification.Category,
				"status":   notification.Status,
				"trakt":    notification.Trakt,
			}).Error("Failed to process notification")
		} else {
			log.WithFields(log.Fields{
				"name":     notification.Name,
				"category": notification.Category,
				"status":   notification.Status,
				"trakt":    notification.Trakt,
			}).Info("Successfully processed notification")
		}
	}()

	h.writeSuccessResponse(w, "Notification received and processing started", nil)
}

// handleMedia handles media listing requests and returns an HTML page
func (h *Handler) handleMedia(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only GET requests are allowed")
		return
	}

	// Get all media with their status
	mediaList, err := h.appService.GetAllMedia()
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get media", err.Error())
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
                <th>Download ID</th>
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
                    {{else if gt .DownloadID 0}}
                        <span class="status-downloading">Downloading</span>
                    {{else}}
                        <span class="status-not-on-disk">Not on Disk</span>
                    {{end}}
                </td>
                <td>{{if gt .DownloadID 0}}{{.DownloadID}}{{else}}-{{end}}</td>
                <td class="file-path">{{if .File}}{{.File}}{{else}}-{{end}}</td>
                <td>{{.CreatedAt.Format "2006-01-02 15:04"}}</td>
                <td>{{.UpdatedAt.Format "2006-01-02 15:04"}}</td>
            </tr>
            {{end}}
        </tbody>
    </table>
</body>
</html>
`

	// Parse and execute template
	t, err := template.New("media").Parse(tmpl)
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Template error", err.Error())
		return
	}

	// Get statistics
	stats, err := h.appService.GetMediaStats()
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get stats", err.Error())
		return
	}

	// Prepare data for template
	data := struct {
		Media []*models.Media
		Stats *services.MediaStats
	}{
		Media: mediaList,
		Stats: stats,
	}

	// Set content type and execute template
	w.Header().Set("Content-Type", "text/html")
	if err := t.Execute(w, data); err != nil {
		log.WithError(err).Error("Failed to execute template")
	}
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

	traktID, err := validateTraktID(traktIDStr)
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid parameter", "trakt_id must be a valid positive integer")
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

// handleRefresh handles manual refresh requests
func (h *Handler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only GET requests are allowed")
		return
	}

	// Run tasks asynchronously with panic recovery
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.WithField("panic", rec).Error("Panic recovered in refresh handler")
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		if err := h.appService.RunTasks(ctx); err != nil {
			log.WithError(err).Error("Failed to run refresh tasks")
		}
	}()

	h.writeSuccessResponse(w, "Refresh initiated", nil)
}

