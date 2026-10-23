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
	"youTobe_transcribe/internal/videos"
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

	collector := videos.NewCollector(cfg.YtDlpPath, cfg.CookiesFile, log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("videos.started", "interval", "3h")

	if err := collector.Run(ctx); err != nil {
		log.Error("videos.failed", "error", err.Error())
	}

	ticker := time.NewTicker(3 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		if err := collector.Run(ctx); err != nil {
			log.Error("videos.failed", "error", err.Error())
		}
	}
}