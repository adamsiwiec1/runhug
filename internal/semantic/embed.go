package semantic

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/adamsiwiec1/runhug/internal/runtime"
)

const (
	// HFMiniLM is a small sentence-transformers model on the Hub. Used only
	// via Inference feature-extraction (no local download).
	HFMiniLM = "sentence-transformers/all-MiniLM-L6-v2"

	embedTimeout   = 20 * time.Second
	discoverBudget = 800 * time.Millisecond
	ollamaBatch    = 16
	hfBatch        = 8
	hfWorkers      = 4
	maxEmbedChars  = 1500
)

// LexicalNote is printed when the user wanted semantic rank but no embedder ran.
const LexicalNote = "lexical Hub search — pull nomic-embed-text or set HF_TOKEN for semantic rank"

// Embedder turns a query and documents into vectors. Chat completions are not used.
type Embedder interface {
	Label() string
	Embed(ctx context.Context, query string, docs []string) (q []float32, docVecs [][]float32, err error)
}

// Discover prefers a local Ollama embedding model (nomic-embed-text if present),
// then Hugging Face Inference when HF_TOKEN is set. Returns nil if neither works.
// Does not pull models and does not use instruct/chat weights as embedders.
func Discover(ctx context.Context, hfToken string) Embedder {
	if ctx == nil {
		ctx = context.Background()
	}
	dctx, cancel := context.WithTimeout(ctx, discoverBudget)
	defer cancel()

	if runtime.PortOpen(runtime.OllamaPort) {
		base := strings.TrimSuffix(runtime.OllamaURL, "/v1")
		if model := listOllamaEmbed(dctx, base); model != "" {
			return &Ollama{BaseURL: base, Model: model}
		}
	}
	if strings.TrimSpace(hfToken) != "" {
		return &HF{Token: strings.TrimSpace(hfToken), Model: HFMiniLM}
	}
	return nil
}

func clipEmbed(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > maxEmbedChars {
		return s[:maxEmbedChars]
	}
	return s
}

func Cosine(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		x, y := float64(a[i]), float64(b[i])
		dot += x * y
		na += x * x
		nb += y * y
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func toF32(in []float64) []float32 {
	out := make([]float32, len(in))
	for i, v := range in {
		out[i] = float32(v)
	}
	return out
}

func meanPool(tokens [][]float64) []float32 {
	if len(tokens) == 0 {
		return nil
	}
	dim := len(tokens[0])
	sum := make([]float64, dim)
	n := 0
	for _, tok := range tokens {
		if len(tok) != dim {
			continue
		}
		n++
		for i, v := range tok {
			sum[i] += v
		}
	}
	if n == 0 {
		return nil
	}
	out := make([]float32, dim)
	inv := 1 / float64(n)
	for i := range sum {
		out[i] = float32(sum[i] * inv)
	}
	return out
}
