# === Стадия сборки Go ===
FROM golang:1.22-alpine AS builder
WORKDIR /app/n8n
RUN apk add --no-cache git gcc musl-dev
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/n8n/main ./cmd/main
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/n8n/transcribe ./cmd/transcribe
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/n8n/webui ./cmd/webui
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/n8n/classifier ./cmd/classifier
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/n8n/publisher ./cmd/publisher

# === Финальный образ на Debian (glibc) ===
FROM python:3.11-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    tzdata ffmpeg curl unzip nodejs ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL https://deno.land/install.sh | sh
ENV PATH="/root/.deno/bin:${PATH}"

RUN pip install --no-cache-dir yt-dlp yt-dlp-ejs faster-whisper

WORKDIR /app/n8n

COPY --from=builder /app/n8n/main /app/n8n/main
COPY --from=builder /app/n8n/transcribe /app/n8n/transcribe
COPY --from=builder /app/n8n/webui /app/n8n/webui
COPY --from=builder /app/n8n/classifier /app/n8n/classifier
COPY --from=builder /app/n8n/publisher /app/n8n/publisher

COPY internal/transcribe/whisper_worker.py /app/n8n/whisper_worker.py
COPY internal/cookies/cookies.txt /cookies/cookies.txt

CMD ["/app/n8n/main"]