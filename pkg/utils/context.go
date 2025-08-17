package utils

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
)

func CheckContextCancellation(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func WrapServiceError(action string, err error) error {
	if err == nil {
		return nil
	}
	log.WithError(err).Errorf("failed to %s", action)
	return fmt.Errorf("failed to %s: %w", action, err)
}
