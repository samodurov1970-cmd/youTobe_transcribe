// Package config загружает конфигурацию приложения из переменных окружения.
package config

import (
	"os"
	"strconv"
	"time"
)

// Config содержит все настройки приложения.
type Config struct {
	DatabaseURL      string
	ObsidianPath     string
	YtDlpPath        string
	CookiesFile      string
	OpenRouterAPIKey string
	OpenRouterModel  string
	TranscriptionURL string
	DeepgramAPIKey   string
	// ChannelsUpdateInterval — период обновления списка подписок channels.
	ChannelsUpdateInterval time.Duration
	// TranscribeSessionDuration — время работы сессии транскрибации (таймаут).
	TranscribeSessionDuration time.Duration
	// TelegramBotToken — токен Telegram-бота для уведомлений о новых подписках.
	TelegramBotToken string
	// TelegramChatID — идентификатор чата Telegram для уведомлений.
	TelegramChatID string
	// OLLAMAURL — базовый адрес Ollama API для классификации тем каналов.
	OLLAMAURL string
	// OLLAMAModel — модель Ollama для классификации тем каналов.
	OLLAMAModel string
	// OLLAMATimeout — таймаут запроса к Ollama.
	OLLAMATimeout time.Duration
	// OLLAMAVideoTitlesLimit — сколько последних названий видео запрашивать у yt-dlp для классификации.
	OLLAMAVideoTitlesLimit int
	// Classifier settings
	ClassifierInterval  time.Duration
	ClassifierBatchSize int
	SummaryChunkSize    int
	// Publisher settings
	PublisherInterval  time.Duration
	PublisherBatchSize int
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// getEnvInt читает целое число из переменной окружения.
func getEnvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	if n, err := strconv.Atoi(v); err == nil && n > 0 {
		return n
	}
	return def
}

// getEnvDuration читает интервал из переменной окружения.
func getEnvDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	if d, err := time.ParseDuration(v); err == nil && d > 0 {
		return d
	}
	if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	return def
}

// Load загружает конфигурацию из переменных окружения.
func Load() Config {
	return Config{
		DatabaseURL:            getEnv("DATABASE_URL", "postgres://peter:peter@localhost:5432/youtube_tracker?sslmode=disable"),
		ObsidianPath:           getEnv("OBSIDIAN_PATH", "/obsidian"),
		YtDlpPath:              getEnv("YT_DLP_PATH", "/usr/bin/yt-dlp"),
		CookiesFile:            getEnv("COOKIES_FILE", "/cookies/cookies.txt"),
		OpenRouterAPIKey:       getEnv("OPENROUTER_API_KEY", ""),
		OpenRouterModel:        getEnv("OPENROUTER_MODEL", "nvidia/nemotron-3-ultra-550b-a55b:free"),
		TranscriptionURL:       getEnv("TRANSCRIPTION_URL", "https://groq-whisper-proxy.matsgraaf17.workers.dev"),
		DeepgramAPIKey:         getEnv("DEEPGRAM_API_KEY", ""),
		ChannelsUpdateInterval: getEnvDuration("CHANNELS_UPDATE_INTERVAL", 24*time.Hour),
		TranscribeSessionDuration: getEnvDuration("TRANSCRIBE_SESSION_HOURS", 2*time.Hour),
		TelegramBotToken:       getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramChatID:         getEnv("TELEGRAM_CHAT_ID", ""),
		OLLAMAURL:              getEnv("OLLAMA_URL", "http://ollama:11434"),
		OLLAMAModel:            getEnv("OLLAMA_MODEL", "qwen2.5:1.5b"),
		OLLAMATimeout:          getEnvDuration("OLLAMA_TIMEOUT", 15*time.Second),
		OLLAMAVideoTitlesLimit: getEnvInt("OLLAMA_VIDEO_TITLES_LIMIT", 10),
		ClassifierInterval:     getEnvDuration("CLASSIFIER_INTERVAL", 2*time.Minute),
		ClassifierBatchSize:    getEnvInt("CLASSIFIER_BATCH_SIZE", 1),
		SummaryChunkSize:       getEnvInt("SUMMARY_CHUNK_SIZE", 20000),
		PublisherInterval:      getEnvDuration("PUBLISHER_INTERVAL", 1*time.Minute),
		PublisherBatchSize:     getEnvInt("PUBLISHER_BATCH_SIZE", 1),
	}
}

// MaskSecret маскирует секрет для безопасного логирования.
func MaskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}