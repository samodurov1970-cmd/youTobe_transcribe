// cd C:\Users\user\GO\youTobe_transcribe
// go build -o .\drive_push.exe .\cmd\drive_push
// .\drive_push.exe

package main

import (
	"database/sql"
	"encoding/csv"
	"os"
	"os/exec"

	"youTobe_transcribe/internal/config"
	"youTobe_transcribe/internal/logger"
	"youTobe_transcribe/internal/storage"
)

const (
	topic    = "обучение_торговле_на_рынках"
	limit    = 500
	csvFile  = "videos_export.csv"
	driveDir = "gdrive:yt_transcribe/"
)

func main() {
	logger.Init()
	log := logger.Log
	cfg := config.Load()

	if err := storage.Init(cfg.DatabaseURL); err != nil {
		log.Error("drive_push.storage_init_failed", "error", err.Error(), "описание", "ОШИБКА: Не удалось подключиться к БД")
		os.Exit(1)
	}
	defer storage.DB.Close()

	rows, err := storage.DB.Query(`
		SELECT video_id, video_url, channel_name, channel_topic
		FROM videos
		WHERE is_processed = false AND channel_topic = $1
		ORDER BY published_at DESC
		LIMIT $2
	`, topic, limit)
	if err != nil {
		log.Error("drive_push.query_failed", "error", err.Error(), "описание", "ОШИБКА: Не удалось выполнить запрос к БД")
		os.Exit(1)
	}
	defer rows.Close()

	file, err := os.Create(csvFile)
	if err != nil {
		log.Error("drive_push.file_failed", "error", err.Error(), "описание", "ОШИБКА: Не удалось создать файл videos_export.csv")
		os.Exit(1)
	}

	writer := csv.NewWriter(file)
	writer.Write([]string{"video_id", "video_url", "channel_name", "channel_topic", "file_name"})

	count := 0
	for rows.Next() {
		var videoID, videoURL, channelName, channelTopic sql.NullString
		if err := rows.Scan(&videoID, &videoURL, &channelName, &channelTopic); err != nil {
			log.Warn("drive_push.scan_failed", "error", err.Error(), "описание", "Пропуск строки при Scan")
			continue
		}
		fileName := videoID.String + ".txt"
		writer.Write([]string{
			videoID.String,
			videoURL.String,
			channelName.String,
			channelTopic.String,
			fileName,
		})
		count++
	}
	writer.Flush()
	file.Close()

	log.Info("drive_push.file_created", "count", count, "описание", "Создан файл videos_export.csv")

	if count == 0 {
		log.Info("drive_push.empty", "описание", "Нет видео для отправки. Выход.")
		return
	}

	log.Info("drive_push.rclone_start", "описание", "Отправка файла на Google Диск...")
	cmd := exec.Command("rclone", "copy", csvFile, driveDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Error("drive_push.rclone_failed", "error", err.Error(), "output", string(out), "описание", "ОШИБКА: rclone не смог отправить файл")
		os.Exit(1)
	}

	log.Info("drive_push.done", "count", count, "описание", "Файл успешно отправлен на Google Диск (папка yt_transcribe)")
}