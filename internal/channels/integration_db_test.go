//go:build integration_db

// Этот файл содержит интеграционные тесты с реальной БД (PostgreSQL).
// Запускать только на локальной машине, если переменная окружения
// TEST_DATABASE_URL указывает на тестовую базу (не боевую!).
//
// Запуск:
//
//	$env:TEST_DATABASE_URL = "postgres://peter:peter@localhost:5432/youtube_tracker_test?sslmode=disable"
//	go test -tags=integration_db ./internal/channels/ -count=1 -v
package channels

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"youTobe_transcribe/internal/logger"
	"youTobe_transcribe/internal/storage"
	trashmanager "youTobe_transcribe/internal/trash_manager"
	validatorpkg "youTobe_transcribe/internal/validator"

	_ "github.com/lib/pq"
)

// setupTestDB подключается к тестовой БД и очищает таблицы.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if err := db.Ping(); err != nil {
		t.Fatalf("ping test db: %v", err)
	}

	_, err = db.Exec(`
		TRUNCATE channels, videos, videos_new, video_files, trash, channels_unsubscribed RESTART IDENTITY CASCADE;
	`)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}

	logger.Init()
	oldDB := storage.DB
	storage.DB = db
	t.Cleanup(func() { storage.DB = oldDB })

	return db
}

// TestUnsubscribeCleanup проверяет полную цепочку:
//   - подписка (канал в channels, is_subscribed = true);
//   - данные в videos, video_files, trash;
//   - отписка (канал в channels, is_subscribed = false);
//   - очистка videos, video_files, trash.
func TestUnsubscribeCleanup(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	const (
		channelID   = "UC_TEST_UNSUB_CLEANUP_1"
		channelName = "Test Channel Cleanup"
	)

	_, err := db.Exec(`
		INSERT INTO channels (channel_id, channel_name, channel_url, description, topic, is_subscribed, last_seen_at)
		VALUES ($1, $2, 'https://www.youtube.com/channel/$1', 'test', 'обучение_торговле_на_рынках', true, NOW())
	`, channelID, channelName)
	if err != nil {
		t.Fatalf("insert channel: %v", err)
	}

	for i := 0; i < 3; i++ {
		_, err = db.Exec(`
			INSERT INTO videos (video_id, title, video_url, channel_id, channel_name, channel_topic, is_processed)
			VALUES ($1, 'Test Video', 'https://youtu.be/test', $2, $3, 'обучение_торговле_на_рынках', false)
		`, "video_test_"+channelID+"_"+string(rune('0'+i)), channelID, channelName)
		if err != nil {
			t.Fatalf("insert video: %v", err)
		}
	}

	_, err = db.Exec(`
		INSERT INTO videos (video_id, title, video_url, channel_id, channel_name, channel_topic, is_processed)
		VALUES ('video_test_file', 'Test Video File', 'https://youtu.be/test', $1, $2, 'обучение_торговле_на_рынках', false)
	`, channelID, channelName)
	if err != nil {
		t.Fatalf("insert video for file: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO video_files (video_id, channel_name, video_url, file_name, file_content, status)
		VALUES ('video_test_file', $1, 'https://youtu.be/test', 'test.md', 'test content', 'pending')
	`, channelName)
	if err != nil {
		t.Fatalf("insert video_file: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO videos (video_id, title, video_url, channel_id, channel_name, channel_topic, is_processed)
		VALUES ('video_test_trash', 'Test Trash Video', 'https://youtu.be/test', $1, $2, 'обучение_торговле_на_рынках', false)
	`, channelID, channelName)
	if err != nil {
		t.Fatalf("insert video for trash: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO trash (video_id, channel_name, video_url, file_content, reason)
		VALUES ('video_test_trash', $1, 'https://youtu.be/test', 'trash content', 'тест')
	`, channelName)
	if err != nil {
		t.Fatalf("insert trash: %v", err)
	}

	gone := []string{channelID}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := purgeUnsubscribedChannels(ctx, tx, gone); err != nil {
		tx.Rollback()
		t.Fatalf("purgeUnsubscribedChannels: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var videosCount, videoFilesCount, trashCount int
	if err := db.QueryRow(`SELECT count(*) FROM videos WHERE channel_id = $1`, channelID).Scan(&videosCount); err != nil {
		t.Fatalf("count videos: %v", err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM video_files WHERE channel_name = $1`, channelName).Scan(&videoFilesCount); err != nil {
		t.Fatalf("count video_files: %v", err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM trash WHERE channel_name = $1`, channelName).Scan(&trashCount); err != nil {
		t.Fatalf("count trash: %v", err)
	}

	if videosCount != 0 {
		t.Errorf("videos: expected 0, got %d", videosCount)
	}
	if videoFilesCount != 0 {
		t.Errorf("video_files: expected 0, got %d", videoFilesCount)
	}
	if trashCount != 0 {
		t.Errorf("trash: expected 0, got %d", trashCount)
	}
}

// TestUnsubscribeProtection проверяет защиту от массовой отписки:
// если activeCount > 2*len(subs), отписки не выполняются.
func TestUnsubscribeProtection(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// Создаём 10 каналов (все подписаны).
	for i := 0; i < 10; i++ {
		_, err := db.Exec(`
			INSERT INTO channels (channel_id, channel_name, channel_url, description, topic, is_subscribed, last_seen_at)
			VALUES ($1, $2, 'https://www.youtube.com/channel/$1', 'test', 'обучение_торговле_на_рынках', true, NOW())
		`, "UC_PROTECT_"+string(rune('A'+i)), "Protected Channel "+string(rune('A'+i)))
		if err != nil {
			t.Fatalf("insert channel: %v", err)
		}
	}

	// Передаём subs только с 1 каналом (activeCount = 10 > 2*1 = 2).
	subs := []subscription{
		{ChannelID: "UC_PROTECT_A", Title: "Protected Channel A"},
	}

	svc := &Service{
		db:  db,
		cfg: &Config{DB: db},
		log: logger.Log.With("component", "channels"),
	}

	_, gone, err := svc.markUnsubscribed(ctx, subs)
	if err != nil {
		t.Fatalf("markUnsubscribed: %v", err)
	}

	if len(gone) != 0 {
		t.Errorf("expected 0 unsubscribed (protection), got %d", len(gone))
	}

	// Проверяем, что все каналы остались подписаны.
	var subscribedCount int
	if err := db.QueryRow(`SELECT count(*) FROM channels WHERE is_subscribed = true`).Scan(&subscribedCount); err != nil {
		t.Fatalf("count channels: %v", err)
	}
	if subscribedCount != 10 {
		t.Errorf("expected 10 subscribed channels, got %d", subscribedCount)
	}
}

// TestResubscribe проверяет, что при повторной подписке канал
// возвращается в is_subscribed = true.
func TestResubscribe(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	const channelID = "UC_TEST_RESUBSCRIBE_1"

	// Канал отписан.
	_, err := db.Exec(`
		INSERT INTO channels (channel_id, channel_name, channel_url, description, topic, is_subscribed, unsubscribed_at, last_seen_at)
		VALUES ($1, 'Test Resubscribe', 'https://www.youtube.com/channel/$1', 'test', 'обучение_торговле_на_рынках', false, NOW(), NOW())
	`, channelID)
	if err != nil {
		t.Fatalf("insert channel: %v", err)
	}

	// Вызываем resubscribeChannels (эмуляция saveSubscriptions).
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	_, err = resubscribeChannels(ctx, tx, []string{channelID})
	if err != nil {
		tx.Rollback()
		t.Fatalf("resubscribeChannels: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Проверяем, что is_subscribed = true.
	var isSubscribed bool
	if err := db.QueryRow(`SELECT is_subscribed FROM channels WHERE channel_id = $1`, channelID).Scan(&isSubscribed); err != nil {
		t.Fatalf("select: %v", err)
	}
	if !isSubscribed {
		t.Error("expected is_subscribed = true after resubscribe")
	}
}

// TestValidatorGarbage проверяет, что мусорный текст попадает в trash.
func TestValidatorGarbage(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	const (
		channelID   = "UC_TEST_VALIDATOR_GARBAGE"
		channelName = "Test Validator Garbage"
		videoID     = "video_test_garbage"
	)

	// Вставляем видео и video_files с мусорным текстом.
	_, err := db.Exec(`
		INSERT INTO videos (video_id, title, video_url, channel_id, channel_name, channel_topic, is_processed)
		VALUES ($1, 'Test Video', 'https://youtu.be/test', $2, $3, 'обучение_торговле_на_рынках', true)
	`, videoID, channelID, channelName)
	if err != nil {
		t.Fatalf("insert video: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO video_files (video_id, channel_name, video_url, file_name, file_content, status)
		VALUES ($1, $2, 'https://youtu.be/test', 'test.md', 'и и и и я я я', 'pending')
	`, videoID, channelName)
	if err != nil {
		t.Fatalf("insert video_file: %v", err)
	}

	// Запускаем валидатор.
	svc := &validatorpkg.Validator{
		Log:       logger.Log.With("component", "validator"),
		BatchSize: 10,
	}

	if err := svc.Run(ctx); err != nil {
		t.Fatalf("Validator.Run: %v", err)
	}

	// Проверяем, что запись попала в trash.
	var trashCount int
	if err := db.QueryRow(`SELECT count(*) FROM trash WHERE video_id = $1`, videoID).Scan(&trashCount); err != nil {
		t.Fatalf("count trash: %v", err)
	}
	if trashCount != 1 {
		t.Errorf("expected 1 trash record, got %d", trashCount)
	}

	// Проверяем, что video_files удалён.
	var videoFilesCount int
	if err := db.QueryRow(`SELECT count(*) FROM video_files WHERE video_id = $1`, videoID).Scan(&videoFilesCount); err != nil {
		t.Fatalf("count video_files: %v", err)
	}
	if videoFilesCount != 0 {
		t.Errorf("expected 0 video_files, got %d", videoFilesCount)
	}
}

// TestTrashManagerDelete проверяет, что trash_manager с action=delete
// удаляет запись из trash и помечает видео как обработанное.
func TestTrashManagerDelete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	const videoID = "video_test_trash_delete"

	// Вставляем видео (is_processed = false) и запись в trash с action=delete.
	_, err := db.Exec(`
		INSERT INTO videos (video_id, title, video_url, channel_id, channel_name, channel_topic, is_processed)
		VALUES ($1, 'Test Video', 'https://youtu.be/test', 'UC_TEST', 'Test', 'обучение_торговле_на_рынках', false)
	`, videoID)
	if err != nil {
		t.Fatalf("insert video: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO trash (video_id, channel_name, video_url, file_content, reason, action)
		VALUES ($1, 'Test', 'https://youtu.be/test', 'garbage', 'тест', 'delete')
	`, videoID)
	if err != nil {
		t.Fatalf("insert trash: %v", err)
	}

	// Запускаем trash_manager.
	svc := &trashmanager.Manager{
		Log:       logger.Log.With("component", "trash_manager"),
		BatchSize: 10,
	}

	if err := svc.Run(ctx); err != nil {
		t.Fatalf("Manager.Run: %v", err)
	}

	// Проверяем, что trash очищен.
	var trashCount int
	if err := db.QueryRow(`SELECT count(*) FROM trash WHERE video_id = $1`, videoID).Scan(&trashCount); err != nil {
		t.Fatalf("count trash: %v", err)
	}
	if trashCount != 0 {
		t.Errorf("expected 0 trash records, got %d", trashCount)
	}

	// Проверяем, что видео помечено как обработанное.
	var isProcessed bool
	if err := db.QueryRow(`SELECT is_processed FROM videos WHERE video_id = $1`, videoID).Scan(&isProcessed); err != nil {
		t.Fatalf("select video: %v", err)
	}
	if !isProcessed {
		t.Error("expected is_processed = true after trash_manager delete")
	}
}

// TestTrashManagerReprocess проверяет, что trash_manager с action=reprocess
// удаляет запись из trash и сбрасывает is_processed = false.
func TestTrashManagerReprocess(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	const videoID = "video_test_trash_reprocess"

	// Вставляем видео (is_processed = true) и запись в trash с action=reprocess.
	_, err := db.Exec(`
		INSERT INTO videos (video_id, title, video_url, channel_id, channel_name, channel_topic, is_processed)
		VALUES ($1, 'Test Video', 'https://youtu.be/test', 'UC_TEST', 'Test', 'обучение_торговле_на_рынках', true)
	`, videoID)
	if err != nil {
		t.Fatalf("insert video: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO trash (video_id, channel_name, video_url, file_content, reason, action)
		VALUES ($1, 'Test', 'https://youtu.be/test', 'garbage', 'тест', 'reprocess')
	`, videoID)
	if err != nil {
		t.Fatalf("insert trash: %v", err)
	}

	// Запускаем trash_manager.
	svc := &trashmanager.Manager{
		Log:       logger.Log.With("component", "trash_manager"),
		BatchSize: 10,
	}

	if err := svc.Run(ctx); err != nil {
		t.Fatalf("Manager.Run: %v", err)
	}

	// Проверяем, что trash очищен.
	var trashCount int
	if err := db.QueryRow(`SELECT count(*) FROM trash WHERE video_id = $1`, videoID).Scan(&trashCount); err != nil {
		t.Fatalf("count trash: %v", err)
	}
	if trashCount != 0 {
		t.Errorf("expected 0 trash records, got %d", trashCount)
	}

	// Проверяем, что is_processed = false (видео снова в очереди).
	var isProcessed bool
	if err := db.QueryRow(`SELECT is_processed FROM videos WHERE video_id = $1`, videoID).Scan(&isProcessed); err != nil {
		t.Fatalf("select video: %v", err)
	}
	if isProcessed {
		t.Error("expected is_processed = false after trash_manager reprocess")
	}
}