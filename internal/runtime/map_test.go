package runtime

import (
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"Ollama": "ollama", "llama.cpp": "llamacpp", "mlk": "mlx", "MLX-LM": "mlx",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Fatalf("%s: got %s want %s", in, got, want)
		}
	}
}

func TestMapOllama(t *testing.T) {
	cases := []struct {
		repo string
		tag  string
	}{
		{"Qwen/Qwen2.5-Coder-1.5B-Instruct-GGUF", "qwen2.5-coder:1.5b"},
		{"bartowski/Qwen2.5-1.5B-Instruct-GGUF", "qwen2.5:1.5b"},
		{"bartowski/Llama-3.2-1B-Instruct-GGUF", "llama3.2:1b"},
		{"microsoft/Phi-3-mini-4k-instruct-gguf", "phi3:mini"},
	}
	for _, tc := range cases {
		got, ok := MapOllama(tc.repo)
		if !ok || got != tc.tag {
			t.Fatalf("%s: got %q %v want %s", tc.repo, got, ok, tc.tag)
		}
	}
}

func TestOllamaRefFallsBackToHF(t *testing.T) {
	ref := OllamaRef("someorg/mystery-gguf", "q4_k_m")
	if ref != "hf.co/someorg/mystery-gguf:Q4_K_M" {
		t.Fatal(ref)
	}
}

func TestMapMLX(t *testing.T) {
	got := MapMLX("Qwen/Qwen2.5-1.5B-Instruct-GGUF")
	if got != "mlx-community/Qwen2.5-1.5B-Instruct-4bit" {
		t.Fatal(got)
	}
	if MapMLX("mlx-community/already-4bit") != "mlx-community/already-4bit" {
		t.Fatal(MapMLX("mlx-community/already-4bit"))
	}
}

func TestCatalogFiltersCode(t *testing.T) {
	code := CatalogModels("code", 4)
	if len(code) == 0 {
		t.Fatal("expected code recipes")
	}
	for _, m := range code {
		if !strings.Contains(m.RepoID(), "Coder") {
			t.Fatalf("non-code in code catalog: %s", m.RepoID())
		}
	}
}
