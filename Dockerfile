FROM golang:1.22-alpine AS builder
WORKDIR /app/n8n
RUN apk add --no-cache git gcc musl-dev
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/

# Сборка основного сервиса (запускает каналы и куки)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/n8n/youtube_transcribe ./cmd/main

# Сборка микросервиса каналов
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/n8n/channels ./cmd/channels

# Сборка микросервиса проверки куки
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/n8n/cookies ./cmd/cookies

FROM alpine:3.23
RUN apk add --no-cache ca-certificates ffmpeg python3 py3-pip bash curl unzip nodejs

# Установка deno
RUN curl -fsSL https://deno.land/install.sh | sh
ENV PATH="/root/.deno/bin:${PATH}"

# Установка yt-dlp и yt-dlp-ejs (скрипты-решатели)
RUN pip3 install --no-cache-dir --break-system-packages yt-dlp yt-dlp-ejs \
    && ln -sf /usr/bin/yt-dlp /usr/local/bin/yt-dlp

WORKDIR /app/n8n

# Копируем все три бинарника
COPY --from=builder /app/n8n/youtube_transcribe /app/n8n/youtube_transcribe
COPY --from=builder /app/n8n/channels /app/n8n/channels
COPY --from=builder /app/n8n/cookies /app/n8n/cookies

# Копируем куки
COPY internal/cookies/cookies.txt /cookies/cookies.txt

# По умолчанию запускаем основной сервис
CMD ["/app/n8n/youtube_transcribe"]