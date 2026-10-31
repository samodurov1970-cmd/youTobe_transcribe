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
	"youTobe_transcribe/internal/trash_manager"
	"youTobe_transcribe/internal/validator"
	"youTobe_transcribe/internal/videos"
)

func main() {
	logger.Init()
	log := logger.Log

	cfg := config.Load()

	if err := storage.Init(cfg.DatabaseURL); err != nil {
		log.Error("storage.init_failed", "error", err.Error(), "описание", "ОШИБКА: Не удалось подключиться к базе данных")
		os.Exit(1)
	}
	defer storage.DB.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 1. Запуск каналов (раз в 60 минут)
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

	// 2. Запуск проверки куки (в 9:00 и 21:00)
	checker := cookies.NewChecker(
		cfg.YtDlpPath,
		cfg.CookiesFile,
		cfg.TelegramBotToken,
		cfg.TelegramChatID,
		log,
	)
	go func() {
		err := checker.Check(ctx)
		checker.SendReport(ctx, err)

		for {
			next := nextTime(9, 0)
			if next.Before(time.Now().Add(1 * time.Minute)) {
				next = nextTime(21, 0)
			}
			log.Info("cookies.next_check", "next", next.Format("15:04"), "описание", "Запланирована следующая проверка куки")
			time.Sleep(time.Until(next))

			err := checker.Check(ctx)
			checker.SendReport(ctx, err)
		}
	}()

	// 3. Запуск сбора видео (БЕСКОНЕЧНЫЙ ЦИКЛ С ПАУЗОЙ 30 МИНУТ)
	collector := videos.NewCollector(cfg.YtDlpPath, cfg.CookiesFile, log)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if err := collector.Run(ctx); err != nil {
				log.Error("videos.failed", "error", err.Error(), "описание", "ОШИБКА: Сбой при сборе видео")
			}
			// Пауза 30 минут между циклами
			time.Sleep(30 * time.Minute)
		}
	}()

	// 4. Ежедневная статистика по видео (раз в сутки в 00:00)
	go func() {
		for {
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
			log.Info("stats.next_daily", "next", next.Format("02.01.2006 15:04"), "описание", "Запланирована ежедневная статистика")
			time.Sleep(time.Until(next))

			stats, err := storage.GetVideoStats()
			if err != nil {
				log.Error("daily_stats.failed", "error", err.Error(), "описание", "ОШИБКА: Не удалось получить статистику видео")
				continue
			}
			checker.SendDailyStats(ctx, stats)
		}
	}()

	// 5. Запуск валидатора (раз в 5 минут)
	validatorSvc := validator.NewValidator(log)
	go func() {
		if err := validatorSvc.Run(ctx); err != nil {
			log.Error("validator.failed", "error", err.Error(), "описание", "ОШИБКА: Сбой валидатора")
		}
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if err := validatorSvc.Run(ctx); err != nil {
				log.Error("validator.failed", "error", err.Error(), "описание", "ОШИБКА: Сбой валидатора")
			}
		}
	}()

	// 6. Запуск trash_manager (раз в 5 минут)
	trashSvc := trash_manager.NewManager(log)
	go func() {
		if err := trashSvc.Run(ctx); err != nil {
			log.Error("trash_manager.failed", "error", err.Error(), "описание", "ОШИБКА: Сбой менеджера корзины")
		}
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if err := trashSvc.Run(ctx); err != nil {
				log.Error("trash_manager.failed", "error", err.Error(), "описание", "ОШИБКА: Сбой менеджера корзины")
			}
		}
	}()

	log.Info("app.started", "services", "channels, cookies, videos, stats, validator, trash_manager", "описание", "Главный процесс запущен, все подсистемы работают")
	<-ctx.Done()
	log.Info("app.stopped", "описание", "Главный процесс остановлен")
}

func nextTime(hour, minute int) time.Time {
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next
}