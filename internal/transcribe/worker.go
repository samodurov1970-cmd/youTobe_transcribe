package transcribe

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"youTobe_transcribe/internal/storage"
	"youTobe_transcribe/internal/ytdlp"
)

type Worker struct {
	YtDlpPath string
	Log       *slog.Logger
	BatchSize int
}

func NewWorker(ytDlpPath string, log *slog.Logger) *Worker {
	return &Worker{
		YtDlpPath: ytDlpPath,
		Log:       log.With("component", "transcribe"),
		BatchSize: 2,
	}
}

func (w *Worker) Run(ctx context.Context) error {
	w.Log.Info("transcribe.collect_started", "описание", "Начат цикл транскрибации")

	videos, err := storage.GetPendingVideos(w.BatchSize)
	if err != nil {
		w.Log.Error("transcribe.fetch_failed", "error", err.Error(), "описание", "ОШИБКА: Не удалось получить видео из очереди")
		return err
	}

	if len(videos) == 0 {
		w.Log.Info("transcribe.no_pending", "описание", "Нет видео для транскрибации")
		return nil
	}

	w.Log.Info("transcribe.batch_fetched", "count", len(videos), "описание", "Видео получены из очереди")

	processed := 0
	for _, v := range videos {
		if err := w.processVideo(ctx, v); err != nil {
			w.Log.Error("transcribe.video_failed",
				"video_id", v.VideoID,
				"error", err.Error(),
				"описание", "ОШИБКА: Не удалось обработать видео")
			continue
		}
		processed++
	}

	w.Log.Info("transcribe.collect_finished", "processed", processed, "описание", "Цикл транскрибации завершён")
	return nil
}

func (w *Worker) processVideo(ctx context.Context, v storage.Video) error {
	start := time.Now()
	fileName := safeFileName(v.Title) + ".md"

	w.Log.Info("transcribe.video_started",
		"video_id", v.VideoID,
		"channel_name", v.ChannelName,
		"описание", "Начата обработка видео")

	// === 1. ПРОВЕРКА ДЛИТЕЛЬНОСТИ ===
	duration, err := ytdlp.GetDuration(w.YtDlpPath, v.VideoURL)
	if err != nil {
		w.Log.Warn("transcribe.duration_failed",
			"video_id", v.VideoID,
			"error", err.Error(),
			"описание", "Не удалось получить длительность видео, пропускаем проверку")
	} else {
		if duration > 3600 {
			w.Log.Warn("transcribe.skipped",
				"video_id", v.VideoID,
				"status", "skipped_too_long",
				"duration", duration,
				"описание", "Видео пропущено: слишком длинное")
			_ = storage.InsertTrash(storage.Trash{
				VideoID:     v.VideoID,
				ChannelName: v.ChannelName,
				VideoURL:    v.VideoURL,
				Reason:      fmt.Sprintf("пропущено: слишком длинное (%d сек)", duration),
				Source:      "transcribe",
			})
			_ = storage.MarkVideoProcessed(v.VideoID)
			return nil
		}
		if duration < 10 {
			w.Log.Warn("transcribe.skipped",
				"video_id", v.VideoID,
				"status", "skipped_too_short",
				"duration", duration,
				"описание", "Видео пропущено: слишком короткое")
			_ = storage.InsertTrash(storage.Trash{
				VideoID:     v.VideoID,
				ChannelName: v.ChannelName,
				VideoURL:    v.VideoURL,
				Reason:      fmt.Sprintf("пропущено: слишком короткое (%d сек)", duration),
				Source:      "transcribe",
			})
			_ = storage.MarkVideoProcessed(v.VideoID)
			return nil
		}
		w.Log.Info("transcribe.duration_ok", "video_id", v.VideoID, "duration", duration, "описание", "Длительность видео в допустимых пределах")
	}

	// === 2. СКАЧИВАНИЕ ===
	audioPath, err := ytdlp.DownloadAudio(w.YtDlpPath, v.VideoURL)
	if err != nil {
		errText := err.Error()
		w.Log.Error("transcribe.download_failed",
			"video_id", v.VideoID,
			"error_text", errText,
			"описание", "ОШИБКА: Не удалось скачать аудио")

		if strings.Contains(errText, "404") || strings.Contains(errText, "403") ||
			strings.Contains(errText, "Video unavailable") || strings.Contains(errText, "Private video") {
			_ = storage.InsertTrash(storage.Trash{
				VideoID:     v.VideoID,
				ChannelName: v.ChannelName,
				VideoURL:    v.VideoURL,
				Reason:      "error: видео недоступно",
				Source:      "transcribe",
			})
			_ = storage.MarkVideoProcessed(v.VideoID)
			return nil
		}

		return fmt.Errorf("download audio: %w", err)
	}
	defer ytdlp.Cleanup(audioPath)

	w.Log.Info("transcribe.audio_downloaded",
		"video_id", v.VideoID,
		"audio_path", audioPath,
		"описание", "Аудио успешно скачано")

	// === 3. ТРАНСКРИБАЦИЯ ===
	text, err := File(audioPath)
	if err != nil {
		errText := err.Error()
		w.Log.Error("transcribe.transcribe_failed",
			"video_id", v.VideoID,
			"error_text", errText,
			"описание", "ОШИБКА: Не удалось получить транскрипцию")

		_ = storage.InsertTrash(storage.Trash{
			VideoID:     v.VideoID,
			ChannelName: v.ChannelName,
			VideoURL:    v.VideoURL,
			Reason:      "error: ошибка транскрибации",
			Source:      "transcribe",
		})
		_ = storage.MarkVideoProcessed(v.VideoID)
		return nil
	}

	w.Log.Info("transcribe.transcribed",
		"video_id", v.VideoID,
		"text_len", len(text),
		"описание", "Транскрипция получена")

	// === 4. ПРОВЕРКА НА "НЕТ РЕЧИ" ===
	words := len(strings.Fields(text))
	if len(text) < 10 || words < 3 {
		w.Log.Warn("transcribe.no_speech",
			"video_id", v.VideoID,
			"text_len", len(text),
			"words", words,
			"описание", "Текст отсутствует или слишком короткий")
		_ = storage.InsertTrash(storage.Trash{
			VideoID:     v.VideoID,
			ChannelName: v.ChannelName,
			VideoURL:    v.VideoURL,
			FileContent: text,
			Reason:      "no_speech: нет речи или слишком короткий текст",
			Source:      "transcribe",
		})
		_ = storage.MarkVideoProcessed(v.VideoID)
		return nil
	}

	// === 5. СОХРАНЕНИЕ УСПЕШНОГО ===
	vf := storage.VideoFile{
		VideoID:      v.VideoID,
		ChannelName:  v.ChannelName,
		VideoURL:     v.VideoURL,
		ChannelTopic: v.ChannelTopic,
		FileName:     fileName,
		FileContent:  text,
	}
	if err := storage.InsertVideoFile(vf); err != nil {
		w.Log.Error("transcribe.save_failed", "video_id", v.VideoID, "error", err.Error(), "описание", "ОШИБКА: Не удалось сохранить видеофайл в БД")
		return fmt.Errorf("insert video_file: %w", err)
	}

	if err := storage.MarkVideoProcessed(v.VideoID); err != nil {
		w.Log.Error("transcribe.mark_processed_failed", "video_id", v.VideoID, "error", err.Error(), "описание", "ОШИБКА: Не удалось пометить видео как обработанное")
		return fmt.Errorf("mark processed: %w", err)
	}

	w.Log.Info("transcribe.video_done",
		"video_id", v.VideoID,
		"status", "ok",
		"duration_ms", time.Since(start).Milliseconds(),
		"text_len", len(text),
		"описание", "Видео успешно обработано")
	return nil
}

func safeFileName(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "video"
	}
	replacer := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_",
	)
	name := replacer.Replace(title)
	if len(name) > 200 {
		name = name[:200]
	}
	return name
}