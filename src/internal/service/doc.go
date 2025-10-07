// Package service contains business logic for momenarr operations.
//
// Services orchestrate between domain repositories and external clients:
// - DownloadService: manages NZB downloads
// - NZBService: searches and filters NZB files
// - MediaService: syncs movies and episodes from Trakt
// - NotificationService: handles download completion notifications
// - CleanupService: removes watched media based on Trakt history
//
// All services follow the single responsibility principle and accept
// context for cancellation support.
package service
