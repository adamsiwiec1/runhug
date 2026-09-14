package index

import (
	"path/filepath"
	"testing"

	"github.com/adamsiwiec1/runhug-cli/internal/hf"
)

func TestHasModelAndPrimaryKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.db")
	idx, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = idx.Close() })

	ok, err := idx.HasModel("org/missing")
	if err != nil || ok {
		t.Fatalf("missing: ok=%v err=%v", ok, err)
	}
	if err := idx.InsertModel(hf.Model{ID: "org/demo", Likes: 1, Downloads: 10}); err != nil {
		t.Fatal(err)
	}
	ok, err = idx.HasModel("org/demo")
	if err != nil || !ok {
		t.Fatalf("present: ok=%v err=%v", ok, err)
	}
	// upsert same primary key
	if err := idx.InsertModel(hf.Model{ID: "org/demo", Likes: 9, Downloads: 100}); err != nil {
		t.Fatal(err)
	}
	n, err := idx.Count()
	if err != nil || n != 1 {
		t.Fatalf("count=%d err=%v", n, err)
	}
}
