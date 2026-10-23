package trash_manager

import (
	"context"
	"fmt"
	"log/slog"

	"youTobe_transcribe/internal/storage"
)

// Manager обрабатывает записи из trash с заданным action.
type Manager struct {
	Log       *slog.Logger
	BatchSize int
}

// NewManager создаёт новый менеджер trash.
func NewManager(log *slog.Logger) *Manager {
	return &Manager{
		Log:       log.With("component", "trash_manager"),
		BatchSize: 10,
	}
}

// Run выполняет один цикл: берёт записи с action и выполняет их.
func (m *Manager) Run(ctx context.Context) error {
	m.Log.Info("trash_manager.collect_started", "описание", "Начат цикл обработки trash")

	items, err := storage.GetTrashWithAction(m.BatchSize)
	if err != nil {
		m.Log.Error("trash_manager.fetch_failed", "error", err.Error(), "описание", "ОШИБКА: Не удалось получить записи из trash")
		return err
	}

	if len(items) == 0 {
		m.Log.Info("trash_manager.no_pending", "описание", "Нет записей с действием")
		return nil
	}

	m.Log.Info("trash_manager.batch_fetched", "count", len(items), "описание", "Записи получены")

	processed := 0
	for _, item := range items {
		if err := m.processItem(ctx, item); err != nil {
			m.Log.Error("trash_manager.item_failed", "video_id", item.VideoID, "error", err.Error(), "описание", "ОШИБКА: Не удалось обработать запись")
			continue
		}
		processed++
	}

	m.Log.Info("trash_manager.collect_finished", "processed", processed, "описание", "Цикл обработки trash завершён")
	return nil
}

// processItem выполняет действие для одной записи.
func (m *Manager) processItem(ctx context.Context, item storage.TrashItem) error {
	m.Log.Info("trash_manager.item_started", "video_id", item.VideoID, "action", item.Action, "описание", "Начата обработка записи")

	switch item.Action {
	case "delete":
		// Удалить из trash и помечать как обработанное (не перерабатывать)
		if err := storage.DeleteTrash(item.ID); err != nil {
			return fmt.Errorf("delete trash: %w", err)
		}
		if err := storage.MarkVideoProcessed(item.VideoID); err != nil {
			return fmt.Errorf("mark processed: %w", err)
		}
		m.Log.Info("trash_manager.deleted", "video_id", item.VideoID, "описание", "Видео удалено из trash")

	case "reprocess":
		// Удалить из trash и сбросить is_processed = false
		if err := storage.DeleteTrash(item.ID); err != nil {
			return fmt.Errorf("delete trash: %w", err)
		}
		if err := storage.ResetVideoProcessed(item.VideoID); err != nil {
			return fmt.Errorf("reset processed: %w", err)
		}
		m.Log.Info("trash_manager.reprocessed", "video_id", item.VideoID, "описание", "Видео отправлено на повторную обработку")

	default:
		return fmt.Errorf("unknown action: %s (допустимо: delete, reprocess)", item.Action)
	}

	return nil
}