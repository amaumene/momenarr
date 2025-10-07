// Package handler implements HTTP request handlers.
//
// This package provides HTTP endpoints for:
// - /api/notify: webhook for download completion notifications
// - /list: list all tracked media
// - /nzbs: list all cached NZB entries
// - /health: health check endpoint
// - /refresh: trigger immediate sync and download
//
// All handlers extract context from requests and pass to services.
package handler
