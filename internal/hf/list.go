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

	"github.com/adamsiwiec1/runhug/internal/version"
)

// ListOpts controls paginated Hub model listing (for index packs / deltas).
// Unlike Search, Limit is a total row cap across pages (0 = unlimited until
// Hub stops returning rows, early-stop filters fire, or MaxPages is hit).
// PageSize defaults to 100.
//
// MaxPages is a safety cap on Hub requests. MaxPages <= 0 means a very high
// practical cap (50000 pages) so callers can pull “as many as possible”
// without an accidental infinite loop; pass a positive value to tighten it.
type ListOpts struct {
	Query        string
	Author       string
	Task         string // pipeline_tag; empty or "any" skips
	Library      string
	Filter       string
	Sort         string // downloads, likes, lastModified, createdAt, …
	Direction    string // "-1" (default) or "1"
	Limit        int    // total models to collect (0 = no cap other than MaxPages / early-stop)
	PageSize     int    // per-request limit (1–1000; default 100)
	MaxPages     int    // safety cap; <=0 → 50000 (no practical cap)
	Full         bool
	Sleep        time.Duration // pause between pages (rate limits)
	SinceUnix    int64         // if >0 and Sort=lastModified, stop when page models are older
	MinLikes     int           // skip models with likes < MinLikes (0 = no likes filter)
	MinDownloads int           // skip models with downloads < MinDownloads; with Sort=downloads
	// desc, also early-exits when a whole page is below the threshold
	FilterSkipped *int // optional: incremented for each model skipped by MinLikes/MinDownloads
}

// defaultMaxPages is used when ListOpts.MaxPages <= 0 (unlimited / “as many as possible”).
const defaultMaxPages = 50000

// ListModels pages through GET /api/models following Link: rel="next".
// It does not re-rank locally; use for bulk index building.
func (c *Client) ListModels(ctx context.Context, opts ListOpts) ([]Model, error) {
	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = 100
	}
	if pageSize > 1000 {
		pageSize = 1000
	}
	maxPages := opts.MaxPages
	if maxPages <= 0 {
		maxPages = defaultMaxPages
	}
	sortKey := strings.TrimSpace(opts.Sort)
	if sortKey == "" {
		sortKey = "downloads"
	}
	dir := strings.TrimSpace(opts.Direction)
	if dir == "" {
		dir = "-1"
	}
	sleep := opts.Sleep
	if sleep <= 0 {
		sleep = 200 * time.Millisecond
	}
	downloadsDesc := strings.EqualFold(sortKey, "downloads") && dir == "-1"

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
	if f := strings.TrimSpace(opts.Filter); f != "" {
		q.Add("filter", f)
	}
	q.Set("sort", sortKey)
	q.Set("direction", dir)
	q.Set("limit", strconv.Itoa(pageSize))
	if opts.Full {
		q.Set("full", "true")
		for _, field := range []string{
			"cardData", "likes", "downloads", "tags", "pipeline_tag",
			"library_name", "gated", "author",
		} {
			q.Add("expand", field)
		}
	}

	path := "/api/models?" + q.Encode()
	var out []Model
	seen := map[string]bool{}

	for page := 0; page < maxPages; page++ {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		models, next, err := c.getModelsPage(ctx, path)
		if err != nil {
			return out, err
		}
		if len(models) == 0 {
			break
		}

		// Early exit: sorted by downloads desc and every model on this page
		// is below MinDownloads → further pages only get worse.
		if opts.MinDownloads > 0 && downloadsDesc && pageAllBelowDownloads(models, opts.MinDownloads) {
			break
		}

		stopOlder := false
		for _, m := range models {
			id := m.RepoID()
			if id == "" || seen[id] {
				continue
			}
			if opts.MinLikes > 0 && m.Likes < opts.MinLikes {
				if opts.FilterSkipped != nil {
					*opts.FilterSkipped++
				}
				continue
			}
			if opts.MinDownloads > 0 && m.Downloads < int64(opts.MinDownloads) {
				if opts.FilterSkipped != nil {
					*opts.FilterSkipped++
				}
				continue
			}
			if opts.SinceUnix > 0 && strings.EqualFold(sortKey, "lastModified") {
				lm := parseLastModifiedUnix(m.LastModified)
				if lm > 0 && lm < opts.SinceUnix {
					stopOlder = true
					continue
				}
			}
			seen[id] = true
			out = append(out, m)
			if opts.Limit > 0 && len(out) >= opts.Limit {
				return out[:opts.Limit], nil
			}
		}
		if stopOlder || next == "" {
			break
		}
		path = next
		if page+1 < maxPages {
			select {
			case <-ctx.Done():
				return out, ctx.Err()
			case <-time.After(sleep):
			}
		}
	}
	return out, nil
}

func pageAllBelowDownloads(models []Model, minDownloads int) bool {
	if len(models) == 0 || minDownloads <= 0 {
		return false
	}
	thresh := int64(minDownloads)
	for _, m := range models {
		if m.Downloads >= thresh {
			return false
		}
	}
	return true
}

func parseLastModifiedUnix(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Unix()
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.Unix()
	}
	return 0
}

// getModelsPage fetches one page. nextPath is either a path starting with /
// or empty when there is no Link rel=next.
func (c *Client) getModelsPage(ctx context.Context, pathOrURL string) (models []Model, nextPath string, err error) {
	base := c.BaseURL
	if base == "" {
		base = BaseURL
	}
	reqURL := pathOrURL
	if strings.HasPrefix(pathOrURL, "/") {
		reqURL = base + pathOrURL
	} else if !strings.HasPrefix(pathOrURL, "http://") && !strings.HasPrefix(pathOrURL, "https://") {
		reqURL = base + "/" + strings.TrimPrefix(pathOrURL, "/")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", version.Name+"/"+version.Version)
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 32<<20))
	if err != nil {
		return nil, "", err
	}
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return nil, "", fmt.Errorf("huggingface: HTTP %d (set HF_TOKEN for gated or private models)", res.StatusCode)
	}
	if res.StatusCode >= 300 {
		return nil, "", fmt.Errorf("huggingface: HTTP %d: %s", res.StatusCode, trimBody(body))
	}
	if err := json.Unmarshal(body, &models); err != nil {
		return nil, "", fmt.Errorf("huggingface: decode: %w", err)
	}
	nextPath = parseLinkNext(res.Header.Get("Link"), base)
	return models, nextPath, nil
}

// parseLinkNext extracts the next URL from an RFC 5988 Link header and
// returns a path (or absolute URL) suitable for getModelsPage.
func parseLinkNext(linkHeader, base string) string {
	if linkHeader == "" {
		return ""
	}
	// e.g. <https://huggingface.co/api/models?…>; rel="next", <…>; rel="last"
	parts := strings.Split(linkHeader, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, `rel="next"`) && !strings.Contains(part, `rel=next`) {
			continue
		}
		start := strings.Index(part, "<")
		end := strings.Index(part, ">")
		if start < 0 || end <= start {
			continue
		}
		raw := strings.TrimSpace(part[start+1 : end])
		if strings.HasPrefix(raw, base) {
			return strings.TrimPrefix(raw, base)
		}
		if u, err := url.Parse(raw); err == nil {
			if u.Host == "" {
				return raw
			}
			return raw // absolute next URL
		}
		return raw
	}
	return ""
}
