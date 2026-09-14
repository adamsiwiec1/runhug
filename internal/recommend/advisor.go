package recommend

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

const DefaultAdvisorBaseURL = "http://127.0.0.1:11434/v1"

// AdvisorOpts configures an OpenAI-compatible /v1/chat/completions call.
// APIKey is never logged by this package.
type AdvisorOpts struct {
	BaseURL string
	Model   string
	APIKey  string
	Timeout time.Duration
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Advise compares ONLY the provided candidate ids for the user query.
// Returns the assistant message text.
func Advise(ctx context.Context, opts AdvisorOpts, query string, candidates []Scored, gpuHints []string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(opts.BaseURL), "/")
	if base == "" {
		base = DefaultAdvisorBaseURL
	}
	model := strings.TrimSpace(opts.Model)
	if model == "" {
		return "", fmt.Errorf("advisor model required (pass --model or: config set advisor_model <name>)")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 90 * time.Second
	}

	var b strings.Builder
	b.WriteString("You are a careful local-model advisor. Compare ONLY the candidate Hugging Face models listed below for the user's use case. ")
	b.WriteString("Do not invent other models. Prefer fits for laptop RAM / VRAM / cloud GPU budget when mentioned. ")
	b.WriteString("Mention rough VRAM and a Runpod-style GPU pool when hints are provided. Be concise: pick a winner, 2–4 bullets why, and one runner-up.\n\n")
	b.WriteString("User request:\n")
	b.WriteString(strings.TrimSpace(query))
	b.WriteString("\n\nCandidates:\n")
	for i, c := range candidates {
		fmt.Fprintf(&b, "%d. %s  score=%.3f  likes=%d  downloads=%d  params≈%.1fB  engine=%s  quant=%s  why=%s\n",
			i+1, c.Model.RepoID(), c.Score, c.Model.Likes, c.Model.Downloads, c.ParamsB, c.Engine, c.Quant, strings.Join(c.Why, ", "))
		if i < len(gpuHints) && gpuHints[i] != "" {
			fmt.Fprintf(&b, "   GPU hint: %s\n", gpuHints[i])
		}
	}

	payload, err := json.Marshal(chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: "You recommend Hugging Face models from a fixed shortlist only."},
			{Role: "user", Content: b.String()},
		},
	})
	if err != nil {
		return "", err
	}

	url := base + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if key := strings.TrimSpace(opts.APIKey); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	client := &http.Client{Timeout: opts.Timeout}
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("advisor unreachable at %s: %w\nHint: start Ollama (`ollama serve`) or pass --base-url / --api-key-env; or use --no-llm for the scored shortlist only", base, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return "", err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if len(msg) > 240 {
			msg = msg[:240] + "…"
		}
		return "", fmt.Errorf("advisor HTTP %d from %s: %s\nHint: check --model / advisor_model, or use --no-llm", res.StatusCode, base, msg)
	}
	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("advisor response: %w", err)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return "", fmt.Errorf("advisor error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("advisor returned empty completion")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}
