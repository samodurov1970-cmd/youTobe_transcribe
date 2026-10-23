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
// Каналы с темой 'обучение_торговле_на_рынках' идут первыми, затем 'мой_морской_аквариум'.
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