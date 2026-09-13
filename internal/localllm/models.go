package localllm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type modelsRes struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// FirstModel returns the first model id from GET /v1/models.
func (c *Client) FirstModel(ctx context.Context) (string, error) {
	if c == nil || c.BaseURL == "" {
		return "", fmt.Errorf("local llm: no base URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/models", nil)
	if err != nil {
		return "", err
	}
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
		return "", fmt.Errorf("local llm: models HTTP %d: %s", res.StatusCode, trim(raw))
	}
	var out modelsRes
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	for _, m := range out.Data {
		id := strings.TrimSpace(m.ID)
		if id != "" {
			return id, nil
		}
	}
	return "", fmt.Errorf("local llm: no models loaded")
}
