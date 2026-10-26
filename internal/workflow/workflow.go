package workflow

import (
	"context"
	"fmt"
	"log/slog"

	"youTobe_transcribe/internal/config"
	"youTobe_transcribe/internal/storage"
	"youTobe_transcribe/internal/transcribe"
	"youTobe_transcribe/internal/ytdlp"
)

type Workflow struct {
	cfg *config.Config
	log *slog.Logger
}

func New(cfg *config.Config, log *slog.Logger) *Workflow {
	return &Workflow{
		cfg: cfg,
		log: log.With("component", "workflow"),
	}
}

// ProcessVideo скачивает аудио и транскрибирует его (пример функции, где была ошибка)
func (w *Workflow) ProcessVideo(ctx context.Context, videoID, videoURL string) error {
	// 1. Скачиваем аудио
	audioPath, err := ytdlp.DownloadAudio(w.cfg.YtDlpPath, videoURL)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer ytdlp.Cleanup(audioPath)

	// 2. Транскрибируем (ВОТ ЗДЕСЬ БЫЛА ОШИБКА, теперь передаем только audioPath)
	text, err := transcribe.File(audioPath)
	if err != nil {
		return fmt.Errorf("transcribe: %w", err)
	}

	// 3. Сохраняем результат
	vf := storage.VideoFile{
		VideoID:     videoID,
		VideoURL:    videoURL,
		FileName:    videoID + ".md",
		FileContent: text,
	}
	if err := storage.InsertVideoFile(vf); err != nil {
		return fmt.Errorf("save: %w", err)
	}

	w.log.Info("workflow.video_done", "video_id", videoID, "text_len", len(text))
	return nil
}