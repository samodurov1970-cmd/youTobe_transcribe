package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"youTobe_transcribe/internal/config"
	"youTobe_transcribe/internal/logger"
	"youTobe_transcribe/internal/publisher"
	"youTobe_transcribe/internal/storage"
)

func main() {
	logger.Init()
	log := logger.Log

	cfg := config.Load()

	if err := storage.Init(cfg.DatabaseURL); err != nil {
		log.Error("storage.init_failed", "error", err.Error(),
			"описание", "ОШИБКА: Не удалось подключиться к базе данных")
		os.Exit(1)
	}
	defer storage.DB.Close()

	svc := publisher.NewPublisher(log, cfg.ObsidianPath, cfg.PublisherBatchSize)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("publisher.started",
		"batch_size", svc.BatchSize,
		"obsidian_path", cfg.ObsidianPath,
		"interval", "по готовности (без пауз)",
		"описание", "Публикатор (запись) запущен")

	// Работаем по готовности: записали батч — сразу следующий, без пауз.
	for {
		select {
		case <-ctx.Done():
			log.Info("publisher.stopped", "описание", "Публикатор остановлен")
			return
		default:
		}

		if err := svc.Run(ctx); err != nil {
			log.Error("publisher.failed", "error", err.Error(),
				"описание", "ОШИБКА: Сбой публикатора, пауза 5с")
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}

		// Если Run ничего не взял (no_pending) — пауза 10с, чтобы не крутить вхолостую
		time.Sleep(10 * time.Second)
	}
}