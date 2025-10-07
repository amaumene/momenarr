// Package app provides application initialization and lifecycle management.
//
// The App type wires all dependencies together and manages:
// - Configuration loading
// - Database initialization
// - Service creation
// - HTTP server lifecycle
// - Graceful shutdown
//
// The Orchestrator runs periodic background tasks for syncing,
// searching, downloading, and cleanup operations.
package app
