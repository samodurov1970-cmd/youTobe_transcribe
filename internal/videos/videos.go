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

func (c *Collector) Run(ctx context.Context) error {
	c.Log.Info("videos.collect_started", "описание", "Начат сбор видео с каналов")

	channels, err := storage.GetActiveChannels()
	if err != nil {
		c.Log.Error("videos.channels_failed", "error", err.Error(),
			"описание", "ОШИБКА: Не удалось получить список активных каналов из БД")
		return fmt.Errorf("get active channels: %w", err)
	}

	c.Log.Info("videos.channels_fetched", "count", len(channels), "описание", "Список активных каналов успешно получен")

	totalAdded := 0
	for _, ch := range channels {
		added, err := c.collectChannelVideos(ctx, ch)
		if err != nil {
			c.Log.Error("videos.channel_failed", "channel_id", ch.ChannelID, "channel_name", ch.ChannelName, "error", err.Error(),
				"описание", "ОШИБКА: Не удалось собрать видео с канала")
			continue
		}
		totalAdded += added
	}

	c.Log.Info("videos.collect_finished", "total_added", totalAdded, "описание", "Сбор видео завершён")
	return nil
}

func (c *Collector) collectChannelVideos(ctx context.Context, ch storage.Channel) (int, error) {
	c.Log.Info("videos.channel_started", "channel_id", ch.ChannelID, "channel_name", ch.ChannelName,
		"описание", "Начат сбор видео с канала")

	videoIDs, err := c.fetchVideoIDs(ctx, ch)
	if err != nil {
		return 0, fmt.Errorf("fetch video ids: %w", err)
	}

	c.Log.Info("videos.fetched", "channel_id", ch.ChannelID, "count", len(videoIDs), "описание", "Список видео канала получен")

	added := 0
	for _, id := range videoIDs {
		v, err := c.fetchVideoDetails(ctx, id, ch)
		if err != nil {
			c.Log.Error("videos.details_failed", "video_id", id, "error", err.Error(),
				"описание", "ОШИБКА: Не удалось получить детали видео")
			continue
		}

		oneYearAgo := time.Now().AddDate(-1, 0, 0)
		if v.PublishedAt.Before(oneYearAgo) {
			continue
		}

		err = storage.InsertVideo(*v)
		if err != nil {
			c.Log.Error("videos.insert_failed", "video_id", v.VideoID, "error", err.Error(),
				"описание", "ОШИБКА: Не удалось сохранить видео в БД")
			continue
		}
		added++
	}

	c.Log.Info("videos.channel_done", "channel_id", ch.ChannelID, "added", added, "описание", "Сбор видео с канала завершён")
	return added, nil
}

// fetchVideoIDs берет только ID видео из списка канала
func (c *Collector) fetchVideoIDs(ctx context.Context, ch storage.Channel) ([]string, error) {
	url := "https://www.youtube.com/channel/" + ch.ChannelID + "/videos"
	args := []string{
		"--flat-playlist",
		"--no-warnings",
		"--playlist-end", "10000", // <--- ИСПРАВЛЕНО: было "0", стало "10000"
		"--print", "id",
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

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var ids []string
	for _, line := range lines {
		if line != "" {
			ids = append(ids, line)
		}
	}
	return ids, nil
}

// fetchVideoDetails получает точные метаданные для одного видео
func (c *Collector) fetchVideoDetails(ctx context.Context, videoID string, ch storage.Channel) (*storage.Video, error) {
	url := "https://www.youtube.com/watch?v=" + videoID
	args := []string{
		"--no-warnings",
		"--dump-single-json",
		"--skip-download",
	}

	if c.CookiesFile != "" {
		args = append(args, "--cookies", c.CookiesFile)
	}
	args = append(args, url)

	cmd := exec.CommandContext(ctx, c.YtDlpPath, args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp failed: %w", err)
	}

	var payload struct {
		ID         string `json:"id"`
		Title      string `json:"title"`
		Timestamp  int64  `json:"timestamp"`
		UploadDate string `json:"upload_date"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}

	var publishedAt time.Time
	if payload.Timestamp > 0 {
		publishedAt = time.Unix(payload.Timestamp, 0)
	} else if payload.UploadDate != "" {
		parsed, err := time.Parse("20060102", payload.UploadDate)
		if err == nil {
			publishedAt = parsed
		} else {
			publishedAt = time.Now()
		}
	} else {
		publishedAt = time.Now()
	}

	return &storage.Video{
		VideoID:      payload.ID,
		Title:        payload.Title,
		VideoURL:     url,
		PublishedAt:  publishedAt,
		ChannelID:    ch.ChannelID,
		ChannelName:  ch.ChannelName,
		ChannelTopic: ch.Topic,
	}, nil
}