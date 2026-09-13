package hf

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/version"
)

const BaseURL = "https://huggingface.co"

type Client struct {
	HTTP    *http.Client
	Token   string
	BaseURL string
}

func New(token string) *Client {
	return &Client{
		HTTP:    &http.Client{Timeout: 30 * time.Second},
		Token:   token,
		BaseURL: BaseURL,
	}
}

type SearchOpts struct {
	Query   string
	Author  string
	Task    string
	Library string
	Filter  string
	License string // apache-2.0, mit, gemma, other, …
	Engine  string // vllm, gguf, or any DetectFormat engine string
	Sort    string // relevance (default), likes, downloads
	Limit   int
	Offset  int // Hub has no official offset; ignored by Search
	Full    bool
}

type Model struct {
	ID           string         `json:"id"`
	ModelID      string         `json:"modelId"`
	Author       string         `json:"author"`
	PipelineTag  string         `json:"pipeline_tag"`
	LibraryName  string         `json:"library_name"`
	Tags         []string       `json:"tags"`
	Downloads    int64          `json:"downloads"`
	Likes        int            `json:"likes"`
	Private      bool           `json:"private"`
	Gated        any            `json:"gated"`
	LastModified string         `json:"lastModified"`
	SHA          string         `json:"sha"`
	UsedStorage  int64          `json:"usedStorage"`
	Siblings     []Sibling      `json:"siblings"`
	Safetensors  *Safetensors   `json:"safetensors"`
	CardData     map[string]any `json:"cardData"`
	Config       map[string]any `json:"config"`
}

type Sibling struct {
	RFilename string `json:"rfilename"`
}

type Safetensors struct {
	Total      int64            `json:"total"`
	Parameters map[string]int64 `json:"parameters"`
}

func (m Model) RepoID() string {
	if m.ID != "" {
		return m.ID
	}
	return m.ModelID
}

func (m Model) IsGated() bool {
	switch v := m.Gated.(type) {
	case nil:
		return false
	case bool:
		return v
	case string:
		s := strings.TrimSpace(strings.ToLower(v))
		return s != "" && s != "false"
	default:
		return true
	}
}

func (m Model) License() string {
	for _, tag := range m.Tags {
		if strings.HasPrefix(tag, "license:") {
			return strings.TrimPrefix(tag, "license:")
		}
	}
	if m.CardData != nil {
		if lic, ok := m.CardData["license"].(string); ok && lic != "" {
			return lic
		}
	}
	return ""
}

func (c *Client) Search(ctx context.Context, opts SearchOpts) ([]Model, error) {
	if opts.Limit <= 0 {
		opts.Limit = 15
	}
	if opts.Limit > 100 {
		opts.Limit = 100
	}
	if opts.Task == "" {
		opts.Task = "text-generation"
	}
	sortKey, err := NormalizeSort(opts.Sort)
	if err != nil {
		return nil, err
	}
	opts.Sort = sortKey

	// likes/downloads: fetch a relevance pool, then re-rank locally.
	// GET /api/models text relevance is the unsorted default (omit sort).
	fetchLimit := opts.Limit
	apiSort := sortKey
	if sortKey == "likes" || sortKey == "downloads" {
		fetchLimit = 100
		if opts.Limit > fetchLimit {
			fetchLimit = opts.Limit
		}
		apiSort = "relevance"
	}

	q := url.Values{}
	if opts.Query != "" {
		q.Set("search", opts.Query)
	}
	if opts.Author != "" {
		q.Set("author", opts.Author)
	}
	if opts.Task != "" && opts.Task != "any" {
		q.Set("pipeline_tag", opts.Task)
	}
	if opts.Library != "" {
		q.Set("library", opts.Library)
	}
	for _, f := range searchFilters(opts) {
		q.Add("filter", f)
	}
	if apiSort != "relevance" {
		q.Set("sort", apiSort)
		q.Set("direction", "-1")
	}
	q.Set("limit", strconv.Itoa(fetchLimit))
	if opts.Full {
		q.Set("full", "true")
	}

	var models []Model
	if err := c.get(ctx, "/api/models?"+q.Encode(), &models); err != nil {
		return nil, err
	}
	models = filterByEngine(models, opts.Engine)
	models = filterByLicense(models, opts.License)
	if sortKey == "likes" || sortKey == "downloads" {
		SortModels(models, sortKey)
	}
	if len(models) > opts.Limit {
		models = models[:opts.Limit]
	}
	return models, nil
}

func (c *Client) Get(ctx context.Context, repoID string) (*Model, error) {
	repoID = strings.TrimSpace(strings.TrimPrefix(repoID, "https://huggingface.co/"))
	repoID = strings.Trim(repoID, "/")
	if repoID == "" {
		return nil, fmt.Errorf("model id is required (org/name)")
	}
	var m Model
	if err := c.get(ctx, "/api/models/"+repoID, &m); err != nil {
		return nil, err
	}
	if m.ID == "" {
		m.ID = repoID
	}
	return &m, nil
}

func (c *Client) get(ctx context.Context, path string, dest any) error {
	base := c.BaseURL
	if base == "" {
		base = BaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", version.Name+"/"+version.Version)
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return err
	}
	if res.StatusCode == http.StatusNotFound {
		return fmt.Errorf("huggingface: model not found")
	}
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return fmt.Errorf("huggingface: HTTP %d (set HF_TOKEN for gated or private models)", res.StatusCode)
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("huggingface: HTTP %d: %s", res.StatusCode, trimBody(body))
	}
	if dest == nil {
		return nil
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("huggingface: decode: %w", err)
	}
	return nil
}

func trimBody(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 300 {
		return s[:300] + "…"
	}
	return s
}
