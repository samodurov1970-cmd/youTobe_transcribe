# YouTube Transcribe

# Правила работы с проектом youTobe_transcribe

## Сборка и запуск
Всегда используй этот блок:
```powershell
cd C:\Users\user\GO\youTobe_transcribe
go build ./...
docker compose build --no-cache
docker compose up -d
## Меньше расуждений больше кода
экономь токены
давай не изменения а полный код в файле который меняем


Проект `youTobe_transcribe` — это набор независимых микросервисов на языке Go для автоматического отслеживания YouTube-подписок, скачивания аудио с каналов, транскрибации речи через Whisper и сохранения результатов в хранилище Obsidian. Приложение развертывается в Docker и взаимодействует с PostgreSQL, Ollama, n8n и Telegram.

Каждый микросервис находится в отдельной папке `cmd/<имя>/` и отвечает за свою конкретную задачу. Все микросервисы используют общую базу данных PostgreSQL и пишут логи в едином JSON-формате через `internal/logger`.

## 🛠️ Стек технологий

| Компонент | Технология |
|-----------|-----------|
| Язык основного кода | Go 1.22 |
| Язык транскрибации | Python 3.11 (`faster-whisper`) |
| База данных | PostgreSQL 15 |
| Драйвер БД | `github.com/lib/pq` |
| Логирование | `log/slog` (JSON) |
| Контейнеризация | Docker + Docker Compose |
| Локальный LLM | Ollama (`qwen2.5:1.5b`) |
| Скачивание видео/аудио | yt-dlp (+ deno, node) |
| Веб-сервер | `net/http` (стандартная библиотека Go) |
| Нотификации | Telegram Bot API |
| Хранилище заметок | Obsidian (Markdown-файлы) |
| Автоматизация | n8n |
| Обмен файлами | rclone + Google Drive |

## ⚙️ Переменные окружения (`.env`)

```bash
# Database
DATABASE_URL=postgres://peter:peter@localhost:5432/youtube_tracker?sslmode=disable

# Obsidian
OBSIDIAN_PATH=/obsidian

# yt-dlp
YT_DLP_PATH=/usr/local/bin/yt-dlp
COOKIES_FILE=/cookies/cookies.txt

# OpenRouter (резервный вариант транскрибации)
OPENROUTER_API_KEY=sk-or-v1-...
OPENROUTER_MODEL=nvidia/nemotron-3-ultra-550b-a55b:free

# Transcription
TRANSCRIPTION_URL=https://groq-whisper-proxy.matsgraaf17.workers.dev
DEEPGRAM_API_KEY=

# App
LOG_LEVEL=info
APP_ENV=production

# Telegram (уведомления о новых подписках)
TELEGRAM_BOT_TOKEN=7818580465:AAH9aGFoeiw_Gd0NGAC4BqQngfeT0D7jOkk
TELEGRAM_CHAT_ID=7069245860

# n8n
N8N_ENCRYPTION_KEY=3hc5qFnYAWrybyyNA3ZDD/odAlN1hMeG

# Ollama (классификация тем каналов и валидация текста)
OLLAMA_URL=http://ollama:11434
OLLAMA_MODEL=qwen2.5:1.5b
OLLAMA_TIMEOUT=15s
OLLAMA_VIDEO_TITLES_LIMIT=10

# Интервал обновления списка каналов
CHANNELS_UPDATE_INTERVAL=60m