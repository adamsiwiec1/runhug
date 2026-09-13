package hf

import (
	"fmt"
	"sort"
	"strings"
)

// NormalizeSort accepts relevance (default), likes, or downloads.
func NormalizeSort(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "", "relevance", "relevant", "rank":
		return "relevance", nil
	case "likes", "downloads":
		return s, nil
	default:
		return "", fmt.Errorf("unsupported --sort %q (use relevance, likes, or downloads)", s)
	}
}

func searchFilters(opts SearchOpts) []string {
	var out []string
	if f := strings.TrimSpace(opts.Filter); f != "" {
		out = append(out, f)
	}
	if lic := strings.TrimSpace(opts.License); lic != "" {
		lic = strings.TrimPrefix(strings.ToLower(lic), "license:")
		out = append(out, "license:"+lic)
	}
	switch strings.ToLower(strings.TrimSpace(opts.Engine)) {
	case "gguf":
		out = append(out, "gguf")
	case "vllm", "safetensors":
		out = append(out, "safetensors")
	}
	return out
}

func filterByEngine(models []Model, engine string) []Model {
	engine = strings.ToLower(strings.TrimSpace(engine))
	if engine == "" || engine == "any" {
		return models
	}
	want := Engine(engine)
	if engine == "safetensors" {
		want = EngineVLLM
	}
	out := models[:0:0]
	for _, m := range models {
		got := DetectFormat(m).Engine
		if got == want {
			out = append(out, m)
		}
	}
	return out
}

func filterByLicense(models []Model, license string) []Model {
	license = strings.TrimSpace(strings.ToLower(license))
	license = strings.TrimPrefix(license, "license:")
	if license == "" || license == "any" {
		return models
	}
	out := models[:0:0]
	for _, m := range models {
		got := strings.ToLower(m.License())
		if got == license || strings.Contains(got, license) {
			out = append(out, m)
		}
	}
	return out
}

// SortModels re-ranks a relevance pool by likes or downloads (descending).
func SortModels(models []Model, key string) {
	switch strings.ToLower(key) {
	case "downloads":
		sort.SliceStable(models, func(i, j int) bool {
			return models[i].Downloads > models[j].Downloads
		})
	case "likes":
		sort.SliceStable(models, func(i, j int) bool {
			return models[i].Likes > models[j].Likes
		})
	}
}
