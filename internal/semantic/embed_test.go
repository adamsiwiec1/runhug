package semantic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/adamsiwiec1/runhug-cli/internal/hf"
)

func TestPickOllamaEmbedSkipsChat(t *testing.T) {
	got := PickOllamaEmbed([]string{"qwen2.5:1.5b", "llama3.2:latest", "nomic-embed-text:latest"})
	if got != "nomic-embed-text:latest" {
		t.Fatalf("got %q", got)
	}
	if PickOllamaEmbed([]string{"qwen2.5:1.5b", "qwen2.5:1.5b-instruct"}) != "" {
		t.Fatal("chat models must not be embedders")
	}
	if PickOllamaEmbed([]string{"all-minilm:latest"}) != "all-minilm:latest" {
		t.Fatal("minilm")
	}
}

func TestCosine(t *testing.T) {
	if Cosine([]float32{1, 0}, []float32{1, 0}) < 0.99 {
		t.Fatal("identical")
	}
	if Cosine([]float32{1, 0}, []float32{0, 1}) > 0.01 {
		t.Fatal("orthogonal")
	}
	if Cosine([]float32{1}, []float32{1, 2}) != 0 {
		t.Fatal("dim mismatch")
	}
}

type stubEmbed struct{}

func (stubEmbed) Label() string { return "stub" }

func (stubEmbed) Embed(_ context.Context, query string, docs []string) ([]float32, [][]float32, error) {
	q := bag(query)
	out := make([][]float32, len(docs))
	for i, d := range docs {
		out[i] = bag(d)
	}
	return q, out, nil
}

func bag(s string) []float32 {
	s = strings.ToLower(s)
	return []float32{
		has(s, "hack", "pentest", "offsec", "cyber"),
		has(s, "chat", "general", "assistant"),
		has(s, "qwen", "instruct"),
	}
}

func has(s string, keys ...string) float32 {
	for _, k := range keys {
		if strings.Contains(s, k) {
			return 1
		}
	}
	return 0
}

func TestRerankPrefersCardSemantics(t *testing.T) {
	models := []hf.Model{
		{ID: "acme/general-chat-7b", Tags: []string{"text-generation"}, Description: "a friendly general chat assistant"},
		{ID: "lab/ops-agent", Tags: []string{"text-generation"}, Description: "authorized pentest and offsec helper"},
	}
	got, err := Rerank(context.Background(), "hacking", models, stubEmbed{})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].RepoID() != "lab/ops-agent" {
		t.Fatalf("semantic rank: %s", got[0].RepoID())
	}
}

func TestRerankKeepsLexicalOnNilEmbedder(t *testing.T) {
	models := []hf.Model{{ID: "a"}, {ID: "b"}}
	got, err := Rerank(context.Background(), "x", models, nil)
	if err != nil || got[0].ID != "a" {
		t.Fatalf("%v %v", got, err)
	}
}

func TestOllamaEmbedBatch(t *testing.T) {
	var gotInputs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embed" {
			t.Errorf("path %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		var req ollamaEmbedReq
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotInputs = req.Input
		emb := make([][]float64, len(req.Input))
		for i := range req.Input {
			emb[i] = []float64{1, 0}
		}
		_ = json.NewEncoder(w).Encode(ollamaEmbedRes{Embeddings: emb})
	}))
	t.Cleanup(srv.Close)
	o := &Ollama{BaseURL: srv.URL, Model: "nomic-embed-text", HTTP: srv.Client()}
	q, docs, err := o.Embed(context.Background(), "hacking", []string{"pentest model", "chat model"})
	if err != nil {
		t.Fatal(err)
	}
	if len(q) != 2 || len(docs) != 2 {
		t.Fatalf("dims q=%d docs=%d", len(q), len(docs))
	}
	if len(gotInputs) != 3 || !strings.HasPrefix(gotInputs[0], "search_query:") {
		t.Fatalf("nomic prefixes: %v", gotInputs)
	}
	if !strings.HasPrefix(gotInputs[1], "search_document:") {
		t.Fatalf("doc prefix: %v", gotInputs)
	}
}

func TestHFFeatureExtractionParse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("auth %s", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`[[0.1,0.9],[0.8,0.1],[0.0,1.0]]`))
	}))
	t.Cleanup(srv.Close)
	h := &HF{Token: "tok", Model: HFMiniLM, Endpoint: srv.URL, HTTP: srv.Client()}
	q, docs, err := h.Embed(context.Background(), "q", []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(q) != 2 || len(docs) != 2 {
		t.Fatalf("q=%v docs=%d", q, len(docs))
	}
}

func TestParseFeatureExtractionShapes(t *testing.T) {
	got, err := parseFeatureExtraction([]byte(`[0.1, 0.2]`), 1)
	if err != nil || len(got) != 1 || len(got[0]) != 2 {
		t.Fatalf("%v %v", got, err)
	}
	got, err = parseFeatureExtraction([]byte(`[[1,0],[0,1]]`), 2)
	if err != nil || len(got) != 2 {
		t.Fatalf("%v %v", got, err)
	}
}
