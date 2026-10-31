package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"youTobe_transcribe/internal/config"
	"youTobe_transcribe/internal/logger"
	"youTobe_transcribe/internal/storage"
	"youTobe_transcribe/internal/transcribe"
)

// sendTelegram отправляет сообщение в Telegram.
func sendTelegram(token, chatID, text string) {
	if token == "" || chatID == "" {
		return
	}
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "HTML",
	}
	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err == nil {
		resp.Body.Close()
	}
}

func main() {
	logger.Init()
	log := logger.Log

	cfg := config.Load()

	if err := storage.Init(cfg.DatabaseURL); err != nil {
		log.Error("storage.init_failed", "error", err.Error(), "описание", "ОШИБКА: Не удалось подключиться к БД")
		os.Exit(1)
	}
	defer storage.DB.Close()

	svc := transcribe.NewWorker(cfg.YtDlpPath, log)

	// Контекст, который отменяется по сигналу ОС (Ctrl+C) или по таймеру
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Если задано время сессии, запускаем таймер
	if cfg.TranscribeSessionDuration > 0 {
		log.Info("transcribe.timer_started", "duration", cfg.TranscribeSessionDuration, "описание", "Установлен таймер работы транскрибации")
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.TranscribeSessionDuration)
		defer cancel()
	}

	log.Info("transcribe.started", "batch_size", svc.BatchSize, "описание", "Микросервис транскрибации запущен")
	
	// Уведомление о старте
	startMsg := fmt.Sprintf("🚀 <b>Транскрибация запущена</b>\n\n⏱ Время работы: %s", cfg.TranscribeSessionDuration)
	sendTelegram(cfg.TelegramBotToken, cfg.TelegramChatID, startMsg)

	if err := svc.Run(ctx); err != nil {
		log.Error("transcribe.failed", "error", err.Error(), "описание", "ОШИБКА: Сбой в цикле транскрибации")
	}

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Info("transcribe.stopped", "описание", "Время работы транскрибации истекло или получен сигнал остановки")
			// Уведомление о завершении
			stopMsg := "🛑 <b>Транскрибация завершена</b>\n\n⏱ Время сессии истекло (или получен сигнал остановки)."
			sendTelegram(cfg.TelegramBotToken, cfg.TelegramChatID, stopMsg)
			return
		case <-ticker.C:
			if err := svc.Run(ctx); err != nil {
				log.Error("transcribe.failed", "error", err.Error(), "описание", "ОШИБКА: Сбой в цикле транскрибации")
			}
		}
	}
}