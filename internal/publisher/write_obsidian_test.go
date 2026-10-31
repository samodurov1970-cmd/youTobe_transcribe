package publisher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"youTobe_transcribe/internal/logger"
)

func TestMain(m *testing.M) {
	logger.Init()
	os.Exit(m.Run())
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"Раздел 6. Фундаментал", "Раздел 6. Фундаментал"},
		{"Раздел 6/Фундаментал", "Раздел 6 — Фундаментал"},
		{"Раздел 6: Фундаментал", "Раздел 6 — Фундаментал"},
		{"a\\b", "a — b"},
		{"a*b?c\"d<e>f|g", "a_b_c_d_e_f_g"},
		{"", "без_названия"},
		{"   ", "без_названия"},
	}
	for _, tt := range tests {
		got := sanitizePath(tt.in)
		if got != tt.want {
			t.Errorf("sanitizePath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTargetPath(t *testing.T) {
	p := &Publisher{ObsidianPath: "/tmp/test"}
	got := p.targetPath("Раздел 6. Фундаментал", "6.3. Оценка")
	want := filepath.Join("Раздел 6. Фундаментал", "6.3. Оценка.md")
	if got != want {
		t.Errorf("targetPath() = %q, want %q", got, want)
	}
}

func TestWriteBlock(t *testing.T) {
	t.Run("новый файл", func(t *testing.T) {
		dir := t.TempDir()
		p := &Publisher{ObsidianPath: dir}

		written, skipped, err := p.writeBlock("Раздел 6. Фундаментал", "6.3. Оценка", "https://youtube.com/watch?v=abc", "Видео 1.txt", "## Конспект 1")
		if err != nil {
			t.Fatalf("writeBlock() error: %v", err)
		}
		if !written || skipped {
			t.Errorf("written=%v, skipped=%v, want written=true, skipped=false", written, skipped)
		}

		fullPath := filepath.Join(dir, "Раздел 6. Фундаментал", "6.3. Оценка.md")
		content, err := os.ReadFile(fullPath)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		s := string(content)
		if !strings.Contains(s, "# 6.3. Оценка") {
			t.Error("file should have header")
		}
		if !strings.Contains(s, "> Раздел: Раздел 6. Фундаментал") {
			t.Error("file should have part")
		}
		if !strings.Contains(s, "https://youtube.com/watch?v=abc") {
			t.Error("file should have URL")
		}
		if !strings.Contains(s, "## Конспект 1") {
			t.Error("file should have block")
		}
		if strings.Contains(s, "Видео 1.txt") {
			t.Error("file should NOT have .txt in title")
		}
		if !strings.Contains(s, "Видео 1") {
			t.Error("file should have title without .txt")
		}
	})

	t.Run("дубликат — skipped", func(t *testing.T) {
		dir := t.TempDir()
		p := &Publisher{ObsidianPath: dir}

		_, _, err := p.writeBlock("Раздел 6", "6.3", "https://youtube.com/watch?v=abc", "Видео 1", "## Конспект 1")
		if err != nil {
			t.Fatalf("first writeBlock: %v", err)
		}

		written, skipped, err := p.writeBlock("Раздел 6", "6.3", "https://youtube.com/watch?v=abc", "Видео 1", "## Конспект 1")
		if err != nil {
			t.Fatalf("second writeBlock: %v", err)
		}
		if written || !skipped {
			t.Errorf("written=%v, skipped=%v, want written=false, skipped=true", written, skipped)
		}
	})

	t.Run("два разных видео — два блока", func(t *testing.T) {
		dir := t.TempDir()
		p := &Publisher{ObsidianPath: dir}

		_, _, _ = p.writeBlock("Раздел 6", "6.3", "https://youtube.com/watch?v=abc", "Видео 1", "## Конспект 1")
		written, skipped, err := p.writeBlock("Раздел 6", "6.3", "https://youtube.com/watch?v=def", "Видео 2", "## Конспект 2")
		if err != nil {
			t.Fatalf("second writeBlock: %v", err)
		}
		if !written || skipped {
			t.Errorf("written=%v, skipped=%v, want written=true, skipped=false", written, skipped)
		}

		fullPath := filepath.Join(dir, "Раздел 6", "6.3.md")
		content, _ := os.ReadFile(fullPath)
		s := string(content)
		if !strings.Contains(s, "v=abc") || !strings.Contains(s, "v=def") {
			t.Error("file should contain both URLs")
		}
		if !strings.Contains(s, "## Конспект 1") || !strings.Contains(s, "## Конспект 2") {
			t.Error("file should contain both blocks")
		}
	})

	t.Run("пустые part/chapter — fallback", func(t *testing.T) {
		dir := t.TempDir()
		p := &Publisher{ObsidianPath: dir}

		written, _, err := p.writeBlock("", "", "https://youtube.com/watch?v=xyz", "Видео", "## Конспект")
		if err != nil {
			t.Fatalf("writeBlock: %v", err)
		}
		if !written {
			t.Error("expected written=true")
		}

		fullPath := filepath.Join(dir, "Раздел 13. Разное", "13.1. Разное.md")
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			t.Errorf("file should exist at fallback path: %s", fullPath)
		}
	})
}