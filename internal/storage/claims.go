package storage

import (
	"fmt"
	"time"

	"youTobe_transcribe/internal/logger"

	"github.com/lib/pq"
)

// ClaimVideosForTranscription перемещает необработанные видео в статус processing.
func ClaimVideosForTranscription(channelTopics []string, limit int) (int, error) {
	start := time.Now()
	log().Info("claim.start",
		"channel_topics", channelTopics,
		"limit", limit,
	)
	tx, err := DB.Begin()
	if err != nil {
		log().Error("claim.begin_failed", logger.Err(err))
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO video_files (video_id, channel_name, video_url, file_name, file_content, status, is_processed, channel_topic)
		SELECT v.video_id, v.channel_name, v.video_url, v.title, '', 'processing', false, v.channel_topic
		FROM videos v
		WHERE v.channel_topic = ANY($1)
		  AND v.is_processed = false
		ORDER BY v.published_at DESC
		LIMIT $2
		RETURNING id, video_id, channel_name, video_url, file_name, file_content
	`

	rows, err := tx.Query(query, pq.Array(channelTopics), limit)
	if err != nil {
		log().Error("claim.query_failed", logger.Err(err), "channel_topics", channelTopics)
		return 0, fmt.Errorf("query claim: %w", err)
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		var v VideoFile
		if err := rows.Scan(&v.ID, &v.VideoID, &v.ChannelName, &v.VideoURL, &v.FileName, &v.FileContent); err != nil {
			log().Error("claim.scan_failed", logger.Err(err))
			return 0, fmt.Errorf("scan claim row: %w", err)
		}
		log().Info("claim.claimed",
			"record_id", v.ID,
			"video_id", v.VideoID,
			"channel_name", v.ChannelName,
			"video_url", v.VideoURL,
		)
		count++
	}
	if err := rows.Err(); err != nil {
		log().Error("claim.rows_err", logger.Err(err))
		return 0, fmt.Errorf("rows err: %w", err)
	}

	if err := tx.Commit(); err != nil {
		log().Error("claim.commit_failed", logger.Err(err))
		return 0, fmt.Errorf("commit tx: %w", err)
	}

	log().Info("claim.ok",
		"count", count,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return count, nil
}
