package utils

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
)

// CheckContextCancellation checks if the context has been cancelled
// and returns the error if it has. Returns nil if context is still active.
func CheckContextCancellation(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// WrapServiceError wraps an error with a descriptive message and logs it
func WrapServiceError(action string, err error) error {
	log.WithError(err).Errorf("Failed to %s", action)
	return fmt.Errorf("%s: %w", action, err)
}
