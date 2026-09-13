package recommend

import (
	"context"
	"testing"
)

func TestExpandHeuristics(t *testing.T) {
	ex := Expand(context.Background(), "coding assistant on a laptop", 8, nil)
	if ex.Source != "heuristics" {
		t.Fatalf("source %s", ex.Source)
	}
	if ex.Intent.Query != "instruct coder" {
		t.Fatalf("intent query %q", ex.Intent.Query)
	}
	found := false
	for _, q := range ex.Queries {
		if q == "instruct coder" || containsAny(q, "coder") {
			found = true
		}
	}
	if !found {
		t.Fatalf("queries %+v", ex.Queries)
	}
}

func TestRerankNoLLM(t *testing.T) {
	ids := []string{"a/b", "c/d"}
	got, _, err := Rerank(context.Background(), "code", ids, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got[0] != "a/b" || got[1] != "c/d" {
		t.Fatalf("%v", got)
	}
}
