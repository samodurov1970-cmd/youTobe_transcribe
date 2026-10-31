package classifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"youTobe_transcribe/internal/logger"
)

type llmClient struct {
	apiKey string
	model  string
	http   *http.Client
}

func newLLMClient(apiKey, model string) *llmClient {
	return &llmClient{
		apiKey: apiKey,
		model:  model,
		http:   &http.Client{Timeout: 10 * time.Minute},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string        `json:"model"`
	Messages       []chatMessage `json:"messages"`
	Temperature    float64       `json:"temperature"`
	MaxTokens      int           `json:"max_tokens"`
	ResponseFormat *respFormat   `json:"response_format,omitempty"`
}

type respFormat struct {
	Type string `json:"type"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *llmClient) Chat(ctx context.Context, prompt string, jsonMode bool, maxTokens int) (string, error) {
	start := time.Now()
	req := chatRequest{
		Model:       c.model,
		Messages:    []chatMessage{{Role: "user", Content: prompt}},
		Temperature: 0.1,
		MaxTokens:   maxTokens,
	}
	if jsonMode {
		req.ResponseFormat = &respFormat{Type: "json_object"}
	}
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://openrouter.ai/api/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		logger.Log.Error("classifier.llm_request_failed", logger.Err(err), "model", c.model,
			"описание", "ОШИБКА: Не удалось выполнить запрос к OpenRouter")
		return "", fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		logger.Log.Error("classifier.llm_bad_status", "status", resp.StatusCode, "body", string(raw),
			"описание", "ОШИБКА: OpenRouter вернул ошибочный статус")
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(raw))
	}

	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("openrouter error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("empty choices")
	}

	content := parsed.Choices[0].Message.Content
	logger.Log.Info("classifier.llm_ok",
		"model", c.model,
		"prompt_len", len(prompt),
		"response_len", len(content),
		"duration_ms", time.Since(start).Milliseconds(),
		"описание", "Запрос к OpenRouter успешно выполнен")
	return content, nil
}