package runtime

import "testing"

func TestDefaultModel(t *testing.T) {
	d := Default()
	if d.HF != "Qwen/Qwen2.5-1.5B-Instruct" || d.Ollama != "qwen2.5:1.5b" {
		t.Fatalf("%+v", d)
	}
	if d.Pull(Ollama) != "qwen2.5:1.5b" {
		t.Fatalf("ollama pull %s", d.Pull(Ollama))
	}
	if d.Pull(MLX) != d.MLX {
		t.Fatalf("mlx pull %s", d.Pull(MLX))
	}
	if d.Pull(LlamaCPP) != d.GGUF {
		t.Fatalf("gguf %s", d.Pull(LlamaCPP))
	}
}
