package hf

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Preferred GGUF quants for a laptop: small enough, still usable.
var quantPref = []string{
	"q4_k_m", "q5_k_m", "q4_k_s", "iq4_xs", "q4_0", "q5_0", "q3_k_m", "q6_k", "q8_0",
}

func GGUFSiblings(m Model) []string {
	var out []string
	for _, s := range m.Siblings {
		name := strings.ToLower(s.RFilename)
		if !strings.HasSuffix(name, ".gguf") {
			continue
		}
		if strings.Contains(name, "mmproj") || strings.Contains(name, "imatrix") {
			continue
		}
		out = append(out, s.RFilename)
	}
	return out
}

func PickGGUF(m Model) (string, string, bool) {
	files := GGUFSiblings(m)
	if len(files) == 0 {
		return "", "", false
	}
	bestScore := -1
	best := files[0]
	bestQ := ""
	for _, f := range files {
		low := strings.ToLower(f)
		for i, q := range quantPref {
			if strings.Contains(low, q) {
				score := len(quantPref) - i
				if score > bestScore {
					bestScore = score
					best = f
					bestQ = q
				}
				break
			}
		}
	}
	if bestScore < 0 {
		return best, "unknown", true
	}
	return best, strings.ToUpper(bestQ), true
}

func CacheDir() (string, error) {
	if override := strings.TrimSpace(os.Getenv("RVP_CACHE")); override != "" {
		return override, nil
	}
	home, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "runhug", "gguf"), nil
}

func (c *Client) Download(ctx context.Context, repoID, filename, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	base := c.BaseURL
	if base == "" {
		base = BaseURL
	}
	u := strings.TrimRight(base, "/") + "/" + repoID + "/resolve/main/" + filename + "?download=true"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "runhug")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	httpClient := &http.Client{Timeout: 30 * time.Minute}
	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 400))
		return fmt.Errorf("huggingface download HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, res.Body); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}
