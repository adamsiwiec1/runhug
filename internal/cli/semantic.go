package cli

import (
	"context"
	"strings"

	"github.com/adamsiwiec1/runhug/internal/hf"
	"github.com/adamsiwiec1/runhug/internal/semantic"
)

const searchPoolCap = 100

func searchRanked(ctx context.Context, client *hf.Client, opts hf.SearchOpts, sortKey string, displayLimit int, wantSemantic bool) ([]hf.Model, string, error) {
	sortKey, err := hf.NormalizeSort(sortKey)
	if err != nil {
		return nil, "", err
	}
	displayLimit = clampLimit(displayLimit)
	fetch := opts
	fetch.Sort = "relevance"
	fetch.Limit = searchPoolCap
	fetch.Full = true
	fetch.Expand = true
	models, err := client.Search(ctx, fetch)
	if err != nil {
		return nil, "", err
	}
	note := ""
	if wantSemantic && sortKey == "relevance" {
		models, note = rankSemantic(ctx, opts.Query, sortKey, true, models, func() semantic.Embedder {
			return semantic.Discover(ctx, client.Token)
		})
	}
	if sortKey == "likes" || sortKey == "downloads" {
		hf.SortModels(models, sortKey)
	}
	if len(models) > displayLimit {
		models = models[:displayLimit]
	}
	return models, note, nil
}

func rankSemantic(ctx context.Context, query, sortKey string, want bool, models []hf.Model, discover func() semantic.Embedder) ([]hf.Model, string) {
	if !want || sortKey != "relevance" || strings.TrimSpace(query) == "" || len(models) < 2 {
		return models, ""
	}
	var e semantic.Embedder
	if discover != nil {
		e = discover()
	}
	if e == nil {
		return models, semantic.LexicalNote
	}
	ranked, err := semantic.Rerank(ctx, query, models, e)
	if err != nil || len(ranked) == 0 {
		return models, semantic.LexicalNote
	}
	return ranked, "semantic rank  " + e.Label()
}
