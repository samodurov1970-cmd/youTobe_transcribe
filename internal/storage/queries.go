package storage

import (
	"database/sql"

	"youTobe_transcribe/internal/logger"
)

// MarkTranscribed сохраняет транскрипт и помечает видео обработанным.
func MarkTranscribed(videoID int, transcript string) error {
	log().Info("db.mark_transcribed.start", "record_id", videoID, "transcript_len", len(transcript))
	res, err := DB.Exec(`
		UPDATE video_files SET status = 'transcribed', file_content = $1
		WHERE id = $2 AND status = 'processing'
	`, transcript, videoID)
	if err != nil {
		log().Error("db.mark_transcribed.failed", logger.Err(err), "record_id", videoID)
		return err
	}
	rows, _ := res.RowsAffected()
	log().Info("db.mark_transcribed.ok", "record_id", videoID, "rows_affected", rows)
	return nil
}

// MarkError сохраняет ошибку обработки для видео.
func MarkError(videoID int, errorMsg string) error {
	log().Info("db.mark_error.start", "record_id", videoID, "error_len", len(errorMsg))
	res, err := DB.Exec(`
		UPDATE video_files SET status = 'error', last_error = $1
		WHERE id = $2 AND status = 'processing'
	`, errorMsg, videoID)
	if err != nil {
		log().Error("db.mark_error.failed", logger.Err(err), "record_id", videoID)
		return err
	}
	rows, _ := res.RowsAffected()
	log().Info("db.mark_error.ok", "record_id", videoID, "rows_affected", rows)
	return nil
}

// FetchNextProcessing получает первое видео в статусе processing.
func FetchNextProcessing() (*VideoFile, error) {
	var v VideoFile
	err := DB.QueryRow(`
		SELECT id, video_id, channel_name, video_url, file_name, file_content
		FROM video_files
		WHERE status = 'processing'
		ORDER BY id
		LIMIT 1
	`).Scan(&v.ID, &v.VideoID, &v.ChannelName, &v.VideoURL, &v.FileName, &v.FileContent)
	if err == sql.ErrNoRows {
		log().Info("db.fetch_processing.empty")
		return nil, err
	}
	if err != nil {
		log().Error("db.fetch_processing.failed", logger.Err(err))
		return nil, err
	}
	log().Info("db.fetch_processing.ok",
		"record_id", v.ID,
		"video_id", v.VideoID,
		"video_url", v.VideoURL,
	)
	return &v, nil
}
