package recommend

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/adamsiwiec1/runhug-cli/internal/localllm"
)

type Expansion struct {
	Queries []string `json:"queries"`
	Intent  Intent
	Source  string // heuristics | local-llm-rerank
	Notes   string `json:"notes,omitempty"`
}

// Expand builds Hub keyword queries from a natural-language use case.
// Query expansion is always heuristic (stable Hub keywords). The optional
// local LLM is used later to re-rank Hub candidates, not to invent search terms.
func Expand(ctx context.Context, raw string, ramGB float64, _ *localllm.Client) Expansion {
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

const rerankSystem = `You re-rank Hugging Face model ids for a use case.
Reply with ONLY JSON: {"order":["org/name", "..."],"notes":"short"}
Rules:
- Reorder the given ids best-first for the use case
- Only use ids from the provided list
- Prefer well-known publishers and instruct/chat/coder variants when relevant
- No markdown`

// Rerank asks the local LLM to reorder candidate repo ids. On failure it
// returns the input order unchanged.
func Rerank(ctx context.Context, useCase string, ids []string, llm *localllm.Client) ([]string, string, error) {
	if llm == nil || len(ids) < 2 {
		return ids, "", nil
	}
	if llm.Model == "" {
		if id, err := llm.FirstModel(ctx); err == nil {
			llm.Model = id
		}
	}
	// Cap prompt size.
	list := ids
	if len(list) > 20 {
		list = list[:20]
	}
	user := fmt.Sprintf("Use case: %s\nCandidates:\n%s", useCase, strings.Join(list, "\n"))
	text, err := llm.Chat(ctx, rerankSystem, user)
	if err != nil {
		return ids, "", err
	}
	text = stripFence(text)
	var parsed struct {
		Order []string `json:"order"`
		Notes string   `json:"notes"`
	}
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		return ids, "", err
	}
	allowed := map[string]bool{}
	for _, id := range ids {
		allowed[id] = true
	}
	var out []string
	seen := map[string]bool{}
	for _, id := range parsed.Order {
		id = strings.TrimSpace(id)
		if !allowed[id] || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	for _, id := range ids {
		if !seen[id] {
			out = append(out, id)
		}
	}
	return out, strings.TrimSpace(parsed.Notes), nil
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

func stripFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```JSON")
		s = strings.TrimPrefix(s, "```")
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = s[:i]
		}
	}
	return strings.TrimSpace(s)
}
