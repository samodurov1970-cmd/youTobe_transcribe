// Package channels реализует подсистему обновления списка YouTube-подписок.
//
// Сервис читает подписки через yt-dlp, классифицирует каждый канал с помощью
// LLM и по ключевым словам.
// Обновление выполняется сразу при старте и далее раз в 1 час.
// Подсистема работает в отдельной горутине основной процесса
// youtube_transcribe и использует общее подключение к базе данных.
//
// При появлении новой подписки отправляется уведомление в Telegram.
// При появлении новой отписки отправляется уведомление в Telegram.
package channels

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"youTobe_transcribe/internal/logger"

	"github.com/lib/pq"
)

// Component — имя подсистемы для атрибута component в логах.
const Component = "channels"

// UpdateInterval — период обновления списка подписок по умолчанию (раз в сутки).
// Фактический интервал задаётся через Config.UpdateInterval (env CHANNELS_UPDATE_INTERVAL).
const UpdateInterval = 24 * time.Hour

// subscriptionsURL — страница YouTube со списком подписок пользователя.
const subscriptionsURL = "https://www.youtube.com/feed/channels"

// Config — настройки подсистемы channels.
type Config struct {
	DB          *sql.DB
	YtDlpPath   string
	CookiesFile string
	// UpdateInterval — период между обновлениями списка подписок.
	// Если не задан, используется UpdateInterval по умолчанию.
	UpdateInterval time.Duration
	// TelegramToken — токен Telegram-бота для уведомлений о новых подписках.
	// Пустое значение отключает уведомления.
	TelegramToken string
	// TelegramChatID — идентификатор чата Telegram для уведомлений.
	// Пустое значение отключает уведомления.
	TelegramChatID string
	// OLLAMAURL — базовый адрес Ollama API для классификации тем каналов.
	OLLAMAURL string
	// OLLAMAModel — модель Ollama для классификации тем каналов.
	OLLAMAModel string
	// OLLAMATimeout — таймаут запроса к Ollama.
	OLLAMATimeout time.Duration
	// OLLAMAVideoTitlesLimit — сколько последних названий видео запрашивать у yt-dlp.
	OLLAMAVideoTitlesLimit int
}

// Service — подсистема обновления подписок.
type Service struct {
	db       *sql.DB
	cfg      *Config
	llm      *LLMClient
	notifier *subscribeNotifier
	log      *slog.Logger
}

// NewService создаёт сервис обновления подписок.
// Логи наследуют базовый логгер приложения и помечаются полем component.
func NewService(cfg *Config) *Service {
	if cfg.UpdateInterval <= 0 {
		cfg.UpdateInterval = UpdateInterval
	}
	log := logger.Log.With("component", Component)
	tg := newNotifier(cfg.TelegramToken, cfg.TelegramChatID)
	return &Service{
		db:       cfg.DB,
		cfg:      cfg,
		llm:      channelsLLMClient(cfg),
		notifier: newSubscribeNotifier(cfg.DB, tg, log),
		log:      log,
	}
}

// Run выполняет обновление подписок сразу при старте, затем
// периодически раз в UpdateInterval.
func (s *Service) Run(ctx context.Context) {
	s.processOnce(ctx)

	ticker := time.NewTicker(s.cfg.UpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.log.Info("channels.stopped")
			return
		case <-ticker.C:
			s.processOnce(ctx)
		}
	}
}

// processOnce один раз выполняет полный цикл обновления подписок:
//   - снимает список каналов из базы (для определения новых подписок);
//   - получает список подписок YouTube через yt-dlp;
//   - классифицирует новые каналы с помощью LLM и ключевых слов;
//   - сохраняет подписки в базу;
//   - отправляет уведомления о новых подписках и отписках в Telegram.
func (s *Service) processOnce(ctx context.Context) {
	s.log.Info("channels.update_started")

	// Снимок известных каналов до изменений — нужен для определения новых подписок.
	known, err := s.notifier.knownChannelIDs(ctx)
	if err != nil {
		s.log.Error("channels.known_channels_failed", "error", err.Error())
		// Продолжаем — это не критичная ошибка, просто не отправим уведомления о новых.
		known = map[string]bool{}
	}

	subs, err := s.fetchSubscriptions(ctx)
	if err != nil {
		s.log.Error("channels.fetch_failed", "error", err.Error())
		return
	}

	s.log.Info("channels.fetched", "count", len(subs))
	if len(subs) == 0 {
		return
	}

	if err := s.saveSubscriptions(ctx, subs, known); err != nil {
		s.log.Error("channels.save_failed", "error", err.Error())
		return
	}

	// Уведомления о новых подписках в Telegram.
	s.notifier.notifyNewSubscriptions(ctx, known, subs)

	// Уведомления об отписках в Telegram.
	_, gone, err := s.markUnsubscribed(ctx, subs)
	if err != nil {
		s.log.Error("channels.unsubscribe_failed", "error", err.Error())
		return
	}
	if len(gone) > 0 {
		s.notifier.notifyUnsubscriptions(ctx, gone)
	}
}

// saveSubscriptions сохраняет список подписок в базу данных.
// Классификация тем (тема канала) выполняется для новых каналов и
// каналов без темы.
func (s *Service) saveSubscriptions(ctx context.Context, subs []subscription, known map[string]bool) error {
	tx, err := s.cfg.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	for _, sub := range subs {
		// Если канал уже подписан, пропускаем (не перезаписываем тему).
		exists, subscribed, err := channelExistsAndSubscribed(ctx, tx, sub.ChannelID)
		if err != nil {
			return fmt.Errorf("check channel %s: %w", sub.ChannelID, err)
		}
		if exists && subscribed {
			continue
		}

		// Нужна классификация темы.
		topic := ""
		if exists && !subscribed {
			// Повторная подписка: используем ранее определённую тему, если есть.
			topic, err = getChannelTopic(ctx, tx, sub.ChannelID)
			if err != nil {
				return fmt.Errorf("get topic for %s: %w", sub.ChannelID, err)
			}
			if topic == "" {
				topic, _ = classifyChannelWithLLM(s.llm, sub.ChannelID, sub.Title, sub.Description, nil)
			}
		} else {
			// Новый канал: получаем темы видео для LLM, если доступен Ollama.
			var titles []string
			if s.llm != nil {
				titles = s.fetchChannelVideoTitles(ctx, sub.ChannelID, sub.URL)
			}
			topic, _ = classifyChannelWithLLM(s.llm, sub.ChannelID, sub.Title, sub.Description, titles)
		}

		if err := upsertChannel(ctx, tx, sub, topic, true); err != nil {
			return fmt.Errorf("upsert channel %s: %w", sub.ChannelID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit save tx: %w", err)
	}

	return nil
}

// subscription — одна запись из списка YouTube подписок.
type subscription struct {
	ChannelID   string
	Title       string
	URL         string
	Description string
}

// channelsLLMClient создаёт клиент Ollama из конфигурации.
// Возвращает nil, если URL не задан (LLM отключён).
func channelsLLMClient(cfg *Config) *LLMClient {
	if cfg.OLLAMAURL == "" {
		return nil
	}
	return NewLLMClient(cfg.OLLAMAURL, cfg.OLLAMAModel, cfg.OLLAMATimeout)
}

// channelExistsAndSubscribed проверяет, существует ли канал в базе и подписан ли он.
func channelExistsAndSubscribed(ctx context.Context, tx *sql.Tx, channelID string) (bool, bool, error) {
	var exists, subscribed bool
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM channels WHERE channel_id = $1),
		       coalesce((SELECT is_subscribed FROM channels WHERE channel_id = $1), true)
	`, channelID).Scan(&exists, &subscribed)
	return exists, subscribed, err
}

// getChannelTopic получает тему канала из базы.
func getChannelTopic(ctx context.Context, tx *sql.Tx, channelID string) (string, error) {
	var topic string
	err := tx.QueryRowContext(ctx,
		`SELECT coalesce(topic, '') FROM channels WHERE channel_id = $1`,
		channelID,
	).Scan(&topic)
	return topic, err
}

// upsertChannel вставляет или обновляет запись канала в базе.
func upsertChannel(ctx context.Context, tx *sql.Tx, sub subscription, topic string, subscribed bool) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO channels (channel_id, channel_name, channel_url, description, topic, is_subscribed, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (channel_id) DO UPDATE SET
			channel_name = EXCLUDED.channel_name,
			channel_url = EXCLUDED.channel_url,
			description = EXCLUDED.description,
			topic = CASE WHEN channels.topic IS NULL OR channels.topic = '' THEN EXCLUDED.topic ELSE channels.topic END,
			is_subscribed = $6,
			last_seen_at = NOW()
	`, sub.ChannelID, sub.Title, sub.URL, sub.Description, topic, subscribed)
	return err
}

// fetchSubscriptions получает список подписок пользователя через yt-dlp.
func (s *Service) fetchSubscriptions(ctx context.Context) ([]subscription, error) {
	args := []string{
		"--flat-playlist",
		"--no-warnings",
		"--dump-single-json",
		"--no-playlist-random",
		subscriptionsURL,
	}

	if s.cfg.CookiesFile != "" {
		args = append(args, "--cookies", s.cfg.CookiesFile)
	}

	cmd := exec.CommandContext(ctx, s.cfg.YtDlpPath, args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("yt-dlp failed: %s: %w", strings.TrimSpace(string(exitErr.Stderr)), err)
		}
		return nil, fmt.Errorf("yt-dlp failed: %w", err)
	}

	return parseSubscriptions(out)
}

// parseSubscriptions разбирает JSON-вывод yt-dlp со списком подписок.
func parseSubscriptions(raw []byte) ([]subscription, error) {
	var payload struct {
		Entries []struct {
			ID          string `json:"id"`
			ChannelID   string `json:"channel_id"`
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("parse yt-dlp json: %w", err)
	}

	subs := make([]subscription, 0, len(payload.Entries))
	seen := make(map[string]struct{}, len(payload.Entries))
	for _, e := range payload.Entries {
		channelID := e.ChannelID
		if channelID == "" {
			channelID = e.ID
		}
		if !validChannelID(channelID) {
			continue
		}
		if _, ok := seen[channelID]; ok {
			continue
		}
		seen[channelID] = struct{}{}

		url := e.URL
		if url == "" {
			url = "https://www.youtube.com/channel/" + channelID
		}

		subs = append(subs, subscription{
			ChannelID:   channelID,
			Title:       e.Title,
			URL:         url,
			Description: e.Description,
		})
	}
	return subs, nil
}

// validChannelID проверяет формат идентификатора канала YouTube (UC...).
func validChannelID(id string) bool {
	return strings.HasPrefix(id, "UC") && len(id) > 10
}

// fetchChannelVideoTitles получает названия последних N видео канала через yt-dlp.
// Используется для LLM-классификации темы канала.
// Возвращает пустой срез при любой ошибке — LLM просто получит только имя и описание.
func (s *Service) fetchChannelVideoTitles(ctx context.Context, channelID, channelURL string) []string {
	limit := s.cfg.OLLAMAVideoTitlesLimit
	if limit <= 0 {
		limit = 10
	}

	// Используем URL канала или формируем его из ID.
	playlistURL := channelURL
	if playlistURL == "" {
		playlistURL = "https://www.youtube.com/channel/" + channelID
	}

	// Добавляем параметры для получения только заголовков и ограничения количества.
	parsedURL, err := url.Parse(playlistURL)
	if err == nil {
		playlistURL = parsedURL.String()
	}

	args := []string{
		"--flat-playlist",
		"--no-warnings",
		fmt.Sprintf("--playlist-end=%d", limit),
		"--dump-single-json",
		"--no-playlist-random",
	}

	if s.cfg.CookiesFile != "" {
		args = append(args, "--cookies", s.cfg.CookiesFile)
	}

	args = append(args, playlistURL)

	cmd := exec.CommandContext(ctx, s.cfg.YtDlpPath, args...)
	out, err := cmd.Output()
	if err != nil {
		s.log.Error("channels.fetch_video_titles_failed",
			"channel_id", channelID,
			"error", err.Error(),
		)
		return nil
	}

	// Парсим JSON-ответ yt-dlp для извлечения заголовков видео.
	var payload struct {
		Entries []struct {
			Title string `json:"title"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		s.log.Error("channels.parse_video_titles_failed",
			"channel_id", channelID,
			"error", err.Error(),
		)
		return nil
	}

	titles := make([]string, 0, len(payload.Entries))
	for _, entry := range payload.Entries {
		if entry.Title != "" {
			titles = append(titles, entry.Title)
		}
	}

	s.log.Info("channels.video_titles_fetched",
		"channel_id", channelID,
		"count", len(titles),
	)

	return titles
}

// markUnsubscribed помечает каналы, которые больше нет в списке подписок,
// как отписанные. Возвращает список ID отписавшихся каналов для уведомлений.
//
// Защита от массовой отметки: если число активных каналов в базе заметно
// больше числа полученных подписок, отписки не обрабатываются (чтобы
// пустой или обрезанный ответ yt-dlp не удалил все каналы).
func (s *Service) markUnsubscribed(ctx context.Context, subs []subscription) (int, []string, error) {
	if len(subs) == 0 {
		s.log.Warn("channels.unsubscribe_skipped", "reason", "empty_subscription_list")
		return 0, nil, nil
	}

	ids := make([]string, 0, len(subs))
	for _, sub := range subs {
		ids = append(ids, sub.ChannelID)
	}

	// Считаем активные каналы в базе, чтобы применить защиту.
	var activeCount int
	if err := s.cfg.DB.QueryRowContext(ctx,
		`SELECT count(*) FROM channels WHERE coalesce(is_subscribed, true) = true`,
	).Scan(&activeCount); err != nil {
		return 0, nil, fmt.Errorf("count active channels: %w", err)
	}

	if activeCount > 2*len(subs) {
		s.log.Warn("channels.unsubscribe_skipped",
			"reason", "subscription_list_too_short",
			"active_channels", activeCount,
			"received", len(subs),
		)
		return 0, nil, nil
	}

	tx, err := s.cfg.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Каналы, которые снова в списке подписок.
	resub, err := tx.ExecContext(ctx, `
		UPDATE channels SET is_subscribed = true, unsubscribed_at = NULL, last_seen_at = NOW()
		WHERE channel_id = ANY($1) AND coalesce(is_subscribed, true) = false
	`, pq.Array(ids))
	if err != nil {
		return 0, nil, fmt.Errorf("resubscribe: %w", err)
	}
	resubCount, _ := resub.RowsAffected()

	// Каналы, которые больше нет в списке подписок.
	rows, err := tx.QueryContext(ctx, `
		UPDATE channels SET is_subscribed = false, unsubscribed_at = NOW()
		WHERE coalesce(is_subscribed, true) = true
		  AND NOT (channel_id = ANY($1))
		RETURNING channel_id
	`, pq.Array(ids))
	if err != nil {
		return 0, nil, fmt.Errorf("mark unsubscribed: %w", err)
	}
	var gone []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, nil, fmt.Errorf("scan unsubscribed: %w", err)
		}
		gone = append(gone, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, nil, fmt.Errorf("rows err: %w", err)
	}

	// Удаляем видео отписавшихся каналов и пишем журнал событий.
	if len(gone) > 0 {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM videos WHERE channel_id = ANY($1)`, pq.Array(gone)); err != nil {
			return 0, nil, fmt.Errorf("delete videos: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM videos_new WHERE channel_id = ANY($1)`, pq.Array(gone)); err != nil {
			return 0, nil, fmt.Errorf("delete videos_new: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO channels_unsubscribed (channel_id, channel_name, topic, videos_deleted, videos_new_deleted, event)
			SELECT channel_id, channel_name, topic, 0, 0, 'unsub'
			FROM channels WHERE channel_id = ANY($1)
			`, pq.Array(gone)); err != nil {
			return 0, nil, fmt.Errorf("log unsubscribe: %w", err)
		}
	}

	if resubCount > 0 {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO channels_unsubscribed (channel_id, channel_name, topic, videos_deleted, videos_new_deleted, event)
			SELECT channel_id, channel_name, topic, 0, 0, 'resub'
			FROM channels WHERE channel_id = ANY($1) AND coalesce(is_subscribed, true) = true
			`, pq.Array(ids)); err != nil {
			return 0, nil, fmt.Errorf("log resubscribe: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, nil, fmt.Errorf("commit tx: %w", err)
	}

	if len(gone) > 0 {
		s.log.Info("channels.unsubscribed", "count", len(gone))
	}
	if resubCount > 0 {
		s.log.Info("channels.resubscribed", "count", resubCount)
	}

	return len(gone), gone, nil
}
