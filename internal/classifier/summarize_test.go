package classifier

import (
	"strings"
	"testing"
)

func TestSplitIntoChunks(t *testing.T) {
	t.Run("короткий текст — 1 чанк", func(t *testing.T) {
		chunks := splitIntoChunks("короткий текст", 100)
		if len(chunks) != 1 {
			t.Errorf("expected 1 chunk, got %d", len(chunks))
		}
	})

	t.Run("пустой текст — nil", func(t *testing.T) {
		chunks := splitIntoChunks("   ", 100)
		if chunks != nil {
			t.Errorf("expected nil, got %v", chunks)
		}
	})

	t.Run("текст больше size — несколько чанков", func(t *testing.T) {
		text := strings.Repeat("Это предложение. ", 50) // 850 символов
		chunks := splitIntoChunks(text, 200)
		if len(chunks) < 2 {
			t.Errorf("expected at least 2 chunks, got %d", len(chunks))
		}
		// Все чанки не пустые
		for i, c := range chunks {
			if strings.TrimSpace(c) == "" {
				t.Errorf("chunk %d is empty", i)
			}
		}
	})

	t.Run("разрез по абзацу", func(t *testing.T) {
		text := "Первый абзац с текстом.\n\nВторой абзац с текстом.\n\nТретий абзац."
		chunks := splitIntoChunks(text, 30)
		if len(chunks) < 2 {
			t.Errorf("expected at least 2 chunks, got %d", len(chunks))
		}
	})
}

func TestExtractBlock(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "обычный JSON",
			raw:  `{"block": "## Конспект\n\nТекст"}`,
			want: "## Конспект\n\nТекст",
		},
		{
			name: "JSON в markdown-обёртке",
			raw:  "```json\n{\"block\": \"## Конспект\"}\n```",
			want: "## Конспект",
		},
		{
			name: "двойная скобка (баг с LLM)",
			raw:  "{\n{\"block\": \"## Конспект\"}\n}",
			want: "## Конспект",
		},
		{
			name: "просто markdown без JSON",
			raw:  "## Конспект\n\nЭто длинный текст без JSON-обёртки, больше 50 символов, чтобы прошёл fallback.",
			want: "## Конспект\n\nЭто длинный текст без JSON-обёртки, больше 50 символов, чтобы прошёл fallback.",
		},
		{
			name: "пустой ответ",
			raw:  "",
			want: "",
		},
		{
			name: "JSON без block",
			raw:  `{"text": "..."}`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractBlock(tt.raw)
			if got != tt.want {
				t.Errorf("extractBlock() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildSummaryPrompt(t *testing.T) {
	transcript := "Это тестовый транскрипт лекции про трейдинг."
	prompt := buildSummaryPrompt("TestChannel", transcript, 1, 1)

	checks := []string{
		"НА РУССКОМ ЯЗЫКЕ",
		"НЕ СОКРАЩАЙ",
		"ПОДРОБНЫЙ",
		"близкий",
		"markdown-блок",
		transcript,
	}
	for _, check := range checks {
		if !strings.Contains(prompt, check) {
			t.Errorf("prompt should contain %q", check)
		}
	}
}

func TestBuildSummaryPromptMultiChunk(t *testing.T) {
	prompt := buildSummaryPrompt("TestChannel", "текст", 2, 5)
	if !strings.Contains(prompt, "фрагмент 2 из 5") {
		t.Error("prompt should mention chunk index for multi-chunk")
	}
}