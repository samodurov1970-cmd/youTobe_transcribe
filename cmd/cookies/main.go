package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"youTobe_transcribe/internal/cookies"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	ytDlpPath := os.Getenv("YT_DLP_PATH")
	if ytDlpPath == "" {
		ytDlpPath = "/usr/local/bin/yt-dlp"
	}
	cookiesFile := os.Getenv("COOKIES_FILE")
	if cookiesFile == "" {
		cookiesFile = "/cookies/cookies.txt"
	}

	checker := cookies.NewChecker(ytDlpPath, cookiesFile, log)

	log.Info("cookies.started", "interval", "1h")

	ctx := context.Background()
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
}