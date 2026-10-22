// Package transcribe отправляет аудио на Groq Whisper для получения текста.
package transcribe

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"youTobe_transcribe/internal/logger"
)

// File отправляет аудиофайл на Groq Whisper и возвращает транскрипт.
func File(transcriptionURL, audioPath string) (string, error) {
	start := time.Now()
	logger.Log.Info("transcribe.start",
		"audio_path", audioPath,
		"model", "whisper-large-v3",
		"language", "ru",
		"transcription_url", transcriptionURL,
	)
	formData := url.Values{}
	formData.Set("file", "@"+audioPath)
	formData.Set("model", "whisper-large-v3")
	formData.Set("language", "ru")

	req, err := http.NewRequest("POST", transcriptionURL, strings.NewReader(formData.Encode()))
	if err != nil {
		logger.Log.Error("transcribe.request_create_failed", logger.Err(err))
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Log.Error("transcribe.request_failed", logger.Err(err), "transcription_url", transcriptionURL)
		return "", fmt.Errorf("transcribe request failed: %w", err)
	}
	defer resp.Body.Close()

	logger.Log.Info("transcribe.response_received",
		"http_status", resp.StatusCode,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Log.Error("transcribe.read_body_failed", logger.Err(err))
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logger.Log.Error("transcribe.http_error",
			"http_status", resp.StatusCode,
			"body_len", len(body),
			"body_preview", string(body[:min(500, len(body))]),
		)
		return "", fmt.Errorf("transcription HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		logger.Log.Error("transcribe.parse_failed", logger.Err(err), "body_preview", string(body[:min(500, len(body))]))
		return "", fmt.Errorf("parse response: %w", err)
	}

	if text, ok := result["text"].(string); ok {
		logger.Log.Info("transcribe.ok",
			"duration_ms", time.Since(start).Milliseconds(),
			"text_len", len(text),
		)
		return strings.TrimSpace(text), nil
	}

	logger.Log.Error("transcribe.no_text_field", "body_preview", string(body[:min(500, len(body))]))
	return "", fmt.Errorf("no 'text' field in response: %s", string(body))
}
