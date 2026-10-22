package cookies

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"log/slog"
)

// Checker проверяет актуальность куки для YouTube через yt-dlp.
type Checker struct {
	YtDlpPath      string
	CookiesFile    string
	TestVideoID    string
	Log            *slog.Logger
	TelegramToken  string
	TelegramChatID string
}

// NewChecker создает новый проверяльщик куки.
func NewChecker(ytDlpPath, cookiesFile, telegramToken, telegramChatID string, log *slog.Logger) *Checker {
	return &Checker{
		YtDlpPath:      ytDlpPath,
		CookiesFile:    cookiesFile,
		TestVideoID:    "jNQXAC9IVRw", // "Me at the zoo"
		Log:            log,
		TelegramToken:  telegramToken,
		TelegramChatID: telegramChatID,
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

// SendReport отправляет отчёт в Telegram.
func (c *Checker) SendReport(ctx context.Context, err error) {
	if c.TelegramToken == "" || c.TelegramChatID == "" {
		c.Log.Warn("cookies.telegram_skipped", "reason", "not_configured")
		return
	}

	// Форматируем дату по-русски
	now := time.Now().Format("02.01.2006 15:04:05")

	var text string
	if err == nil {
		text = fmt.Sprintf("✅ <b>YOUTUBE: ВСЁ ХОРОШО</b>\n\n"+
			"📅 Дата проверки: %s\n"+
			"🍪 Куки актуальны. Всё работает в штатном режиме.", now)
	} else {
		// Обрезаем ошибку до 300 символов
		errText := err.Error()
		if len(errText) > 300 {
			errText = errText[:300] + "..."
		}
		text = fmt.Sprintf("❌ <b>YOUTUBE: ПРОБЛЕМА</b>\n\n"+
			"📅 Дата проверки: %s\n"+
			"⚠️ Причина: %s\n\n"+
			"<b>Что делать:</b>\n"+
			"1. Откройте YouTube в браузере (под аккаунтом).\n"+
			"2. Обновите cookies.txt (расширение Get cookies.txt LOCALLY).\n"+
			"3. Замените файл internal/cookies/cookies.txt", now, errText)
	}

	payload := map[string]interface{}{
		"chat_id":                  c.TelegramChatID,
		"text":                     text,
		"parse_mode":               "HTML",
		"disable_web_page_preview": true,
	}
	body, _ := json.Marshal(payload)

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.TelegramToken)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.Log.Error("cookies.telegram_send_failed", "error", err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.Log.Error("cookies.telegram_send_failed", "status", resp.StatusCode)
		return
	}
	c.Log.Info("cookies.report_sent")
}