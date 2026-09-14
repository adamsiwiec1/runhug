package hf

import (
	"sort"
	"strings"
	"unicode"
)

const (
	maxExtraHubQueries = 4
	maxCardGets        = 100
	cardGetConcurrency = 8
)

// QueryAliases is a small, documented map of extra Hub `search=` terms.
// hacking/hack → pentest, offsec, cybersecurity, bug hunter
// offsec → pentest, red team, cyber
// cybersec → cybersecurity
//
// These are additional list queries (capped with other extras at 4), not
// silent rewrites of the user query.
var QueryAliases = map[string][]string{
	"hacking":     {"pentest", "offsec", "cybersecurity", "bug hunter"},
	"hack":        {"pentest", "offsec", "cybersecurity", "bug hunter"},
	"offsec":      {"pentest", "red team", "cyber"},
	"cybersec":    {"cybersecurity"},
	"penetration": {"pentest", "pentester", "offsec"},
	"pentest":     {"pentester", "offsec", "bug hunter"},
	"cartoon":     {"anime", "animation", "toon"},
	"animated":    {"animation", "anime", "cartoon"},
	"animation":   {"anime", "cartoon", "animated"},
	"uncensored":  {"unfiltered", "abliterated"},
}

// HubCall is one GET /api/models request used for recall.
type HubCall struct {
	Search string
	Task   string
	Filter string // extra filter= tag
}

// HubSearchQueries is the user query plus alias / token extras (search= only).
func HubSearchQueries(query string) []string {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	out := []string{query}
	seen := map[string]bool{strings.ToLower(query): true}
	add := func(s string) {
		s = strings.TrimSpace(s)
		k := strings.ToLower(s)
		if s == "" || seen[k] {
			return
		}
		seen[k] = true
		out = append(out, s)
	}
	for _, a := range AliasTerms(query) {
		add(a)
	}
	tokens := queryTokens(query, 3)
	if len(tokens) <= 3 {
		for _, t := range tokens {
			add(t)
		}
	}
	return out
}

// HubQueryPlan is the primary Hub list call plus at most 4 extras:
// alias/token search=, then task=any when the primary task is specific
// (so image/audio intents still recall mistagged cards), then a single-token
// tag filter.
func HubQueryPlan(query, task string) []HubCall {
	searches := HubSearchQueries(query)
	if len(searches) == 0 {
		return []HubCall{{Task: task}}
	}
	out := []HubCall{{Search: searches[0], Task: task}}
	var extras []HubCall
	for _, s := range searches[1:] {
		extras = append(extras, HubCall{Search: s, Task: task})
	}
	if task != "" && task != "any" {
		extras = append(extras, HubCall{Search: searches[0], Task: "any"})
	}
	if tok := singleQueryToken(query); tok != "" {
		extras = append(extras, HubCall{Filter: tok, Task: task})
	}
	if len(extras) > maxExtraHubQueries {
		extras = extras[:maxExtraHubQueries]
	}
	return append(out, extras...)
}

// AliasTerms returns documented aliases for the query and its tokens.
func AliasTerms(query string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(s string) {
		s = strings.TrimSpace(s)
		k := strings.ToLower(s)
		if s == "" || seen[k] {
			return
		}
		seen[k] = true
		out = append(out, s)
	}
	keys := append([]string{strings.ToLower(strings.TrimSpace(query))}, queryTokens(query, 2)...)
	for _, k := range keys {
		for _, a := range QueryAliases[k] {
			add(a)
		}
	}
	return out
}

// RelevanceTerms is the query, its tokens, and aliases used for local scoring.
func RelevanceTerms(query string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(s string) {
		s = strings.TrimSpace(strings.ToLower(s))
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	add(query)
	for _, t := range queryTokens(query, 2) {
		add(t)
	}
	for _, a := range AliasTerms(query) {
		add(a)
	}
	return out
}

// SearchableText is id + tags + pipeline + card description.
func SearchableText(m Model) string {
	var b strings.Builder
	b.WriteString(m.RepoID())
	b.WriteByte(' ')
	b.WriteString(m.PipelineTag)
	b.WriteByte(' ')
	b.WriteString(strings.Join(m.Tags, " "))
	b.WriteByte(' ')
	b.WriteString(m.CardDescription())
	return b.String()
}

// ScoreRelevance ranks by token/substring overlap of the query (+ aliases)
// against id, tags, pipeline, and description. Description and tag hits are
// boosted. Ties break on likes.
func ScoreRelevance(models []Model, query string) {
	terms := RelevanceTerms(query)
	type row struct {
		m Model
		s float64
	}
	scored := make([]row, len(models))
	for i, m := range models {
		var s float64
		for _, t := range terms {
			s += termScore(m, t)
		}
		scored[i] = row{m: m, s: s}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].s != scored[j].s {
			return scored[i].s > scored[j].s
		}
		return scored[i].m.Likes > scored[j].m.Likes
	})
	for i := range scored {
		models[i] = scored[i].m
	}
}

func termScore(m Model, term string) float64 {
	if term == "" {
		return 0
	}
	var s float64
	if matchTerm(m.RepoID(), term) {
		s += 3
	}
	if matchTerm(strings.Join(m.Tags, " "), term) {
		s += 4
	}
	if matchTerm(m.PipelineTag, term) {
		s += 2
	}
	if d := m.CardDescription(); d != "" && matchTerm(d, term) {
		s += 5
	}
	return s
}

func matchTerm(haystack, term string) bool {
	if term == "" || haystack == "" {
		return false
	}
	h := strings.ToLower(haystack)
	t := strings.ToLower(term)
	if strings.Contains(h, t) {
		return true
	}
	return strings.Contains(compactAlnum(h), compactAlnum(t))
}

func compactAlnum(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return -1
	}, s)
}

func matchesTerms(text string, terms []string) bool {
	for _, t := range terms {
		if matchTerm(text, t) {
			return true
		}
	}
	return false
}

func queryTokens(query string, minLen int) []string {
	var out []string
	for _, w := range strings.Fields(strings.ToLower(query)) {
		w = strings.Trim(w, ",.?!:;\"'+()[]{}")
		if len(w) >= minLen {
			out = append(out, w)
		}
	}
	return out
}

func singleQueryToken(query string) string {
	toks := queryTokens(query, 2)
	if len(toks) == 1 {
		return toks[0]
	}
	return ""
}
