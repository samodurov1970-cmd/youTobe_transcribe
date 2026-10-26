package transcribe

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"youTobe_transcribe/internal/logger"
)

// File вызывает локальный Python-скрипт с faster-whisper
func File(audioPath string) (string, error) {
	log := logger.Log.With("component", "transcribe")
	start := time.Now()

	log.Info("transcribe.start", "audio_path", audioPath, "model", "faster-whisper-medium", "описание", "Запуск распознавания речи")

	cmd := exec.Command("python3", "/app/n8n/whisper_worker.py", audioPath)
	out, err := cmd.Output() // Берем только stdout, stderr игнорируем (там предупреждения)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			log.Error("transcribe.exec_failed", "error", err.Error(), "stderr", string(exitErr.Stderr), "описание", "ОШИБКА: Сбой Python-скрипта")
			return "", fmt.Errorf("python exec failed: %s", string(exitErr.Stderr))
		}
		log.Error("transcribe.exec_failed", "error", err.Error(), "описание", "ОШИБКА: Сбой Python-скрипта")
		return "", fmt.Errorf("python exec failed: %w", err)
	}

	text := strings.TrimSpace(string(out))
	if text == "" {
		return "", fmt.Errorf("empty text from whisper")
	}

	log.Info("transcribe.ok",
		"duration_ms", time.Since(start).Milliseconds(),
		"text_len", len(text),
		"описание", "Распознавание успешно завершено")

	return text, nil
}