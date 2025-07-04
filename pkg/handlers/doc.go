// Package handlers provides HTTP request handlers for the Momenarr API.
//
// The API includes endpoints for:
//   - Health checks and readiness probes
//   - Media status and statistics
//   - Manual workflow triggers
//   - Download management (retry, cancel, status)
//   - NZBGet webhook notifications
//   - Trakt OAuth token management
//
// All handlers include proper error handling, request validation,
// and JSON response formatting.
package handlers