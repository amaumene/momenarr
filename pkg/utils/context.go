package utils

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
)

// CheckContextCancellation checks if the context has been cancelled.
func CheckContextCancellation(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// WrapServiceError wraps an error with a descriptive message and logs it.
func WrapServiceError(action string, err error) error {
	if err == nil {
		return nil
	}
	log.WithError(err).Errorf("failed to %s", action)
	return fmt.Errorf("failed to %s: %w", action, err)
}
