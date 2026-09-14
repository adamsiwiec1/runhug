package runtime

// DefaultModel is a sample local serve mapping (optional init --model / tests).
// Search init no longer installs this; prefer nomic-embed-text for embeddings.
type DefaultModel struct {
	HF     string
	GGUF   string
	Ollama string
	MLX    string
	Why    string
}

func Default() DefaultModel {
	return DefaultModel{
		HF:     "Qwen/Qwen2.5-1.5B-Instruct",
		GGUF:   "Qwen/Qwen2.5-1.5B-Instruct-GGUF",
		Ollama: "qwen2.5:1.5b",
		MLX:    "mlx-community/Qwen2.5-1.5B-Instruct-4bit",
		Why:    "official Qwen Instruct, Apache-2.0, ~1.5B, fits a laptop",
	}
}

func (d DefaultModel) Pull(kind string) string {
	switch Normalize(kind) {
	case Ollama:
		return d.Ollama
	case MLX:
		return d.MLX
	case LlamaCPP:
		return d.GGUF
	default:
		return d.HF
	}
}
