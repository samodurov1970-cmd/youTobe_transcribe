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

// Classify сначала использует жесткие правила (ключевые слова), потом LLM.
func (c *LLMClient) Classify(ctx context.Context, channelID, channelName, description string, videoTitles []string) (string, error) {
	// 1. Проверяем по ключевым словам (быстро и точно)
	topic := classifyByKeywords(channelName, description)
	if topic != "" {
		c.log.Info("channels.llm.keywords_success", "step", "llm_classify", "channel_id", channelID, "topic", topic)
		return topic, nil
	}

	// 2. Если ключевые слова не сработали — идем в LLM
	c.log.Info("channels.llm.request", "step", "llm_classify", "channel_id", channelID, "channel_name", channelName, "model", c.model)

	prompt := buildPrompt(channelName, description)
	reqBody := ollamaGenerateRequest{Model: c.model, Prompt: prompt, Stream: false}
	reqBody.Options.Temperature = 0
	reqBody.Options.NumPredict = 4

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal ollama request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+"/api/generate", strings.NewReader(string(body)))
	if err != nil {
		return "", fmt.Errorf("build ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		c.log.Error("channels.llm.request_failed", "step", "llm_classify", "channel_id", channelID, "error", err.Error())
		return "", fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read ollama response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama api error (status %d): %s", resp.StatusCode, string(raw))
	}

	var parsed ollamaGenerateResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("parse ollama response: %w", err)
	}

	topic = parseTopic(parsed.Response)
	if topic == "" {
		c.log.Error("channels.llm.unrecognized_topic", "step", "llm_classify", "channel_id", channelID, "response", parsed.Response)
		return "", fmt.Errorf("ollama returned unrecognized topic: %q", parsed.Response)
	}

	c.log.Info("channels.llm.success", "step", "llm_classify", "channel_id", channelID, "topic", topic)
	return topic, nil
}

// classifyByKeywords — жесткие правила по ключевым словам.
func classifyByKeywords(name, description string) string {
	text := strings.ToLower(name + " " + description)

	// 1. АКВАРИУМ (самое важное — reef, coral, aquarium)
	aquarium := []string{"reef", "coral", "aquarium", "saltwater", "marine", "reefkeeping", "frag", "anemone", "fish tank"}
	for _, kw := range aquarium {
		if strings.Contains(text, kw) {
			return TopicAquarium
		}
	}

	// 2. AI / ТЕХНОЛОГИИ (это НЕ трейдинг, отправляем в новости)
	aiTech := []string{"ai", "artificial intelligence", "нейросет", "chatgpt", "machine learning", "deep learning", "gpt", "llm"}
	for _, kw := range aiTech {
		if strings.Contains(text, kw) {
			return TopicNews
		}
	}

	// 3. НОВОСТИ (news, politics, breaking)
	news := []string{"news", "politics", "breaking", "world events", "war", "disaster", "bbc", "cnn", "cbs", "fox news", "associated press", "reuters", "bloomberg", "financial news"}
	for _, kw := range news {
		if strings.Contains(text, kw) {
			return TopicNews
		}
	}

	// 4. ТРЕЙДИНГ (trading, forex, crypto, stock, financial services)
	trading := []string{"trading", "forex", "stock", "crypto", "bitcoin", "trader", "price action", "scalping", "market analysis", "invest", "gerchik", "financial services", "wealth", "portfolio"}
	for _, kw := range trading {
		if strings.Contains(text, kw) {
			return TopicTrading
		}
	}

	return ""
}

// buildPrompt — используется только если ключевые слова не сработали.
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