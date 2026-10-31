// Package classifier делает LLM-классификацию и суммаризацию,
// результат кладёт в obsidian_targets(status='ready').
package classifier

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"youTobe_transcribe/internal/storage"
)

// Classifier делает классификацию + суммаризацию.
type Classifier struct {
	Log       *slog.Logger
	BatchSize int
	ChunkSize int
	llm       *llmClient
}

// NewClassifier создаёт классификатор.
func NewClassifier(log *slog.Logger, apiKey, model string, batchSize, chunkSize int) *Classifier {
	if batchSize <= 0 {
		batchSize = 1
	}
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}
	return &Classifier{
		Log:       log.With("component", "classifier"),
		BatchSize: batchSize,
		ChunkSize: chunkSize,
		llm:       newLLMClient(apiKey, model),
	}
}

// Run выполняет один цикл классификации.
func (c *Classifier) Run(ctx context.Context) error {
	c.Log.Info("classifier.collect_started", "описание", "Начат цикл классификации")

	items, err := storage.GetValidatedForClassify(c.BatchSize)
	if err != nil {
		c.Log.Error("classifier.fetch_failed", "error", err.Error(),
			"описание", "ОШИБКА: Не удалось получить валидированные видео")
		return err
	}

	if len(items) == 0 {
		c.Log.Info("classifier.no_pending", "описание", "Нет валидированных видео для классификации")
		return nil
	}

	c.Log.Info("classifier.batch_fetched", "count", len(items),
		"описание", "Видео получены для классификации")

	processed := 0
	for _, vf := range items {
		if err := c.classifyVideo(ctx, vf); err != nil {
			c.Log.Error("classifier.video_failed", "video_id", vf.VideoID, "error", err.Error(),
				"описание", "ОШИБКА: Не удалось классифицировать видео")
			_ = storage.MarkVideoClassifyFailed(vf.VideoID)
			continue
		}
		processed++
	}

	c.Log.Info("classifier.collect_finished", "processed", processed,
		"описание", "Цикл классификации завершён")
	return nil
}

func (c *Classifier) classifyVideo(ctx context.Context, vf storage.VideoFile) error {
	start := time.Now()
	c.Log.Info("classifier.video_started", "video_id", vf.VideoID, "channel_name", vf.ChannelName,
		"описание", "Начата классификация видео")

	// === ШАГ 1: Классификация ===
	res, err := c.classify(ctx, vf.FileContent)
	if err != nil {
		return fmt.Errorf("classify: %w", err)
	}
	c.Log.Info("classifier.classified",
		"video_id", vf.VideoID, "part", res.Part, "chapter", res.Chapter,
		"описание", "Видео классифицировано")

	if res.Part == FallbackPart {
		if err := storage.InsertTrash(storage.Trash{
			VideoID:     vf.VideoID,
			ChannelName: vf.ChannelName,
			VideoURL:    vf.VideoURL,
			FileContent: vf.FileContent,
			Reason:      "unclassified: LLM не смогла отнести к разделу",
			Source:      "classifier",
		}); err != nil {
			return fmt.Errorf("insert trash: %w", err)
		}
		_ = storage.DeleteVideoFile(vf.VideoID)
		c.Log.Info("classifier.unclassified", "video_id", vf.VideoID,
			"описание", "Видео отправлено в trash (не классифицировано)")
		return nil
	}

	// === ШАГ 2: Сохраняем «куда» в obsidian_targets ===
	targetID, err := storage.InsertObsidianTarget(storage.ObsidianTarget{
		VideoID:     vf.VideoID,
		ChannelName: vf.ChannelName,
		VideoURL:    vf.VideoURL,
		VideoTitle:  vf.FileName,
		Part:        res.Part,
		Chapter:     res.Chapter,
	})
	if err != nil {
		return fmt.Errorf("insert obsidian_target: %w", err)
	}
	c.Log.Info("classifier.target_created", "video_id", vf.VideoID, "target_id", targetID,
		"part", res.Part, "chapter", res.Chapter,
		"описание", "Создана запись obsidian_targets (куда писать)")

	// === ШАГ 3: Суммаризация ===
	block, err := c.summarize(ctx, vf.ChannelName, vf.FileContent)
	if err != nil {
		_ = storage.MarkObsidianTargetError(targetID)
		return fmt.Errorf("summarize: %w", err)
	}
	if err := storage.UpdateObsidianTargetReady(targetID, block); err != nil {
		return fmt.Errorf("update target ready: %w", err)
	}
	c.Log.Info("classifier.summarized", "video_id", vf.VideoID, "target_id", targetID,
		"block_len", len(block), "описание", "Конспект сформирован, obsidian_targets(status='ready')")

	// === ШАГ 4: video_files(validation_status='classified') ===
	if err := storage.MarkVideoClassified(vf.VideoID); err != nil {
		return fmt.Errorf("mark classified: %w", err)
	}

	c.Log.Info("classifier.video_done",
		"video_id", vf.VideoID, "target_id", targetID,
		"duration_ms", time.Since(start).Milliseconds(),
		"описание", "Видео успешно классифицировано")
	return nil
}