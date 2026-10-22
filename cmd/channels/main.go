package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"youTobe_transcribe/internal/channels"
	"youTobe_transcribe/internal/config"
	"youTobe_transcribe/internal/storage"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg := config.Load()

	// storage.Init открывает БД и сохраняет её в глобальную переменную storage.DB
	if err := storage.Init(cfg.DatabaseURL); err != nil {
		log.Error("storage.init_failed", "error", err.Error())
		os.Exit(1)
	}
	defer storage.DB.Close()

	svc := channels.NewService(&channels.Config{
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("channels.started")
	svc.Run(ctx)
}