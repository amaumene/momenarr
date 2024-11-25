package main

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
)

func handleAPIRequests(appConfig *App) {
	http.HandleFunc("/api/notify", func(w http.ResponseWriter, r *http.Request) {
		handleApiNotify(w, r, *appConfig)
	})
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	http.HandleFunc("/refresh", func(w http.ResponseWriter, r *http.Request) {
		go func() {
			appConfig.runTasks()
		}()
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("Refresh initiated")); err != nil {
			log.WithFields(log.Fields{"err": err}).Error("writing refresh response")
		}
	})
}

func handleApiNotify(w http.ResponseWriter, r *http.Request, appConfig App) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.WithFields(log.Fields{"err": err}).Error("failed to close request body")
		}
	}()

	var notification Notification
	err = json.Unmarshal(body, &notification)
	if err != nil {
		http.Error(w, "Failed to parse JSON", http.StatusBadRequest)
		return
	}
	go func() {
		err := processNotification(notification, appConfig)
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("processing notification")
		}
	}()

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write([]byte(`{"message": "Data received and processing started"}`)); err != nil {
		log.WithFields(log.Fields{"err": err}).Error("writing response")
	}
}
