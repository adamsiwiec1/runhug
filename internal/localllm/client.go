package localllm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL string
	Model   string
	HTTP    *http.Client
}

type chatReq struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRes struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
}

func New(baseURL, model string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Model:   model,
		HTTP:    &http.Client{Timeout: 45 * time.Second},
	}
}

func (c *Client) Chat(ctx context.Context, system, user string) (string, error) {
	if c == nil || c.BaseURL == "" {
		return "", fmt.Errorf("local llm: no base URL")
	}
	model := c.Model
	if model == "" {
		model = "llama3.2"
	}
	body, err := json.Marshal(chatReq{
		Model: model,
		Messages: []message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature: 0.2,
		MaxTokens:   400,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("local llm: HTTP %d: %s", res.StatusCode, trim(raw))
	}
	var out chatRes
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("local llm: decode: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("local llm: empty response")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

func trim(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
