package handlers

import (
	"net/http"
	"os"
)

// authMiddleware provides simple API key authentication
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health check
		if r.URL.Path == "/health" {
			next(w, r)
			return
		}
		
		// Skip auth for Trakt OAuth callback
		if r.URL.Path == "/api/trakt/callback" {
			next(w, r)
			return
		}
		
		// Get API key from environment
		apiKey := os.Getenv("MOMENARR_API_KEY")
		if apiKey == "" {
			// If no API key is configured, allow access (backward compatibility)
			// Log a warning in production
			next(w, r)
			return
		}
		
		// Check Authorization header
		authHeader := r.Header.Get("Authorization")
		expectedHeader := "Bearer " + apiKey
		
		if authHeader != expectedHeader {
			// Check X-API-Key header as alternative
			if r.Header.Get("X-API-Key") != apiKey {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"Unauthorized","message":"Invalid or missing API key"}`))
				return
			}
		}
		
		next(w, r)
	}
}