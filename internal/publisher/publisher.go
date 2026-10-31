// Package publisher пишет готовые конспекты из obsidian_targets в Obsidian.
package publisher

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"youTobe_transcribe/internal/storage"
)

// Publisher пишет obsidian_targets(status='ready') в .md-файлы.
type Publisher struct {
	Log          *slog.Logger
	BatchSize    int
	ObsidianPath string
}

// NewPublisher создаёт публикатор (только запись).
func NewPublisher(log *slog.Logger, obsidianPath string, batchSize int) *Publisher {
	if batchSize <= 0 {
		batchSize = 1
	}
	return &Publisher{
		Log:          log.With("component", "publisher"),
		BatchSize:    batchSize,
		ObsidianPath: obsidianPath,
	}
}

// Run выполняет один цикл записи.
func (p *Publisher) Run(ctx context.Context) error {
	p.Log.Info("publisher.collect_started", "описание", "Начат цикл записи в Obsidian")

	items, err := storage.GetReadyForWrite(p.BatchSize)
	if err != nil {
		p.Log.Error("publisher.fetch_failed", "error", err.Error(),
			"описание", "ОШИБКА: Не удалось получить готовые obsidian_targets")
		return err
	}

	if len(items) == 0 {
		p.Log.Info("publisher.no_pending", "описание", "Нет готовых конспектов для записи")
		return nil
	}

	p.Log.Info("publisher.batch_fetched", "count", len(items),
		"описание", "Конспекты получены для записи")

	processed := 0
	for _, t := range items {
		if err := p.writeTarget(t); err != nil {
			p.Log.Error("publisher.target_failed", "target_id", t.ID, "video_id", t.VideoID,
				"error", err.Error(), "описание", "ОШИБКА: Не удалось записать конспект")
			_ = storage.MarkObsidianTargetError(t.ID)
			continue
		}
		processed++
	}

	p.Log.Info("publisher.collect_finished", "processed", processed,
		"описание", "Цикл записи завершён")
	return nil
}

func (p *Publisher) writeTarget(t storage.ObsidianTarget) error {
	start := time.Now()
	p.Log.Info("publisher.target_started", "target_id", t.ID, "video_id", t.VideoID,
		"part", t.Part, "chapter", t.Chapter, "описание", "Начата запись конспекта")

	written, skipped, err := p.writeBlock(t.Part, t.Chapter, t.VideoURL, t.VideoTitle, t.Block)
	if err != nil {
		return fmt.Errorf("writeBlock: %w", err)
	}
	if skipped {
		p.Log.Info("publisher.duplicate_skipped", "target_id", t.ID, "video_id", t.VideoID,
			"video_url", t.VideoURL, "описание", "Блок уже был в файле, дубликат пропущен")
	} else if written {
		p.Log.Info("publisher.written", "target_id", t.ID, "video_id", t.VideoID,
			"part", t.Part, "chapter", t.Chapter, "описание", "Блок записан в Obsidian")
	}

	if err := storage.MarkObsidianTargetWritten(t.ID, p.targetPath(t.Part, t.Chapter)); err != nil {
		return fmt.Errorf("mark target written: %w", err)
	}
	if err := storage.MarkVideoInObsidian(t.VideoID); err != nil {
		return fmt.Errorf("mark in obsidian: %w", err)
	}

	p.Log.Info("publisher.target_done",
		"target_id", t.ID, "video_id", t.VideoID,
		"duration_ms", time.Since(start).Milliseconds(),
		"описание", "Конспект успешно записан")
	return nil
}