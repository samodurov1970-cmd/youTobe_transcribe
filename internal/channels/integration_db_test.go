package channels_test

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"youTobe_transcribe/internal/config"
	"youTobe_transcribe/internal/logger"
	"youTobe_transcribe/internal/storage"
	"youTobe_transcribe/internal/trash_manager"
	validatorpk "youTobe_transcribe/internal/validator"
)

// setupDB подключается к тестовой базе, очищает таблицы и возвращает *sql.DB.
func setupDB(t *testing.T) *sql.DB {
	t.Helper()

	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	// КРИТИЧЕСКАЯ ЗАЩИТА: тесты разрешены ТОЛЬКО если в имени БД есть "_test".
	// Это предотвращает случайное удаление данных из рабочей базы (youtube_tracker).
	if !strings.Contains(cfg.DatabaseURL, "_test") {
		t.Skip("skipping integration test: DATABASE_URL does not contain '_test'. Refusing to wipe production DB.")
	}

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping db: %v", err)
	}

	// Очищаем таблицы перед тестом (безопасно, т.к. это тестовая БД).
	_, _ = db.Exec("DELETE FROM trash")
	_, _ = db.Exec("DELETE FROM obsidian_targets")
	_, _ = db.Exec("DELETE FROM video_files")
	_, _ = db.Exec("DELETE FROM videos")
	_, _ = db.Exec("DELETE FROM channels")

	return db
}

func TestValidatorGarbage(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	ctx := context.Background()
	videoID := fmt.Sprintf("vid_garbage_%d", time.Now().Unix())

	// 1. Создаём видео
	_, err := db.Exec(`
		INSERT INTO videos (video_id, title, video_url, channel_id, channel_name, channel_topic, is_processed)
		VALUES ($1, 'Тестовое видео', 'https://youtube.com/watch?v=test', 'UC123', 'Test Channel', 'обучение_торговле_на_рынках', true)
	`, videoID)
	if err != nil {
		t.Fatalf("insert video: %v", err)
	}

	// 2. Создаём video_files с мусорным текстом
	_, err = db.Exec(`
		INSERT INTO video_files (video_id, channel_name, video_url, channel_topic, file_name, file_content, validation_status)
		VALUES ($1, 'Test Channel', 'https://youtube.com/watch?v=test', 'обучение_торговле_на_рынках', 'test.md', 'мусор', 'pending')
	`, videoID)
	if err != nil {
		t.Fatalf("insert video_file: %v", err)
	}

	// 3. Инициализируем глобальный логгер и storage.DB
	if logger.Log == nil {
		logger.Init()
	}
	if storage.DB == nil {
		_ = storage.Init(config.Load().DatabaseURL)
	}

	// 4. Запускаем валидатор
	svc := &validatorpk.Validator{
		Log:       logger.Log.With("component", "validator"),
		BatchSize: 10,
	}

	if err := svc.Run(ctx); err != nil {
		t.Fatalf("Validator.Run: %v", err)
	}

	// 5. Проверяем, что запись попала в trash
	var trashCount int
	if err := db.QueryRow("SELECT count(*) FROM trash WHERE video_id = $1", videoID).Scan(&trashCount); err != nil {
		t.Fatalf("count trash: %v", err)
	}
	if trashCount != 1 {
		t.Errorf("expected 1 trash record, got %d", trashCount)
	}

	// 6. Проверяем, что video_files удалён
	var videoFilesCount int
	if err := db.QueryRow("SELECT count(*) FROM video_files WHERE video_id = $1", videoID).Scan(&videoFilesCount); err != nil {
		t.Fatalf("count video_files: %v", err)
	}
	if videoFilesCount != 0 {
		t.Errorf("expected 0 video_files, got %d", videoFilesCount)
	}
}

func TestTrashManagerDelete(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	ctx := context.Background()
	videoID := fmt.Sprintf("vid_trash_del_%d", time.Now().Unix())

	// 1. Создаём видео
	_, _ = db.Exec(`
		INSERT INTO videos (video_id, title, video_url, channel_id, channel_name, channel_topic, is_processed)
		VALUES ($1, 'Тест', 'url', 'UC123', 'Test', 'обучение_торговле_на_рынках', true)
	`, videoID)

	// 2. Создаём запись в trash с action = 'delete'
	_, err := db.Exec(`
		INSERT INTO trash (video_id, channel_name, video_url, reason, action)
		VALUES ($1, 'Test', 'url', 'test_reason', 'delete')
	`, videoID)
	if err != nil {
		t.Fatalf("insert trash: %v", err)
	}

	// 3. Запускаем trash_manager
	if logger.Log == nil {
		logger.Init()
	}
	if storage.DB == nil {
		_ = storage.Init(config.Load().DatabaseURL)
	}

	mgr := trash_manager.NewManager(logger.Log)
	if err := mgr.Run(ctx); err != nil {
		t.Fatalf("TrashManager.Run: %v", err)
	}

	// 4. Проверяем, что trash очищен
	var trashCount int
	_ = db.QueryRow("SELECT count(*) FROM trash WHERE video_id = $1", videoID).Scan(&trashCount)
	if trashCount != 0 {
		t.Errorf("expected 0 trash records, got %d", trashCount)
	}

	// 5. Проверяем, что видео помечено как is_processed = true
	var isProcessed bool
	_ = db.QueryRow("SELECT is_processed FROM videos WHERE video_id = $1", videoID).Scan(&isProcessed)
	if !isProcessed {
		t.Error("expected is_processed=true")
	}
}

func TestTrashManagerReprocess(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	ctx := context.Background()
	videoID := fmt.Sprintf("vid_trash_re_%d", time.Now().Unix())

	// 1. Создаём видео
	_, _ = db.Exec(`
		INSERT INTO videos (video_id, title, video_url, channel_id, channel_name, channel_topic, is_processed)
		VALUES ($1, 'Тест', 'url', 'UC123', 'Test', 'обучение_торговле_на_рынках', true)
	`, videoID)

	// 2. Создаём запись в trash с action = 'reprocess'
	_, err := db.Exec(`
		INSERT INTO trash (video_id, channel_name, video_url, reason, action)
		VALUES ($1, 'Test', 'url', 'test_reason', 'reprocess')
	`, videoID)
	if err != nil {
		t.Fatalf("insert trash: %v", err)
	}

	// 3. Запускаем trash_manager
	if logger.Log == nil {
		logger.Init()
	}
	if storage.DB == nil {
		_ = storage.Init(config.Load().DatabaseURL)
	}

	mgr := trash_manager.NewManager(logger.Log)
	if err := mgr.Run(ctx); err != nil {
		t.Fatalf("TrashManager.Run: %v", err)
	}

	// 4. Проверяем, что trash очищен
	var trashCount int
	_ = db.QueryRow("SELECT count(*) FROM trash WHERE video_id = $1", videoID).Scan(&trashCount)
	if trashCount != 0 {
		t.Errorf("expected 0 trash records, got %d", trashCount)
	}

	// 5. Проверяем, что видео сброшено в is_processed = false
	var isProcessed bool
	_ = db.QueryRow("SELECT is_processed FROM videos WHERE video_id = $1", videoID).Scan(&isProcessed)
	if isProcessed {
		t.Error("expected is_processed=false")
	}
}