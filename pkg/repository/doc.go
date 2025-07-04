// Package repository provides the data access layer for the Momenarr application.
//
// It defines the Repository interface and implements it using BoltDB as the
// underlying storage engine. The repository handles:
//   - Media persistence and queries
//   - NZB storage and retrieval with quality-based prioritization
//   - Batch operations for improved performance
//   - Context-aware operations for graceful cancellation
//
// The implementation uses BoltDB for embedded, serverless persistence with
// ACID guarantees and efficient concurrent access.
package repository