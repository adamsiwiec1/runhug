package runpod

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/config"
	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/version"
)

const (
	APIBase      = "https://api.runpod.io"
	OpenAIBase   = "https://api.runpod.ai/v2"
	DefaultImage = "runpod/worker-v1-vllm:v2.27.0"
)

type Client struct {
	HTTP    *http.Client
	APIKey  string
	BaseURL string
}

func New(apiKey string) *Client {
	return &Client{
		HTTP:    &http.Client{Timeout: 60 * time.Second},
		APIKey:  config.SanitizeAPIKey(apiKey),
		BaseURL: APIBase,
	}
}

type GPU struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Pool         *string      `json:"pool"`
	Memory       float64      `json:"memory"`
	Availability string       `json:"availability"`
	Price        Price        `json:"price"`
	DataCenters  []DataCenter `json:"dataCenters"`
	Manufacturer string       `json:"manufacturer"`
}

type Price struct {
	Community  float64 `json:"community"`
	Secure     float64 `json:"secure"`
	Serverless float64 `json:"serverless"`
}

type DataCenter struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Availability string `json:"availability"`
}

type Endpoint struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Type          string            `json:"type"`
	Image         string            `json:"image"`
	Disk          int               `json:"disk"`
	Env           map[string]string `json:"env"`
	GPU           *GPUConfig        `json:"gpu"`
	Workers       *Workers          `json:"workers"`
	Scaling       *Scaling          `json:"scaling"`
	Timeout       int               `json:"timeout"`
	Flashboot     string            `json:"flashboot"`
	CreatedAt     string            `json:"createdAt"`
	DataCenterIDs []string          `json:"dataCenterIds"`
	RequestURLs   *RequestURLs      `json:"requestUrls"`
}

type GPUConfig struct {
	Pools []string `json:"pools"`
	Count int      `json:"count,omitempty"`
}

type Workers struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

type Scaling struct {
	Type        string  `json:"type"`
	Value       float64 `json:"value"`
	IdleTimeout int     `json:"idleTimeout"`
}

type RequestURLs struct {
	Run     string `json:"run"`
	RunSync string `json:"runSync"`
	Base    string `json:"base"`
}

type CreateEndpointRequest struct {
	Name      string            `json:"name"`
	Image     string            `json:"image"`
	Disk      int               `json:"disk,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	GPU       GPUConfig         `json:"gpu"`
	Workers   *Workers          `json:"workers,omitempty"`
	Scaling   *Scaling          `json:"scaling,omitempty"`
	Timeout   int               `json:"timeout,omitempty"`
	Flashboot string            `json:"flashboot,omitempty"`
}

type Problem struct {
	Title  string   `json:"title"`
	Status int      `json:"status"`
	Detail string   `json:"detail"`
	Errors []string `json:"errors"`
}

func (c *Client) ListGPUs(ctx context.Context) ([]GPU, error) {
	q := url.Values{}
	q.Set("include", "AVAILABILITY")
	q.Set("product", "SERVERLESS")
	var wrap struct {
		GPUs  []GPU `json:"gpus"`
		Items []GPU `json:"items"`
	}
	if err := c.do(ctx, http.MethodGet, "/v2/catalog/gpus?"+q.Encode(), nil, &wrap); err != nil {
		return nil, err
	}
	if len(wrap.GPUs) > 0 {
		return wrap.GPUs, nil
	}
	return wrap.Items, nil
}

func (c *Client) ListEndpoints(ctx context.Context, limit int) ([]Endpoint, error) {
	if limit <= 0 {
		limit = 50
	}
	q := url.Values{}
	q.Set("limit", strconv.Itoa(limit))
	var wrap struct {
		Endpoints []Endpoint `json:"endpoints"`
		Items     []Endpoint `json:"items"`
	}
	if err := c.do(ctx, http.MethodGet, "/v2/serverless?"+q.Encode(), nil, &wrap); err != nil {
		return nil, err
	}
	if len(wrap.Endpoints) > 0 {
		return wrap.Endpoints, nil
	}
	return wrap.Items, nil
}

func (c *Client) GetEndpoint(ctx context.Context, id string) (*Endpoint, error) {
	var ep Endpoint
	if err := c.do(ctx, http.MethodGet, "/v2/serverless/"+url.PathEscape(id), nil, &ep); err != nil {
		return nil, err
	}
	return &ep, nil
}

func (c *Client) CreateEndpoint(ctx context.Context, req CreateEndpointRequest) (*Endpoint, error) {
	var ep Endpoint
	if err := c.do(ctx, http.MethodPost, "/v2/serverless", req, &ep); err != nil {
		return nil, err
	}
	return &ep, nil
}

func (c *Client) DeleteEndpoint(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/v2/serverless/"+url.PathEscape(id), nil, nil)
}

func OpenAIURL(endpointID string) string {
	return strings.TrimRight(OpenAIBase, "/") + "/" + endpointID + "/openai/v1"
}

func (c *Client) do(ctx context.Context, method, path string, body any, dest any) error {
	base := c.BaseURL
	if base == "" {
		base = APIBase
	}
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", version.Name+"/"+version.Version)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key := config.SanitizeAPIKey(c.APIKey); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return err
	}
	if res.StatusCode == http.StatusNoContent {
		return nil
	}
	if res.StatusCode >= 300 {
		return formatAPIError(res.StatusCode, raw)
	}
	if dest == nil || len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return fmt.Errorf("runpod: decode: %w", err)
	}
	return nil
}

func formatAPIError(status int, raw []byte) error {
	var p Problem
	if json.Unmarshal(raw, &p) == nil && (p.Detail != "" || p.Title != "" || len(p.Errors) > 0) {
		msg := strings.TrimSpace(p.Title + ": " + p.Detail)
		if len(p.Errors) > 0 {
			msg += " (" + strings.Join(p.Errors, "; ") + ")"
		}
		return fmt.Errorf("runpod: HTTP %d: %s", status, strings.Trim(msg, ": "))
	}
	s := strings.TrimSpace(string(raw))
	if len(s) > 400 {
		s = s[:400] + "…"
	}
	if s == "" {
		s = http.StatusText(status)
	}
	return fmt.Errorf("runpod: HTTP %d: %s", status, s)
}
