package cli

import (
	"path/filepath"
	"testing"

	"github.com/adamsiwiec1/runhug-cli/internal/hf"
	"github.com/adamsiwiec1/runhug-cli/internal/index"
)

func TestFilterHubDeltaModelsKeepsExistingBelowThreshold(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.db")
	idx, err := index.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = idx.Close() })
	if err := idx.InsertModel(hf.Model{ID: "org/old", Likes: 1, Downloads: 1}); err != nil {
		t.Fatal(err)
	}

	in := []hf.Model{
		{ID: "org/old", Likes: 1, Downloads: 1},                 // existing — keep
		{ID: "org/new-low", Likes: 1, Downloads: 10},            // new below threshold — drop
		{ID: "org/new-ok", Likes: 5, Downloads: 200},            // new ok — keep
	}
	out := filterHubDeltaModels(idx, in)
	ids := map[string]bool{}
	for _, m := range out {
		ids[m.ID] = true
	}
	if !ids["org/old"] || !ids["org/new-ok"] || ids["org/new-low"] {
		t.Fatalf("ids=%v", ids)
	}
}
