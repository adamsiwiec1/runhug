package recommend

import (
	"context"
	"strings"
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

func TestExpandDistinctiveTerms(t *testing.T) {
	ex := Expand(context.Background(), "chat heretic cybersecurity", 8, nil)
	if ex.Source != "heuristics" {
		t.Fatalf("source %s", ex.Source)
	}
	foundHeretic := false
	foundCybersecurity := false
	for _, q := range ex.Queries {
		qLower := strings.ToLower(q)
		if strings.Contains(qLower, "heretic") {
			foundHeretic = true
		}
		if strings.Contains(qLower, "cybersecurity") {
			foundCybersecurity = true
		}
	}
	if !foundHeretic || !foundCybersecurity {
		t.Fatalf("expected queries to contain 'heretic' and 'cybersecurity', got: %+v", ex.Queries)
	}
}

func TestExtractDistinctiveTokens(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{
			input:    "chat heretic cybersecurity",
			expected: []string{"heretic", "cybersecurity"},
		},
		{
			input:    "coding assistant for scripts",
			expected: []string{"coding", "assistant", "scripts"},
		},
		{
			input:    "the model with chat",
			expected: []string{},
		},
	}
	for _, tt := range tests {
		got := ExtractDistinctiveTokens(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("ExtractDistinctiveTokens(%q) = %v, want %v", tt.input, got, tt.expected)
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("ExtractDistinctiveTokens(%q) = %v, want %v", tt.input, got, tt.expected)
				break
			}
		}
	}
}

func TestExpandSingleWordHeretic(t *testing.T) {
	ex := Expand(context.Background(), "heretic", 8, nil)
	t.Logf("Queries = %+v", ex.Queries)
	t.Logf("Intent.Query = %q", ex.Intent.Query)
	found := false
	for _, q := range ex.Queries {
		if q == "heretic" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected 'heretic' in queries, got: %+v", ex.Queries)
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
