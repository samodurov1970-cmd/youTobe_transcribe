package publisher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"youTobe_transcribe/internal/logger"
)

// FallbackPart / FallbackChapter — раздел «Разное».
const (
	FallbackPart    = "Раздел 13. Разное"
	FallbackChapter = "13.1. Разное"
)

// writeBlock дописывает markdown-блок в файл <obsidianPath>/<part>/<chapter>.md.
// Возвращает (written, skippedDuplicate, error).
func (p *Publisher) writeBlock(part, chapter, videoURL, videoTitle, block string) (bool, bool, error) {
	start := time.Now()

	if part == "" {
		part = FallbackPart
	}
	if chapter == "" {
		chapter = FallbackChapter
	}

	safePart := sanitizePath(part)
	safeChapter := sanitizePath(chapter)
	dir := filepath.Join(p.ObsidianPath, safePart)
	fullPath := filepath.Join(dir, safeChapter+".md")

	if err := os.MkdirAll(dir, 0755); err != nil {
		logger.Log.Error("publisher.obsidian_mkdir_failed", logger.Err(err), "dir", dir,
			"описание", "ОШИБКА: Не удалось создать директорию раздела")
		return false, false, err
	}

	existing, err := os.ReadFile(fullPath)
	if err != nil && !os.IsNotExist(err) {
		logger.Log.Error("publisher.obsidian_read_failed", logger.Err(err), "full_path", fullPath,
			"описание", "ОШИБКА: Не удалось прочитать файл Obsidian")
		return false, false, err
	}

	content := string(existing)
	if content == "" {
		content = fmt.Sprintf("# %s\n\n> Раздел: %s\n\n", chapter, part)
	}

	if videoURL != "" && strings.Contains(content, videoURL) {
		logger.Log.Info("publisher.obsidian_duplicate_skipped",
			"video_url", videoURL,
			"full_path", fullPath,
			"описание", "Блок уже есть в файле, пропускаем запись")
		return false, true, nil
	}

	title := strings.TrimSuffix(videoTitle, ".md")
	title = strings.TrimSuffix(title, ".txt")
	date := time.Now().Format("2006-01-02")
	header := fmt.Sprintf("\n\n---\n### %s — %s\n🔗 %s\n\n", date, title, videoURL)
	if err := os.WriteFile(fullPath, []byte(content+header+block), 0644); err != nil {
		logger.Log.Error("publisher.obsidian_block_write_failed", logger.Err(err), "full_path", fullPath,
			"описание", "ОШИБКА: Не удалось дописать блок в файл Obsidian")
		return false, false, err
	}

	logger.Log.Info("publisher.obsidian_block_written",
		"full_path", fullPath,
		"part", part,
		"chapter", chapter,
		"block_len", len(block),
		"duration_ms", time.Since(start).Milliseconds(),
		"описание", "Блок успешно дописан в файл Obsidian")
	return true, false, nil
}

// targetPath возвращает относительный путь к .md-файлу для part/chapter.
func (p *Publisher) targetPath(part, chapter string) string {
	return filepath.Join(sanitizePath(part), sanitizePath(chapter)+".md")
}

// sanitizePath заменяет символы, недопустимые в именах файлов.
func sanitizePath(s string) string {
	s = strings.TrimSpace(s)
	replacer := strings.NewReplacer(
		"/", " — ", "\\", " — ", ":", " — ",
		"*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_",
	)
	out := replacer.Replace(s)
	out = strings.Join(strings.Fields(out), " ") // схлопываем двойные пробелы
	if out == "" {
		out = "без_названия"
	}
	return out
}