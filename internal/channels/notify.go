package channels

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
)

// subscribedChannel — данные канала для уведомления о новой подписке.
type subscribedChannel struct {
	ChannelID   string
	ChannelName string
	ChannelURL  string
	Description string
	Topic       string
}

// subscribeNotifier определяет новые подписки и отправляет о них уведомления.
//
// Логика: перед циклом обновления снимается список каналов, которые уже есть
// в базе. После записи подписок в базу вычисляется разница — каналы, которых
// раньше не было. Только для них отправляется уведомление в Telegram.
type subscribeNotifier struct {
	db  *sql.DB
	tg  *notifier
	log *slog.Logger
	// disabled отключает отправку уведомлений (например, в тестах).
	disabled bool
}

// newSubscribeNotifier создаёт отправитель уведомлений о новых подписках.
func newSubscribeNotifier(db *sql.DB, tg *notifier, log *slog.Logger) *subscribeNotifier {
	return &subscribeNotifier{db: db, tg: tg, log: log}
}

// knownChannelIDs возвращает снимок состояния каналов на момент начала цикла.
//
// Ключ — идентификатор канала, значение — был ли канал подписан
// (is_subscribed = true) на момент снятия снимка.
//
// Канал считается новой подпиской, если его не было в базе вовсе
// либо он ранее был отписан (is_subscribed = false) и снова появился
// в списке подписок.
func (n *subscribeNotifier) knownChannelIDs(ctx context.Context) (map[string]bool, error) {
	rows, err := n.db.QueryContext(ctx, `
		SELECT channel_id, coalesce(is_subscribed, true) FROM channels
	`)
	if err != nil {
		return nil, fmt.Errorf("select known channels: %w", err)
	}
	defer rows.Close()

	known := map[string]bool{}
	for rows.Next() {
		var id string
		var subscribed bool
		if err := rows.Scan(&id, &subscribed); err != nil {
			return nil, fmt.Errorf("scan known channel: %w", err)
		}
		known[id] = subscribed
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate known channels: %w", err)
	}
	return known, nil
}

// loadChannel загружает данные канала из базы для формирования уведомления.
func (n *subscribeNotifier) loadChannel(ctx context.Context, channelID string) (subscribedChannel, error) {
	var ch subscribedChannel
	err := n.db.QueryRowContext(ctx, `
		SELECT channel_id, coalesce(channel_name, ''), coalesce(channel_url, ''),
		       coalesce(description, ''), coalesce(topic, '')
		FROM channels WHERE channel_id = $1
	`, channelID).Scan(&ch.ChannelID, &ch.ChannelName, &ch.ChannelURL, &ch.Description, &ch.Topic)
	if err != nil {
		return subscribedChannel{}, fmt.Errorf("load channel %s: %w", channelID, err)
	}
	return ch, nil
}

// newSubscriptionIDs возвращает идентификаторы каналов, которые считаются
// новыми подписками.
//
// Канал новый, если:
//   - его не было в снимке known (значение отсутствует) — подписка впервые;
//   - он был в снимке, но помечен как отписанный (known[id] == false) —
//     повторная подписка после отписки.
//
// Канал, который и до этого был подписан (known[id] == true), не новый.
//
// Порядок сохраняется: идентификаторы идут в порядке subs.
func newSubscriptionIDs(known map[string]bool, subs []subscription) []string {
	ids := make([]string, 0, len(subs))
	for _, sub := range subs {
		subscribed, exists := known[sub.ChannelID]
		if exists && subscribed {
			continue
		}
		ids = append(ids, sub.ChannelID)
	}
	return ids
}

// notifyNewSubscriptions отправляет уведомления о каналах, которых не было
// в базе на момент снятия снимка known.
//
// Ошибка отправки не прерывает цикл: уведомление считается необязательным,
// его потеря не влияет на синхронизацию подписок.
func (n *subscribeNotifier) notifyNewSubscriptions(ctx context.Context, known map[string]bool, subs []subscription) {
	if n.disabled || !n.tg.enabled() {
		return
	}

	newSubs := 0
	for _, channelID := range newSubscriptionIDs(known, subs) {
		ch, err := n.loadChannel(ctx, channelID)
		if err != nil {
			n.log.Error("channels.notify_load_failed",
				"channel_id", channelID,
				"error", err.Error(),
			)
			continue
		}

		if err := n.tg.send(ctx, formatSubscriptionMessage(ch)); err != nil {
			n.log.Error("channels.notify_failed",
				"channel_id", ch.ChannelID,
				"channel_name", ch.ChannelName,
				"error", err.Error(),
			)
			continue
		}

		newSubs++
		n.log.Info("channels.notified",
			"channel_id", ch.ChannelID,
			"channel_name", ch.ChannelName,
		)
	}

	if newSubs > 0 {
		n.log.Info("channels.notify_summary", "new_subscriptions", newSubs)
	}
}

// notifyUnsubscriptions отправляет уведомления об отписке от каналов.
func (n *subscribeNotifier) notifyUnsubscriptions(ctx context.Context, gone []string) {
	n.log.Info("channels.notify_unsub_enter", "count", len(gone), "ctx_err", ctx.Err())
	if n.disabled || !n.tg.enabled() {
		n.log.Warn("channels.notify_unsub_skipped", "reason", "disabled_or_not_configured")
		return
	}

	sent := 0
	for _, channelID := range gone {
		ch, err := n.loadChannel(ctx, channelID)
		if err != nil {
			n.log.Error("channels.notify_unsub_load_failed",
				"channel_id", channelID,
				"error", err.Error(),
			)
			continue
		}

		if err := n.tg.send(ctx, formatUnsubscriptionMessage(ch)); err != nil {
			n.log.Error("channels.notify_unsub_failed",
				"channel_id", ch.ChannelID,
				"channel_name", ch.ChannelName,
				"error", err.Error(),
			)
			continue
		}

		sent++
		n.log.Info("channels.notified_unsub",
			"channel_id", ch.ChannelID,
			"channel_name", ch.ChannelName,
		)
	}

	if sent > 0 {
		n.log.Info("channels.notify_unsub_summary", "unsubscriptions", sent)
	}
}
