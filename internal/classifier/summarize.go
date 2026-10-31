package classifier

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"youTobe_transcribe/internal/logger"
)

const DefaultChunkSize = 20000

func (c *Classifier) summarize(ctx context.Context, channelName, transcript string) (string, error) {
	log := logger.Log.With("component", "classifier")
	start := time.Now()

	chunkSize := c.ChunkSize
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}
	chunks := splitIntoChunks(transcript, chunkSize)
	if len(chunks) == 0 {
		return "", fmt.Errorf("empty transcript")
	}

	log.Info("classifier.summarize_start",
		"chunks", len(chunks),
		"total_len", len(transcript),
		"описание", "Начата суммаризация транскрипта")

	blocks := make([]string, 0, len(chunks))
	for i, c2 := range chunks {
		prompt := buildSummaryPrompt(channelName, c2, i+1, len(chunks))
		raw, err := c.llm.Chat(ctx, prompt, true, 16000)
		if err != nil {
			log.Error("classifier.summarize_chunk_failed", logger.Err(err), "chunk_index", i+1,
				"описание", "ОШИБКА: LLM не ответила при суммаризации чанка")
			return "", err
		}
		block := extractBlock(raw)
		if block == "" {
			log.Warn("classifier.summarize_chunk_empty", "chunk_index", i+1, "raw", truncateStr(raw, 300),
				"описание", "LLM вернула пустой блок, чанк пропущен")
			continue
		}
		blocks = append(blocks, block)
		log.Info("classifier.summarize_chunk_ok", "chunk_index", i+1, "block_len", len(block),
			"описание", "Чанк суммаризирован")
	}

	if len(blocks) == 0 {
		return "", fmt.Errorf("no blocks produced")
	}

	result := strings.Join(blocks, "\n\n")
	log.Info("classifier.summarize_ok",
		"blocks", len(blocks),
		"result_len", len(result),
		"duration_ms", time.Since(start).Milliseconds(),
		"описание", "Суммаризация завершена")
	return result, nil
}

func splitIntoChunks(text string, size int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if len(text) <= size {
		return []string{text}
	}
	var parts []string
	start := 0
	for start < len(text) {
		end := start + size
		if end > len(text) {
			end = len(text)
		} else {
			tailEnd := end + 500
			if tailEnd > len(text) {
				tailEnd = len(text)
			}
			tail := text[end:tailEnd]
			cut := strings.Index(tail, "\n\n")
			if cut < 0 || cut > 400 {
				if idx := strings.IndexAny(tail, ".!?"); idx >= 0 && idx < 400 {
					cut = idx + 1
				}
			}
			if cut >= 0 && cut < 400 {
				end += cut
			}
		}
		piece := strings.TrimSpace(text[start:end])
		if piece != "" {
			parts = append(parts, piece)
		}
		start = end
	}
	return parts
}

func buildSummaryPrompt(channelName, transcript string, idx, total int) string {
	partNote := ""
	if total > 1 {
		partNote = fmt.Sprintf(" (фрагмент %d из %d)", idx, total)
	}
	return `ВАЖНО: Пиши конспект ТОЛЬКО НА РУССКОМ ЯЗЫКЕ. Имена людей, названия компаний,
тикеры и финансовые термины (EBITDA, P/E, UPRO, TMF, XAU/USD и т.д.) оставляй
как в оригинале (латиницей). Весь остальной текст конспекта — строго по-русски.

Ты — эксперт по финансам, трейдингу, криптовалютам и IT. Сделай ПОДРОБНЫЙ конспект
транскрипта видео (расшифровка речи ASR)` + partNote + `, максимально близкий
к содержанию лекции.

ГЛАВНОЕ ПРАВИЛО: НЕ СОКРАЩАЙ СОДЕРЖАНИЕ. Сохраняй все мысли, аргументы, примеры,
объяснения, цифры, имена, названия инструментов, компании, тикеры. Конспект должен
быть таким, чтобы по нему можно было восстановить ход лекции. Лучше длиннее, чем короче.

ЧТО УБИРАТЬ (только это):
- Слова-паразиты: "э-э-э", "мм", "как бы", "ну", "вот", "короче".
- ASR-мусор: "it it it", повторы одного слова подряд, обрывки фраз.
- Оговорки и самоисправления ("то есть нет, я имел в виду...").
- Приветствия, прощания, реклама спонсоров, призывы подписаться.

ЧТО СОХРАНЯТЬ (обязательно):
- Все ключевые идеи и тезисы — полностью, не сокращая.
- Все примеры и истории — как они рассказаны.
- Все цифры, даты, имена, названия компаний, тикеры.
- Все объяснения "почему" и "как" — не убирай причинно-следственные связи.
- Структуру лекции: введение, основная часть, выводы — если она есть.
- Прямые цитаты спикера, если они важны (в кавычках).

ФОРМАТ (Markdown):
- Заголовки ## и ### по смысловым блокам.
- Списки, таблицы, выделение **жирным** для ключевых терминов.
- Если спикер приводит последовательность шагов — нумерованный список.

Верни JSON: {"block": "markdown-блок"}

ТРАНСКРИПТ:
---
` + transcript + `
---

НАПОМИНАЮ: конспект должен быть ПОДРОБНЫМ, на русском языке, близким к лекции.
Не сокращай мысли и примеры. Латиницей только имена, компании, тикеры, термины.
Верни JSON:`
}

func extractBlock(raw string) string {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	// Ищем начало JSON-объекта с "block"
	start := strings.Index(cleaned, `{"block"`)
	if start < 0 {
		start = strings.Index(cleaned, "{")
	}
	if start < 0 {
		if len(cleaned) >= 50 && !strings.HasPrefix(cleaned, "{") {
			return cleaned
		}
		return ""
	}

	// Балансировка скобок
	depth := 0
	end := -1
	for i := start; i < len(cleaned); i++ {
		switch cleaned[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 {
		if len(cleaned) >= 50 && !strings.HasPrefix(cleaned, "{") {
			return cleaned
		}
		return ""
	}

	var obj struct {
		Block string `json:"block"`
	}
	if err := json.Unmarshal([]byte(cleaned[start:end+1]), &obj); err == nil && obj.Block != "" {
		return strings.TrimSpace(obj.Block)
	}
	if len(cleaned) >= 50 && !strings.HasPrefix(cleaned, "{") {
		return cleaned
	}
	return ""
}