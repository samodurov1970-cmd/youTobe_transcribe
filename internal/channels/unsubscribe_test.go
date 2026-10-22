package channels

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestFormatUnsubscriptionMessage проверяет формирование текста уведомления об отписке:
// наличие названия, кликабельной ссылки, ID и темы.
func TestFormatUnsubscriptionMessage(t *testing.T) {
	ch := subscribedChannel{
		ChannelID:   "UCabcdefghijklmnopqrstu",
		ChannelName: "Trading Daily",
		ChannelURL:  "https://www.youtube.com/channel/UCabcdefghijklmnopqrstu",
		Topic:       TopicTrading,
	}

	msg := formatUnsubscriptionMessage(ch)

	for _, want := range []string{
		"Вы отписались от:",
		"Trading Daily",
		`<a href="https://www.youtube.com/channel/UCabcdefghijklmnopqrstu">`,
		"UCabcdefghijklmnopqrstu",
		TopicTrading,
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message does not contain %q:\n%s", want, msg)
		}
	}
}

// TestFormatUnsubscriptionMessageEscapesHTML проверяет экранирование
// пользовательских значений при формировании сообщения об отписке.
func TestFormatUnsubscriptionMessageEscapesHTML(t *testing.T) {
	ch := subscribedChannel{
		ChannelID:   "UCabcdefghijklmnopqrstu",
		ChannelName: "<b>News & Co</b>",
		ChannelURL:  "https://www.youtube.com/channel/UCabcdefghijklmnopqrstu?a=1&b=2",
		Topic:       TopicNews,
	}

	msg := formatUnsubscriptionMessage(ch)

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

// TestFormatUnsubscriptionMessageEmptyFields проверяет обработку пустых полей.
func TestFormatUnsubscriptionMessageEmptyFields(t *testing.T) {
	msg := formatUnsubscriptionMessage(subscribedChannel{ChannelID: "UCabcdefghijklmnopqrstu"})

	if !strings.Contains(msg, "без названия") {
		t.Errorf("expected fallback name:\n%s", msg)
	}
	if !strings.Contains(msg, "не определена") {
		t.Errorf("expected fallback topic:\n%s", msg)
	}
	if !strings.Contains(msg, "https://www.youtube.com/channel/UCabcdefghijklmnopqrstu") {
		t.Errorf("expected generated url:\n%s", msg)
	}
}

// TestNotifierSendSubscriptionMessage проверяет отправку сообщения о новой подписке
// в Telegram через тестовый HTTP-сервер.
func TestNotifierSendSubscriptionMessage(t *testing.T) {
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

	ch := subscribedChannel{
		ChannelID:   "UCabcdefghijklmnopqrstu",
		ChannelName: "Trading Daily",
		ChannelURL:  "https://www.youtube.com/channel/UCabcdefghijklmnopqrstu",
		Topic:       TopicTrading,
	}
	msg := formatSubscriptionMessage(ch)

	if err := n.send(context.Background(), msg); err != nil {
		t.Fatalf("send: %v", err)
	}

	if !strings.Contains(gotPath, "/bottest-token/sendMessage") {
		t.Errorf("unexpected request path %q", gotPath)
	}
	if gotPayload["chat_id"] != "12345" {
		t.Errorf("unexpected chat_id %v", gotPayload["chat_id"])
	}
	if gotPayload["text"] != msg {
		t.Errorf("unexpected text %v", gotPayload["text"])
	}
	if gotPayload["parse_mode"] != "HTML" {
		t.Errorf("unexpected parse_mode %v", gotPayload["parse_mode"])
	}
}

// TestNotifierSendUnsubscriptionMessage проверяет отправку сообщения об отписке
// в Telegram через тестовый HTTP-сервер.
func TestNotifierSendUnsubscriptionMessage(t *testing.T) {
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

	ch := subscribedChannel{
		ChannelID:   "UCabcdefghijklmnopqrstu",
		ChannelName: "Trading Daily",
		ChannelURL:  "https://www.youtube.com/channel/UCabcdefghijklmnopqrstu",
		Topic:       TopicTrading,
	}
	msg := formatUnsubscriptionMessage(ch)

	if err := n.send(context.Background(), msg); err != nil {
		t.Fatalf("send: %v", err)
	}

	if !strings.Contains(gotPath, "/bottest-token/sendMessage") {
		t.Errorf("unexpected request path %q", gotPath)
	}
	if gotPayload["chat_id"] != "12345" {
		t.Errorf("unexpected chat_id %v", gotPayload["chat_id"])
	}
	if gotPayload["text"] != msg {
		t.Errorf("unexpected text %v", gotPayload["text"])
	}
	if gotPayload["parse_mode"] != "HTML" {
		t.Errorf("unexpected parse_mode %v", gotPayload["parse_mode"])
	}
}

// TestNotifyUnsubscriptionsDisabled проверяет, что при отключённом уведомлении
// (disabled=true) не происходит отправки.
func TestNotifyUnsubscriptionsDisabled(t *testing.T) {
	var gotRequest bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequest = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	n := newNotifier("test-token", "12345")
	n.apiBase = srv.URL

	sn := &subscribeNotifier{
		tg:       n,
		disabled: true,
		log:      slog.Default(),
	}

	sn.notifyUnsubscriptions(context.Background(), []string{"UCabcdefghijklmnopqrstu"})

	if gotRequest {
		t.Error("expected no HTTP request when disabled")
	}
}

// TestNotifyNewSubscriptionsDisabled проверяет, что при отключённом уведомлении
// (disabled=true) не происходит отправки.
func TestNotifyNewSubscriptionsDisabled(t *testing.T) {
	var gotRequest bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequest = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	n := newNotifier("test-token", "12345")
	n.apiBase = srv.URL

	sn := &subscribeNotifier{
		tg:       n,
		disabled: true,
		log:      slog.Default(),
	}

	sn.notifyNewSubscriptions(context.Background(),
		map[string]bool{"UCabcdefghijklmnopqrstu": false},
		[]subscription{{ChannelID: "UCabcdefghijklmnopqrstu", Title: "Test Channel"}},
	)

	if gotRequest {
		t.Error("expected no HTTP request when disabled")
	}
}
