package recommend

import (
	"fmt"
	"math"
	"strings"

	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/hf"
	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/sizing"
)

// Known Hub publishers. Likes are Hub "stars".
var credibility = map[string]float64{
	"huggingface":           1.0,
	"huggingfacetb":         1.0,
	"qwen":                  0.95,
	"meta-llama":            0.95,
	"google":                0.95,
	"microsoft":             0.95,
	"mistralai":             0.95,
	"deepseek-ai":           0.90,
	"ggml-org":              0.90,
	"bartowski":             0.88,
	"unsloth":               0.85,
	"tinyllama":             0.75,
	"stabilityai":           0.80,
	"thebloke":              0.70,
	"lmstudio-community":    0.80,
	"quantfactory":          0.78,
	"maziyarpanahi":         0.75,
	"ibm-granite":           0.90,
	"nvidia":                0.85,
	"apple":                 0.85,
	"01-ai":                 0.80,
	"thudm":                 0.80,
	"internlm":              0.78,
	"nousresearch":          0.78,
	"cognitivecomputations": 0.72,
	"open-orca":             0.72,
}

type Scored struct {
	Model   hf.Model
	Score   float64
	Why     []string
	Engine  hf.Engine
	ParamsB float64
	Quant   string
	License string
}

func Score(models []hf.Model, in Intent) []Scored {
	out := make([]Scored, 0, len(models))
	var maxDL, maxLikes float64
	for _, m := range models {
		if float64(m.Downloads) > maxDL {
			maxDL = float64(m.Downloads)
		}
		if float64(m.Likes) > maxLikes {
			maxLikes = float64(m.Likes)
		}
	}
	if maxDL < 1 {
		maxDL = 1
	}
	if maxLikes < 1 {
		maxLikes = 1
	}

	for _, m := range models {
		format := hf.DetectFormat(m)
		if format.Engine == hf.EngineGGUF {
			if _, q, ok := hf.PickGGUF(m); ok {
				format.Quant = q
			} else if format.Quant == "" {
				format.Quant = "q4"
			}
		}
		est := sizing.EstimateModel(m, format, 4096)
		paramsB := float64(est.Params) / 1e9
		if in.MaxParamsB > 0 && paramsB > in.MaxParamsB && paramsB > 0 {
			continue
		}

		author := authorOf(m)
		cred, known := credibility[strings.ToLower(author)]
		if !known {
			cred = 0.40
			if m.Likes >= 100 {
				cred = 0.55
			}
		}

		pop := 0.35*math.Log1p(float64(m.Downloads))/math.Log1p(maxDL) +
			0.25*math.Log1p(float64(m.Likes))/math.Log1p(maxLikes)
		sizeFit := 0.45
		if in.MaxParamsB > 0 && paramsB > in.MaxParamsB && paramsB > 0 {
			sizeFit = 0.15
		} else if in.TargetParamsB > 0 && paramsB > 0 {
			delta := math.Abs(paramsB-in.TargetParamsB) / (in.TargetParamsB + 0.5)
			sizeFit = math.Max(0.25, 1-0.45*delta)
		}
		lic := licenseScore(m.License())
		instruct := 0.0
		id := strings.ToLower(m.RepoID())
		if containsAny(id, "instruct", "-it", "chat", "coder") {
			instruct = 1
		}

		overlap, hasDistinctive := queryOverlap(id, m.PipelineTag, in.Raw, in.Query, in.Task)
		
		// When distinctive terms exist in the query, boost matches dramatically.
		hasDistinctiveQuery := len(ExtractDistinctiveTokens(in.Raw)) > 0
		overlapWeight := 0.25
		popWeight := pop
		credWeight := 0.20
		instructWeight := 0.14
		
		if hasDistinctiveQuery {
			if hasDistinctive {
				// Model matches the distinctive term: massive boost, popularity barely matters.
				overlapWeight = 1.20
				popWeight = pop * 0.15
				credWeight = 0.06
				instructWeight = 0.05
			} else {
				// Model does NOT match distinctive term: penalize heavily.
				popWeight = pop * 0.40
				credWeight = 0.12
			}
		}
		
		score := popWeight + credWeight*cred + 0.10*lic + 0.22*sizeFit + instructWeight*instruct
		score += overlapWeight * overlap
		
		// Only apply engine preference when explicitly requested (PreferGGUF set by user flag).
		// For distinctive queries without explicit engine preference, stay neutral on GGUF vs vLLM.
		if in.PreferGGUF && format.Engine == hf.EngineGGUF {
			score += 0.12
		} else if in.PreferGGUF && format.Engine == hf.EngineVLLM {
			// User wants GGUF, penalize vLLM
			score -= 0.08
		}
		if containsAny(id, "base") && instruct == 0 {
			score -= 0.06
		}

		why := []string{
			fmt.Sprintf("likes %d", m.Likes),
			fmt.Sprintf("downloads %s", formatCount(m.Downloads)),
		}
		if known {
			why = append(why, "known publisher")
		}
		if paramsB > 0 {
			why = append(why, fmt.Sprintf("%.1fB", paramsB))
		}
		if format.Quant != "" {
			why = append(why, format.Quant)
		}

		out = append(out, Scored{
			Model:   m,
			Score:   score,
			Why:     why,
			Engine:  format.Engine,
			ParamsB: paramsB,
			Quant:   format.Quant,
			License: m.License(),
		})
	}

	for i := 1; i < len(out); i++ {
		j := i
		for j > 0 && out[j].Score > out[j-1].Score {
			out[j], out[j-1] = out[j-1], out[j]
			j--
		}
	}
	return out
}

// queryOverlap returns (score, hasDistinctiveMatch).
// score is 0.0-1.0 based on term overlap.
// hasDistinctiveMatch is true if any raw distinctive term matched the model id.
func queryOverlap(id, pipeline, raw, query, task string) (float64, bool) {
	hay := strings.ToLower(id + " " + pipeline)
	stopWords := map[string]bool{
		"for": true, "and": true, "the": true, "with": true, "use": true,
		"case": true, "want": true, "need": true, "that": true, "this": true,
		"chat": true, "model": true, "help": true, "find": true, "get": true,
		"run": true, "try": true, "test": true, "make": true, "can": true,
		"instruct": true, "coder": true, "code": true, "coding": true,
	}
	var terms []string
	var rawTerms []string
	for _, part := range []string{raw, query, task} {
		for _, tok := range strings.Fields(strings.ToLower(part)) {
			tok = strings.Trim(tok, ",.?!:;\"'+()[]{}")
			if len(tok) < 3 || stopWords[tok] {
				continue
			}
			terms = append(terms, tok)
			if part == raw {
				rawTerms = append(rawTerms, tok)
			}
		}
	}
	if len(terms) == 0 {
		return 0, false
	}
	hit := 0
	rawHit := 0
	seen := map[string]bool{}
	rawSeen := map[string]bool{}
	for _, tok := range terms {
		if seen[tok] {
			continue
		}
		seen[tok] = true
		if strings.Contains(hay, tok) {
			hit++
		}
	}
	for _, tok := range rawTerms {
		if rawSeen[tok] {
			continue
		}
		rawSeen[tok] = true
		if strings.Contains(hay, tok) {
			rawHit++
		}
	}
	hasDistinctive := rawHit > 0
	baseScore := float64(hit) / float64(len(seen))
	if len(rawTerms) > 0 {
		rawScore := float64(rawHit) / float64(len(rawTerms))
		return 0.7*rawScore + 0.3*baseScore, hasDistinctive
	}
	return baseScore, hasDistinctive
}


func authorOf(m hf.Model) string {
	if m.Author != "" {
		return m.Author
	}
	id := m.RepoID()
	if i := strings.IndexByte(id, '/'); i > 0 {
		return id[:i]
	}
	return id
}

func licenseScore(lic string) float64 {
	l := strings.ToLower(lic)
	switch {
	case strings.Contains(l, "mit"), strings.Contains(l, "apache"), strings.Contains(l, "bsd"):
		return 1
	case strings.Contains(l, "llama"), strings.Contains(l, "gemma"):
		return 0.7
	case l == "":
		return 0.4
	default:
		return 0.5
	}
}

func formatCount(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}
