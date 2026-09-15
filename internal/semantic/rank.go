package semantic

import (
	"context"
	"fmt"
	"sort"

	"github.com/adamsiwiec1/runhug/internal/hf"
)

// Rerank orders models by cosine similarity of query vs SearchableText
// (repo id + tags + card description). On embedder failure it returns an error;
// the caller keeps lexical order.
func Rerank(ctx context.Context, query string, models []hf.Model, e Embedder) ([]hf.Model, error) {
	if e == nil || len(models) < 2 {
		return models, nil
	}
	query = clipEmbed(query)
	if query == "" {
		return models, nil
	}
	docs := make([]string, len(models))
	for i, m := range models {
		docs[i] = hf.SearchableText(m)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ectx, cancel := context.WithTimeout(ctx, embedTimeout)
	defer cancel()
	q, dvecs, err := e.Embed(ectx, query, docs)
	if err != nil {
		return nil, err
	}
	if len(dvecs) != len(models) {
		return nil, fmt.Errorf("embed: got %d doc vectors want %d", len(dvecs), len(models))
	}
	type row struct {
		i int
		s float64
	}
	rows := make([]row, len(models))
	for i := range models {
		rows[i] = row{i: i, s: Cosine(q, dvecs[i])}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].s > rows[j].s
	})
	out := make([]hf.Model, len(models))
	for i, r := range rows {
		out[i] = models[r.i]
	}
	return out, nil
}
