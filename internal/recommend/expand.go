package recommend

import (
	"context"
	"strings"
)

type Expansion struct {
	Queries []string `json:"queries"`
	Intent  Intent
	Source  string // heuristics
	Notes   string `json:"notes,omitempty"`
}

// Expand builds Hub keyword queries from a natural-language use case.
// Query expansion is always heuristic (stable Hub keywords). Chat/local LLM
// re-rank is not used — search ranking is lexical + semantic embeddings.
func Expand(ctx context.Context, raw string, ramGB float64) Expansion {
	_ = ctx
	in := ParseIntent(raw, ramGB)
	ex := Expansion{Intent: in, Source: "heuristics", Queries: []string{}}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		ex.Queries = []string{in.Query}
		return ex
	}
	// First queries: preserve distinctive raw tokens from user input as separate queries for OR coverage.
	rawTokens := ExtractDistinctiveTokens(raw)
	for _, tok := range rawTokens {
		ex.Queries = append(ex.Queries, tok)
	}
	// Also try the full phrase if there are multiple distinctive tokens.
	if len(rawTokens) > 1 {
		ex.Queries = append(ex.Queries, strings.Join(rawTokens, " "))
	}
	// Add intent-based query if different from raw tokens.
	if in.Query != "" {
		ex.Queries = append(ex.Queries, in.Query)
	}
	// Extra Hub-friendly seeds from the raw text (family names, task words).
	for _, seed := range hubSeeds(raw, in) {
		ex.Queries = append(ex.Queries, seed)
	}
	ex.Queries = uniqQueries(ex.Queries)
	return ex
}

// ExtractDistinctiveTokens returns meaningful tokens from raw query,
// filtering out common stop words and short tokens.
func ExtractDistinctiveTokens(raw string) []string {
	stopWords := map[string]bool{
		"for": true, "and": true, "the": true, "with": true, "use": true,
		"case": true, "want": true, "need": true, "that": true, "this": true,
		"chat": true, "model": true, "help": true, "find": true, "get": true,
		"run": true, "try": true, "test": true, "make": true, "can": true,
		"how": true, "what": true, "which": true, "when": true, "where": true,
	}
	var tokens []string
	for _, word := range strings.Fields(strings.ToLower(raw)) {
		word = strings.Trim(word, ",.?!:;\"'+()[]{}")
		if len(word) < 3 || stopWords[word] {
			continue
		}
		tokens = append(tokens, word)
	}
	return tokens
}

func hubSeeds(raw string, in Intent) []string {
	s := strings.ToLower(raw)
	var out []string
	switch in.Task {
	case "code":
		out = append(out, "qwen2.5 coder instruct", "deepseek coder instruct")
	case "reason":
		out = append(out, "instruct", "qwen2.5 instruct")
	default:
		out = append(out, "instruct")
	}
	for _, fam := range []string{"qwen", "llama", "mistral", "gemma", "deepseek", "phi"} {
		if strings.Contains(s, fam) {
			out = append(out, fam+" instruct")
		}
	}
	return out
}

func uniqQueries(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, q := range in {
		q = strings.TrimSpace(q)
		k := strings.ToLower(q)
		if q == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, q)
		if len(out) >= 6 {
			break
		}
	}
	if len(out) == 0 {
		return []string{"instruct"}
	}
	return out
}
