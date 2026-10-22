package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"youTobe_transcribe/internal/channels"
	"youTobe_transcribe/internal/config"
	"youTobe_transcribe/internal/cookies"
	"youTobe_transcribe/internal/logger"
	"youTobe_transcribe/internal/storage"
)

func main() {
	// 1. Инициализируем глобальный логгер (для storage и channels)
	logger.Init()

	// 2. Используем глобальный логгер
	log := logger.Log

	cfg := config.Load()

	// 3. Подключаемся к БД
	if err := storage.Init(cfg.DatabaseURL); err != nil {
		log.Error("storage.init_failed", "error", err.Error())
		os.Exit(1)
	}
	defer storage.DB.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 4. Запускаем каналы
	channelsSvc := channels.NewService(&channels.Config{
		DB:                     storage.DB,
		YtDlpPath:              cfg.YtDlpPath,
		CookiesFile:            cfg.CookiesFile,
		UpdateInterval:         cfg.ChannelsUpdateInterval,
		TelegramToken:          cfg.TelegramBotToken,
		TelegramChatID:         cfg.TelegramChatID,
		OLLAMAURL:              cfg.OLLAMAURL,
		OLLAMAModel:            cfg.OLLAMAModel,
		OLLAMATimeout:          cfg.OLLAMATimeout,
		OLLAMAVideoTitlesLimit: cfg.OLLAMAVideoTitlesLimit,
	})
	go channelsSvc.Run(ctx)

	// 5. Запускаем проверку куки
	checker := cookies.NewChecker(cfg.YtDlpPath, cfg.CookiesFile, log)
	go func() {
		if err := checker.Check(ctx); err != nil {
			log.Error("cookies.failed", "error", err.Error())
		}
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if err := checker.Check(ctx); err != nil {
				log.Error("cookies.failed", "error", err.Error())
			}
		}
	}()

	log.Info("app.started", "services", "channels, cookies")
	<-ctx.Done()
	log.Info("app.stopped")
}