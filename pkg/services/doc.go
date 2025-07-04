// Package services provides the core business logic for the Momenarr application.
//
// It includes services for:
//   - Trakt integration: Syncing watchlists and favorites
//   - NZB management: Searching and managing NZB files from Usenet indexers
//   - Download orchestration: Managing downloads via NZBGet
//   - Notification processing: Handling download completion/failure notifications
//   - Cleanup operations: Removing watched media based on Trakt history
//
// All services support context-based cancellation for graceful shutdown.
package services