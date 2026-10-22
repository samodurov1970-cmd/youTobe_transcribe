package cookies

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"log/slog"
)

// Checker проверяет актуальность куки для YouTube через yt-dlp.
type Checker struct {
	YtDlpPath   string
	CookiesFile string
	TestVideoID string
	Log         *slog.Logger
}

// NewChecker создает новый проверяльщик куки.
func NewChecker(ytDlpPath, cookiesFile string, log *slog.Logger) *Checker {
	return &Checker{
		YtDlpPath:   ytDlpPath,
		CookiesFile: cookiesFile,
		TestVideoID: "jNQXAC9IVRw", // "Me at the zoo"
		Log:         log,
	}
}

// Check запускает yt-dlp с куки и проверяет, возвращает ли он длительность видео.
func (c *Checker) Check(ctx context.Context) error {
	url := "https://youtu.be/" + c.TestVideoID
	args := []string{
		"--js-runtimes", "deno:/root/.deno/bin/deno",
		"--remote-components", "ejs:npm",
		"--cookies", c.CookiesFile,
		"--no-playlist",
		"--get-duration",
		url,
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.YtDlpPath, args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("yt-dlp failed: %s: %w", strings.TrimSpace(string(exitErr.Stderr)), err)
		}
		return fmt.Errorf("yt-dlp failed: %w", err)
	}

	duration := strings.TrimSpace(string(out))
	if duration == "" {
		return fmt.Errorf("yt-dlp returned empty duration")
	}

	for _, r := range duration {
		if (r < '0' || r > '9') && r != '.' {
			return fmt.Errorf("yt-dlp returned non-numeric duration: %s", duration)
		}
	}

	c.Log.Info("cookies.valid", "duration", duration)
	return nil
}