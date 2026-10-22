// Package logger предоставляет JSON-логгер на базе slog.
package logger

import (
	"log/slog"
	"os"
	"strings"
)

// Имена микросервисов для поля service_name в логах.
const (
	ServiceNameTranscribe = "youtube_transcribe"
)

// Log — глобальный логгер приложения.
var Log *slog.Logger

// GetEnv возвращает значение переменной окружения или значение по умолчанию.
func GetEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Init инициализирует глобальный JSON-логгер.
func Init() {
	Log = New(ServiceNameTranscribe)
}

// New создает JSON-логгер с указанным именем сервиса.
func New(serviceName string) *slog.Logger {
	level := slog.LevelInfo
	if err := level.UnmarshalText([]byte(GetEnv("LOG_LEVEL", "info"))); err != nil {
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 && a.Key == "source" {
				a.Key = "caller"
				if v, ok := a.Value.Any().(string); ok {
					a.Value = slog.StringValue(shortenSource(v))
				}
			}
			return a
		},
	}
	handler := slog.NewJSONHandler(os.Stdout, opts)
	return slog.New(handler).With(
		slog.String("service_name", serviceName),
		slog.String("environment", GetEnv("APP_ENV", "production")),
	)
}

// shortenSource укорачивает путь файла до вида "internal/pkg/file.go:NN".
func shortenSource(src string) string {
	idx := strings.Index(src, "youTobe_transcribe/")
	if idx >= 0 {
		src = src[idx+len("youTobe_transcribe/"):]
	}
	return src
}

// Err возвращает slog.Attr с описанием ошибки (или пустой атрибут).
func Err(err error) slog.Attr {
	if err != nil {
		return slog.Attr{Key: "error_message", Value: slog.StringValue(err.Error())}
	}
	return slog.Attr{}
}
