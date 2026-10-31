//cd C:\Users\user\GO\youTobe_transcribe
//go build -o .\drive_pull.exe .\cmd\drive_pull
//.\drive_pull.exe

package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"youTobe_transcribe/internal/config"
	"youTobe_transcribe/internal/logger"
	"youTobe_transcribe/internal/storage"
)

const (
	remoteFile = "gdrive:yt_transcribe/results.csv"
	localFile  = "results.csv"
)

func main() {
	logger.Init()
	log := logger.Log

	cfg := config.Load()

	if err := storage.Init(cfg.DatabaseURL); err != nil {
		log.Error("storage.init_failed", "error", err.Error(),
			"описание", "ОШИБКА: Не удалось подключиться к базе данных")
		os.Exit(1)
	}
	defer storage.DB.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	if err := rclonePull(ctx, log); err != nil {
		log.Error("drive_pull.rclone_failed", "error", err.Error(),
			"описание", "ОШИБКА: Не удалось скачать results.csv с Google Drive")
		os.Exit(1)
	}

	success, trash, err := processResults(log)
	if err != nil {
		log.Error("drive_pull.process_failed", "error", err.Error(),
			"описание", "ОШИБКА: Не удалось обработать results.csv")
		os.Exit(1)
	}

	log.Info("drive_pull.done", "success", success, "trash", trash,
		"описание", "Обработка results.csv завершена")
}

// rclonePull скачивает results.csv с Google Drive в текущую папку.
func rclonePull(ctx context.Context, log *slog.Logger) error {
	start := time.Now()
	absPath, _ := filepath.Abs(localFile)

	cmd := exec.CommandContext(ctx, "rclone", "copy", remoteFile, ".", "-v")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rclone copy: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	log.Info("drive_pull.rclone_done",
		"local_path", absPath,
		"duration_ms", time.Since(start).Milliseconds(),
		"описание", "results.csv скачан с Google Drive")
	return nil
}

// processResults читает results.csv и пишет в video_files / trash.
func processResults(log *slog.Logger) (int, int, error) {
	f, err := os.Open(localFile)
	if err != nil {
		return 0, 0, fmt.Errorf("open %s: %w", localFile, err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return 0, 0, fmt.Errorf("read csv: %w", err)
	}

	if len(rows) < 2 {
		return 0, 0, nil
	}

	header := rows[0]
	col := make(map[string]int, len(header))
	for i, h := range header {
		col[strings.TrimSpace(strings.ToLower(h))] = i
	}

	success := 0
	trash := 0

	for i, row := range rows[1:] {
		videoID := getCol(row, col, "video_id")
		if videoID == "" {
			continue
		}
		status := getCol(row, col, "status")
		transcript := getCol(row, col, "transcript")
		errMsg := getCol(row, col, "error")

		switch status {
		case "ok":
			if transcript == "" {
				log.Warn("drive_pull.empty_transcript", "video_id", videoID, "row", i+1,
					"описание", "Пустой transcript при status=ok, отправляем в trash")
				_ = storage.InsertTrash(storage.Trash{
					VideoID:     videoID,
					ChannelName: getCol(row, col, "channel_name"),
					VideoURL:    getCol(row, col, "video_url"),
					Reason:      "empty transcript from Colab",
					Source:      "drive_pull",
				})
				_ = storage.MarkVideoProcessed(videoID)
				trash++
				continue
			}
			vf := storage.VideoFile{
				VideoID:      videoID,
				ChannelName:  getCol(row, col, "channel_name"),
				VideoURL:     getCol(row, col, "video_url"),
				ChannelTopic: getCol(row, col, "channel_topic"),
				FileName:     getCol(row, col, "file_name"),
				FileContent:  transcript,
			}
			if err := storage.InsertVideoFile(vf); err != nil {
				log.Error("drive_pull.insert_failed", "video_id", videoID, "error", err.Error(),
					"описание", "ОШИБКА: Не удалось сохранить video_files")
				continue
			}
			_ = storage.MarkVideoProcessed(videoID)
			success++
			log.Info("drive_pull.saved", "video_id", videoID, "text_len", len(transcript),
				"описание", "Транскрипт сохранён в video_files")

		case "no_speech", "error":
			reason := status
			if errMsg != "" {
				reason = status + ": " + errMsg
			}
			_ = storage.InsertTrash(storage.Trash{
				VideoID:     videoID,
				ChannelName: getCol(row, col, "channel_name"),
				VideoURL:    getCol(row, col, "video_url"),
				FileContent: transcript,
				Reason:      reason,
				Source:      "drive_pull",
			})
			_ = storage.MarkVideoProcessed(videoID)
			trash++
			log.Info("drive_pull.trashed", "video_id", videoID, "reason", reason,
				"описание", "Видео отправлено в trash")

		default:
			log.Warn("drive_pull.unknown_status", "video_id", videoID, "status", status,
				"описание", "Неизвестный статус в results.csv, пропускаем")
		}
	}

	return success, trash, nil
}

// getCol возвращает значение колонки по имени (регистронезависимо).
func getCol(row []string, col map[string]int, name string) string {
	idx, ok := col[strings.ToLower(name)]
	if !ok || idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}