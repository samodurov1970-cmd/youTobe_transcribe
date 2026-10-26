.\run_tests.ps1

$ErrorActionPreference = "Stop"
$DB_CONTAINER = "postgres"
$DB_USER = "peter"
$DB_PASS = "peter"
$TEST_DB = "youtube_tracker_test"
$DSN_LOCAL = "postgres://${DB_USER}:${DB_PASS}@localhost:5432/${TEST_DB}?sslmode=disable"
$DSN_DOCKER = "postgres://${DB_USER}:${DB_PASS}@postgres:5432/${TEST_DB}?sslmode=disable"

# --- ШАГ 1 ---
Write-Host "`n=== ШАГ 1: Проверка контейнера базы данных ($DB_CONTAINER) ===" -ForegroundColor Cyan
$isRunning = docker ps --filter "name=^/${DB_CONTAINER}$" --format "{{.Names}}"
if (-not $isRunning) {
    Write-Host "[ИНФО] Контейнер $DB_CONTAINER не запущен. Запускаем..." -ForegroundColor Yellow
    docker compose up -d $DB_CONTAINER
    Start-Sleep -Seconds 5
} else {
    Write-Host "[УСПЕХ] Контейнер $DB_CONTAINER уже работает." -ForegroundColor Green
}

# --- ШАГ 2 ---
Write-Host "`n=== ШАГ 2: Проверка наличия тестовой базы ($TEST_DB) ===" -ForegroundColor Cyan
$dbExists = docker exec -i $DB_CONTAINER psql -U $DB_USER -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname='$TEST_DB'"
if ($dbExists -ne "1") {
    Write-Host "[ИНФО] База $TEST_DB не найдена. Создаем..." -ForegroundColor Yellow
    docker exec -i $DB_CONTAINER psql -U $DB_USER -d postgres -c "CREATE DATABASE $TEST_DB;"
    Write-Host "[УСПЕХ] База $TEST_DB создана." -ForegroundColor Green
} else {
    Write-Host "[УСПЕХ] База $TEST_DB уже существует." -ForegroundColor Green
}

# --- ШАГ 3 ---
Write-Host "`n=== ШАГ 3: Запуск ЮНИТ-ТЕСТОВ (без БД) ===" -ForegroundColor Cyan
go test ./... -count=1
if ($LASTEXITCODE -ne 0) {
    Write-Host "[ОШИБКА] Юнит-тесты провалены." -ForegroundColor Red
    exit 1
}
Write-Host "[УСПЕХ] Все юнит-тесты пройдены." -ForegroundColor Green

# --- ШАГ 4 ---
Write-Host "`n=== ШАГ 4: Запуск ИНТЕГРАЦИОННЫХ ТЕСТОВ (с реальной БД) ===" -ForegroundColor Cyan
$env:TEST_DATABASE_URL = $DSN_LOCAL
go test -tags=integration_db ./internal/channels/ -count=1 -v
$integrationExitCode = $LASTEXITCODE
Remove-Item Env:\TEST_DATABASE_URL -ErrorAction SilentlyContinue

if ($integrationExitCode -ne 0) {
    Write-Host "[ОШИБКА] Интеграционные тесты провалены." -ForegroundColor Red
    exit 1
}
Write-Host "[УСПЕХ] Все интеграционные тесты пройдены." -ForegroundColor Green

# --- ШАГ 5 ---
Write-Host "`n=== ШАГ 5: Проверка transcribe (сборка с кэшем и запуск) ===" -ForegroundColor Cyan

Write-Host "[ИНФО] Сборка Docker-образа transcribe (используется кэш для ускорения)..." -ForegroundColor Yellow
docker compose build transcribe
if ($LASTEXITCODE -ne 0) {
    Write-Host "[ОШИБКА] Не удалось собрать образ transcribe." -ForegroundColor Red
    exit 1
}
Write-Host "[УСПЕХ] Образ transcribe собран." -ForegroundColor Green

Write-Host "[ИНФО] Запуск transcribe (один цикл)..." -ForegroundColor Yellow
docker compose run --rm -e DATABASE_URL=$DSN_DOCKER transcribe
if ($LASTEXITCODE -ne 0) {
    Write-Host "[ОШИБКА] Контейнер transcribe завершился с ошибкой." -ForegroundColor Red
    exit 1
}
Write-Host "[УСПЕХ] Микросервис transcribe успешно отработал." -ForegroundColor Green

# --- ИТОГ ---
Write-Host "`n=========================================================" -ForegroundColor Green
Write-Host "=== ВСЕ ТЕСТЫ УСПЕШНО ПРОЙДЕНЫ ===" -ForegroundColor Green
Write-Host "=========================================================" -ForegroundColor Green