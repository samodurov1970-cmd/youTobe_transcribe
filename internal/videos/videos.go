package videos

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"youTobe_transcribe/internal/storage"
)

const channelsPerBatch = 5

type Collector struct {
	YtDlpPath   string
	CookiesFile string
	Log         *slog.Logger
}

func NewCollector(ytDlpPath, cookiesFile string, log *slog.Logger) *Collector {
	return &Collector{
		YtDlpPath:   ytDlpPath,
		CookiesFile: cookiesFile,
		Log:         log.With("component", "videos"),
	}
}

// Run собирает видео со всех активных каналов по 5 штук.
func (c *Collector) Run(ctx context.Context) error {
	c.Log.Info("videos.collect_started", "описание", "Начат сбор видео с каналов")

	channels, err := storage.GetActiveChannels()
	if err != nil {
		c.Log.Error("videos.channels_failed", "error", err.Error(),
			"описание", "ОШИБКА: Не удалось получить список активных каналов из БД")
		return fmt.Errorf("get active channels: %w", err)
	}

	c.Log.Info("videos.channels_fetched", "count", len(channels),
		"описание", "Список активных каналов успешно получен")

	totalAdded := 0
	for i := 0; i < len(channels); i += channelsPerBatch {
		end := i + channelsPerBatch
		if end > len(channels) {
			end = len(channels)
		}
		batch := channels[i:end]

		c.Log.Info("videos.batch_started", "batch_index", i/channelsPerBatch+1, "batch_size", len(batch),
			"описание", "Начата обработка пачки каналов")

		for _, ch := range batch {
			added, err := c.collectChannelVideos(ctx, ch)
			if err != nil {
				c.Log.Error("videos.channel_failed", "channel_id", ch.ChannelID, "channel_name", ch.ChannelName, "error", err.Error(),
					"описание", "ОШИБКА: Не удалось собрать видео с канала")
				continue
			}
			totalAdded += added
		}

		c.Log.Info("videos.batch_finished", "batch_index", i/channelsPerBatch+1,
			"описание", "Пачка каналов обработана")
	}

	c.Log.Info("videos.collect_finished", "total_added", totalAdded,
		"описание", "Сбор видео завершён")
	return nil
}

// collectChannelVideos собирает видео с одного канала.
func (c *Collector) collectChannelVideos(ctx context.Context, ch storage.Channel) (int, error) {
	c.Log.Info("videos.channel_started", "channel_id", ch.ChannelID, "channel_name", ch.ChannelName,
		"описание", "Начат сбор видео с канала")

	videos, err := c.fetchChannelVideos(ctx, ch)
	if err != nil {
		return 0, fmt.Errorf("fetch videos: %w", err)
	}

	c.Log.Info("videos.fetched", "channel_id", ch.ChannelID, "count", len(videos),
		"описание", "Список видео канала получен")

	added := 0
	for _, v := range videos {
		// 1. Проверяем, есть ли видео уже в БД
		exists, err := storage.VideoExists(v.VideoID)
		if err != nil {
			c.Log.Error("videos.check_exists_failed", "video_id", v.VideoID, "error", err.Error(),
				"описание", "ОШИБКА: Не удалось проверить наличие видео в БД")
			continue
		}
		if exists {
			continue
		}

		// 2. Проверяем дату (не старше 1 года)
		oneYearAgo := time.Now().AddDate(-1, 0, 0)
		if v.PublishedAt.Before(oneYearAgo) {
			continue
		}

		// 3. Проверяем длительность (не длиннее 1 часа)
		duration, err := c.getVideoDuration(ctx, v.VideoID)
		if err != nil {
			c.Log.Warn("videos.duration_failed", "video_id", v.VideoID, "error", err.Error(),
				"описание", "Не удалось получить длительность видео, пропускаем проверку")
		} else if duration > 3600 {
			c.Log.Info("videos.skipped_too_long", "video_id", v.VideoID, "duration", duration,
				"описание", "Видео пропущено: длиннее 1 часа")
			continue
		}

		// 4. Вставляем в БД
		if err := storage.InsertVideo(v); err != nil {
			c.Log.Error("videos.insert_failed", "video_id", v.VideoID, "error", err.Error(),
				"описание", "ОШИБКА: Не удалось сохранить видео в БД")
			continue
		}
		added++
	}

	c.Log.Info("videos.channel_done", "channel_id", ch.ChannelID, "added", added,
		"описание", "Сбор видео с канала завершён")
	return added, nil
}

// fetchChannelVideos получает список видео канала (один запрос).
func (c *Collector) fetchChannelVideos(ctx context.Context, ch storage.Channel) ([]storage.Video, error) {
	url := "https://www.youtube.com/channel/" + ch.ChannelID + "/videos"
	args := []string{
		"--flat-playlist",
		"--no-warnings",
		"--dump-single-json",
		"--playlist-end", "100",
		"--extractor-args", "youtubetab:approximate_date",
	}

	if c.CookiesFile != "" {
		args = append(args, "--cookies", c.CookiesFile)
	}
	args = append(args, url)

	cmd := exec.CommandContext(ctx, c.YtDlpPath, args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("yt-dlp failed: %s: %w", strings.TrimSpace(string(exitErr.Stderr)), err)
		}
		return nil, fmt.Errorf("yt-dlp failed: %w", err)
	}

	var payload struct {
		Entries []struct {
			ID         string `json:"id"`
			Title      string `json:"title"`
			URL        string `json:"url"`
			Timestamp  int64  `json:"timestamp"`
			UploadDate string `json:"upload_date"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("parse yt-dlp json: %w", err)
	}

	videos := make([]storage.Video, 0, len(payload.Entries))
	for _, e := range payload.Entries {
		if e.ID == "" {
			continue
		}

		var publishedAt time.Time
		if e.Timestamp > 0 {
			publishedAt = time.Unix(e.Timestamp, 0)
		} else if e.UploadDate != "" {
			parsed, err := time.Parse("20060102", e.UploadDate)
			if err == nil {
				publishedAt = parsed
			} else {
				publishedAt = time.Now()
			}
		} else {
			publishedAt = time.Now()
		}

		videos = append(videos, storage.Video{
			VideoID:      e.ID,
			Title:        e.Title,
			VideoURL:     "https://www.youtube.com/watch?v=" + e.ID,
			PublishedAt:  publishedAt,
			ChannelID:    ch.ChannelID,
			ChannelName:  ch.ChannelName,
			ChannelTopic: ch.Topic,
		})
	}
	return videos, nil
}

// getVideoDuration получает длительность видео через yt-dlp.
func (c *Collector) getVideoDuration(ctx context.Context, videoID string) (int, error) {
	url := "https://www.youtube.com/watch?v=" + videoID
	args := []string{
		"--no-warnings",
		"--print", "duration",
		"--skip-download",
	}

	if c.CookiesFile != "" {
		args = append(args, "--cookies", c.CookiesFile)
	}
	args = append(args, url)

	cmd := exec.CommandContext(ctx, c.YtDlpPath, args...)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("yt-dlp failed: %w", err)
	}

	var duration int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &duration); err != nil {
		return 0, fmt.Errorf("parse duration: %w", err)
	}
	return duration, nil
}