//cd C:\Users\user\GO\youTobe_transcribe
//go build -o .\drive_pull.exe .\cmd\drive_pull
//.\drive_pull.exe

package main

import (
	"context"
	"database/sql"
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

	success, trash, reset, err := processResults(log)
	if err != nil {
		log.Error("drive_pull.process_failed", "error", err.Error(),
			"описание", "ОШИБКА: Не удалось обработать results.csv")
		os.Exit(1)
	}

	log.Info("drive_pull.done", "success", success, "trash", trash, "reset", reset,
		"описание", "Обработка results.csv завершена")
}

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

func normalizeStatus(raw, errMsg string) (status, errOut string) {
	s := strings.TrimSpace(raw)
	low := strings.ToLower(s)

	switch {
	case low == "ok":
		return "ok", ""
	case low == "no_speech":
		return "no_speech", ""
	case strings.HasPrefix(low, "error"):
		msg := errMsg
		if msg == "" {
			msg = s
		}
		return "error", msg
	default:
		return "", s
	}
}

// sanitizeField убирает BOM и прочие непечатные префиксы из значения.
func sanitizeField(s string) string {
	s = strings.TrimPrefix(s, "\ufeff")
	s = strings.TrimPrefix(s, "\uFEFF")
	return strings.TrimSpace(s)
}

// lookupVideoMeta дотягивает channel_name и channel_topic из таблицы videos по video_id.
// Возвращает found=false, если видео нет в videos (тогда вставка в video_files упадёт по FK).
func lookupVideoMeta(videoID string) (channelName, channelTopic string, found bool, err error) {
	var cn, ct sql.NullString
	row := storage.DB.QueryRow(`
		SELECT channel_name, channel_topic
		FROM videos
		WHERE video_id = $1
	`, videoID)
	if scanErr := row.Scan(&cn, &ct); scanErr != nil {
		if scanErr == sql.ErrNoRows {
			return "", "", false, nil
		}
		return "", "", false, scanErr
	}
	return cn.String, ct.String, true, nil
}

func processResults(log *slog.Logger) (int, int, int, error) {
	data, err := os.ReadFile(localFile)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("read %s: %w", localFile, err)
	}

	// Убираем BOM в начале файла
	content := strings.TrimPrefix(string(data), "\ufeff")
	content = strings.TrimPrefix(content, "\uFEFF")

	reader := csv.NewReader(strings.NewReader(content))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("parse csv: %w", err)
	}

	if len(rows) < 2 {
		return 0, 0, 0, nil
	}

	header := rows[0]
	col := make(map[string]int, len(header))
	for i, h := range header {
		key := strings.ToLower(sanitizeField(h))
		col[key] = i
	}

	success := 0
	trash := 0
	reset := 0

	for i, row := range rows[1:] {
		videoID := sanitizeField(getCol(row, col, "video_id"))
		if videoID == "" {
			continue
		}

		rawStatus := getCol(row, col, "status")
		transcript := getCol(row, col, "transcript")
		errMsg := getCol(row, col, "error")

		status, normErr := normalizeStatus(rawStatus, errMsg)

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

			// === Дотягиваем channel_name / channel_topic из videos по video_id ===
			dbChannelName, dbChannelTopic, foundInVideos, lookupErr := lookupVideoMeta(videoID)
			if lookupErr != nil {
				log.Warn("drive_pull.lookup_failed", "video_id", videoID, "error", lookupErr.Error(),
					"описание", "Не удалось дотянуть channel_name/channel_topic из videos, продолжаем")
			}
			if !foundInVideos {
				log.Warn("drive_pull.video_not_in_videos", "video_id", videoID,
					"описание", "Видео нет в videos — вставка в video_files упадёт по FK. Пропускаем.")
				// возвращаем в очередь, чтобы видео сначала попало в videos
				_ = storage.ResetVideoProcessed(videoID)
				reset++
				continue
			}

			// если в CSV есть — берём оттуда, иначе из videos
			cn := getCol(row, col, "channel_name")
			if cn == "" {
				cn = dbChannelName
			}
			ct := getCol(row, col, "channel_topic")
			if ct == "" {
				ct = dbChannelTopic
			}
			fn := getCol(row, col, "file_name")
			if fn == "" {
				fn = videoID + ".txt"
			}

			vf := storage.VideoFile{
				VideoID:      videoID,
				ChannelName:  cn,
				VideoURL:     getCol(row, col, "video_url"),
				ChannelTopic: ct,
				FileName:     fn,
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
				"channel_name", cn, "channel_topic", ct, "file_name", fn,
				"описание", "Транскрипт сохранён в video_files")

		case "no_speech":
			_ = storage.InsertTrash(storage.Trash{
				VideoID:     videoID,
				ChannelName: getCol(row, col, "channel_name"),
				VideoURL:    getCol(row, col, "video_url"),
				FileContent: transcript,
				Reason:      "no_speech",
				Source:      "drive_pull",
			})
			_ = storage.MarkVideoProcessed(videoID)
			trash++
			log.Info("drive_pull.trashed", "video_id", videoID, "reason", "no_speech",
				"описание", "Видео отправлено в trash")

		case "error":
			reason := normErr
			if reason == "" {
				reason = "error"
			} else if !strings.HasPrefix(strings.ToLower(reason), "error") {
				reason = "error: " + reason
			}

			_ = storage.InsertTrash(storage.Trash{
				VideoID:     videoID,
				ChannelName: getCol(row, col, "channel_name"),
				VideoURL:    getCol(row, col, "video_url"),
				FileContent: transcript,
				Reason:      reason,
				Source:      "drive_pull",
			})
			trash++

			if strings.Contains(strings.ToLower(reason), "members-only") {
				_ = storage.MarkVideoProcessed(videoID)
				log.Info("drive_pull.members_only", "video_id", videoID,
					"описание", "members-only, повтор невозможен, is_processed=true")
				continue
			}

			_ = storage.ResetVideoProcessed(videoID)
			reset++
			log.Info("drive_pull.error_reset", "video_id", videoID, "reason", reason,
				"описание", "Видео с ошибкой возвращено в очередь (is_processed=false)")

		default:
			log.Warn("drive_pull.unknown_status", "video_id", videoID, "status", rawStatus,
				"описание", "Неизвестный статус в results.csv, пропускаем")
		}
	}

	return success, trash, reset, nil
}

func getCol(row []string, col map[string]int, name string) string {
	idx, ok := col[strings.ToLower(name)]
	if !ok || idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}