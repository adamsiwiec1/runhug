package runtime

import (
	"os"
	"strings"
)

const (
	Ollama   = "ollama"
	LlamaCPP = "llamacpp"
	MLX      = "mlx"
)

const (
	OllamaURL  = "http://127.0.0.1:11434/v1"
	OllamaPort = 11434
	LlamaPort  = 8081
	MLXPort    = 8082
	MLXURL     = "http://127.0.0.1:8082/v1"
)

func Normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "ollama":
		return Ollama
	case "llama", "llama.cpp", "llamacpp", "llama-cpp", "llama-server":
		return LlamaCPP
	case "mlx", "mlk", "mlx-lm", "mlx_lm", "apple":
		return MLX
	default:
		return s
	}
}

func FromEnv() string {
	return Normalize(os.Getenv("RVP_RUNTIME"))
}

func AllKinds() []string {
	return []string{Ollama, LlamaCPP, MLX}
}
