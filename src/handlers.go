package main

import (
	"encoding/json"
	"fmt"
	"github.com/amaumene/momenarr/bolthold"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
)

const (
	routeNotify           = "/api/notify"
	routeList             = "/list"
	routeNZBs             = "/nzbs"
	routeHealth           = "/health"
	routeRefresh          = "/refresh"
	emptyString           = ""
	msgInvalidMethod      = "Invalid request method"
	msgReadBodyFailed     = "Failed to read request body"
	msgParseJSONFailed    = "Failed to parse JSON"
	msgRefreshInitiated   = "Refresh initiated"
	msgProcessingStarted  = `{"message": "Data received and processing started"}`
	contentTypeJSON       = "application/json"
	mediaListFormat       = "IMDB: %s, Title: %s, OnDisk: %t, File:%s\n"
	nzbListFormat         = "Trakt: %d\nTitle: %s\nLink: %s\nLength: %d\n"
)

func listMedia(w http.ResponseWriter, appConfig App) {
	medias := appConfig.getAllMediaWithIMDB()
	data := formatMediaList(medias)
	writeResponse(w, http.StatusOK, data)
}

func (app App) getAllMediaWithIMDB() []Media {
	var medias []Media
	err := app.Store.Find(&medias, bolthold.Where("IMDB").Ne(emptyString))
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("getting medias from database")
	}
	return medias
}

func formatMediaList(medias []Media) string {
	var data string
	for _, media := range medias {
		data = data + fmt.Sprintf(mediaListFormat, media.IMDB, media.Title, media.OnDisk, media.File)
	}
	return data
}

func writeResponse(w http.ResponseWriter, statusCode int, data string) {
	w.WriteHeader(statusCode)
	if _, err := w.Write([]byte(data)); err != nil {
		log.WithFields(log.Fields{"err": err}).Error("writing response")
	}
}

func listNZBs(w http.ResponseWriter, appConfig App) {
	nzbs := appConfig.getAllNZBs()
	data := formatNZBList(nzbs)
	writeResponse(w, http.StatusOK, data)
}

func (app App) getAllNZBs() []NZB {
	var nzbs []NZB
	q := &bolthold.Query{}
	err := app.Store.Find(&nzbs, q)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("getting NZBs from database")
	}
	return nzbs
}

func formatNZBList(nzbs []NZB) string {
	var data string
	for _, nzb := range nzbs {
		data = data + fmt.Sprintf(nzbListFormat, nzb.Trakt, nzb.Title, nzb.Link, nzb.Length)
	}
	return data
}

func handleAPIRequests(appConfig *App) {
	registerNotifyHandler(appConfig)
	registerListHandler(appConfig)
	registerNZBsHandler(appConfig)
	registerHealthHandler()
	registerRefreshHandler(appConfig)
}

func registerNotifyHandler(appConfig *App) {
	http.HandleFunc(routeNotify, func(w http.ResponseWriter, r *http.Request) {
		handleApiNotify(w, r, *appConfig)
	})
}

func registerListHandler(appConfig *App) {
	http.HandleFunc(routeList, func(w http.ResponseWriter, r *http.Request) {
		listMedia(w, *appConfig)
	})
}

func registerNZBsHandler(appConfig *App) {
	http.HandleFunc(routeNZBs, func(w http.ResponseWriter, r *http.Request) {
		listNZBs(w, *appConfig)
	})
}

func registerHealthHandler() {
	http.HandleFunc(routeHealth, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func registerRefreshHandler(appConfig *App) {
	http.HandleFunc(routeRefresh, func(w http.ResponseWriter, r *http.Request) {
		go appConfig.runTasks()
		writeResponse(w, http.StatusOK, msgRefreshInitiated)
	})
}

func handleApiNotify(w http.ResponseWriter, r *http.Request, appConfig App) {
	if !isPostMethod(r, w) {
		return
	}

	notification, err := parseNotificationRequest(r, w)
	if err != nil {
		return
	}

	go processNotificationAsync(notification, appConfig)
	sendSuccessResponse(w)
}

func isPostMethod(r *http.Request, w http.ResponseWriter) bool {
	if r.Method != http.MethodPost {
		http.Error(w, msgInvalidMethod, http.StatusMethodNotAllowed)
		return false
	}
	return true
}

func parseNotificationRequest(r *http.Request, w http.ResponseWriter) (Notification, error) {
	body, err := readRequestBody(r, w)
	if err != nil {
		return Notification{}, err
	}

	return unmarshalNotification(body, w)
}

func readRequestBody(r *http.Request, w http.ResponseWriter) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, msgReadBodyFailed, http.StatusInternalServerError)
		return nil, err
	}
	defer closeRequestBody(r)
	return body, nil
}

func closeRequestBody(r *http.Request) {
	if err := r.Body.Close(); err != nil {
		log.WithFields(log.Fields{"err": err}).Error("failed to close request body")
	}
}

func unmarshalNotification(body []byte, w http.ResponseWriter) (Notification, error) {
	var notification Notification
	err := json.Unmarshal(body, &notification)
	if err != nil {
		http.Error(w, msgParseJSONFailed, http.StatusBadRequest)
		return notification, err
	}
	return notification, nil
}

func processNotificationAsync(notification Notification, appConfig App) {
	err := processNotification(notification, appConfig)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("processing notification")
	}
}

func sendSuccessResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(msgProcessingStarted)); err != nil {
		log.WithFields(log.Fields{"err": err}).Error("writing response")
	}
}
