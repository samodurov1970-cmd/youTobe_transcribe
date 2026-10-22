BINARY=youtube_transcribe

.PHONY: help build run test fmt vet tidy docker-up docker-down clean

help: ## Показать справку
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

build: ## Собрать бинарь
	go build -o $(BINARY) ./cmd/transcribe

run: ## Запустить локально (читает .env)
	set -a; . ./.env; set +a; go run ./cmd/transcribe

test: ## Прогнать тесты
	go test ./...

fmt: ## Форматирование
	go fmt ./...

vet: ## Статический анализ
	go vet ./...

tidy: ## Привести go.mod/go.sum в порядок
	go mod tidy

docker-up: ## Поднять сервис
	docker compose up -d --build

docker-down: ## Остановить
	docker compose down

clean: ## Убрать артефакты
	rm -f $(BINARY)
