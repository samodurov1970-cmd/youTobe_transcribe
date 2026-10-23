// Package storage обеспечивает работу с PostgreSQL.
package storage

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"youTobe_transcribe/internal/logger"

	_ "github.com/lib/pq"
)

// DB — глобальное подключение к базе данных.
var DB *sql.DB

// log возвращает логгер подсистемы хранения (component = storage).
func log() *slog.Logger {
	return logger.Log.With("component", "storage")
}

// VideoFile — строка таблицы video_files.
type VideoFile struct {
	ID           int
	VideoID      string
	ChannelName  string
	VideoURL     string
	FileName     string
	FileContent  string
	Status       string
	IsProcessed  bool
	ChannelTopic string
	LastError    string // <-- ДОБАВЛЕНО
}

// Init открывает подключение к PostgreSQL и проверяет его.
func Init(databaseURL string) error {
	start := time.Now()
	log().Info("db.opening", "max_open_conns", 5, "max_idle_conns", 2, "conn_max_lifetime_min", 5)
	var err error
	DB, err = sql.Open("postgres", databaseURL)
	if err != nil {
		log().Error("db.open_failed", logger.Err(err))
		return fmt.Errorf("open db: %w", err)
	}
	DB.SetMaxOpenConns(5)
	DB.SetMaxIdleConns(2)
	DB.SetConnMaxLifetime(5 * time.Minute)
	if err := DB.Ping(); err != nil {
		log().Error("db.ping_failed", logger.Err(err), "duration_ms", time.Since(start).Milliseconds())
		return fmt.Errorf("ping db: %w", err)
	}
	stats := DB.Stats()
	log().Info("db.connected",
		"duration_ms", time.Since(start).Milliseconds(),
		"max_open", stats.MaxOpenConnections,
		"open_conns", stats.OpenConnections,
		"idle_conns", stats.Idle,
	)
	return nil
}

// Close закрывает подключение к базе данных.
func Close() {
	if DB != nil {
		if err := DB.Close(); err != nil {
			log().Error("db.close_failed", logger.Err(err))
		}
	}
}