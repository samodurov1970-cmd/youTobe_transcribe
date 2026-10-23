package storage

import (
	"time"
)

// Channel — запись о канале.
type Channel struct {
	ChannelID   string
	ChannelName string
	Topic       string
	IsNews      bool
}

// Video — запись о видео.
type Video struct {
	VideoID      string
	Title        string
	VideoURL     string
	PublishedAt  time.Time
	ChannelID    string
	ChannelName  string
	ChannelTopic string
	IsNews       bool
}

// VideoStats — статистика по видео.
type VideoStats struct {
	Total       int
	Today       int
	Processed   int
	Unprocessed int
}

// GetActiveChannels возвращает все активные (подписанные) каналы.
func GetActiveChannels() ([]Channel, error) {
	rows, err := DB.Query(`
		SELECT channel_id, channel_name, coalesce(topic, ''), coalesce(is_news, false)
		FROM channels
		WHERE coalesce(is_subscribed, true) = true
		  AND topic IN ('обучение_торговле_на_рынках', 'мой_морской_аквариум')
		ORDER BY 
			CASE 
				WHEN topic = 'обучение_торговле_на_рынках' THEN 1
				WHEN topic = 'мой_морской_аквариум' THEN 2
				ELSE 3
			END,
			channel_name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []Channel
	for rows.Next() {
		var ch Channel
		if err := rows.Scan(&ch.ChannelID, &ch.ChannelName, &ch.Topic, &ch.IsNews); err != nil {
			return nil, err
		}
		channels = append(channels, ch)
	}
	return channels, rows.Err()
}

// InsertVideo вставляет видео в БД (если его там нет).
func InsertVideo(v Video) error {
	_, err := DB.Exec(`
		INSERT INTO videos (video_id, title, video_url, published_at, channel_id, channel_name, channel_topic, is_news, processed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW() AT TIME ZONE 'Europe/Minsk')
		ON CONFLICT (video_id) DO NOTHING
	`, v.VideoID, v.Title, v.VideoURL, v.PublishedAt, v.ChannelID, v.ChannelName, v.ChannelTopic, v.IsNews)
	return err
}

// GetVideoStats возвращает статистику по видео за сегодня.
func GetVideoStats() (VideoStats, error) {
	var stats VideoStats
	err := DB.QueryRow(`
		SELECT 
			count(*) AS total,
			count(*) FILTER (WHERE created_at::date = CURRENT_DATE) AS today,
			count(*) FILTER (WHERE is_processed = true) AS processed,
			count(*) FILTER (WHERE is_processed = false OR is_processed IS NULL) AS unprocessed
		FROM videos
	`).Scan(&stats.Total, &stats.Today, &stats.Processed, &stats.Unprocessed)
	return stats, err
}

// GetPendingVideos возвращает видео, которые ещё не обработаны (is_processed = false).
func GetPendingVideos(limit int) ([]Video, error) {
	rows, err := DB.Query(`
		SELECT video_id, title, video_url, channel_id, channel_name, channel_topic, is_news, published_at
		FROM videos
		WHERE is_processed = false
		  AND channel_topic IN ('обучение_торговле_на_рынках', 'мой_морской_аквариум')
		ORDER BY published_at DESC NULLS LAST
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []Video
	for rows.Next() {
		var v Video
		if err := rows.Scan(&v.VideoID, &v.Title, &v.VideoURL, &v.ChannelID, &v.ChannelName, &v.ChannelTopic, &v.IsNews, &v.PublishedAt); err != nil {
			return nil, err
		}
		videos = append(videos, v)
	}
	return videos, rows.Err()
}

// MarkVideoProcessed ставит is_processed = true для видео.
func MarkVideoProcessed(videoID string) error {
	_, err := DB.Exec(`UPDATE videos SET is_processed = true WHERE video_id = $1`, videoID)
	return err
}

// InsertVideoFile вставляет запись в video_files.
func InsertVideoFile(vf VideoFile) error {
	_, err := DB.Exec(`
		INSERT INTO video_files (video_id, channel_name, video_url, channel_topic, file_name, file_content, status, last_error, attempts)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending', $7, 1)
		ON CONFLICT (video_id) DO UPDATE SET
			file_content = EXCLUDED.file_content,
			status = EXCLUDED.status,
			last_error = EXCLUDED.last_error,
			attempts = COALESCE(video_files.attempts, 0) + 1
	`, vf.VideoID, vf.ChannelName, vf.VideoURL, vf.ChannelTopic, vf.FileName, vf.FileContent, vf.LastError)
	return err
}

// MarkVideoFailed записывает ошибку в video_files и увеличивает attempts.
func MarkVideoFailed(videoID, fileName, lastError string) error {
	_, err := DB.Exec(`
		UPDATE video_files
		SET last_error = $1, attempts = COALESCE(attempts, 0) + 1,
		    status = CASE WHEN COALESCE(attempts, 0) + 1 >= 3 THEN 'error' ELSE 'pending' END
		WHERE video_id = $2
	`, lastError, videoID)
	return err
}

// ============================================================================
// ФУНКЦИИ ДЛЯ ВАЛИДАТОРА
// ============================================================================

// Trash — запись в таблице trash.
type Trash struct {
	VideoID     string
	ChannelName string
	VideoURL    string
	FileContent string
	LastError   string
	Reason      string
}

// GetPendingValidation возвращает видео из video_files со статусом 'pending'.
func GetPendingValidation(limit int) ([]VideoFile, error) {
	rows, err := DB.Query(`
		SELECT video_id, 
		       coalesce(channel_name, ''), 
		       coalesce(video_url, ''), 
		       coalesce(file_name, ''),
		       coalesce(file_content, ''), 
		       coalesce(status, ''), 
		       coalesce(last_error, ''), 
		       coalesce(channel_topic, '')
		FROM video_files
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []VideoFile
	for rows.Next() {
		var vf VideoFile
		if err := rows.Scan(&vf.VideoID, &vf.ChannelName, &vf.VideoURL, &vf.FileName, &vf.FileContent, &vf.Status, &vf.LastError, &vf.ChannelTopic); err != nil {
			return nil, err
		}
		videos = append(videos, vf)
	}
	return videos, rows.Err()
}

// MarkVideoValidated ставит статус 'validated' для видео.
func MarkVideoValidated(videoID string) error {
	_, err := DB.Exec(`UPDATE video_files SET status = 'validated' WHERE video_id = $1`, videoID)
	return err
}

// InsertTrash вставляет запись в trash.
func InsertTrash(t Trash) error {
	_, err := DB.Exec(`
		INSERT INTO trash (video_id, channel_name, video_url, file_content, last_error, reason)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, t.VideoID, t.ChannelName, t.VideoURL, t.FileContent, t.LastError, t.Reason)
	return err
}

// DeleteVideoFile удаляет запись из video_files.
func DeleteVideoFile(videoID string) error {
	_, err := DB.Exec(`DELETE FROM video_files WHERE video_id = $1`, videoID)
	return err
}

// ResetVideoProcessed сбрасывает is_processed = false в videos.
func ResetVideoProcessed(videoID string) error {
	_, err := DB.Exec(`UPDATE videos SET is_processed = false WHERE video_id = $1`, videoID)
	return err
}

// ============================================================================
// ФУНКЦИИ ДЛЯ TRASH_MANAGER
// ============================================================================

// TrashItem — запись в таблице trash с action (для trash_manager).
type TrashItem struct {
	ID          int
	VideoID     string
	ChannelName string
	VideoURL    string
	FileContent string
	LastError   string
	Reason      string
	Action      string
}

// GetTrashWithAction возвращает записи из trash, где action задан.
func GetTrashWithAction(limit int) ([]TrashItem, error) {
	rows, err := DB.Query(`
		SELECT id, video_id, coalesce(channel_name, ''), coalesce(video_url, ''),
		       coalesce(file_content, ''), coalesce(last_error, ''), coalesce(reason, ''), coalesce(action, '')
		FROM trash
		WHERE action IS NOT NULL AND action <> ''
		ORDER BY created_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []TrashItem
	for rows.Next() {
		var t TrashItem
		if err := rows.Scan(&t.ID, &t.VideoID, &t.ChannelName, &t.VideoURL, &t.FileContent, &t.LastError, &t.Reason, &t.Action); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

// DeleteTrash удаляет запись из trash по ID.
func DeleteTrash(id int) error {
	_, err := DB.Exec(`DELETE FROM trash WHERE id = $1`, id)
	return err
}