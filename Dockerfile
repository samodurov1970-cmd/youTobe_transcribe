FROM golang:1.22-alpine AS builder
WORKDIR /app/n8n
RUN apk add --no-cache git gcc musl-dev
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/

# Сборка основного сервиса (каналы, куки, видео, статистика, валидатор, trash_manager)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/n8n/main ./cmd/main

# Сборка микросервиса транскрибации (отдельно, для быстрой остановки)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/n8n/transcribe ./cmd/transcribe

# Сборка веб-интерфейса (trash и другие микросервисы)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/n8n/webui ./cmd/webui

FROM alpine:3.23
RUN apk add --no-cache ca-certificates ffmpeg python3 py3-pip bash curl unzip nodejs

# Установка deno
RUN curl -fsSL https://deno.land/install.sh | sh
ENV PATH="/root/.deno/bin:${PATH}"

# Установка yt-dlp и yt-dlp-ejs
RUN pip3 install --no-cache-dir --break-system-packages yt-dlp yt-dlp-ejs \
    && ln -sf /usr/bin/yt-dlp /usr/local/bin/yt-dlp

WORKDIR /app/n8n

# Копируем бинарники
COPY --from=builder /app/n8n/main /app/n8n/main
COPY --from=builder /app/n8n/transcribe /app/n8n/transcribe
COPY --from=builder /app/n8n/webui /app/n8n/webui

# Копируем куки
COPY internal/cookies/cookies.txt /cookies/cookies.txt

# По умолчанию запускаем основной сервис
CMD ["/app/n8n/main"]