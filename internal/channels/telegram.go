package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"time"
)

// Настройки Telegram Bot API для уведомлений о новых подписках.
const (
	// telegramAPIBase — базовый адрес Telegram Bot API.
	telegramAPIBase = "https://api.telegram.org"
	// telegramTimeout — таймаут одного HTTP-запроса к Telegram.
	telegramTimeout = 30 * time.Second
	// telegramAttempts — количество попыток отправки одного уведомления.
	telegramAttempts = 3
	// telegramRetryDelay — пауза между попытками отправки.
	telegramRetryDelay = 2 * time.Second
	// descriptionLimit — максимальная длина описания канала в сообщении.
	descriptionLimit = 500
)

// notifier отправляет уведомления о новых подписках в Telegram.
//
// Нулевой указатель и пустые токен/чат означают, что уведомления выключены:
// в этом случае notifier.enabled() возвращает false, а отправка не выполняется.
type notifier struct {
	token   string
	chatID  string
	apiBase string
	client  *http.Client
}

// newNotifier создаёт отправитель уведомлений Telegram.
// apiBase по умолчанию — telegramAPIBase (подменяется в тестах).
func newNotifier(token, chatID string) *notifier {
	return &notifier{
		token:   strings.TrimSpace(token),
		chatID:  strings.TrimSpace(chatID),
		apiBase: telegramAPIBase,
		client:  &http.Client{Timeout: telegramTimeout},
	}
}

// enabled сообщает, настроены ли уведомления.
func (n *notifier) enabled() bool {
	return n != nil && n.token != "" && n.chatID != ""
}

// telegramResponse — ответ Telegram Bot API на sendMessage.
type telegramResponse struct {
	OK          bool            `json:"ok"`
	Description string          `json:"description"`
	Result      json.RawMessage `json:"result,omitempty"`
}

// send отправляет текстовое сообщение в настроенный чат.
// При сетевых ошибках и отрицательном ответе API выполняются повторные попытки.
func (n *notifier) send(ctx context.Context, text string) error {
	if !n.enabled() {
		return fmt.Errorf("telegram notifier is not configured")
	}

	payload := map[string]any{
		"chat_id":                  n.chatID,
		"text":                     text,
		"parse_mode":               "HTML",
		"disable_web_page_preview": true,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal telegram payload: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", n.apiBase, n.token)

	var lastErr error
	for attempt := 1; attempt <= telegramAttempts; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(telegramRetryDelay):
			}
		}

		if err := n.do(ctx, url, body); err != nil {
			lastErr = err
		} else {
			return nil
		}
	}

	return fmt.Errorf("telegram send failed after %d attempts: %w", telegramAttempts, lastErr)
}

// do выполняет один HTTP-запрос к Telegram и разбирает ответ.
func (n *notifier) do(ctx context.Context, url string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read telegram response: %w", err)
	}

	var parsed telegramResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return fmt.Errorf("parse telegram response (status %d): %w", resp.StatusCode, err)
	}
	if !parsed.OK {
		return fmt.Errorf("telegram api error (status %d): %s", resp.StatusCode, parsed.Description)
	}
	return nil
}

// formatSubscriptionMessage формирует текст уведомления о новой подписке.
//
// Сообщение содержит: название канала, кликабельную ссылку на канал,
// идентификатор канала, тему и описание. Все пользовательские значения
// экранируются, так как используется HTML-разметка Telegram.
func formatSubscriptionMessage(ch subscribedChannel) string {
	var b strings.Builder

	b.WriteString("Вы подписались на:\n\n")

	name := ch.ChannelName
	if name == "" {
		name = "без названия"
	}
	b.WriteString("<b>" + html.EscapeString(name) + "</b>\n")

	url := ch.ChannelURL
	if url == "" {
		url = "https://www.youtube.com/channel/" + ch.ChannelID
	}
	b.WriteString("<a href=\"" + html.EscapeString(url) + "\">" + html.EscapeString(url) + "</a>\n\n")

	b.WriteString("ID: <code>" + html.EscapeString(ch.ChannelID) + "</code>\n")

	topic := ch.Topic
	if topic == "" {
		topic = "не определена"
	}
	b.WriteString("Тема: " + html.EscapeString(topic) + "\n")

	if desc := truncateDescription(ch.Description); desc != "" {
		b.WriteString("\nОписание:\n" + html.EscapeString(desc))
	}

	return b.String()
}

// formatUnsubscriptionMessage формирует текст уведомления об отписке.
func formatUnsubscriptionMessage(ch subscribedChannel) string {
	var b strings.Builder

	b.WriteString("Вы отписались от:\n\n")

	name := ch.ChannelName
	if name == "" {
		name = "без названия"
	}
	b.WriteString("<b>" + html.EscapeString(name) + "</b>\n")

	url := ch.ChannelURL
	if url == "" {
		url = "https://www.youtube.com/channel/" + ch.ChannelID
	}
	b.WriteString("<a href=\"" + html.EscapeString(url) + "\">" + html.EscapeString(url) + "</a>\n\n")

	b.WriteString("ID: <code>" + html.EscapeString(ch.ChannelID) + "</code>\n")

	topic := ch.Topic
	if topic == "" {
		topic = "не определена"
	}
	b.WriteString("Тема: " + html.EscapeString(topic) + "\n")

	return b.String()
}

// truncateDescription обрезает описание канала до descriptionLimit символов.
func truncateDescription(desc string) string {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return ""
	}
	runes := []rune(desc)
	if len(runes) <= descriptionLimit {
		return desc
	}
	return strings.TrimSpace(string(runes[:descriptionLimit])) + "…"
}
