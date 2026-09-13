package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/config"
	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/hf"
	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/index"
	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/localllm"
	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/recommend"
	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/runtime"
	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/store"
)

type searchRequest struct {
	Query   string
	Author  string
	Task    string
	Library string
	Filter  string
	License string
	Engine  string
	Sort    string
	Limit   int
}

type searchMeta struct {
	RankSource string
	Queries    []string
	Notes      string
}

func searchModels(ctx context.Context, req searchRequest) ([]hf.Model, searchMeta, error) {
	sortKey, err := hf.NormalizeSort(req.Sort)
	if err != nil {
		return nil, searchMeta{}, err
	}

	// Try local index first
	indexPath := indexFilePath()
	if index.Exists(indexPath) {
		return searchLocalIndex(ctx, req, sortKey)
	}

	// Fall back to HF API
	client := hf.New(config.Load().HFToken)
	meta := searchMeta{RankSource: "hub"}

	// Natural-language / relevance path: expand → fetch pool → score.
	useSemantic := sortKey == "relevance" || strings.TrimSpace(req.Query) != ""
	if !useSemantic {
		models, err := client.Search(ctx, hf.SearchOpts{
			Query:   req.Query,
			Author:  req.Author,
			Task:    req.Task,
			Library: req.Library,
			Filter:  req.Filter,
			License: req.License,
			Engine:  req.Engine,
			Sort:    sortKey,
			Limit:   req.Limit,
		})
		return models, meta, err
	}

	llm := localSearchLLM(ctx)
	ram := recommend.RAMGB()
	ex := recommend.Expand(ctx, req.Query, ram, llm)
	// Search is for Hub discovery (often Runpod deploy) — do not drop large models
	// because of laptop RAM caps from ParseIntent.
	ex.Intent.MaxParamsB = 0
	// Only set engine preference when user explicitly requests it.
	// When no --engine flag, keep PreferGGUF from intent (neutral for distinctive queries).
	if strings.EqualFold(req.Engine, "gguf") {
		ex.Intent.PreferGGUF = true
	} else if req.Engine == "vllm" || req.Engine == "safetensors" {
		ex.Intent.PreferGGUF = false
	}
	// else: keep the PreferGGUF from ParseIntent (often true for local contexts)
	meta.Queries = ex.Queries
	meta.Notes = ex.Notes
	if ex.Source == "local-llm" {
		meta.RankSource = "local-llm + hub"
	} else {
		meta.RankSource = "heuristics + hub"
	}

	task := req.Task
	if task == "" || task == "text-generation" {
		// Keep text-generation default for Hub; intent may still be chat/code.
		task = "text-generation"
	}

	poolLimit := 100
	seen := map[string]hf.Model{}
	for _, q := range ex.Queries {
		batch, err := client.Search(ctx, hf.SearchOpts{
			Query:   q,
			Author:  req.Author,
			Task:    task,
			Library: req.Library,
			Filter:  req.Filter,
			License: req.License,
			Engine:  req.Engine,
			Sort:    "relevance",
			Limit:   poolLimit,
		})
		if err != nil {
			return nil, meta, err
		}
		for _, m := range batch {
			id := m.RepoID()
			if id == "" {
				continue
			}
			if _, ok := seen[id]; !ok {
				seen[id] = m
			}
		}
	}
	pool := make([]hf.Model, 0, len(seen))
	for _, m := range seen {
		pool = append(pool, m)
	}

	scored := recommend.Score(pool, ex.Intent)
	models := make([]hf.Model, 0, len(scored))
	for _, s := range scored {
		models = append(models, s.Model)
	}
	// If scoring filtered everything (e.g. size caps), fall back to Hub order.
	if len(models) == 0 {
		models = pool
		meta.RankSource = "hub"
	} else {
		meta.RankSource = "heuristics + hub"
	}

	// Optional local-LLM re-rank of the scored shortlist (does not invent Hub queries).
	if llm != nil && sortKey == "relevance" && len(models) > 1 {
		ids := make([]string, len(models))
		byID := map[string]hf.Model{}
		for i, m := range models {
			ids[i] = m.RepoID()
			byID[ids[i]] = m
		}
		ordered, notes, err := recommend.Rerank(ctx, req.Query, ids, llm)
		if err == nil && len(ordered) > 0 {
			out := make([]hf.Model, 0, len(ordered))
			for _, id := range ordered {
				if m, ok := byID[id]; ok {
					out = append(out, m)
				}
			}
			if len(out) > 0 {
				models = out
				meta.RankSource = "local-llm rerank + hub"
				if notes != "" {
					meta.Notes = notes
				}
			}
		} else if err != nil {
			// Keep heuristics ranking; local models often fail JSON — stay quiet.
			_ = err
		}
	}

	switch sortKey {
	case "likes", "downloads":
		hf.SortModels(models, sortKey)
		meta.RankSource = sortKey + " (from semantic pool)"
	}

	if len(models) > req.Limit {
		models = models[:req.Limit]
	}
	if meta.Notes != "" && strings.Contains(meta.RankSource, "local-llm") {
		fmt.Fprintln(os.Stderr, dim(meta.Notes))
	}
	return models, meta, nil
}

func searchLocalIndex(ctx context.Context, req searchRequest, sortKey string) ([]hf.Model, searchMeta, error) {
	idx, err := index.Open(indexFilePath())
	if err != nil {
		return nil, searchMeta{}, fmt.Errorf("open local index: %w", err)
	}
	defer idx.Close()

	meta := searchMeta{RankSource: "local index"}

	// Get distinctive tokens for boost calculation
	ram := recommend.RAMGB()
	ex := recommend.Expand(ctx, req.Query, ram, nil)
	meta.Queries = ex.Queries

	// Search local index
	filters := index.SearchFilters{
		Author:      req.Author,
		Library:     req.Library,
		License:     req.License,
		PipelineTag: req.Task,
		Engine:      req.Engine,
		Sort:        sortKey,
		Limit:       100, // Get pool for ranking
	}

	models, err := idx.Search(ctx, req.Query, filters)
	if err != nil {
		return nil, searchMeta{}, fmt.Errorf("search local index: %w", err)
	}

	// Apply distinctive term ranking if relevance sort
	if sortKey == "relevance" {
		ex.Intent.MaxParamsB = 0
		if strings.EqualFold(req.Engine, "gguf") {
			ex.Intent.PreferGGUF = true
		} else if req.Engine == "vllm" || req.Engine == "safetensors" {
			ex.Intent.PreferGGUF = false
		}

		scored := recommend.Score(models, ex.Intent)
		models = make([]hf.Model, 0, len(scored))
		for _, s := range scored {
			models = append(models, s.Model)
		}
		meta.RankSource = "local index + heuristics"
	}

	// Apply final sort for likes/downloads after distinctive ranking
	if sortKey == "likes" || sortKey == "downloads" {
		// Split into distinctive matches and non-matches
		hasDistinctive := len(recommend.ExtractDistinctiveTokens(req.Query)) > 0
		if hasDistinctive {
			var distinctive, nonDistinctive []hf.Model
			for _, m := range models {
				isDistinctive := false
				idLower := strings.ToLower(m.RepoID())
				for _, tok := range recommend.ExtractDistinctiveTokens(req.Query) {
					if strings.Contains(idLower, tok) {
						isDistinctive = true
						break
					}
				}
				if isDistinctive {
					distinctive = append(distinctive, m)
				} else {
					nonDistinctive = append(nonDistinctive, m)
				}
			}
			// Sort each group separately
			hf.SortModels(distinctive, sortKey)
			hf.SortModels(nonDistinctive, sortKey)
			// Concatenate: distinctive first, then non-distinctive
			models = append(distinctive, nonDistinctive...)
			meta.RankSource = "local index + " + sortKey + " (distinctive first)"
		} else {
			hf.SortModels(models, sortKey)
			meta.RankSource = "local index + " + sortKey
		}
	}

	// Limit results
	if len(models) > req.Limit {
		models = models[:req.Limit]
	}

	return models, meta, nil
}

func localSearchLLM(ctx context.Context) *localllm.Client {
	snap := runtime.Detect()
	eng := snap.Preferred("")
	if eng == nil || !eng.Running || eng.BaseURL == "" {
		return nil
	}
	model := ""
	if reg, _, err := store.Load(); err == nil && reg != nil {
		if cur, ok := reg.Models[reg.Current]; ok && cur.Kind() == store.BackendLocal {
			model = cur.UpstreamModel()
		}
	}
	c := localllm.New(eng.BaseURL, model)
	return c
}
