package cli

import (
	"testing"

	"github.com/adamsiwiec/runpod-vllm-proxy/internal/hf"
	"github.com/adamsiwiec/runpod-vllm-proxy/internal/runtime"
)

func TestSpecFromOverride(t *testing.T) {
	s := specFromOverride("qwen3:8b", runtime.Ollama)
	if s.Ollama != "qwen3:8b" || s.HF != "qwen3:8b" {
		t.Fatalf("%+v", s)
	}
	s = specFromOverride("Qwen/Qwen2.5-7B-Instruct", runtime.Ollama)
	if s.HF != "Qwen/Qwen2.5-7B-Instruct" || s.Ollama != "" {
		t.Fatalf("%+v", s)
	}
}

func TestSpecFromDefault(t *testing.T) {
	s := specFromDefault(runtime.Default())
	if s.Ollama != "qwen2.5:1.5b" || s.HF != "Qwen/Qwen2.5-1.5B-Instruct" {
		t.Fatalf("%+v", s)
	}
}

func TestSpecFromHub(t *testing.T) {
	s := specFromHub(hf.Model{ID: "Qwen/Qwen2.5-1.5B-Instruct-GGUF"}, runtime.Ollama)
	if s.Ollama != "qwen2.5:1.5b" {
		t.Fatalf("mapped %s", s.Ollama)
	}
}
