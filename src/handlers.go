package main

import (
	"encoding/json"
	"fmt"
	"github.com/amaumene/momenarr/bolthold"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
)

func listMedia(w http.ResponseWriter, appConfig App) {
	w.WriteHeader(http.StatusOK)
	var medias []Media
	err := appConfig.Store.Find(&medias, bolthold.Where("IMDB").Ne(""))
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("getting medias from database")
	}
	var data string
	for _, media := range medias {
		data = data + fmt.Sprintf("IMDB: %s\t, Title: %s\t, OnDisk: %t\t, DownloadID:%s\n", media.IMDB, media.Title, media.OnDisk, media.DownloadID)
	}
	if _, err := w.Write([]byte(data)); err != nil {
		log.WithFields(log.Fields{"err": err}).Error("writing response")
	}
}
func listNZBs(w http.ResponseWriter, appConfig App) {
	w.WriteHeader(http.StatusOK)
	var nzbs []NZB
	err := appConfig.Store.Find(&nzbs, bolthold.Where("IMDB").Ne(""))
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("getting NZBs from database")
	}
	var data string
	for _, nzb := range nzbs {
		data = data + fmt.Sprintf("IMDB: %s\t, Title: %s\t, Link: %s\t, Length: %d\n", nzb.IMDB, nzb.Title, nzb.Link, nzb.Length)
	}
	if _, err := w.Write([]byte(data)); err != nil {
		log.WithFields(log.Fields{"err": err}).Error("writing response")
	}
}
func handleAPIRequests(appConfig *App) {
	http.HandleFunc("/api/success", func(w http.ResponseWriter, r *http.Request) {
		handleApiSuccess(w, r, *appConfig)
	})
	http.HandleFunc("/api/failure", func(w http.ResponseWriter, r *http.Request) {
		handleApiFailure(w, r, *appConfig)
	})
	http.HandleFunc("/list", func(w http.ResponseWriter, r *http.Request) {
		listMedia(w, *appConfig)
	})
	http.HandleFunc("/nzbs", func(w http.ResponseWriter, r *http.Request) {
		listNZBs(w, *appConfig)
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

func handleApiSuccess(w http.ResponseWriter, r *http.Request, appConfig App) {
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

	var notification Success
	err = json.Unmarshal(body, &notification)
	if err != nil {
		http.Error(w, "Failed to parse JSON", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write([]byte(`{"message": "Data received and processing started"}`)); err != nil {
		log.WithFields(log.Fields{"err": err}).Error("writing response")
	}
	err = processSuccess(notification, appConfig)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("processing notification")
	}
}

func handleApiFailure(w http.ResponseWriter, r *http.Request, appConfig App) {
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

	var notification Failure
	err = json.Unmarshal(body, &notification)
	if err != nil {
		http.Error(w, "Failed to parse JSON", http.StatusBadRequest)
		return
	}
	go func() {
		err := processFailure(notification, appConfig)
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
