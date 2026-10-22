// Package obsidian записывает транскрипты в хранилище Obsidian.
package obsidian

import (
	"os"
	"path/filepath"
	"time"

	"youTobe_transcribe/internal/logger"
)

// Write записывает содержимое в файл внутри хранилища Obsidian.
func Write(obsidianPath, relativePath, content string) error {
	start := time.Now()
	fullPath := filepath.Join(obsidianPath, relativePath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		logger.Log.Error("obsidian.mkdir_failed", logger.Err(err), "relative_path", relativePath)
		return err
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		logger.Log.Error("obsidian.write_failed", logger.Err(err), "relative_path", relativePath, "full_path", fullPath)
		return err
	}
	logger.Log.Info("obsidian.write_ok",
		"relative_path", relativePath,
		"full_path", fullPath,
		"content_len", len(content),
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return nil
}
