//go:build integration

// Этот файл содержит интеграционные тесты, которые отправляют РЕАЛЬНЫЕ
// сообщения в Telegram. Запускать только на локальной машине и только
// если переменные окружения TELEGRAM_BOT_TOKEN и TELEGRAM_CHAT_ID
// установлены в реальные значения.
//
// Запуск:
//
//	set TELEGRAM_BOT_TOKEN=...
//	set TELEGRAM_CHAT_ID=...
//	go test -tags=integration -run TestTelegram ./internal/channels/
package channels

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
)

// TestTelegramSendRealNotification отправляет реальное сообщение в
// ваш Telegram-чат и проверяет, что Telegram API не вернул ошибку.
func TestTelegramSendRealNotification(t *testing.T) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")
	if token == "" || chatID == "" {
		t.Skip("TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID are required for integration tests")
	}

	n := newNotifier(token, chatID)
	if !n.enabled() {
		t.Fatal("notifier is not enabled despite env vars being set")
	}

	ch := subscribedChannel{
		ChannelID:   "UC_TEST_REAL_NOTIFIER",
		ChannelName: "🧪 Тестовое уведомление о подписке",
		ChannelURL:  "https://www.youtube.com/channel/UC_TEST_REAL_NOTIFIER",
		Description: "Это тестовое сообщение из интеграционного теста Go. " +
			"Оно должно прийти в ваш Telegram чат.",
		Topic: "Тестовая тема",
	}

	msg := formatSubscriptionMessage(ch)
	t.Logf("formatted message:\n%s", msg)

	if err := n.send(context.Background(), msg); err != nil {
		t.Fatalf("send to real Telegram failed: %v", err)
	}

	t.Log("✅ message sent successfully to Telegram chat")
}

// TestTelegramSendRealUnsubscribeNotification отправляет реальное
// сообщение об отписке в ваш Telegram-чат.
func TestTelegramSendRealUnsubscribeNotification(t *testing.T) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")
	if token == "" || chatID == "" {
		t.Skip("TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID are required for integration tests")
	}

	n := newNotifier(token, chatID)
	if !n.enabled() {
		t.Fatal("notifier is not enabled despite env vars being set")
	}

	ch := subscribedChannel{
		ChannelID:   "UC_TEST_UNSUB_12345678",
		ChannelName: "🧪 Тестовое уведомление об отписке",
		ChannelURL:  "https://www.youtube.com/channel/UC_TEST_UNSUB_12345678",
		Topic:       "Тестовая тема отписки",
	}

	msg := formatUnsubscriptionMessage(ch)
	t.Logf("formatted message:\n%s", msg)

	if err := n.send(context.Background(), msg); err != nil {
		t.Fatalf("send to real Telegram failed: %v", err)
	}

	t.Log("✅ unsubscribe notification sent successfully to Telegram chat")
}

// TestTelegramNotifierViaSubscribeNotifier отправляет реальное уведомление
// через subscribeNotifier, эмулируя сценарий новой подписки в системе.
func TestTelegramNotifierViaSubscribeNotifier(t *testing.T) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")
	if token == "" || chatID == "" {
		t.Skip("TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID are required for integration tests")
	}

	n := newNotifier(token, chatID)
	if !n.enabled() {
		t.Fatal("notifier is not enabled despite env vars being set")
	}

	sn := &subscribeNotifier{
		tg:       n,
		disabled: false,
		log:      slog.Default(),
	}

	// subscribeNotifier.notifyNewSubscriptions ожидает []subscription,
	// но чтобы не зависеть от БД, мы вызовем tg.send напрямую с
	// отформатированным сообщением — как это делает notifyNewSubscriptions.
	ch := subscribedChannel{
		ChannelID:   "UC_REAL_TEST_NEW_SUB",
		ChannelName: "🚀 Новая подписка — реальный тест",
		ChannelURL:  "https://www.youtube.com/channel/UC_REAL_TEST_NEW_SUB",
		Description: "Это сообщение отправлено через subscribeNotifier. " +
			"Подписка UC_REAL_TEST_NEW_SUB только что появилась.",
		Topic: "Тестовая тема через subscribeNotifier",
	}

	msg := formatSubscriptionMessage(ch)
	t.Logf("formatted message:\n%s", msg)

	if err := sn.tg.send(context.Background(), msg); err != nil {
		t.Fatalf("subscribeNotifier.send failed: %v", err)
	}

	t.Logf("✅ subscribeNotifier message sent to Telegram (chat_id=%s)", chatID)
}

// TestTelegramUnsubscribeViaSubscribeNotifier отправляет реальное
// уведомление об отписке через subscribeNotifier.
func TestTelegramUnsubscribeViaSubscribeNotifier(t *testing.T) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")
	if token == "" || chatID == "" {
		t.Skip("TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID are required for integration tests")
	}

	n := newNotifier(token, chatID)

	sn := &subscribeNotifier{
		tg:       n,
		disabled: false,
		log:      slog.Default(),
	}

	ch := subscribedChannel{
		ChannelID:   "UC_REAL_TEST_UNSUB",
		ChannelName: "🔥 Отписка — реальный тест",
		ChannelURL:  "https://www.youtube.com/channel/UC_REAL_TEST_UNSUB",
		Topic:       "Тестовая тема отписки через subscribeNotifier",
	}

	msg := formatUnsubscriptionMessage(ch)
	t.Logf("formatted message:\n%s", msg)

	if err := sn.tg.send(context.Background(), msg); err != nil {
		t.Fatalf("subscribeNotifier.send unsubscribe failed: %v", err)
	}

	t.Logf("✅ unsubscribe notification sent to Telegram (chat_id=%s)", chatID)
}

// TestTelegramDisabledWithEmptyEnv проверяет, что notifier корректно
// отключается, если переменные окружения пусты (например, в CI без Telegram).
func TestTelegramDisabledWithEmptyEnv(t *testing.T) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")

	os.Unsetenv("TELEGRAM_BOT_TOKEN")
	os.Unsetenv("TELEGRAM_CHAT_ID")
	t.Cleanup(func() {
		os.Setenv("TELEGRAM_BOT_TOKEN", token)
		os.Setenv("TELEGRAM_CHAT_ID", chatID)
	})

	n := newNotifier("", "")
	if n.enabled() {
		t.Error("notifier should be disabled with empty token/chat")
	}

	if err := n.send(context.Background(), "test"); err == nil {
		t.Error("expected error when notifier is disabled")
	} else if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured' error, got: %v", err)
	}
}