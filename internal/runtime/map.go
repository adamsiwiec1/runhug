package runtime

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/adamsiwiec1/runhug-cli/internal/hf"
)

var sizeToken = regexp.MustCompile(`(?i)(?:^|[^0-9])(\d+(?:\.\d+)?)b(?:[-_]|$)`)

type Recipe struct {
	HF      string
	Ollama  string
	MLX     string
	ParamsB float64
	Task    string
}

func Catalog() []Recipe {
	return []Recipe{
		{HF: "Qwen/Qwen2.5-1.5B-Instruct-GGUF", Ollama: "qwen2.5:1.5b", MLX: "mlx-community/Qwen2.5-1.5B-Instruct-4bit", ParamsB: 1.5, Task: "chat"},
		{HF: "Qwen/Qwen2.5-3B-Instruct-GGUF", Ollama: "qwen2.5:3b", MLX: "mlx-community/Qwen2.5-3B-Instruct-4bit", ParamsB: 3, Task: "chat"},
		{HF: "meta-llama/Llama-3.2-1B-Instruct", Ollama: "llama3.2:1b", MLX: "mlx-community/Llama-3.2-1B-Instruct-4bit", ParamsB: 1, Task: "chat"},
		{HF: "meta-llama/Llama-3.2-3B-Instruct", Ollama: "llama3.2:3b", MLX: "mlx-community/Llama-3.2-3B-Instruct-4bit", ParamsB: 3, Task: "chat"},
		{HF: "HuggingFaceTB/SmolLM2-1.7B-Instruct", Ollama: "smollm2:1.7b", MLX: "mlx-community/SmolLM2-1.7B-Instruct-4bit", ParamsB: 1.7, Task: "chat"},
		{HF: "Qwen/Qwen2.5-Coder-1.5B-Instruct-GGUF", Ollama: "qwen2.5-coder:1.5b", MLX: "mlx-community/Qwen2.5-Coder-1.5B-Instruct-4bit", ParamsB: 1.5, Task: "code"},
		{HF: "Qwen/Qwen2.5-Coder-3B-Instruct-GGUF", Ollama: "qwen2.5-coder:3b", MLX: "mlx-community/Qwen2.5-Coder-3B-Instruct-4bit", ParamsB: 3, Task: "code"},
		{HF: "Qwen/Qwen2.5-Coder-7B-Instruct-GGUF", Ollama: "qwen2.5-coder:7b", MLX: "mlx-community/Qwen2.5-Coder-7B-Instruct-4bit", ParamsB: 7, Task: "code"},
		{HF: "microsoft/Phi-3-mini-4k-instruct", Ollama: "phi3:mini", MLX: "mlx-community/Phi-3-mini-4k-instruct-4bit", ParamsB: 3.8, Task: "chat"},
	}
}

func CatalogModels(task string, maxParamsB float64) []hf.Model {
	var out []hf.Model
	for _, r := range Catalog() {
		if maxParamsB > 0 && r.ParamsB > maxParamsB {
			continue
		}
		if task == "code" && r.Task != "code" {
			continue
		}
		if task != "code" && task != "" && r.Task == "code" {
			continue
		}
		out = append(out, hf.Model{
			ID:          r.HF,
			Author:      author(r.HF),
			Downloads:   50_000,
			Likes:       200,
			Tags:        []string{"gguf", "license:apache-2.0"},
			LibraryName: "gguf",
		})
	}
	return out
}

func MapOllama(repo string) (string, bool) {
	id := strings.ToLower(repo)
	if tag, ok := catalogOllama(repo); ok {
		return tag, true
	}
	size := sizeFromID(id)
	switch {
	case strings.Contains(id, "qwen2.5-coder") || strings.Contains(id, "qwen2.5_coder"):
		return "qwen2.5-coder:" + ollamaSize(size, "1.5b"), true
	case strings.Contains(id, "qwen2.5"):
		return "qwen2.5:" + ollamaSize(size, "1.5b"), true
	case strings.Contains(id, "llama-3.2") || strings.Contains(id, "llama3.2"):
		return "llama3.2:" + ollamaSize(size, "1b"), true
	case strings.Contains(id, "llama-3.1") || strings.Contains(id, "llama3.1"):
		return "llama3.1:" + ollamaSize(size, "8b"), true
	case strings.Contains(id, "phi-3") || strings.Contains(id, "phi3"):
		return "phi3:mini", true
	case strings.Contains(id, "smollm2"):
		return "smollm2:" + ollamaSize(size, "1.7b"), true
	case strings.Contains(id, "gemma-2") || strings.Contains(id, "gemma2"):
		return "gemma2:" + ollamaSize(size, "2b"), true
	}
	return "", false
}

func OllamaRef(repo, quant string) string {
	if tag, ok := MapOllama(repo); ok {
		return tag
	}
	ref := "hf.co/" + strings.TrimPrefix(repo, "https://huggingface.co/")
	if quant != "" {
		return ref + ":" + strings.ToUpper(quant)
	}
	return ref
}

func MapMLX(repo string) string {
	if m, ok := catalogMLX(repo); ok {
		return m
	}
	name := repo
	if i := strings.LastIndex(repo, "/"); i >= 0 {
		name = repo[i+1:]
	}
	name = strings.TrimSuffix(name, "-GGUF")
	name = strings.TrimSuffix(name, "-gguf")
	low := strings.ToLower(name)
	if !strings.Contains(low, "bit") && !strings.Contains(low, "mlx") {
		name += "-4bit"
	}
	return "mlx-community/" + name
}

func catalogOllama(repo string) (string, bool) {
	want := strings.ToLower(shortName(repo))
	for _, r := range Catalog() {
		if strings.EqualFold(r.HF, repo) || strings.ToLower(shortName(r.HF)) == want {
			if r.Ollama != "" {
				return r.Ollama, true
			}
		}
	}
	return "", false
}

func catalogMLX(repo string) (string, bool) {
	if strings.HasPrefix(strings.ToLower(repo), "mlx-community/") {
		return repo, true
	}
	want := strings.ToLower(shortName(repo))
	for _, r := range Catalog() {
		if strings.EqualFold(r.HF, repo) || strings.ToLower(shortName(r.HF)) == want {
			if r.MLX != "" {
				return r.MLX, true
			}
		}
	}
	return "", false
}

func shortName(repo string) string {
	if i := strings.LastIndex(repo, "/"); i >= 0 {
		repo = repo[i+1:]
	}
	repo = strings.TrimSuffix(repo, "-GGUF")
	return strings.TrimSuffix(repo, "-gguf")
}

func author(repo string) string {
	if i := strings.IndexByte(repo, '/'); i > 0 {
		return repo[:i]
	}
	return repo
}

func sizeFromID(id string) float64 {
	ms := sizeToken.FindAllStringSubmatch(id, -1)
	if len(ms) == 0 {
		return 0
	}
	var n float64
	_, _ = fmt.Sscanf(ms[len(ms)-1][1], "%f", &n)
	return n
}

func ollamaSize(n float64, fallback string) string {
	if n <= 0 {
		return fallback
	}
	if n == float64(int(n)) {
		return fmt.Sprintf("%db", int(n))
	}
	return fmt.Sprintf("%gb", n)
}
