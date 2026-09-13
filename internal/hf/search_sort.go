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
		if lic != "other" && lic != "any" {
			out = append(out, "license:"+lic)
		}
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
			continue
		}
		// Unknown --engine values are still allowed; match library or tags.
		if want != EngineVLLM && want != EngineGGUF {
			if strings.EqualFold(m.LibraryName, engine) || hasTagFold(m, engine) {
				out = append(out, m)
			}
		}
	}
	return out
}

func hasTagFold(m Model, want string) bool {
	for _, tag := range m.Tags {
		if strings.EqualFold(tag, want) {
			return true
		}
	}
	return false
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
		if license == "other" {
			if !isCommonLicense(got) {
				out = append(out, m)
			}
			continue
		}
		if licenseMatch(got, license) {
			out = append(out, m)
		}
	}
	return out
}

func licenseMatch(got, want string) bool {
	if got == "" {
		return false
	}
	if got == want || strings.Contains(got, want) || strings.Contains(want, got) {
		return true
	}
	norm := func(s string) string {
		s = strings.ReplaceAll(s, "-", "")
		s = strings.ReplaceAll(s, " ", "")
		return s
	}
	g, w := norm(got), norm(want)
	return g == w || strings.Contains(g, w) || strings.Contains(w, g)
}

func isCommonLicense(lic string) bool {
	if lic == "" {
		return false
	}
	for _, c := range []string{
		"apache", "mit", "bsd", "gpl", "lgpl", "agpl", "mpl",
		"cc-by", "cc0", "cc-sa", "gemma", "llama", "qwen",
		"unlicense", "isc", "zlib", "odc", "openrail",
		"bigscience", "creativeml", "wtfpl", "artistic",
	} {
		if strings.Contains(lic, c) {
			return true
		}
	}
	return false
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
