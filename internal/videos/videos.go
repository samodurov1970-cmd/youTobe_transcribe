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

// Collector собирает видео с каналов.
type Collector struct {
	YtDlpPath   string
	CookiesFile string
	Log         *slog.Logger
}

// NewCollector создает новый коллектор видео.
func NewCollector(ytDlpPath, cookiesFile string, log *slog.Logger) *Collector {
	return &Collector{
		YtDlpPath:   ytDlpPath,
		CookiesFile: cookiesFile,
		Log:         log.With("component", "videos"),
	}
}

// Run собирает видео со всех активных каналов.
func (c *Collector) Run(ctx context.Context) error {
	c.Log.Info("videos.collect_started", "описание", "Начат сбор видео с каналов")

	channels, err := storage.GetActiveChannels()
	if err != nil {
		c.Log.Error("videos.channels_failed",
			"error", err.Error(),
			"описание", "ОШИБКА: Не удалось получить список активных каналов из БД")
		return fmt.Errorf("get active channels: %w", err)
	}

	c.Log.Info("videos.channels_fetched",
		"count", len(channels),
		"описание", "Список активных каналов успешно получен")

	totalAdded := 0
	for _, ch := range channels {
		added, err := c.collectChannelVideos(ctx, ch)
		if err != nil {
			c.Log.Error("videos.channel_failed",
				"channel_id", ch.ChannelID,
				"channel_name", ch.ChannelName,
				"error", err.Error(),
				"описание", "ОШИБКА: Не удалось собрать видео с канала")
			continue
		}
		totalAdded += added
	}

	c.Log.Info("videos.collect_finished",
		"total_added", totalAdded,
		"описание", "Сбор видео завершён")
	return nil
}

// collectChannelVideos собирает видео с одного канала и сохраняет новые в БД.
func (c *Collector) collectChannelVideos(ctx context.Context, ch storage.Channel) (int, error) {
	c.Log.Info("videos.channel_started",
		"channel_id", ch.ChannelID,
		"channel_name", ch.ChannelName,
		"описание", "Начат сбор видео с канала")

	videos, err := c.fetchChannelVideos(ctx, ch)
	if err != nil {
		return 0, fmt.Errorf("fetch videos: %w", err)
	}

	c.Log.Info("videos.fetched",
		"channel_id", ch.ChannelID,
		"count", len(videos),
		"описание", "Список видео канала получен")

	added := 0
	for _, v := range videos {
		err := storage.InsertVideo(v)
		if err != nil {
			c.Log.Error("videos.insert_failed",
				"video_id", v.VideoID,
				"error", err.Error(),
				"описание", "ОШИБКА: Не удалось сохранить видео в БД")
			continue
		}
		added++
	}

	c.Log.Info("videos.channel_done",
		"channel_id", ch.ChannelID,
		"added", added,
		"описание", "Сбор видео с канала завершён")
	return added, nil
}

// fetchChannelVideos получает список видео канала через yt-dlp.
func (c *Collector) fetchChannelVideos(ctx context.Context, ch storage.Channel) ([]storage.Video, error) {
	url := "https://www.youtube.com/channel/" + ch.ChannelID + "/videos"
	args := []string{
		"--flat-playlist",
		"--no-warnings",
		"--dump-single-json",
		"--playlist-end", "100",
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
			ID        string `json:"id"`
			Title     string `json:"title"`
			URL       string `json:"url"`
			Timestamp int64  `json:"timestamp"`
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
		publishedAt := time.Unix(e.Timestamp, 0)
		videos = append(videos, storage.Video{
			VideoID:      e.ID,
			Title:        e.Title,
			VideoURL:     "https://www.youtube.com/watch?v=" + e.ID,
			PublishedAt:  publishedAt,
			ChannelID:    ch.ChannelID,
			ChannelName:  ch.ChannelName,
			ChannelTopic: ch.Topic,
			IsNews:       ch.IsNews,
		})
	}

	return videos, nil
}