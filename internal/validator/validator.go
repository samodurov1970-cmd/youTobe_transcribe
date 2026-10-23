package validator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"youTobe_transcribe/internal/storage"
)

// Validator проверяет качество текста.
type Validator struct {
	Log       *slog.Logger
	BatchSize int
}

// NewValidator создаёт новый валидатор.
func NewValidator(log *slog.Logger) *Validator {
	return &Validator{
		Log:       log.With("component", "validator"),
		BatchSize: 10,
	}
}

// Run выполняет один цикл проверки.
func (v *Validator) Run(ctx context.Context) error {
	v.Log.Info("validator.collect_started", "описание", "Начат цикл проверки текстов")

	// Берём видео из video_files со статусом 'pending'
	videos, err := storage.GetPendingValidation(v.BatchSize)
	if err != nil {
		v.Log.Error("validator.fetch_failed", "error", err.Error(), "описание", "ОШИБКА: Не удалось получить видео для проверки")
		return err
	}

	if len(videos) == 0 {
		v.Log.Info("validator.no_pending", "описание", "Нет видео для проверки")
		return nil
	}

	v.Log.Info("validator.batch_fetched", "count", len(videos), "описание", "Видео получены для проверки")

	validated := 0
	for _, vf := range videos {
		if err := v.validateVideo(ctx, vf); err != nil {
			v.Log.Error("validator.video_failed", "video_id", vf.VideoID, "error", err.Error(), "описание", "ОШИБКА: Не удалось проверить видео")
			continue
		}
		validated++
	}

	v.Log.Info("validator.collect_finished", "validated", validated, "описание", "Цикл проверки завершён")
	return nil
}

// validateVideo проверяет одно видео.
func (v *Validator) validateVideo(ctx context.Context, vf storage.VideoFile) error {
	start := time.Now()
	v.Log.Info("validator.video_started", "video_id", vf.VideoID, "channel_name", vf.ChannelName, "описание", "Начата проверка видео")

	// Уровень 1: Эвристики
	if reason := isGarbageText(vf.FileContent, vf.ChannelName); reason != "" {
		v.Log.Warn("validator.garbage", "video_id", vf.VideoID, "reason", reason, "описание", "Текст признан мусором")
		return v.moveToTrash(vf, reason)
	}

	// Уровень 2: LLM (проверка через Ollama)
	if reason := v.isGarbageByLLM(ctx, vf.FileContent); reason != "" {
		v.Log.Warn("validator.llm_garbage", "video_id", vf.VideoID, "reason", reason, "описание", "LLM признала текст мусором")
		return v.moveToTrash(vf, reason)
	}

	// Уровень 3: Полнота (сравнение с длительностью видео)
	// (пока пропускаем, нужно запрашивать длительность)

	// Если всё хорошо — помечаем как validated
	if err := storage.MarkVideoValidated(vf.VideoID); err != nil {
		return fmt.Errorf("mark validated: %w", err)
	}

	v.Log.Info("validator.video_done", "video_id", vf.VideoID, "duration_ms", time.Since(start).Milliseconds(), "описание", "Видео успешно проверено")
	return nil
}

// moveToTrash перемещает видео в trash и сбрасывает is_processed.
func (v *Validator) moveToTrash(vf storage.VideoFile, reason string) error {
	// Записываем в trash
	if err := storage.InsertTrash(storage.Trash{
		VideoID:     vf.VideoID,
		ChannelName: vf.ChannelName,
		VideoURL:    vf.VideoURL,
		FileContent: vf.FileContent,
		LastError:   vf.LastError,
		Reason:      reason,
	}); err != nil {
		return fmt.Errorf("insert trash: %w", err)
	}

	// Удаляем из video_files
	if err := storage.DeleteVideoFile(vf.VideoID); err != nil {
		return fmt.Errorf("delete video_file: %w", err)
	}

	// Сбрасываем is_processed = false
	if err := storage.ResetVideoProcessed(vf.VideoID); err != nil {
		return fmt.Errorf("reset processed: %w", err)
	}

	return nil
}

// isGarbageText проверяет текст на мусор (уровень 1).
func isGarbageText(text, channelName string) string {
	// 1. Слишком короткий
	if len(text) < 50 {
		return "слишком короткий текст"
	}

	// 2. Доля кириллицы (если канал англоязычный)
	if isEnglishChannel(channelName) {
		cyrillic := 0
		for _, r := range text {
			if r >= 'А' && r <= 'я' {
				cyrillic++
			}
		}
		if float64(cyrillic)/float64(len(text)) > 0.7 {
			return "кириллица в англоязычном канале"
		}
	}

	// 3. Повторяющиеся слова
	words := strings.Fields(strings.ToLower(text))
	garbage := 0
	for _, w := range words {
		switch w {
		case "и", "я", "it", "is", "the", "a":
			garbage++
		}
	}
	if float64(garbage)/float64(len(words)) > 0.5 {
		return "повторяющиеся слова"
	}

	return ""
}

// isEnglishChannel проверяет, англоязычный ли канал.
func isEnglishChannel(channelName string) bool {
	for _, r := range channelName {
		if r >= 'А' && r <= 'я' {
			return false
		}
	}
	return true
}

// isGarbageByLLM проверяет текст через Ollama.
func (v *Validator) isGarbageByLLM(ctx context.Context, text string) string {
	// TODO: реализовать проверку через Ollama
	return ""
}