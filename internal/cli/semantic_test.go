package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/adamsiwiec1/runhug/internal/hf"
	"github.com/adamsiwiec1/runhug/internal/semantic"
)

func TestRankSemanticSkipsLikes(t *testing.T) {
	models := []hf.Model{{ID: "low", Likes: 1}, {ID: "high", Likes: 9}}
	got, note := rankSemantic(context.Background(), "hacking", "likes", true, models, func() semantic.Embedder {
		t.Fatal("must not discover embedder for likes")
		return nil
	})
	if note != "" || got[0].ID != "low" {
		t.Fatalf("likes must keep pool order here: %s %q", got[0].ID, note)
	}
}

func TestRankSemanticLexicalNote(t *testing.T) {
	models := []hf.Model{
		{ID: "acme/chat", Description: "general chat"},
		{ID: "lab/pentest", Description: "offsec helper"},
	}
	got, note := rankSemantic(context.Background(), "hacking", "relevance", true, models, func() semantic.Embedder {
		return nil
	})
	if note != semantic.LexicalNote {
		t.Fatalf("note %q", note)
	}
	if got[0].ID != "acme/chat" {
		t.Fatal("lexical order preserved")
	}
}

type stubCLIEmbed struct{}

func (stubCLIEmbed) Label() string { return "stub (test)" }

func (stubCLIEmbed) Embed(_ context.Context, query string, docs []string) ([]float32, [][]float32, error) {
	q := cliBag(query)
	out := make([][]float32, len(docs))
	for i, d := range docs {
		out[i] = cliBag(d)
	}
	return q, out, nil
}

func cliBag(s string) []float32 {
	s = strings.ToLower(s)
	hit := float32(0)
	for _, k := range []string{"hack", "pentest", "offsec"} {
		if strings.Contains(s, k) {
			hit = 1
		}
	}
	return []float32{hit, 1 - hit}
}

func TestRankSemanticStubRerank(t *testing.T) {
	models := []hf.Model{
		{ID: "acme/chat", Description: "general assistant"},
		{ID: "lab/ops", Description: "authorized pentest agent"},
	}
	got, note := rankSemantic(context.Background(), "hacking", "relevance", true, models, func() semantic.Embedder {
		return stubCLIEmbed{}
	})
	if !strings.Contains(note, "stub") {
		t.Fatalf("note %q", note)
	}
	if got[0].ID != "lab/ops" {
		t.Fatalf("got %s", got[0].ID)
	}
}

func TestRankSemanticNoSemanticFlag(t *testing.T) {
	models := []hf.Model{{ID: "a"}, {ID: "b"}}
	got, note := rankSemantic(context.Background(), "q", "relevance", false, models, func() semantic.Embedder {
		t.Fatal("disabled")
		return nil
	})
	if note != "" || got[0].ID != "a" {
		t.Fatalf("%v %q", got, note)
	}
}
