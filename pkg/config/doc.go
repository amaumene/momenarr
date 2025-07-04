// Package config provides configuration management for the Momenarr application.
//
// Configuration is loaded from environment variables with sensible defaults.
// The package supports:
//   - Trakt API credentials and OAuth tokens
//   - NZBGet connection settings
//   - Usenet indexer (Newsnab) configuration
//   - File paths for downloads and blacklists
//   - HTTP server settings
//   - Sync intervals and cleanup periods
//
// All configuration values are validated during startup to ensure
// the application has the required settings to function properly.
package config