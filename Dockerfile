FROM golang:1.22-alpine AS builder
WORKDIR /app/n8n
RUN apk add --no-cache git gcc musl-dev
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/n8n/youtube_transcribe ./cmd/transcribe

FROM alpine:3.23
RUN apk add --no-cache ca-certificates ffmpeg python3 py3-pip bash curl unzip nodejs
# nodejs нужен yt-dlp для решения JS-challenge (подпись форматов YouTube),
# как в оригинальном n8n-воркфлоу: --js-runtimes deno --remote-components ejs:github
RUN pip3 install --no-cache-dir --break-system-packages yt-dlp yt-dlp-ejs \
    && ln -sf /usr/bin/yt-dlp /usr/local/bin/yt-dlp
WORKDIR /app/n8n
COPY --from=builder /app/n8n/youtube_transcribe /app/n8n/youtube_transcribe
COPY cookies.txt /cookies/cookies.txt
CMD ["/app/n8n/youtube_transcribe"]