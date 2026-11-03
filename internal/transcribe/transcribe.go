// cd C:\Users\user\GO\youTobe_transcribe
// docker compose build transcribe
// docker compose up -d
// docker logs -f transcribe

// утром проверить
//docker exec postgres psql -U peter -d youtube_tracker -c "SELECT count(*) FROM video_files WHERE created_at > current_date;"

package transcribe

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"youTobe_transcribe/internal/logger"
)

func File(audioPath string) (string, error) {
	log := logger.Log
	if log == nil {
		log = slog.Default()
	}
	log = log.With("component", "transcribe")
	start := time.Now()

	log.Info("transcribe.start", "audio_path", audioPath, "model", "faster-whisper", "описание", "Запуск распознавания речи")

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("docker", "exec", "transcribe", "python3", "/app/n8n/whisper_worker.py", audioPath)
		cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8", "PYTHONUTF8=1")
	} else {
		cmd = exec.Command("python3", "/app/n8n/whisper_worker.py", audioPath)
	}

	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			log.Error("transcribe.exec_failed", "error", err.Error(), "stderr", string(exitErr.Stderr), "описание", "ОШИБКА: Сбой Python-скрипта")
			return "", fmt.Errorf("python exec failed: %s", string(exitErr.Stderr))
		}
		log.Error("transcribe.exec_failed", "error", err.Error(), "stderr", stderr.String(), "описание", "ОШИБКА: Сбой Python-скрипта")
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