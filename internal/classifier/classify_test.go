package classifier

import (
	"strings"
	"testing"
)

func TestParseClassify(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    classifyResult
		wantErr bool
	}{
		{
			name: "обычный JSON",
			raw:  `{"part": "Раздел 6. Фундаментал и анализ", "chapter": "6.3. Оценка и мультипликаторы", "summary": "Краткое описание"}`,
			want: classifyResult{
				Part:    "Раздел 6. Фундаментал и анализ",
				Chapter: "6.3. Оценка и мультипликаторы",
				Summary: "Краткое описание",
			},
		},
		{
			name: "JSON в markdown-обёртке",
			raw:  "```json\n{\"part\": \"Раздел 5. Криптовалюты\", \"chapter\": \"5.1. Биткоин\", \"summary\": \"BTC\"}\n```",
			want: classifyResult{
				Part:    "Раздел 5. Криптовалюты",
				Chapter: "5.1. Биткоин",
				Summary: "BTC",
			},
		},
		{
			name: "двойная скобка (баг с LLM)",
			raw:  "{\n{\"part\": \"Раздел 10. Новости\", \"chapter\": \"10.8. Разбор\", \"summary\": \"...\"}\n}",
			want: classifyResult{
				Part:    "Раздел 10. Новости",
				Chapter: "10.8. Разбор",
				Summary: "...",
			},
		},
		{
			name: "без chapter — fallback",
			raw:  `{"part": "Раздел 1. Трейдинг", "summary": "..."}`,
			want: classifyResult{
				Part:    "Раздел 1. Трейдинг",
				Chapter: FallbackChapter,
				Summary: "...",
			},
		},
		{
			name:    "мусор без JSON",
			raw:     "это не JSON, а просто текст",
			wantErr: true,
		},
		{
			name:    "пустой part",
			raw:     `{"part": "", "chapter": "1.1"}`,
			wantErr: true,
		},
		{
			name:    "пустой ответ",
			raw:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseClassify(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseClassify() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseClassify() unexpected error: %v", err)
			}
			if got.Part != tt.want.Part {
				t.Errorf("Part = %q, want %q", got.Part, tt.want.Part)
			}
			if got.Chapter != tt.want.Chapter {
				t.Errorf("Chapter = %q, want %q", got.Chapter, tt.want.Chapter)
			}
			if got.Summary != tt.want.Summary {
				t.Errorf("Summary = %q, want %q", got.Summary, tt.want.Summary)
			}
		})
	}
}

func TestBuildClassifyPrompt(t *testing.T) {
	transcript := "Это тестовый транскрипт про биткоин и криптовалюты."
	prompt := buildClassifyPrompt(transcript)

	if !strings.Contains(prompt, topicStructure) {
		t.Error("prompt should contain topicStructure")
	}
	if !strings.Contains(prompt, transcript) {
		t.Error("prompt should contain transcript")
	}
	if !strings.Contains(prompt, `"part"`) {
		t.Error("prompt should mention 'part' field")
	}
	if !strings.Contains(prompt, `"chapter"`) {
		t.Error("prompt should mention 'chapter' field")
	}
	if !strings.Contains(prompt, `"summary"`) {
		t.Error("prompt should mention 'summary' field")
	}
	if !strings.Contains(prompt, "НА РУССКОМ ЯЗЫКЕ") {
		t.Error("prompt should mention Russian language for summary")
	}
}

func TestTruncateStr(t *testing.T) {
	tests := []struct {
		s    string
		n    int
		want string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a long string", 10, "this is a ..."},
		{"", 10, ""},
	}
	for _, tt := range tests {
		got := truncateStr(tt.s, tt.n)
		if got != tt.want {
			t.Errorf("truncateStr(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
		}
	}
}