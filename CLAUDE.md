# Правила работы
!!!! Вначале каждого ответа пиши - Я прочла правила!!!!
## Язык
- Всегда отвечать на русском языке. Кратко экономь токены
- Логи, код, комментарии в коде — на английском

## Код
- Соблюдать структуру проекта: `cmd/` + `internal/`
- Не использовать библиотеки, которых нет в `go.mod`
- Перед изменением кода — читать текущий файл
- После изменения — всегда проверять `go vet ./...` и `go build`
- Все логи через `internal/logger` (slog, JSON), с ключевыми полями `step`, `video_id`, `record_id`

## Ошибки
- Читать логи программы обязательно в Elastic
- При ошибке сборки — показывать полный текст ошибки, не молчать
- При ошибке в логах Docker/ELK — сначала исправить причину, а не маскировать

## Docker
- Сборка: `docker compose up -d --build` из `C:\Users\user\GO\youTobe_transcribe`
- Просмотр логов: `docker logs youtube_transcribe --tail 50`
- Перед пересборкой — `docker compose stop` если нужно отладить

## База данных
- Подключение: `docker exec postgres psql -U peter -d youtube_tracker`
- Перед изменением SQL-запроса — проверять реальную схему через `information_schema.columns`

## ELK / Мониторинг
- `service_name` в логгере должен быть `youtube_transcribe` (фильтр Filebeat)
- Kibana: http://localhost:5601 → Discover → индекс `youtube-transcribe-*`
- Подсистемы различаются полем `component`: `channels` / `workflow` / `transcribe` / `validator` / `trash_manager` / `storage` / `webui`

## Общение
- Отвечать по существу, без воды
- **Сначала — план.** Перед любым изменением кода/файлов изложить план действий и ЖДАТЬ команду «делай»
- Не начинать выполнение без явной команды «делай»
- Показывать что сделано в конце каждого ответа (список изменённых файлов)

## Архитектура

Единый процесс `youtube_transcribe` (один бинарник, один контейнер, одно подключение к БД).
Внутри — **8 независимых микросервисов** в папке `cmd/`, каждый со своей ответственностью:

### Микросервис 1: `channels` (`cmd/channels/main.go`, `internal/channels`)
- **Что делает:** читает подписки YouTube через `yt-dlp` (`--flat-playlist --dump-single-json` по `https://www.youtube.com/feed/channels`)
- **Пишет в:** таблицу `channels` (+ чистит `videos`/`videos_new` отписанных, пишет `channels_unsubscribed`)
- **Темы (только три):** `новости`, `мой_морской_аквариум`, `обучение_торговле_на_рынках`
- **Расписание:** сразу при старте, далее раз в 1 час
- **Уведомления:** отправляет в Telegram о новых и повторных подписках и об отписках

### Микросервис 2: `cookies` (`cmd/cookies/main.go`, `internal/cookies`)
- **Что делает:** проверяет актуальность cookies YouTube через `yt-dlp --get-duration` на тестовом видео
- **Расписание:** сразу при старте, в 9:00 и 21:00 (или раз в 1 час в standalone режиме)
- **Уведомления:** отправляет отчёт в Telegram

### Микросервис 3: `videos` (`cmd/videos/main.go`, `internal/videos`)
- **Что делает:** собирает видео с активных каналов через `yt-dlp` (`--flat-playlist --dump-single-json --playlist-end 100`)
- **Пишет в:** таблицу `videos` (только темы аквариум и трейдинг)
- **Расписание:** сразу при старте, далее раз в 3 часа

### Микросервис 4: `transcribe` (`cmd/transcribe/main.go`, `internal/transcribe`)
- **Что делает:** заявляет видео → скачивает аудио → транскрибирует через faster-whisper → пишет в `video_files` → сохраняет в Obsidian
- **Расписание:** раз в 5 минут

### Микросервис 5: `validator` (`cmd/validator/main.go`, `internal/validator`)
- **Что делает:** проверяет качество транскриптов (эвристики + LLM-заглушка), плохие отправляет в trash
- **Расписание:** раз в 5 минут

### Микросервис 6: `trash_manager` (`cmd/trash_manager/main.go`, `internal/trash_manager`)
- **Что делает:** выполняет действия `delete`/`reprocess` над записями в `trash`
- **Расписание:** раз в 5 минут

### Микросервис 7: `webui` (`cmd/webui/main.go`, `internal/web`)
- **Что делает:** веб-интерфейс управления trash (`GET /trash`, `POST /trash/action`, `GET /healthz`)
- **Порт:** `:8081`

### Микросервис 8: `main` (`cmd/main/globalmain.go`)
- **Что делает:** запускает все микросервисы 1-6 в отдельных горутинах внутри одного процесса

### Логи в Kibana
- `service_name` всегда `youtube_transcribe` (единственное значение, фильтр Filebeat).
- Подсистемы различаются полем `component`: `channels` / `workflow` / `transcribe` / `validator` / `trash_manager` / `storage` / `webui`.
- Kibana: http://localhost:5601 → Discover → индекс `youtube-transcribe-*`

## Структура проекта (кратко)

### Корень
| Файл | Назначение |
|------|-----------|
| `.env` | Переменные окружения (не коммитится) |
| `.gitignore` | Исключения для Git |
| `CLAUDE.md` | Правила и архитектура проекта |
| `Dockerfile` | Многостадийная сборка: Go-билдер + Python-рантайм |
| `docker-compose.yml` | Оркестрация: n8n, postgres, ollama, main, transcribe, webui |
| `go.mod` / `go.sum` | Модуль Go, только зависимость `github.com/lib/pq` |
| `Makefile` | build, run, test, fmt, vet, tidy, docker-up, docker-down, clean |
| `README.md` | Полное описание проекта |
| `test_audio.webm` | Тестовый аудиофайл |

### cmd/ — точки входа
- `cmd/main/globalmain.go` — главный процесс, все подсистемы в горутинах
- `cmd/channels/main.go` — только подсистема каналов
- `cmd/cookies/main.go` — проверка cookies YouTube
- `cmd/transcribe/main.go` — только подсистема транскрибации
- `cmd/trash_manager/main.go` — менеджер корзины (delete/reprocess)
- `cmd/validator/main.go` — валидатор качества текста
- `cmd/videos/main.go` — сборщик видео с каналов
- `cmd/webui/main.go` — веб-интерфейс управления trash (порт 8081)

### internal/ — пакеты
- `channels/` — подписки, классификация тем, LLM (Ollama), уведомления, Telegram
- `config/` — загрузка конфигурации из переменных окружения
- `cookies/` — проверка актуальности cookies YouTube
- `logger/` — JSON-логгер на базе slog
- `obsidian/` — запись транскриптов в файлы Obsidian
- `storage/` — PostgreSQL: подключение, claim, queries, videos, web (trash)
- `transcribe/` — вызов whisper_worker.py, воркер транскрибации
- `trash_manager/` — обработка записей trash с action delete/reprocess
- `validator/` — проверка качества текста (эвристики + LLM-заглушка)
- `videos/` — сбор видео с каналов через yt-dlp
- `web/` — HTML-шаблон и обработчики /trash, /trash/action
- `workflow/` — пример сквозного процесса скачать → транскрибировать → сохранить
- `ytdlp/` — обёртка над yt-dlp (аргументы, скачивание аудио, извлечение ID)

### backup/ — резервные копии
- `all_queries.txt` — SQL-запросы из n8n-воркфлоу
- `list_names.py` — вывод имён JSON-файлов воркфлоу
- `pg_dumpall_*.sql` — дамп БД (исключён из Git, >100MB)
- `wf_*.json` — экспортированные воркфлоу n8n
- `wf_list.txt`, `wflist.txt` — списки воркфлоу

## Поток данных
1. `channels` читает подписки YouTube → классифицирует каналы → пишет в `channels` → уведомляет в Telegram
2. `videos` собирает видео с активных каналов → пишет в `videos` (только темы аквариум и трейдинг)
3. `transcribe` заявляет видео (`video_files`, статус `processing`) → скачивает аудио → транскрибирует → пишет в `video_files` → сохраняет в Obsidian → помечает обработанным
4. `validator` проверяет качество транскриптов → хорошие помечает `validated`, плохие — в `trash`
5. `trash_manager` выполняет действия `delete`/`reprocess` над записями в `trash` (через веб-UI)
6. `webui` предоставляет HTML-интерфейс для управления trash

## Важные замечания
- `.env` содержит реальные токены (Telegram, OpenRouter) — не коммитится
- `cookies.txt` содержит настоящие куки YouTube — не коммитится
- Большой SQL-дамп в `backup/` исключён из отслеживания Git
- В Docker контейнере `main` рабочая директория `/app/n8n`, команды `/app/n8n/main`, `/app/n8n/transcribe`, `/app/n8n/webui`
- Ollama доступен по адресу `http://ollama:11434` внутри Docker-сети

