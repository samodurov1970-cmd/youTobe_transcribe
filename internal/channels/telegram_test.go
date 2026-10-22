package channels

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestFormatSubscriptionMessage проверяет формирование текста уведомления:
// наличие названия, кликабельной ссылки, ID, темы и описания.
func TestFormatSubscriptionMessage(t *testing.T) {
	ch := subscribedChannel{
		ChannelID:   "UCabcdefghijklmnopqrstu",
		ChannelName: "Trading Daily",
		ChannelURL:  "https://www.youtube.com/channel/UCabcdefghijklmnopqrstu",
		Description: "Ежедневный обзор рынков",
		Topic:       TopicTrading,
	}

	msg := formatSubscriptionMessage(ch)

	for _, want := range []string{
		"Вы подписались на:",
		"Trading Daily",
		`<a href="https://www.youtube.com/channel/UCabcdefghijklmnopqrstu">`,
		"UCabcdefghijklmnopqrstu",
		TopicTrading,
		"Ежедневный обзор рынков",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message does not contain %q:\n%s", want, msg)
		}
	}
}

// TestFormatSubscriptionMessageEscapesHTML проверяет экранирование
// пользовательских значений: HTML-разметка не должна ломать сообщение.
func TestFormatSubscriptionMessageEscapesHTML(t *testing.T) {
	ch := subscribedChannel{
		ChannelID:   "UCabcdefghijklmnopqrstu",
		ChannelName: "<b>News & Co</b>",
		ChannelURL:  "https://www.youtube.com/channel/UCabcdefghijklmnopqrstu?a=1&b=2",
		Description: "A & B",
		Topic:       TopicNews,
	}

	msg := formatSubscriptionMessage(ch)

	if strings.Contains(msg, "<b>News") {
		t.Errorf("channel name was not escaped:\n%s", msg)
	}
	if !strings.Contains(msg, "&lt;b&gt;News &amp; Co&lt;/b&gt;") {
		t.Errorf("expected escaped channel name:\n%s", msg)
	}
	if !strings.Contains(msg, "&amp;") {
		t.Errorf("expected ampersand to be escaped:\n%s", msg)
	}
}

// TestFormatSubscriptionMessageEmptyFields проверяет обработку пустых полей.
func TestFormatSubscriptionMessageEmptyFields(t *testing.T) {
	msg := formatSubscriptionMessage(subscribedChannel{ChannelID: "UCabcdefghijklmnopqrstu"})

	if !strings.Contains(msg, "без названия") {
		t.Errorf("expected fallback name:\n%s", msg)
	}
	if !strings.Contains(msg, "не определена") {
		t.Errorf("expected fallback topic:\n%s", msg)
	}
	// Ссылка строится из идентификатора канала.
	if !strings.Contains(msg, "https://www.youtube.com/channel/UCabcdefghijklmnopqrstu") {
		t.Errorf("expected generated url:\n%s", msg)
	}
}

// TestTruncateDescription проверяет обрезку длинного описания.
func TestTruncateDescription(t *testing.T) {
	if got := truncateDescription("   "); got != "" {
		t.Errorf("expected empty result, got %q", got)
	}
	if got := truncateDescription("short"); got != "short" {
		t.Errorf("expected unchanged description, got %q", got)
	}
	long := strings.Repeat("a", descriptionLimit+100)
	got := truncateDescription(long)
	if len([]rune(got)) != descriptionLimit+1 { // +1 — символ многоточия
		t.Errorf("expected truncated description, got %d runes", len([]rune(got)))
	}
}

// TestNotifierDisabled проверяет, что без токена и чата уведомления выключены.
func TestNotifierDisabled(t *testing.T) {
	if newNotifier("", "").enabled() {
		t.Error("expected notifier to be disabled without token and chat id")
	}
	if newNotifier("token", "").enabled() {
		t.Error("expected notifier to be disabled without chat id")
	}
	if newNotifier("", "chat").enabled() {
		t.Error("expected notifier to be disabled without token")
	}
	if !newNotifier("token", "chat").enabled() {
		t.Error("expected notifier to be enabled")
	}
}

// TestNotifierSend проверяет отправку сообщения в Telegram и разбор ответа.
func TestNotifierSend(t *testing.T) {
	var gotPayload map[string]any
	var gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":42}}`))
	}))
	defer srv.Close()

	n := newNotifier("test-token", "12345")
	n.apiBase = srv.URL

	if err := n.send(context.Background(), "тест"); err != nil {
		t.Fatalf("send: %v", err)
	}

	if !strings.Contains(gotPath, "/bottest-token/sendMessage") {
		t.Errorf("unexpected request path %q", gotPath)
	}
	if gotPayload["chat_id"] != "12345" {
		t.Errorf("unexpected chat_id %v", gotPayload["chat_id"])
	}
	if gotPayload["text"] != "тест" {
		t.Errorf("unexpected text %v", gotPayload["text"])
	}
	if gotPayload["parse_mode"] != "HTML" {
		t.Errorf("unexpected parse_mode %v", gotPayload["parse_mode"])
	}
}

// TestNotifierSendAPIError проверяет обработку ошибки Telegram API.
func TestNotifierSendAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ok":false,"description":"chat not found"}`))
	}))
	defer srv.Close()

	n := newNotifier("test-token", "12345")
	n.apiBase = srv.URL

	err := n.send(context.Background(), "тест")
	if err == nil {
		t.Fatal("expected error for failed api response")
	}
	if !strings.Contains(err.Error(), "chat not found") {
		t.Errorf("expected api description in error, got %v", err)
	}
}
