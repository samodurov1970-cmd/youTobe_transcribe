package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"youTobe_transcribe/internal/logger"
)

// ollamaGenerateRequest — запрос к Ollama /api/generate.
type ollamaGenerateRequest struct {
	Model   string `json:"model"`
	Prompt  string `json:"prompt"`
	Stream  bool   `json:"stream"`
	Options struct {
		Temperature float64 `json:"temperature"`
		NumPredict  int     `json:"num_predict"`
	} `json:"options"`
}

// ollamaGenerateResponse — ответ от Ollama /api/generate.
type ollamaGenerateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// LLMClient — клиент для классификации тем каналов через Ollama.
type LLMClient struct {
	url     string
	model   string
	timeout time.Duration
	client  *http.Client
	log     *slog.Logger
}

// NewLLMClient создаёт клиент Ollama.
func NewLLMClient(url, model string, timeout time.Duration) *LLMClient {
	log := logger.Log.With("component", "channels", "subcomponent", "llm")
	return &LLMClient{
		url:     strings.TrimSuffix(url, "/"),
		model:   model,
		timeout: timeout,
		client:  &http.Client{Timeout: timeout},
		log:     log,
	}
}

// Classify отправляет запрос к Ollama и возвращает тему канала (1/2/3).
// Возвращает пустую строку при любой ошибке — вызывающий код должен использовать fallback.
// Логирует все запросы и ответы для аудита.
func (c *LLMClient) Classify(ctx context.Context, channelID, channelName, description string, videoTitles []string) (string, error) {
	prompt := buildPrompt(channelName, description, videoTitles)

	// Логируем запрос в LLM.
	c.log.Info("channels.llm.request",
		"step", "llm_classify",
		"channel_id", channelID,
		"channel_name", channelName,
		"model", c.model,
		"prompt_length", len(prompt),
		"video_titles_count", len(videoTitles),
	)

	reqBody := ollamaGenerateRequest{
		Model:  c.model,
		Prompt: prompt,
		Stream: false,
	}
	reqBody.Options.Temperature = 0
	reqBody.Options.NumPredict = 4

	body, err := json.Marshal(reqBody)
	if err != nil {
		c.log.Error("channels.llm.request_marshal_failed",
			"step", "llm_classify",
			"channel_id", channelID,
			"error", err.Error(),
		)
		return "", fmt.Errorf("marshal ollama request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+"/api/generate", strings.NewReader(string(body)))
	if err != nil {
		c.log.Error("channels.llm.request_build_failed",
			"step", "llm_classify",
			"channel_id", channelID,
			"error", err.Error(),
		)
		return "", fmt.Errorf("build ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		c.log.Error("channels.llm.request_failed",
			"step", "llm_classify",
			"channel_id", channelID,
			"error", err.Error(),
		)
		return "", fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		c.log.Error("channels.llm.response_read_failed",
			"step", "llm_classify",
			"channel_id", channelID,
			"error", err.Error(),
		)
		return "", fmt.Errorf("read ollama response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		c.log.Error("channels.llm.api_error",
			"step", "llm_classify",
			"channel_id", channelID,
			"status", resp.StatusCode,
			"response", string(raw),
		)
		return "", fmt.Errorf("ollama api error (status %d): %s", resp.StatusCode, string(raw))
	}

	var parsed ollamaGenerateResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		c.log.Error("channels.llm.parse_failed",
			"step", "llm_classify",
			"channel_id", channelID,
			"error", err.Error(),
		)
		return "", fmt.Errorf("parse ollama response: %w", err)
	}

	// Логируем ответ LLM.
	c.log.Info("channels.llm.response",
		"step", "llm_classify",
		"channel_id", channelID,
		"response", parsed.Response,
		"raw_length", len(parsed.Response),
	)

	topic := parseTopic(parsed.Response)
	if topic == "" {
		c.log.Error("channels.llm.unrecognized_topic",
			"step", "llm_classify",
			"channel_id", channelID,
			"response", parsed.Response,
		)
		return "", fmt.Errorf("ollama returned unrecognized topic: %q", parsed.Response)
	}

	c.log.Info("channels.llm.success",
		"step", "llm_classify",
		"channel_id", channelID,
		"topic", topic,
	)

	return topic, nil
}

// buildPrompt формирует строгий промпт для Ollama: только цифра 1/2/3.
func buildPrompt(channelName, description string, videoTitles []string) string {
	var b strings.Builder

	b.WriteString("You are a YouTube channel topic classifier. Answer with ONLY one digit: 1, 2, or 3.\n")
	b.WriteString("Topics: 1 = news, 2 = my_marine_aquarium, 3 = learning_trading_on_markets.\n")
	b.WriteString("Do not explain. Do not add punctuation. Answer with exactly one digit.\n\n")

	b.WriteString("Channel name: " + channelName + "\n")
	if description != "" {
		b.WriteString("Channel description: " + description + "\n")
	}

	if len(videoTitles) > 0 {
		b.WriteString("Recent video titles:\n")
		for i, t := range videoTitles {
			b.WriteString(fmt.Sprintf("  %d. %s\n", i+1, t))
		}
	}

	b.WriteString("\nAnswer: ")
	return b.String()
}

// parseTopic извлекает тему из ответа Ollama.
// Принимает только первые цифры 1, 2, 3.
func parseTopic(response string) string {
	response = strings.TrimSpace(response)
	for _, ch := range response {
		switch ch {
		case '1':
			return TopicNews
		case '2':
			return TopicAquarium
		case '3':
			return TopicTrading
		}
	}
	return ""
}

// ensure LLMClient implements required interfaces at compile time.
var _ = (*LLMClient)(nil)
