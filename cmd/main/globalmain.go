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
	logger.Init()
	log := logger.Log

	cfg := config.Load()

	if err := storage.Init(cfg.DatabaseURL); err != nil {
		log.Error("storage.init_failed", "error", err.Error())
		os.Exit(1)
	}
	defer storage.DB.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Запуск каналов
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

	// Запуск проверки куки (раз в час с отчётом)
	checker := cookies.NewChecker(
		cfg.YtDlpPath,
		cfg.CookiesFile,
		cfg.TelegramBotToken,
		cfg.TelegramChatID,
		log,
	)
	go func() {
		// Первая проверка и отчёт
		err := checker.Check(ctx)
		checker.SendReport(ctx, err)

		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			err := checker.Check(ctx)
			checker.SendReport(ctx, err)
		}
	}()

	log.Info("app.started", "services", "channels, cookies")
	<-ctx.Done()
	log.Info("app.stopped")
}