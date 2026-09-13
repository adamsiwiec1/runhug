package jobs

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/runpod/go-sdk/pkg/sdk"
	"github.com/runpod/go-sdk/pkg/sdk/config"
	rpEndpoint "github.com/runpod/go-sdk/pkg/sdk/endpoint"
)

func newEndpoint(apiKey, endpointID string) (*rpEndpoint.Endpoint, error) {
	return rpEndpoint.New(
		&config.Config{ApiKey: sdk.String(apiKey)},
		&rpEndpoint.Option{EndpointId: sdk.String(endpointID)},
	)
}

func Health(apiKey, endpointID string) (*rpEndpoint.HealthOutput, error) {
	ep, err := newEndpoint(apiKey, endpointID)
	if err != nil {
		return nil, err
	}
	return ep.Health(&rpEndpoint.HealthInput{})
}

func Chat(apiKey, endpointID, prompt string, maxTokens int) (string, error) {
	if maxTokens <= 0 {
		maxTokens = 256
	}
	ep, err := newEndpoint(apiKey, endpointID)
	if err != nil {
		return "", err
	}
	out, err := ep.RunSync(&rpEndpoint.RunSyncInput{
		JobInput: &rpEndpoint.JobInput{
			Input: map[string]any{
				"messages": []map[string]string{
					{"role": "user", "content": prompt},
				},
				"sampling_params": map[string]any{
					"max_tokens":  maxTokens,
					"temperature": 0.7,
				},
			},
		},
		Timeout: sdk.Int(300),
	})
	if err != nil {
		return "", err
	}
	if out != nil && out.Error != nil && *out.Error != "" {
		return "", fmt.Errorf("runpod job: %s", *out.Error)
	}
	if out != nil && out.Status != nil && !strings.EqualFold(*out.Status, "COMPLETED") {
		return "", fmt.Errorf("runpod job status %s", *out.Status)
	}
	if out == nil || out.Output == nil {
		return "", fmt.Errorf("runpod job returned no output (cold start may still be in progress)")
	}
	return extractText(*out.Output), nil
}

func extractText(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		if s, ok := t["text"].(string); ok && s != "" {
			return s
		}
		if choices, ok := t["choices"].([]any); ok && len(choices) > 0 {
			if c, ok := choices[0].(map[string]any); ok {
				if msg, ok := c["message"].(map[string]any); ok {
					if s, ok := msg["content"].(string); ok {
						return s
					}
				}
				if s, ok := c["text"].(string); ok {
					return s
				}
				if tokens, ok := c["tokens"].([]any); ok {
					var b strings.Builder
					for _, tok := range tokens {
						if s, ok := tok.(string); ok {
							b.WriteString(s)
						}
					}
					if b.Len() > 0 {
						return b.String()
					}
				}
			}
		}
		if arr, ok := t["output"].([]any); ok && len(arr) > 0 {
			return extractText(arr[0])
		}
	case []any:
		if len(t) > 0 {
			return extractText(t[0])
		}
	}
	raw, _ := json.MarshalIndent(v, "", "  ")
	return string(raw)
}
