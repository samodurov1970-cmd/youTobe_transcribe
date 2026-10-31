package classifier

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"youTobe_transcribe/internal/logger"
)

type classifyResult struct {
	Part    string `json:"part"`
	Chapter string `json:"chapter"`
	Summary string `json:"summary"`
}

const (
	FallbackPart    = "Раздел 13. Разное"
	FallbackChapter = "13.1. Разное"
)

var topicStructure = `Раздел 1. Трейдинг и PRICE ACTION
  1.1. Smart Money Concepts / ICT
  1.2. Смарт-мани
  1.3. Volume Profile и Order Flow
  1.4. Свечные паттерны
  1.5. Волновой анализ
  1.6. Скальпинг (Williams, Raschke, Seiden)
  1.7. Уровни и зоны
  1.8. Мультитаймфрейм и Top-Down анализ
  1.9. Психология и дисциплина

Раздел 2. Скальпинг и краткосрочная торговля
  2.1. Скальпинг
  2.2. Интрадей
  2.3. Ордер-флоу / микроструктура
  2.4. Спред и комиссии
  2.5. Риск-менеджмент
  2.6. Психология
  2.7. Новости и волатильность
  2.8. Золото (XAU/USD)
  2.9. Форекс и крипта
  2.10. Психология и дисциплина

Раздел 3. Макро-экономика и фундаментал
  3.1. Ставки и инфляция
  3.2. ФРС, ЕЦБ, ЦБ
  3.3. Данные по рынку труда
  3.4. Геополитика и риски
  3.5. Рецессия
  3.6. Психология и поведение

Раздел 4. Психология трейдинга
  4.1. Жадность и страх
  4.2. FOMO и revenge-трейдинг
  4.3. Дисциплина и план
  4.4. Ошибки новичков
  4.5. Эмоции
  4.6. Стресс и сон
  4.7. Установки
  4.8. Мотивация и выгорание
  4.9. Психология и дисциплина

Раздел 5. Криптовалюты и альткоины
  5.1. Биткоин
  5.2. Эфир
  5.3. Альткоины
  5.4. DeFi и смарт-контракты
  5.5. Регуляция
  5.6. Биржи и кастоди
  5.7. Кошельки
  5.8. NFT
  5.9. Стейблкоины
  5.10. Приватность
  5.11. Масштабирование
  5.12. Аналитика
  5.13. Prediction Markets
  5.14. Стейкинг и доход
  5.15. Психология и дисциплина

Раздел 6. Фундаментал и анализ
  6.1. Отчётность
  6.2. Дивиденды, выкуп, M&A
  6.3. Оценка и мультипликаторы
  6.4. Секторный анализ
  6.5. Новости
  6.6. Данные
  6.7. Сентимент
  6.8. Сравнение с конкурентами
  6.9. Риски
  6.10. Отчётность и аудит
  6.11. Психология

Раздел 7. IT и автоматизация
  7.1. IT-инструменты
  7.2. Терминал и торговые роботы
  7.3. n8n и автоматизация
  7.4. Obsidian и заметки
  7.5. Telegram-боты
  7.6. Парсинг и сбор данных
  7.7. IT и риск
  7.8. Безопасность
  7.9. API
  7.10. Производительность
  7.11. Google-таблицы
  7.12. Автоматизация
  7.13. Психология и IT

Раздел 8. Программирование
  8.1. IT Fundamentals
  8.2. System Design
  8.3. JavaScript
  8.4. TypeScript и React
  8.5. CSS
  8.6. Docker и DevOps
  8.7. AWS и Cloud
  8.8. Python и Web Scraping
  8.9. Three.js и 3D
  8.10. CUDA и H100
  8.11. Drone Programming
  8.12. LaTeX
  8.13. Quant Dev
  8.14. InfoSec и AI
  8.15. Кибербезопасность
  8.16. Vibe Coding
  8.17. Figma и дизайн
  8.18. Психология и программирование

Раздел 9. Нумизматика и драгоценные металлы
  9.1. Germania Mint
  9.2. SD Bullion
  9.3. Royal Mint
  9.4. Слитки и монеты
  9.5. Инвестиции в металлы
  9.6. Хранение
  9.7. Психология

Раздел 10. Новости и инфополе
  10.1. Утренний обзор
  10.2. Обзор рынка
  10.3. Итоги недели
  10.4. Прогнозы
  10.5. Слухи
  10.6. Срочные новости ЦБ
  10.7. Срочные новости ФРС
  10.8. Разбор компаний
  10.9. Интервью
  10.10. Мемы и юмор
  10.11. Шоу и игры
  10.12. Срочные новости (Breaking News)
  10.13. Слухи и утечки
  10.14. Скандалы и расследования
  10.15. Психология

Раздел 11. Финансы и банки
  11.1. Вклады и накопления
  11.2. Кредиты и ипотека
  11.3. Страхование
  11.4. Налоги
  11.5. Бюджет
  11.6. Психология и финансы

Раздел 12. Личные финансы
  12.1. Учёт доходов
  12.2. Планирование
  12.3. Сбережения
  12.4. Инвестиции
  12.5. Психология

Раздел 13. Разное
  13.1. Разное`

func (c *Classifier) classify(ctx context.Context, transcript string) (classifyResult, error) {
	start := time.Now()
	log := logger.Log.With("component", "classifier")

	prompt := buildClassifyPrompt(transcript)
	raw, err := c.llm.Chat(ctx, prompt, true, 2000)
	if err != nil {
		log.Error("classifier.classify_llm_failed", logger.Err(err),
			"описание", "ОШИБКА: LLM не ответила при классификации")
		return classifyResult{}, err
	}

	res, err := parseClassify(raw)
	if err != nil {
		log.Warn("classifier.classify_parse_failed", logger.Err(err), "raw", truncateStr(raw, 300),
			"описание", "Не удалось разобрать ответ LLM, используем fallback")
		return classifyResult{Part: FallbackPart, Chapter: FallbackChapter}, nil
	}

	log.Info("classifier.classify_ok",
		"part", res.Part,
		"chapter", res.Chapter,
		"duration_ms", time.Since(start).Milliseconds(),
		"описание", "Транскрипт успешно классифицирован")
	return res, nil
}

func buildClassifyPrompt(transcript string) string {
	return `Ты — эксперт по финансам, трейдингу, криптовалютам и IT. Твоя задача —
классифицировать транскрипт видео (расшифровку речи) по структуре (Раздел и Подраздел).

СТРУКТУРА:
` + topicStructure + `

Если транскрипт не подходит ни под один раздел — верни "Раздел 13. Разное" / "13.1. Разное".

Ответь строго JSON: {"part": "Раздел N. Название", "chapter": "N.N. Название", "summary": "краткое описание 5-8 предложений НА РУССКОМ ЯЗЫКЕ"}

ТРАНСКРИПТ:
` + transcript + `

JSON:`
}

func parseClassify(raw string) (classifyResult, error) {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	// Ищем начало JSON-объекта с "part"
	start := strings.Index(cleaned, `{"part"`)
	if start < 0 {
		start = strings.Index(cleaned, "{")
	}
	if start < 0 {
		return classifyResult{}, fmt.Errorf("no json object")
	}

	// Балансировка скобок: находим закрывающую } для этого объекта
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
		return classifyResult{}, fmt.Errorf("no closing brace")
	}

	var r classifyResult
	if err := json.Unmarshal([]byte(cleaned[start:end+1]), &r); err != nil {
		return classifyResult{}, err
	}
	if r.Part == "" {
		return classifyResult{}, fmt.Errorf("empty part")
	}
	if r.Chapter == "" {
		r.Chapter = FallbackChapter
	}
	return r, nil
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}