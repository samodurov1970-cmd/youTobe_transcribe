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

// Worker скачивает аудио, транскрибирует и сохраняет текст.
type Worker struct {
	YtDlpPath        string
	TranscriptionURL string
	Log              *slog.Logger
	BatchSize        int
}

// NewWorker создаёт новый воркер транскрибации.
func NewWorker(ytDlpPath, transcriptionURL string, log *slog.Logger) *Worker {
	return &Worker{
		YtDlpPath:        ytDlpPath,
		TranscriptionURL: transcriptionURL,
		Log:              log.With("component", "transcribe"),
		BatchSize:        2,
	}
}

// Run выполняет один цикл: берёт видео, скачивает, транскрибирует, сохраняет.
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
				"channel_name", v.ChannelName,
				"error", err.Error(),
				"описание", "ОШИБКА: Не удалось обработать видео")
			continue
		}
		processed++
	}

	w.Log.Info("transcribe.collect_finished", "processed", processed, "описание", "Цикл транскрибации завершён")
	return nil
}

// processVideo обрабатывает одно видео.
func (w *Worker) processVideo(ctx context.Context, v storage.Video) error {
	start := time.Now()
	w.Log.Info("transcribe.video_started", "video_id", v.VideoID, "channel_name", v.ChannelName, "описание", "Начата обработка видео")

	fileName := safeFileName(v.Title) + ".md"

	// 1. Скачиваем аудио
	audioPath, err := ytdlp.DownloadAudio(w.YtDlpPath, v.VideoURL)
	if err != nil {
		errText := err.Error()
		w.Log.Error("transcribe.download_failed", "video_id", v.VideoID, "error", errText, "описание", "ОШИБКА: Не удалось скачать аудио")

		// Если видео удалено / приватное — помечаем как обработанное (не повторять)
		if strings.Contains(errText, "404") || strings.Contains(errText, "403") ||
			strings.Contains(errText, "Video unavailable") || strings.Contains(errText, "Private video") {
			_ = storage.InsertVideoFile(storage.VideoFile{
				VideoID:      v.VideoID,
				ChannelName:  v.ChannelName,
				VideoURL:     v.VideoURL,
				ChannelTopic: v.ChannelTopic,
				FileName:     fileName,
				FileContent:  "",
				LastError:    "видео недоступно: " + errText,
			})
			_ = storage.MarkVideoProcessed(v.VideoID)
			return nil
		}

		// Таймаут или другая ошибка — не помечаем как обработанное (повторить позже)
		_ = storage.MarkVideoFailed(v.VideoID, fileName, errText)
		return fmt.Errorf("download audio: %w", err)
	}
	defer ytdlp.Cleanup(audioPath)

	w.Log.Info("transcribe.audio_downloaded", "video_id", v.VideoID, "audio_path", audioPath, "описание", "Аудио скачано")

	// 2. Транскрибируем
	text, err := File(w.TranscriptionURL, audioPath)
	if err != nil {
		errText := err.Error()
		w.Log.Error("transcribe.transcribe_failed", "video_id", v.VideoID, "error", errText, "описание", "ОШИБКА: Не удалось получить транскрипцию")

		// 413 (слишком большое аудио) — помечаем как обработанное (не повторять)
		if strings.Contains(errText, "413") {
			_ = storage.InsertVideoFile(storage.VideoFile{
				VideoID:      v.VideoID,
				ChannelName:  v.ChannelName,
				VideoURL:     v.VideoURL,
				ChannelTopic: v.ChannelTopic,
				FileName:     fileName,
				FileContent:  "",
				LastError:    "аудио слишком большое (413)",
			})
			_ = storage.MarkVideoProcessed(v.VideoID)
			return nil
		}

		// 429 (лимит) — не помечаем (повторить позже)
		if strings.Contains(errText, "429") {
			_ = storage.MarkVideoFailed(v.VideoID, fileName, "превышен лимит запросов (429)")
			return fmt.Errorf("whisper rate limit: %w", err)
		}

		// Другие ошибки — не помечаем (повторить позже)
		_ = storage.MarkVideoFailed(v.VideoID, fileName, errText)
		return fmt.Errorf("transcribe: %w", err)
	}

	w.Log.Info("transcribe.transcribed", "video_id", v.VideoID, "text_len", len(text), "описание", "Транскрипция получена")

	// 3. Проверяем, что текст осмысленный (не пустой и не мусор)
	words := len(strings.Fields(text))
	if len(text) < 10 || words < 3 {
		w.Log.Warn("transcribe.short_text",
			"video_id", v.VideoID,
			"text_len", len(text),
			"words", words,
			"описание", "Whisper вернул слишком короткий текст (нет речи?)")
		_ = storage.InsertVideoFile(storage.VideoFile{
			VideoID:      v.VideoID,
			ChannelName:  v.ChannelName,
			VideoURL:     v.VideoURL,
			ChannelTopic: v.ChannelTopic,
			FileName:     fileName,
			FileContent:  "",
			LastError:    "слишком короткий текст (нет речи)",
		})
		_ = storage.MarkVideoProcessed(v.VideoID)
		return nil
	}

	// 4. Сохраняем текст
	vf := storage.VideoFile{
		VideoID:      v.VideoID,
		ChannelName:  v.ChannelName,
		VideoURL:     v.VideoURL,
		ChannelTopic: v.ChannelTopic,
		FileName:     fileName,
		FileContent:  text,
	}
	if err := storage.InsertVideoFile(vf); err != nil {
		return fmt.Errorf("insert video_file: %w", err)
	}

	if err := storage.MarkVideoProcessed(v.VideoID); err != nil {
		return fmt.Errorf("mark processed: %w", err)
	}

	w.Log.Info("transcribe.video_done",
		"video_id", v.VideoID,
		"duration_ms", time.Since(start).Milliseconds(),
		"описание", "Видео успешно обработано")
	return nil
}

// safeFileName заменяет недопустимые символы в имени файла.
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