// Package workflow реализует основной конвейер транскрибации YouTube.
package workflow

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	"youTobe_transcribe/internal/config"
	"youTobe_transcribe/internal/logger"
	"youTobe_transcribe/internal/obsidian"
	"youTobe_transcribe/internal/storage"
	"youTobe_transcribe/internal/transcribe"
	"youTobe_transcribe/internal/ytdlp"
)

// Component — имя подсистемы для атрибута component в логах.
const Component = "workflow"

// Run выполняет полный цикл: claim → download → transcribe → DB → Obsidian.
func Run(cfg config.Config) {
	log := logger.Log.With("component", Component)
	cycleStart := time.Now()
	log.Info("workflow.start",
		"workflow", "youtube_transcription",
		"yt_dlp_path", cfg.YtDlpPath,
		"obsidian_path", cfg.ObsidianPath,
		"transcription_url", cfg.TranscriptionURL,
	)

	channelTopics := []string{"обучение_торговле_на_рынкам", "мой_морской_аквариум"}
	claims, err := storage.ClaimVideosForTranscription(channelTopics, 2)
	if err != nil {
		log.Error("workflow.claim_failed", logger.Err(err), "workflow", "youtube_transcription")
		return
	}
	if claims == 0 {
		log.Info("workflow.no_claims", "workflow", "youtube_transcription",
			"cycle_duration_ms", time.Since(cycleStart).Milliseconds())
		return
	}
	log.Info("workflow.claimed", "claims", claims, "workflow", "youtube_transcription")

	processed := 0
	failed := 0

	for i := 0; i < claims; i++ {
		start := time.Now()
		log.Info("workflow.record_index", "index", i+1, "of", claims)

		v, err := storage.FetchNextProcessing()
		if err == sql.ErrNoRows {
			log.Info("workflow.no_more_processing", "workflow", "youtube_transcription")
			break
		}
		if err != nil {
			log.Error("workflow.fetch_processing_failed", logger.Err(err), "workflow", "youtube_transcription")
			break
		}

		rec := log.With(
			"workflow", "youtube_transcription",
			"record_id", v.ID,
			"video_id", v.VideoID,
			"video_url", v.VideoURL,
			"channel_name", v.ChannelName,
		)
		rec.Info("record.start", "step", "begin")

		// Шаг 1: скачивание аудио
		rec.Info("record.step", "step", "download", "action", "downloading_audio")
		audioPath, err := ytdlp.DownloadAudio(cfg.YtDlpPath, v.VideoURL)
		if err != nil {
			storage.MarkError(v.ID, err.Error())
			rec.Error("record.download_failed", logger.Err(err), "step", "download")
			failed++
			continue
		}
		rec.Info("record.download_done", "step", "download", "audio_path", audioPath)

		// Шаг 2: транскрибация
		rec.Info("record.step", "step", "transcribe", "action", "sending_to_whisper")
		transcript, err := transcribe.File(cfg.TranscriptionURL, audioPath)
		ytdlp.Cleanup(audioPath)
		if err != nil {
			storage.MarkError(v.ID, err.Error())
			rec.Error("record.transcribe_failed", logger.Err(err), "step", "transcribe")
			failed++
			continue
		}
		rec.Info("record.transcribe_done", "step", "transcribe", "transcript_len", len(transcript))

		// Шаг 3: сохранение в БД
		rec.Info("record.step", "step", "db_update", "action", "marking_transcribed")
		if err := storage.MarkTranscribed(v.ID, transcript); err != nil {
			rec.Error("record.db_update_failed", logger.Err(err), "step", "db_update")
			failed++
			continue
		}
		rec.Info("record.db_update_done", "step", "db_update")

		// Шаг 4: запись в Obsidian
		relPath := filepath.Join("DB_Obsidian", "Работа_с_YouTube", "Транскрипты", fmt.Sprintf("%s.md", v.VideoID))
		rec.Info("record.step", "step", "obsidian_write", "action", "writing_file", "relative_path", relPath)
		if err := obsidian.Write(cfg.ObsidianPath, relPath, transcript); err != nil {
			rec.Error("record.obsidian_write_failed", logger.Err(err), "step", "obsidian_write")
		} else {
			rec.Info("record.obsidian_write_done", "step", "obsidian_write", "relative_path", relPath)
		}

		processed++
		rec.Info("record.done",
			"transcript_len", len(transcript),
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}

	log.Info("workflow.finish",
		"workflow", "youtube_transcription",
		"total_claims", claims,
		"processed", processed,
		"failed", failed,
		"cycle_duration_ms", time.Since(cycleStart).Milliseconds(),
	)
}
