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

	"youTobe_transcribe/internal/logger"
	"youTobe_transcribe/internal/storage"
)

// Component — имя подсистемы для атрибута component в логах.
const Component = "cookies"

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
// Внешний логгер игнорируется, чтобы гарантировать component=cookies.
func NewChecker(ytDlpPath, cookiesFile, telegramToken, telegramChatID string, _ *slog.Logger) *Checker {
	log := logger.Log.With("component", Component)
	return &Checker{
		YtDlpPath:      ytDlpPath,
		CookiesFile:    cookiesFile,
		TestVideoID:    "jNQXAC9IVRw", // "Me at the zoo"
		Log:            log,
		TelegramToken:  telegramToken,
		TelegramChatID: telegramChatID,
	}
}

// getMinskLocation возвращает часовой пояс Минска или UTC+3, если загрузить не удалось.
func getMinskLocation() *time.Location {
	loc, err := time.LoadLocation("Europe/Minsk")
	if err != nil {
		// Если tzdata не установлена, используем фиксированный сдвиг +3 часа
		return time.FixedZone("MSK", 3*60*60)
	}
	return loc
}

// Check запускает yt-dlp с куки и проверяет, возвращает ли он длительность видео.
func (c *Checker) Check(ctx context.Context) error {
	url := "https://youtu.be/" + c.TestVideoID
	args := []string{
		"--js-runtimes", "node",
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

	c.Log.Info("cookies.valid", "duration", duration, "описание", "Куки валидны, yt-dlp успешно получил длительность видео")
	return nil
}

// SendReport отправляет отчёт о статусе куки в Telegram.
func (c *Checker) SendReport(ctx context.Context, err error) {
	if c.TelegramToken == "" || c.TelegramChatID == "" {
		c.Log.Warn("cookies.telegram_skipped", "reason", "not_configured", "описание", "Отправка в Telegram пропущена: не задан токен или chat_id")
		return
	}

	// Получаем минское время
	now := time.Now().In(getMinskLocation()).Format("02.01.2006 15:04:05")

	var text string
	if err == nil {
		text = fmt.Sprintf("✅ <b>YOUTUBE: ВСЁ ХОРОШО</b>\n\n"+
			"📅 Дата проверки: %s\n"+
			"🍪 Куки актуальны. Всё работает в штатном режиме.", now)
	} else {
		errText := err.Error()
		if len(errText) > 300 {
			errText = errText[:300] + "..."
		}
		text = fmt.Sprintf("❌ <b>YOUTUBE: ПРОБЛЕМА</b>\n\n"+
			"📅 Дата проверки: %s\n"+
			"⚠️ Причина: %s\n\n"+
			"<b>Что делать:</b>\n"+
			"1. Проверьте логи контейнера (возможно, проблема с yt-dlp или node).\n"+
			"2. Если ошибка про куки — обновите cookies.txt.\n"+
			"3. Перезапустите контейнер.", now, errText)
	}

	c.sendTelegram(ctx, text, "cookies.report_sent", "Отчёт о статусе куки отправлен в Telegram")
}

// SendDailyStats отправляет ежедневный отчёт по видео в Telegram.
func (c *Checker) SendDailyStats(ctx context.Context, stats storage.VideoStats) {
	if c.TelegramToken == "" || c.TelegramChatID == "" {
		c.Log.Warn("cookies.stats_telegram_skipped", "reason", "not_configured", "описание", "Отправка статистики в Telegram пропущена: не задан токен или chat_id")
		return
	}

	// Получаем минское время
	now := time.Now().In(getMinskLocation()).Format("02.01.2006")
	text := fmt.Sprintf("📊 <b>Статистика за %s</b>\n\n"+
		"🆕 Добавлено за сутки: <b>%d</b>\n"+
		"📚 Всего видео в базе: <b>%d</b>\n"+
		"✅ Обработано: <b>%d</b>\n"+
		"⏳ Не обработано: <b>%d</b>",
		now, stats.Today, stats.Total, stats.Processed, stats.Unprocessed)

	c.sendTelegram(ctx, text, "cookies.stats_sent", "Ежедневная статистика по видео отправлена в Telegram")
}

// sendTelegram — вспомогательная функция для отправки сообщений.
func (c *Checker) sendTelegram(ctx context.Context, text string, logMsg string, logDesc string) {
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
		c.Log.Error(logMsg+"_failed", "error", err.Error(), "описание", "ОШИБКА: Не удалось отправить сообщение в Telegram")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.Log.Error(logMsg+"_failed", "status", resp.StatusCode, "описание", "ОШИБКА: Telegram API вернул ошибочный статус")
		return
	}
	c.Log.Info(logMsg, "описание", logDesc)
}