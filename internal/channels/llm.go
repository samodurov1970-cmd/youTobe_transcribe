package channels

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"youTobe_transcribe/internal/logger"
)

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

// Classify использует ТОЛЬКО ключевые слова (LLM для каналов не используется).
// Если ключевые слова не сработали — возвращает пустую строку.
func (c *LLMClient) Classify(ctx context.Context, channelID, channelName, description string, videoTitles []string) (string, error) {
	topic, _ := classifyChannel(channelName, description)
	if topic != "" {
		c.log.Info("channels.llm.keywords_success", "step", "llm_classify", "channel_id", channelID, "topic", topic)
		return topic, nil
	}

	c.log.Info("channels.llm.no_match", "step", "llm_classify", "channel_id", channelID,
		"описание", "Ключевые слова не нашли тему, LLM не используется")
	return "", nil
}

// buildPrompt — используется только для тестов, в реальной классификации не применяется.
func buildPrompt(channelName, description string) string {
	var b strings.Builder
	b.WriteString("Classify this YouTube channel into one of three topics.\n\n")
	b.WriteString("1 = news\n")
	b.WriteString("2 = my_marine_aquarium\n")
	b.WriteString("3 = learning_trading_on_markets\n\n")
	b.WriteString("Channel name: " + channelName + "\n")
	if description != "" {
		b.WriteString("Description: " + description + "\n")
	}
	b.WriteString("\nAnswer with one digit (1, 2, or 3): ")
	return b.String()
}

// parseTopic извлекает тему из ответа Ollama.
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