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
	"youTobe_transcribe/internal/trash_manager"
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

	svc := trash_manager.NewManager(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("trash_manager.started", "batch_size", svc.BatchSize)

	if err := svc.Run(ctx); err != nil {
		log.Error("trash_manager.failed", "error", err.Error())
	}

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		if err := svc.Run(ctx); err != nil {
			log.Error("trash_manager.failed", "error", err.Error())
		}
	}
}