package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"youTobe_transcribe/internal/config"
	"youTobe_transcribe/internal/logger"
	"youTobe_transcribe/internal/storage"
	"youTobe_transcribe/internal/transcribe"
)

func main() {
	logger.Init()
	log := logger.Log

	cfg := config.Load()

	if err := storage.Init(cfg.DatabaseURL); err != nil {
		log.Error("storage.init_failed", "error", err.Error())
		os.Exit(1)
	}
	defer storage.DB.Close()

	// Передаем только YtDlpPath и logger
	svc := transcribe.NewWorker(cfg.YtDlpPath, log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("transcribe.started", "batch_size", svc.BatchSize)

	if err := svc.Run(ctx); err != nil {
		log.Error("transcribe.failed", "error", err.Error())
	}

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Info("transcribe.stopped")
			return
		case <-ticker.C:
			if err := svc.Run(ctx); err != nil {
				log.Error("transcribe.failed", "error", err.Error())
			}
		}
	}
}