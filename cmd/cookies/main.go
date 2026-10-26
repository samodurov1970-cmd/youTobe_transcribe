package main

import (
	"context"
	"os"
	"time"

	"youTobe_transcribe/internal/cookies"
	"youTobe_transcribe/internal/logger"
)

func main() {
	logger.Init()
	log := logger.Log.With("component", "cookies")

	ytDlpPath := os.Getenv("YT_DLP_PATH")
	if ytDlpPath == "" {
		ytDlpPath = "/usr/local/bin/yt-dlp"
	}
	cookiesFile := os.Getenv("COOKIES_FILE")
	if cookiesFile == "" {
		cookiesFile = "/cookies/cookies.txt"
	}
	telegramToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	telegramChatID := os.Getenv("TELEGRAM_CHAT_ID")

	checker := cookies.NewChecker(ytDlpPath, cookiesFile, telegramToken, telegramChatID, log)

	log.Info("cookies.started", "interval", "1h", "описание", "Микросервис проверки куки запущен")

	ctx := context.Background()
	err := checker.Check(ctx)
	checker.SendReport(ctx, err)

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		err := checker.Check(ctx)
		checker.SendReport(ctx, err)
	}
}