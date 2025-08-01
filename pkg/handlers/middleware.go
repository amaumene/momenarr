package handlers

import (
	"net/http"
	"os"
)

const (
	healthPath        = "/health"
	traktCallbackPath = "/api/trakt/callback"
	apiKeyEnvVar      = "MOMENARR_API_KEY"
)

// authMiddleware provides simple API key authentication.
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if shouldSkipAuth(r.URL.Path) {
			next(w, r)
			return
		}

		apiKey := os.Getenv(apiKeyEnvVar)
		if apiKey == "" {
			next(w, r)
			return
		}

		if !isAuthorized(r, apiKey) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized","message":"invalid or missing API key"}`))
			return
		}

		next(w, r)
	}
}

func shouldSkipAuth(path string) bool {
	return path == healthPath || path == traktCallbackPath
}

func isAuthorized(r *http.Request, apiKey string) bool {
	authHeader := r.Header.Get("Authorization")
	expectedHeader := "Bearer " + apiKey

	if authHeader == expectedHeader {
		return true
	}

	return r.Header.Get("X-API-Key") == apiKey
}
