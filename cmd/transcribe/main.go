// Command transcribe — сервис транскрибации YouTube-видео.
//
// Скачивает аудио через yt-dlp, отправляет на Groq Whisper,
// сохраняет транскрипт в PostgreSQL и Obsidian.
package main

import (
	"context"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"

	"youTobe_transcribe/internal/channels"
	"youTobe_transcribe/internal/config"
	"youTobe_transcribe/internal/logger"
	"youTobe_transcribe/internal/storage"
	// "youTobe_transcribe/internal/workflow" — подсистема транскрибации временно
	// отключена: работаем только с подсистемой channels.
)

func main() {
	logger.Init()
	log := logger.Log
	log.Info("app.start",
		"component", "youTobe_transcribe",
		"pid", os.Getpid(),
		"go_version", runtime.Version(),
	)

	cfg := config.Load()
	log.Info("config.loaded",
		"database_url_host", cfg.DatabaseURL[:min(40, len(cfg.DatabaseURL))]+"...",
		"obsidian_path", cfg.ObsidianPath,
		"yt_dlp_path", cfg.YtDlpPath,
		"transcription_url", cfg.TranscriptionURL,
		"openrouter_model", cfg.OpenRouterModel,
		"openrouter_api_key", config.MaskSecret(cfg.OpenRouterAPIKey),
	)

	if err := storage.Init(cfg.DatabaseURL); err != nil {
		log.Error("db.init_failed", logger.Err(err))
		os.Exit(1)
	}
	defer storage.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Подсистема обновления подписок: стартует сразу и повторяется раз в сутки.
	// Работает в отдельной горутине с общим подключением к базе данных.
	channelsSvc := channels.NewService(&channels.Config{
		DB:                  storage.DB,
		YtDlpPath:           cfg.YtDlpPath,
		CookiesFile:         cfg.CookiesFile,
		UpdateInterval:      cfg.ChannelsUpdateInterval,
		TelegramToken:       cfg.TelegramBotToken,
		TelegramChatID:      cfg.TelegramChatID,
		OLLAMAURL:           cfg.OLLAMAURL,
		OLLAMAModel:         cfg.OLLAMAModel,
		OLLAMATimeout:       cfg.OLLAMATimeout,
		OLLAMAVideoTitlesLimit: cfg.OLLAMAVideoTitlesLimit,
	})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		channelsSvc.Run(ctx)
	}()

	// Подсистема транскрибации (internal/workflow) временно отключена.
	// Единственная активная подсистема — channels, она работает в своей горутине.
	log.Info("scheduler.start", "description", "channels_only")

	<-ctx.Done()
	log.Info("app.stop", "reason", "signal")
	wg.Wait()
}
