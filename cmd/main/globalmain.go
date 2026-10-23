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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 1. Запуск каналов
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

	// 2. Запуск проверки куки (раз в час с отчётом)
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

	// 3. Запуск сбора видео (раз в 3 часа)
	collector := videos.NewCollector(cfg.YtDlpPath, cfg.CookiesFile, log)
	go func() {
		// Первый запуск сразу при старте
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
	}()

	// 4. Часовой отчёт по видео в Telegram
	go func() {
		// Первый отчёт сразу при старте
		stats, err := storage.GetVideoStats()
		if err == nil {
			checker.SendDailyStats(ctx, stats)
		} else {
			log.Error("hourly_stats.failed", "error", err.Error())
		}

		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			stats, err := storage.GetVideoStats()
			if err != nil {
				log.Error("hourly_stats.failed", "error", err.Error())
				continue
			}
			checker.SendDailyStats(ctx, stats)
		}
	}()

	log.Info("app.started", "services", "channels, cookies, videos, hourly_stats")
	<-ctx.Done()
	log.Info("app.stopped")
}