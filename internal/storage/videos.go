package storage

import (
	"time"
)

// Channel — запись о канале.
type Channel struct {
	ChannelID   string
	ChannelName string
	Topic       string
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
}

// VideoStats — статистика по видео.
type VideoStats struct {
	Total       int
	Today       int
	Processed   int
	Unprocessed int
}

// GetActiveChannels возвращает ТОЛЬКО торговые каналы.
func GetActiveChannels() ([]Channel, error) {
	rows, err := DB.Query(`
		SELECT channel_id, channel_name, coalesce(topic, '')
		FROM channels
		WHERE coalesce(is_subscribed, true) = true
		  AND topic = 'обучение_торговле_на_рынках'
		ORDER BY channel_name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []Channel
	for rows.Next() {
		var ch Channel
		if err := rows.Scan(&ch.ChannelID, &ch.ChannelName, &ch.Topic); err != nil {
			return nil, err
		}
		channels = append(channels, ch)
	}
	return channels, rows.Err()
}

// InsertVideo вставляет видео в БД.
func InsertVideo(v Video) error {
	_, err := DB.Exec(`
		INSERT INTO videos (video_id, title, video_url, published_at, channel_id, channel_name, channel_topic, processed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW() AT TIME ZONE 'Europe/Minsk')
		ON CONFLICT (video_id) DO NOTHING
	`, v.VideoID, v.Title, v.VideoURL, v.PublishedAt, v.ChannelID, v.ChannelName, v.ChannelTopic)
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

// GetPendingVideos возвращает видео, которые ещё не обработаны.
func GetPendingVideos(limit int) ([]Video, error) {
	rows, err := DB.Query(`
		SELECT video_id, title, video_url, channel_id, channel_name, channel_topic, published_at
		FROM videos
		WHERE is_processed = false
		  AND channel_topic = 'обучение_торговле_на_рынках'
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
		if err := rows.Scan(&v.VideoID, &v.Title, &v.VideoURL, &v.ChannelID, &v.ChannelName, &v.ChannelTopic, &v.PublishedAt); err != nil {
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
		INSERT INTO video_files (video_id, channel_name, video_url, channel_topic, file_name, file_content, validation_status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending', now())
		ON CONFLICT (video_id) DO UPDATE SET
			file_content = EXCLUDED.file_content,
			validation_status = 'pending',
			validation_reason = NULL,
			validated_at = NULL,
			is_in_obsidian = false
	`, vf.VideoID, vf.ChannelName, vf.VideoURL, vf.ChannelTopic, vf.FileName, vf.FileContent)
	return err
}

// ============================================================================
// ВАЛИДАТОР
// ============================================================================

// Trash — запись в таблице trash.
type Trash struct {
	VideoID     string
	ChannelName string
	VideoURL    string
	FileContent string
	Reason      string
	Part        string
	Chapter     string
	Source      string
}

// GetPendingValidation возвращает видео из video_files со статусом 'pending'.
func GetPendingValidation(limit int) ([]VideoFile, error) {
	rows, err := DB.Query(`
		SELECT video_id, 
		       coalesce(channel_name, ''), 
		       coalesce(video_url, ''), 
		       coalesce(file_name, ''),
		       coalesce(file_content, ''), 
		       coalesce(channel_topic, '')
		FROM video_files
		WHERE validation_status = 'pending'
		  AND channel_topic = 'обучение_торговле_на_рынках'
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
		if err := rows.Scan(&vf.VideoID, &vf.ChannelName, &vf.VideoURL, &vf.FileName, &vf.FileContent, &vf.ChannelTopic); err != nil {
			return nil, err
		}
		videos = append(videos, vf)
	}
	return videos, rows.Err()
}

// MarkVideoValidated ставит validation_status='validated'.
func MarkVideoValidated(videoID string) error {
	_, err := DB.Exec(`UPDATE video_files SET validation_status='validated', validation_reason=NULL, validated_at=now() WHERE video_id = $1`, videoID)
	return err
}

// MarkVideoRejected ставит validation_status='rejected' и пишет причину.
func MarkVideoRejected(videoID, reason string) error {
	_, err := DB.Exec(`UPDATE video_files SET validation_status='rejected', validation_reason=$1 WHERE video_id = $2`, reason, videoID)
	return err
}

// InsertTrash вставляет запись в trash.
func InsertTrash(t Trash) error {
	_, err := DB.Exec(`
		INSERT INTO trash (video_id, channel_name, video_url, file_content, reason, part, chapter, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, t.VideoID, t.ChannelName, t.VideoURL, t.FileContent, t.Reason, t.Part, t.Chapter, t.Source)
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
// CLASSIFIER: video_files → obsidian_targets
// ============================================================================

// GetValidatedForClassify атомарно захватывает video_files(validated) → classifying.
func GetValidatedForClassify(limit int) ([]VideoFile, error) {
	rows, err := DB.Query(`
		WITH picked AS (
			SELECT video_id FROM video_files
			WHERE validation_status = 'validated'
			  AND is_in_obsidian = false
			  AND channel_topic = 'обучение_торговле_на_рынках'
			ORDER BY created_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE video_files vf
		SET validation_status = 'classifying'
		WHERE vf.video_id IN (SELECT video_id FROM picked)
		RETURNING vf.video_id, coalesce(vf.channel_name, ''),
		          coalesce(vf.video_url, ''), coalesce(vf.file_name, ''),
		          coalesce(vf.file_content, ''), coalesce(vf.channel_topic, '')
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []VideoFile
	for rows.Next() {
		var vf VideoFile
		if err := rows.Scan(&vf.VideoID, &vf.ChannelName, &vf.VideoURL, &vf.FileName,
			&vf.FileContent, &vf.ChannelTopic); err != nil {
			return nil, err
		}
		videos = append(videos, vf)
	}
	return videos, rows.Err()
}

// MarkVideoClassified ставит validation_status='classified'.
func MarkVideoClassified(videoID string) error {
	_, err := DB.Exec(`UPDATE video_files SET validation_status='classified' WHERE video_id = $1`, videoID)
	return err
}

// MarkVideoClassifyFailed возвращает video_files в 'validated'.
func MarkVideoClassifyFailed(videoID string) error {
	_, err := DB.Exec(`UPDATE video_files SET validation_status='validated' WHERE video_id = $1`, videoID)
	return err
}

// ============================================================================
// PUBLISHER: obsidian_targets(ready) → writing
// ============================================================================

// ObsidianTarget — запись в таблице obsidian_targets.
type ObsidianTarget struct {
	ID          int
	VideoID     string
	ChannelName string
	VideoURL    string
	VideoTitle  string
	Part        string
	Chapter     string
	TargetPath  string
	Block       string
	Status      string
	Attempts    int
}

// InsertObsidianTarget создаёт запись после классификации.
func InsertObsidianTarget(t ObsidianTarget) (int, error) {
	var id int
	err := DB.QueryRow(`
		INSERT INTO obsidian_targets (video_id, channel_name, video_url, video_title, part, chapter, status, classified_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'classified', now(), now())
		RETURNING id
	`, t.VideoID, t.ChannelName, t.VideoURL, t.VideoTitle, t.Part, t.Chapter).Scan(&id)
	return id, err
}

// UpdateObsidianTargetReady сохраняет markdown-блок.
func UpdateObsidianTargetReady(id int, block string) error {
	_, err := DB.Exec(`UPDATE obsidian_targets SET block=$1, status='ready', summarized_at=now() WHERE id=$2`, block, id)
	return err
}

// MarkObsidianTargetWritten ставит status='written' и target_path.
func MarkObsidianTargetWritten(id int, targetPath string) error {
	_, err := DB.Exec(`UPDATE obsidian_targets SET status='written', target_path=$1, written_at=now() WHERE id=$2`, targetPath, id)
	return err
}

// MarkObsidianTargetError инкрементит attempts и ставит status='error'.
func MarkObsidianTargetError(id int) error {
	_, err := DB.Exec(`UPDATE obsidian_targets SET status='error', attempts=attempts+1 WHERE id=$1`, id)
	return err
}

// GetReadyForWrite атомарно захватывает obsidian_targets(ready) → writing.
func GetReadyForWrite(limit int) ([]ObsidianTarget, error) {
	rows, err := DB.Query(`
		WITH picked AS (
			SELECT ot.id FROM obsidian_targets ot
			JOIN video_files vf ON vf.video_id = ot.video_id
			WHERE ot.status = 'ready'
			  AND vf.channel_topic = 'обучение_торговле_на_рынках'
			ORDER BY ot.created_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE obsidian_targets ot
		SET status = 'writing'
		WHERE ot.id IN (SELECT id FROM picked)
		RETURNING ot.id, ot.video_id, coalesce(ot.channel_name, ''),
		          coalesce(ot.video_url, ''), coalesce(ot.video_title, ''),
		          coalesce(ot.part, ''), coalesce(ot.chapter, ''),
		          coalesce(ot.block, ''), coalesce(ot.status, ''),
		          coalesce(ot.attempts, 0)
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ObsidianTarget
	for rows.Next() {
		var t ObsidianTarget
		if err := rows.Scan(&t.ID, &t.VideoID, &t.ChannelName, &t.VideoURL, &t.VideoTitle,
			&t.Part, &t.Chapter, &t.Block, &t.Status, &t.Attempts); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

// MarkVideoInObsidian ставит is_in_obsidian=true и validation_status='published'.
func MarkVideoInObsidian(videoID string) error {
	_, err := DB.Exec(`UPDATE video_files SET is_in_obsidian=true, validation_status='published' WHERE video_id = $1`, videoID)
	return err
}

// ============================================================================
// TRASH_MANAGER
// ============================================================================

// TrashItem — запись в таблице trash с action.
type TrashItem struct {
	ID          int
	VideoID     string
	ChannelName string
	VideoURL    string
	FileContent string
	Reason      string
	Action      string
}

// GetTrashWithAction возвращает записи из trash, где action задан.
func GetTrashWithAction(limit int) ([]TrashItem, error) {
	rows, err := DB.Query(`
		SELECT id, video_id, coalesce(channel_name, ''), coalesce(video_url, ''),
		       coalesce(file_content, ''), coalesce(reason, ''), coalesce(action, '')
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
		if err := rows.Scan(&t.ID, &t.VideoID, &t.ChannelName, &t.VideoURL, &t.FileContent, &t.Reason, &t.Action); err != nil {
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