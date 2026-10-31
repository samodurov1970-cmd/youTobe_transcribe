// cd C:\Users\user\GO\youTobe_transcribe
//rclone copy .\videos_export.csv gdrive:yt_transcribe/ -v

package main

import (
	"encoding/csv"
	"os"
	"os/exec"

	"youTobe_transcribe/internal/config"
	"youTobe_transcribe/internal/logger"
	"youTobe_transcribe/internal/storage"
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

	// 1. Берем 500 новых видео по торговле
	rows, err := storage.DB.Query(`
		SELECT video_id, video_url 
		FROM videos 
		WHERE is_processed = false AND channel_topic = 'обучение_торговле_на_рынках'
		ORDER BY published_at DESC 
		LIMIT 500
	`)
	if err != nil {
		log.Error("drive_push.query_failed", "error", err.Error(), "описание", "ОШИБКА: Не удалось выполнить запрос к БД")
		os.Exit(1)
	}
	defer rows.Close()

	file, err := os.Create("videos_export.csv")
	if err != nil {
		log.Error("drive_push.file_failed", "error", err.Error(), "описание", "ОШИБКА: Не удалось создать файл videos_export.csv")
		os.Exit(1)
	}

	writer := csv.NewWriter(file)
	writer.Write([]string{"video_id", "video_url"})

	count := 0
	for rows.Next() {
		var videoID, videoURL string
		if err := rows.Scan(&videoID, &videoURL); err != nil {
			continue
		}
		writer.Write([]string{videoID, videoURL})
		count++
	}
	writer.Flush()
	file.Close()

	log.Info("drive_push.file_created", "count", count, "описание", "Создан файл videos_export.csv")

	if count == 0 {
		log.Info("drive_push.empty", "описание", "Нет видео для отправки. Выход.")
		return
	}

	// 2. Отправляем на Google Диск через rclone
	log.Info("drive_push.rclone_start", "описание", "Отправка файла на Google Диск...")
	cmd := exec.Command("rclone", "copy", "videos_export.csv", "gdrive:yt_transcribe/")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Error("drive_push.rclone_failed", "error", err.Error(), "output", string(out), "описание", "ОШИБКА: rclone не смог отправить файл")
		os.Exit(1)
	}

	log.Info("drive_push.done", "описание", "Файл успешно отправлен на Google Диск (папка yt_transcribe)")
}