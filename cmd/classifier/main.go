package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"youTobe_transcribe/internal/classifier"
	"youTobe_transcribe/internal/config"
	"youTobe_transcribe/internal/logger"
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

	svc := classifier.NewClassifier(log, cfg.OpenRouterAPIKey, cfg.OpenRouterModel,
		cfg.ClassifierBatchSize, cfg.SummaryChunkSize)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("classifier.started",
		"batch_size", svc.BatchSize,
		"chunk_size", svc.ChunkSize,
		"interval", "по готовности (без пауз)",
		"описание", "Классификатор запущен")

	// Работаем по готовности: обработали батч — сразу следующий, без пауз.
	for {
		select {
		case <-ctx.Done():
			log.Info("classifier.stopped", "описание", "Классификатор остановлен")
			return
		default:
		}

		if err := svc.Run(ctx); err != nil {
			log.Error("classifier.failed", "error", err.Error(),
				"описание", "ОШИБКА: Сбой классификатора, пауза 5с")
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