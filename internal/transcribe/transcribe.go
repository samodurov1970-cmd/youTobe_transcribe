package transcribe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"youTobe_transcribe/internal/logger"
)

// File отправляет аудиофайл на Groq Whisper и возвращает транскрипт.
func File(transcriptionURL, audioPath string) (string, error) {
	log := logger.Log.With("component", "transcribe")

	start := time.Now()
	log.Info("transcribe.start",
		"audio_path", audioPath,
		"model", "whisper-large-v3",
		"transcription_url", transcriptionURL,
	)

	// Открываем файл
	file, err := os.Open(audioPath)
	if err != nil {
		log.Error("transcribe.open_failed", "error", err.Error())
		return "", fmt.Errorf("open audio file: %w", err)
	}
	defer file.Close()

	// Создаем multipart/form-data
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Добавляем файл
	part, err := writer.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		log.Error("transcribe.create_form_failed", "error", err.Error())
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		log.Error("transcribe.copy_failed", "error", err.Error())
		return "", fmt.Errorf("copy file to form: %w", err)
	}

	// Добавляем поля (без language, чтобы Whisper сам определил язык)
	_ = writer.WriteField("model", "whisper-large-v3")
	writer.Close()

	// Создаем запрос
	req, err := http.NewRequest("POST", transcriptionURL, body)
	if err != nil {
		log.Error("transcribe.request_create_failed", "error", err.Error())
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Error("transcribe.request_failed", "error", err.Error(), "transcription_url", transcriptionURL)
		return "", fmt.Errorf("transcribe request failed: %w", err)
	}
	defer resp.Body.Close()

	log.Info("transcribe.response_received",
		"http_status", resp.StatusCode,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("transcribe.read_body_failed", "error", err.Error())
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Error("transcribe.http_error",
			"http_status", resp.StatusCode,
			"body_len", len(bodyBytes),
			"body_preview", string(bodyBytes[:min(500, len(bodyBytes))]),
		)
		return "", fmt.Errorf("transcription HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		log.Error("transcribe.parse_failed", "error", err.Error(), "body_preview", string(bodyBytes[:min(500, len(bodyBytes))]))
		return "", fmt.Errorf("parse response: %w", err)
	}

	if text, ok := result["text"].(string); ok {
		log.Info("transcribe.ok",
			"duration_ms", time.Since(start).Milliseconds(),
			"text_len", len(text),
		)
		return strings.TrimSpace(text), nil
	}

	log.Error("transcribe.no_text_field", "body_preview", string(bodyBytes[:min(500, len(bodyBytes))]))
	return "", fmt.Errorf("no 'text' field in response: %s", string(bodyBytes))
}