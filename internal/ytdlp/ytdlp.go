// Package ytdlp работает с утилитой yt-dlp для скачивания аудио.
package ytdlp

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"youTobe_transcribe/internal/logger"
)

// Args возвращает базовые аргументы yt-dlp.
func Args(extra ...string) []string {
	base := []string{
		"--js-runtimes", "deno",
		"--remote-components", "ejs:github",
		"--no-playlist",
		"--extractor-args", "youtube:player_client=web,mweb",
	}
	return append(base, extra...)
}

// Run запускает yt-dlp с переданными аргументами и возвращает вывод.
func Run(ytDlpPath string, args ...string) (string, error) {
	start := time.Now()
	cmd := exec.Command(ytDlpPath, Args(args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Log.Error("yt_dlp.failed",
			logger.Err(err),
			"args", strings.Join(args, " "),
			"output", strings.TrimSpace(string(out)),
			"duration_ms", time.Since(start).Milliseconds(),
		)
	} else {
		logger.Log.Info("yt_dlp.ok",
			"args", strings.Join(args, " "),
			"output", strings.TrimSpace(string(out)),
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}
	return string(out), err
}

// GetDuration возвращает длительность видео в секундах.
func GetDuration(ytDlpPath, videoURL string) (int, error) {
	out, err := Run(ytDlpPath, "--get-duration", videoURL)
	if err != nil {
		return 0, err
	}

	// yt-dlp может выводить WARNING в stdout перед длительностью.
	// Берем последнюю непустую строку, в ней должна быть длительность.
	lines := strings.Split(strings.TrimSpace(out), "\n")
	var raw string
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			raw = line
			break
		}
	}
	if raw == "" {
		return 0, fmt.Errorf("empty duration")
	}

	// Парсим формат "HH:MM:SS" или "MM:SS" или просто секунды
	parts := strings.Split(raw, ":")
	switch len(parts) {
	case 3:
		var h, m, s int
		fmt.Sscanf(parts[0], "%d", &h)
		fmt.Sscanf(parts[1], "%d", &m)
		fmt.Sscanf(parts[2], "%d", &s)
		return h*3600 + m*60 + s, nil
	case 2:
		var m, s int
		fmt.Sscanf(parts[0], "%d", &m)
		fmt.Sscanf(parts[1], "%d", &s)
		return m*60 + s, nil
	default:
		var s int
		fmt.Sscanf(raw, "%d", &s)
		return s, nil
	}
}

// DownloadAudio скачивает аудио с YouTube и возвращает путь к файлу.
func DownloadAudio(ytDlpPath, videoURL string) (string, error) {
	start := time.Now()
	id := ExtractVideoID(videoURL)
	logger.Log.Info("download.start",
		"video_id", id,
		"video_url", videoURL,
		"yt_dlp_path", ytDlpPath,
	)
	out, err := Run(ytDlpPath, "--cookies", "/cookies/cookies.txt", "-x", "--audio-format", "mp3", "-o", "/tmp/%(id)s.%(ext)s", videoURL)
	if err != nil {
		logger.Log.Error("download.failed",
			logger.Err(err),
			"video_id", id,
			"yt_dlp_output", strings.TrimSpace(out),
		)
		return "", fmt.Errorf("yt-dlp download failed: %w (output: %s)", err, out)
	}
	candidates := []string{
		fmt.Sprintf("/tmp/%s.mp3", id),
		fmt.Sprintf("/tmp/%s.webm", id),
		fmt.Sprintf("/tmp/%s.m4a", id),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			size, _ := os.Stat(c)
			logger.Log.Info("download.audio_ready",
				"video_id", id,
				"audio_path", c,
				"file_size_bytes", size.Size(),
				"duration_ms", time.Since(start).Milliseconds(),
			)
			return c, nil
		}
	}
	logger.Log.Error("download.audio_not_found",
		"video_id", id,
		"video_url", videoURL,
		"checked_paths", candidates,
	)
	return "", fmt.Errorf("downloaded audio file not found for %s", videoURL)
}

// ExtractVideoID извлекает ID видео из URL.
func ExtractVideoID(videoURL string) string {
	re := regexp.MustCompile(`(?:v=|youtu\.be/|embed/)([a-zA-Z0-9_-]{11})`)
	m := re.FindStringSubmatch(videoURL)
	if len(m) > 1 {
		return m[1]
	}
	return strings.ReplaceAll(videoURL, "/", "_")
}

// Cleanup удаляет временный аудиофайл.
func Cleanup(path string) {
	if path == "" {
		return
	}
	if err := os.Remove(path); err != nil {
		logger.Log.Warn("cleanup.remove_failed", logger.Err(err), "path", path)
	} else {
		logger.Log.Info("cleanup.ok", "path", path)
	}
}