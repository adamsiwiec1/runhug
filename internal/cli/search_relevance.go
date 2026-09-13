package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/adamsiwiec1/runhug-cli/internal/config"
	"github.com/adamsiwiec1/runhug-cli/internal/hf"
	"github.com/adamsiwiec1/runhug-cli/internal/index"
	"github.com/adamsiwiec1/runhug-cli/internal/semantic"
)

type searchRequest struct {
	Query           string
	Author          string
	Task            string
	Library         string
	Filter          string
	License         string
	Engine          string
	Sort            string
	Limit           int
	DisableSemantic bool
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

	// Try user-local index first, then bundled index, then HF API
	indexPath := indexFilePath()
	if index.Exists(indexPath) {
		return searchLocalIndex(ctx, req, sortKey)
	}

	// Try bundled index
	bundledPath := bundledIndexPath()
	if index.Exists(bundledPath) {
		return searchBundledIndex(ctx, req, sortKey, bundledPath)
	}

	// Fall back to HF API (alias expansion + card descriptions + optional semantic rank).
	client := hf.New(config.Load().HFToken)
	meta := searchMeta{
		RankSource: "hub",
		Queries:    hf.HubSearchQueries(req.Query),
	}
	models, note, err := searchRanked(ctx, client, hf.SearchOpts{
		Query:   req.Query,
		Author:  req.Author,
		Task:    req.Task,
		Library: req.Library,
		Filter:  req.Filter,
		License: req.License,
		Engine:  req.Engine,
	}, sortKey, req.Limit, !req.DisableSemantic)
	if err != nil {
		return nil, meta, err
	}
	if note != "" {
		meta.Notes = note
		fmt.Fprintln(os.Stderr, dim(note))
	}
	if sortKey == "likes" || sortKey == "downloads" {
		meta.RankSource = sortKey + " (from expanded pool)"
	} else if strings.HasPrefix(note, "semantic rank") {
		meta.RankSource = "semantic + hub"
	} else {
		meta.RankSource = "hub + descriptions"
	}
	return models, meta, nil
}

func searchLocalIndex(ctx context.Context, req searchRequest, sortKey string) ([]hf.Model, searchMeta, error) {
	return searchIndexAtPath(ctx, req, sortKey, indexFilePath(), "local index")
}

func searchBundledIndex(ctx context.Context, req searchRequest, sortKey string, path string) ([]hf.Model, searchMeta, error) {
	return searchIndexAtPath(ctx, req, sortKey, path, "bundled index")
}

func searchIndexAtPath(ctx context.Context, req searchRequest, sortKey string, path string, source string) ([]hf.Model, searchMeta, error) {
	idx, err := index.Open(path)
	if err != nil {
		return nil, searchMeta{}, fmt.Errorf("open %s: %w", source, err)
	}
	defer idx.Close()

	meta := searchMeta{RankSource: source, Queries: hf.HubSearchQueries(req.Query)}

	q := req.Query
	if extra := hf.AliasTerms(req.Query); len(extra) > 0 {
		q = strings.TrimSpace(req.Query + " " + strings.Join(extra, " "))
	}

	filters := index.SearchFilters{
		Author:      req.Author,
		Library:     req.Library,
		License:     req.License,
		PipelineTag: req.Task,
		Engine:      req.Engine,
		Sort:        sortKey,
		Limit:       100,
	}

	models, err := idx.Search(ctx, q, filters)
	if err != nil {
		return nil, searchMeta{}, fmt.Errorf("search local index: %w", err)
	}

	if sortKey == "relevance" {
		hf.ScoreRelevance(models, req.Query)
		meta.RankSource = source + " + descriptions"
		var note string
		models, note = rankSemantic(ctx, req.Query, sortKey, !req.DisableSemantic, models, func() semantic.Embedder {
			return semantic.Discover(ctx, config.Load().HFToken)
		})
		if note != "" {
			meta.Notes = note
			fmt.Fprintln(os.Stderr, dim(note))
		}
		if strings.HasPrefix(note, "semantic rank") {
			meta.RankSource = source + " + semantic"
		}
	}
	if sortKey == "likes" || sortKey == "downloads" {
		hf.SortModels(models, sortKey)
		meta.RankSource = source + " + " + sortKey
	}

	// Limit results
	if len(models) > req.Limit {
		models = models[:req.Limit]
	}

	return models, meta, nil
}
