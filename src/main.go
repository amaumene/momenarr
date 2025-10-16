package main

import (
	"context"
	"os"

	"github.com/amaumene/momenarr/internal/app"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	application, err := app.New()
	if err != nil {
		log.WithField("error", err).Error("application initialization failed")
		os.Exit(1)
	}

	ctx := context.Background()
	if err := application.Run(ctx); err != nil {
		log.WithField("error", err).Error("application runtime error")
		os.Exit(1)
	}

	log.Info("application exited cleanly")
}
