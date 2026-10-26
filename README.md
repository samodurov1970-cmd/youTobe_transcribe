# YouTube Transcribe

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
```

## 🚀 Запуск

### Локально (Go)

```bash
# Собрать бинарь главного микросервиса
make build

# Запустить с переменными из .env
make run

# Прогнать тесты
make test

# Статический анализ
make vet

# Форматирование кода
make fmt
```

### Через Docker Compose

```bash
docker compose up -d --build
```

Микросервисы в `docker-compose.yml`:
- `main` — главный процесс (все микросервисы в одном контейнере)
- `transcribe` — отдельный контейнер только для транскрибации
- `webui` — веб-интерфейс управления trash (порт 8081)
- `postgres` — база данных PostgreSQL 15 (порт 5432)
- `n8n` — система автоматизации (порт 5678)
- `ollama` — локальный LLM (порт 11435 на хосте, внутри контейнера 11434)

Посмотреть логи:
```bash
docker logs main --tail 50
docker logs transcribe --tail 50
docker logs webui --tail 50
```

## 🗄️ База данных

Подключение к PostgreSQL:
```bash
docker exec postgres psql -U peter -d youtube_tracker
```

Основные таблицы:
- `channels` — каналы YouTube (ID, название, URL, описание, тема, статус подписки, дата последнего визита)
- `videos` — видео с каналов (ID, название, URL, дата публикации, канал, тема, флаг обработки)
- `video_files` — файлы для транскрибации (статус: processing/transcribed/error/pending, содержимое транскрипта, ошибки)
- `trash` — проблемные записи (причина, действие delete/reprocess, содержимое)
- `channels_unsubscribed` — журнал отписок и повторных подписок

## 📱 Веб-интерфейс

Веб-UI доступен по адресу `http://localhost:8081/trash` и позволяет:
- Просматривать записи в корзине (trash)
- Устанавливать действие `delete` или `reprocess` для каждой записи через выпадающий список
- `delete` — удалить запись из trash и пометить видео как обработанное
- `reprocess` — удалить запись из trash и сбросить `is_processed = false` для повторной обработки
- healthz endpoint: `http://localhost:8081/healthz`

## 📁 Структура проекта

```
youTobe_transcribe/
├── .env                         # Переменные окружения (не коммитится в Git)
├── .gitignore                   # Исключения для Git
├── CLAUDE.md                    # Правила и архитектура проекта
├── Dockerfile                   # Многостадийная сборка: Go-билдер + Python-рантайм
├── docker-compose.yml           # Оркестрация: n8n, postgres, ollama, main, transcribe, webui
├── go.mod                       # Модуль Go (зависимость: github.com/lib/pq)
├── go.sum                       # Контрольные суммы зависимостей
├── Makefile                     # Команды сборки, запуска, тестов, Docker
├── README.md                    # Этот файл
├── test_audio.webm              # Тестовый аудиофайл
│
├── cmd/                         # Микросервисы (точки входа)
│   ├── main/
│   │   └── globalmain.go        # Главный процесс: все микросервисы в одном бинаре
│   ├── channels/
│   │   └── main.go              # Микросервис отслеживания YouTube-подписок
│   ├── cookies/
│   │   └── main.go              # Микросервис проверки актуальности cookies YouTube
│   ├── transcribe/
│   │   └── main.go              # Микросервис транскрибации видео
│   ├── trash_manager/
│   │   └── main.go              # Микросервис управления корзиной (delete/reprocess)
│   ├── validator/
│   │   └── main.go              # Микросервис валидации качества транскриптов
│   ├── videos/
│   │   └── main.go              # Микросервис сбора видео с каналов
│   └── webui/
│       └── main.go              # Микросервис веб-интерфейса управления trash
│
├── internal/                    # Внутренние пакеты (общие библиотеки)
│   ├── channels/                # Подписки, классификация тем, уведомления, Telegram
│   ├── config/                  # Загрузка конфигурации из переменных окружения
│   ├── cookies/                 # Проверка cookies YouTube
│   ├── logger/                  # JSON-логгер на базе slog
│   ├── obsidian/                # Запись транскриптов в файлы Obsidian
│   ├── storage/                 # Работа с PostgreSQL
│   ├── transcribe/              # Вызов whisper_worker.py, воркер транскрибации
│   ├── trash_manager/           # Обработка записей trash с action delete/reprocess
│   ├── validator/               # Проверка качества текста (эвристики + LLM)
│   ├── videos/                  # Сбор видео с каналов через yt-dlp
│   ├── web/                     # HTML-шаблон и обработчики /trash, /trash/action
│   ├── workflow/                # Пример сквозного процесса
│   └── ytdlp/                   # Обёртка над yt-dlp
│
└── backup/                      # Резервные копии и артефакты n8n
    ├── all_queries.txt          # SQL-запросы из воркфлоу n8n
    └── wf_list.txt / wflist.txt # Списки воркфлоу
```

## 📦 Внутренние пакеты (internal/) — подробное описание каждого файла

### `internal/channels/` — Подписки, классификация тем, уведомления, Telegram

| Файл | Назначение |
|------|-----------|
| `channels.go` | Основной сервис подписок. Читает список подписок через `yt-dlp`, делает снимок известных каналов, проверяет отписки, сохраняет новые подписки, классифицирует темы, отправляет уведомления в Telegram. |
| `classify.go` | Классификация тем каналов по ключевым словам. Три темы: `новости`, `мой_морской_аквариум`, `обучение_торговле_на_рынках`. Списки ключевых слов и функция проверки вхождения слова. |
| `classify_test.go` | Тесты классификации тем. |
| `channels_test.go` | Тесты сервиса каналов. |
| `integration_db_test.go` | Интеграционные тесты с БД для каналов. |
| `llm.go` | Клиент Ollama для классификации тем через `/api/generate`. Ожидает ответ одной цифрой (1/2/3). |
| `llm_test.go` | Тесты клиента LLM. |
| `notify.go` | Уведомления о новых и повторных подписках, об отписках. Снимок известных каналов → разница → уведомления. |
| `notify_test.go` | Тесты уведомлений. |
| `telegram.go` | Отправка в Telegram Bot API. Повторы (3 попытки, 2с задержка), HTML-разметка, экранирование. |
| `telegram_integration_test.go` | Интеграционные тесты Telegram. |
| `telegram_test.go` | Тесты Telegram. |
| `unsubscribe_test.go` | Тесты отписок. |

### `internal/config/` — Конфигурация приложения

| Файл | Назначение |
|------|-----------|
| `config.go` | Загрузка всех настроек из переменных окружения в структуру `Config`. Чтение строк, целых чисел и временных интервалов. Функция `MaskSecret` для маскирования секретов в логах. |
| `config_test.go` | Тесты конфигурации. |

### `internal/cookies/` — Проверка cookies YouTube

| Файл | Назначение |
|------|-----------|
| `cookies.go` | Проверка актуальности cookies-файла YouTube через `yt-dlp --get-duration` на тестовом видео. Отправка отчётов и ежедневной статистики в Telegram. |
| `cookies.txt` | Файл куки YouTube в формате Netscape HTTP Cookie File. |

### `internal/logger/` — Логирование

| Файл | Назначение |
|------|-----------|
| `logger.go` | JSON-логгер на базе `log/slog`. Глобальная переменная `Log`. Поля: `service_name` (`youtube_transcribe`), `environment`, `caller`. Уровень из `LOG_LEVEL`. |

### `internal/obsidian/` — Интеграция с Obsidian

| Файл | Назначение |
|------|-----------|
| `obsidian.go` | Запись транскрипта в Markdown-файл Obsidian. Создание директорий, логирование результата. |

### `internal/storage/` — Работа с PostgreSQL

| Файл | Назначение |
|------|-----------|
| `storage.go` | Подключение к PostgreSQL (`sql.Open`, `Ping`). Пул соединений. Тип `VideoFile`. |
| `claims.go` | `ClaimVideosForTranscription` — перевод видео в статус `processing` через транзакцию с `RETURNING`. |
| `queries.go` | `MarkTranscribed`, `MarkError`, `FetchNextProcessing` — обновление статуса и получение следующего видео. |
| `videos.go` | Типы `Channel`, `Video`, `VideoStats`. Функции: `GetActiveChannels`, `InsertVideo`, `GetVideoStats`, `GetPendingVideos`, `MarkVideoProcessed`, `GetPendingValidation`, `MarkVideoValidated`, `InsertTrash`, `DeleteVideoFile`, `ResetVideoProcessed`, `GetTrashWithAction`, `DeleteTrash`. |
| `web.go` | Тип `TrashRow`, функции `ListTrash`, `SetTrashAction` для веб-интерфейса. |

### `internal/transcribe/` — Транскрибация аудио

| Файл | Назначение |
|------|-----------|
| `transcribe.go` | Вызов Python-скрипта `whisper_worker.py`. Парсинг JSON-ответа Whisper (`text`, `language`, `error`). |
| `whisper_worker.py` | Python-скрипт на `faster-whisper` (модель medium, CPU, int8, язык ru). Принимает путь к аудио, возвращает JSON. |
| `worker.go` | Воркер транскрибации: получение необработанных видео, проверка длительности (10–3600 сек), скачивание аудио, транскрипт, сохранение в `video_files` и Obsidian, запись в trash при ошибках. |

### `internal/trash_manager/` — Менеджер корзины

| Файл | Назначение |
|------|-----------|
| `trash_manager.go` | Обработка записей `trash` с действием `delete` (удалить, пометить обработанным) или `reprocess` (удалить, сбросить `is_processed = false`). Размер партии 10. |

### `internal/validator/` — Валидатор качества текста

| Файл | Назначение |
|------|-----------|
| `validator.go` | Проверка транскриптов: эвристики (длина, кириллица в английском канале, повторяющиеся слова) + заглушка под LLM. Плохие тексты → `trash`, хорошие → `validated`. Размер партии 10. |

### `internal/videos/` — Сбор видео с каналов

| Файл | Назначение |
|------|-----------|
| `videos.go` | Сборщик видео: получение активных каналов, запрос через `yt-dlp` (`--flat-playlist --dump-single-json --playlist-end 100`), парсинг JSON, вставка в БД. |

### `internal/web/` — Веб-интерфейс
| Файл | Назначение |
|------|-----------|
| `web.go` | HTML-шаблон страницы trash и обработчики HTTP: `GET /trash`, `POST /trash/action`. |

### `internal/workflow/` — Пример сквозного процесса

| Файл | Назначение |
|------|-----------|
| `workflow.go` | Пример: скачать аудио → транскрибировать → сохранить в `video_files`. |

### `internal/ytdlp/` — Обёртка над yt-dlp

| Файл | Назначение |
|------|-----------|
| `ytdlp.go` | Базовые аргументы yt-dlp, запуск с логированием, скачивание аудио в mp3, извлечение video ID из URL, очистка временных файлов. |

## 💾 Резервные копии (backup/)

| Файл | Назначение |
|------|-----------|
| `all_queries.txt` | SQL-запросы из воркфлоу n8n (claim, insert video_files, update status, insert errors и т.д.). |
| `list_names.py` | Скрипт для вывода имён JSON-файлов воркфлоу из папки backup. |
| `pg_dumpall_20261021_1507.sql` | Дамп базы данных PostgreSQL. Исключён из Git через `.gitignore` (размер >100MB). |
| `wf_0TeslXCJTLKz07RJ.json` | Экспортированный воркфлоу n8n (транскрибация/очередь). |
| `wf_LXuPO870f6WAvdSY.json` | Экспортированный воркфлоу n8n (Telegram/каналы). |
| `wf_piwlAmVtqMKYm7f7.json` | Экспортированный воркфлоу n8n. |
| `wf_x0NxUqqgTpU0VoCB.json` | Экспортированный воркфлоу n8n. |
| `wf_youtube.json` | Экспортированный воркфлоу n8n (YouTube/видео). |
| `wf_list.txt` | Список воркфлоу n8n. |
| `wflist.txt` | Список воркфлоу n8n. |

## 🔄 Поток данных между микросервисами

1. **channels** читает подписки YouTube через yt-dlp → классифицирует каналы по темам → пишет в `channels` → уведомляет в Telegram о новых/повторных подписках и отписках
2. **videos** собирает видео с активных каналов через yt-dlp → пишет в `videos` (только темы аквариум и трейдинг)
3. **transcribe** заявляет видео (`video_files`, статус `processing`) → скачивает аудио → транскрибирует через Whisper → пишет в `video_files` → сохраняет в Obsidian → помечает обработанным
4. **validator** проверяет качество транскриптов (эвристики + LLM) → хорошие помечает `validated`, плохие — в `trash`
5. **trash_manager** выполняет действия `delete`/`reprocess` над записями в `trash` (инициируются через веб-UI)
6. **webui** предоставляет HTML-интерфейс для управления trash по адресу `http://localhost:8081/trash`

## 📊 Мониторинг

- Логи: JSON в stdout, собираются Filebeat → Elasticsearch → Kibana (`http://localhost:5601`, индекс `youtube-transcribe-*`)
- Поле `service_name` всегда `youtube_transcribe` (фильтр Filebeat)
- Подсистемы различаются полем `component`: `channels`, `workflow`, `transcribe`, `validator`, `trash_manager`, `storage`, `webui`
- healthz endpoint: `http://localhost:8081/healthz`

## ⚠️ Важные замечания

- Файл `.env` содержит реальные токены (Telegram, OpenRouter) — не коммитится в Git
- `cookies.txt` содержит настоящие куки YouTube — не коммитится в Git
- Большой SQL-дамп `backup/pg_dumpall_*.sql` исключён из отслеживания Git (размер >100MB)
- В Docker контейнере `main` рабочая директория `/app/n8n`, команды `/app/n8n/main`, `/app/n8n/transcribe`, `/app/n8n/webui`
- Ollama доступен по адресу `http://ollama:11434` внутри Docker-сети
- Все логи пишутся через `internal/logger` с ключевыми полями `step`, `video_id`, `record_id`
- Это не 2 подсистемы, а 8 независимых микросервисов в папке `cmd/`, каждый со своей ответственностью

| Файл | Назначение |
|------|-----------|
| `web.go` | HTML-шаблон страницы trash и обработчики HTTP: `GET /trash`, `POST /trash/action`. |

### `internal/workflow/` — Пример сквозного процесса

| Файл | Назначение |
|------|-----------|
| `workflow.go` | Пример: скачать аудио → транскрибировать → сохранить в `video_files`. |

### `internal/ytdlp/` — Обёртка над yt-dlp

| Файл | Назначение |
|------|-----------|
| `ytdlp.go` | Базовые аргументы yt-dlp, запуск с логированием, скачивание аудио в mp3, извлечение video ID из URL, очистка временных файлов. |
| Файл | Назначение |
|------|-----------|
| `obsidian.go` | Запись транскрипта в Markdown-файл Obsidian. Создание директорий, логирование результата. |

### `internal/storage/` — Работа с PostgreSQL

| Файл | Назначение |
|------|-----------|
| `storage.go` | Подключение к PostgreSQL (`sql.Open`, `Ping`). Пул соединений. Тип `VideoFile`. |
| `claims.go` | `ClaimVideosForTranscription` — перевод видео в статус `processing` через транзакцию с `RETURNING`. |
| `queries.go` | `MarkTranscribed`, `MarkError`, `FetchNextProcessing` — обновление статуса и получение следующего видео. |
| `videos.go` | Типы `Channel`, `Video`, `VideoStats`. Функции: `GetActiveChannels`, `InsertVideo`, `GetVideoStats`, `GetPendingVideos`, `MarkVideoProcessed`, `GetPendingValidation`, `MarkVideoValidated`, `InsertTrash`, `DeleteVideoFile`, `ResetVideoProcessed`, `GetTrashWithAction`, `DeleteTrash`. |
| `web.go` | Тип `TrashRow`, функции `ListTrash`, `SetTrashAction` для веб-интерфейса. |
    ├── list_names.py            # Скрипт вывода имён JSON-файлов воркфлоу
    ├── pg_dumpall_20261021_1507.sql   # Дамп БД (исключён из Git, >100MB)
    ├── wf_*.json                # Экспортированные воркфлоу n8n
    └── wf_list.txt / wflist.txt # Списки воркфлоу
```

## 🔧 Микросервисы (cmd/) — подробное описание каждого

### `cmd/main/globalmain.go` — Главный процесс
Точка входа для основного контейнера `main`. Запускает все микросервисы в отдельных горутинах внутри одного процесса:
1. `channels` — отслеживание подписок (раз в 1 час)
2. `cookies` — проверка актуальности cookies YouTube (в 9:00 и 21:00)
3. `videos` — сбор видео с каналов (раз в 3 часа)
4. Ежедневная статистика (в 00:00)
5. `validator` — валидация качества транскриптов (раз в 5 минут)
6. `trash_manager` — обработка корзины (раз в 5 минут)

Обрабатывает сигналы остановки (SIGINT/SIGTERM) для корректного завершения.

### `cmd/channels/main.go` — Микросервис отслеживания подписок
Запускает только подсистему отслеживания YouTube-подписок. Инициализирует подключение к БД, создаёт сервис каналов и запускает цикл обновления подписок.

### `cmd/cookies/main.go` — Микросервис проверки cookies YouTube
Проверяет актуальность cookies-файла YouTube через `yt-dlp --get-duration` на тестовом видео (`jNQXAC9IVRw` — "Me at the zoo"). Первый запуск сразу при старте, затем раз в 1 час. Отправляет отчёт в Telegram.

### `cmd/transcribe/main.go` — Микросервис транскрибации
Запускает воркер транскрибации. Первый запуск сразу при старте, затем раз в 5 минут. Получает необработанные видео, скачивает аудио, транскрибирует, сохраняет в Obsidian.

### `cmd/trash_manager/main.go` — Микросервис управления корзиной
Обрабатывает записи из таблицы `trash` с заданным действием (`delete` или `reprocess`). Запуск раз в 5 минут.

### `cmd/validator/main.go` — Микросервис валидации текста
Проверяет качество транскрибированных текстов (эвристики + заглушка под LLM). Плохие тексты отправляются в trash. Запуск раз в 5 минут.

### `cmd/videos/main.go` — Микросервис сбора видео
Собирает видео с активных каналов через `yt-dlp` и сохраняет в таблицу `videos`. Первый запуск сразу при старте, затем раз в 3 часа.

### `cmd/webui/main.go` — Микросервис веб-интерфейса
Веб-сервер на `net/http` с маршрутами:
- `GET /trash` — страница со списком записей в корзине
- `POST /trash/action` — установка действия `delete` или `reprocess` для записи
- `GET /healthz` — проверка здоровья сервиса

Порт по умолчанию: `:8081`.