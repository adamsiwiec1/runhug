package index

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/adamsiwiec1/runhug-cli/internal/hf"
)

func TestWatermarkAndMaxLastModified(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "models.db")
	idx, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()

	if err := idx.InsertModel(hf.Model{
		ID: "a/b", PipelineTag: "text-generation",
		LastModified: "2026-02-01T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	max, err := idx.MaxLastModified()
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	if !max.Equal(want) {
		t.Fatalf("max=%v", max)
	}
	if err := idx.SetMetadata("watermark", "2026-03-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	wm, err := idx.Watermark()
	if err != nil {
		t.Fatal(err)
	}
	if !wm.Equal(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("wm=%v", wm)
	}
}

func TestAllModelsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "models.db")
	idx, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.InsertModel(hf.Model{
		ID: "org/model", Author: "org", Description: "desc",
		Tags: []string{"foo"}, Likes: 1, Downloads: 2,
		LibraryName: "gguf", PipelineTag: "text-generation",
		LastModified: "2026-01-15T12:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	idx.Close()

	ro, err := OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	models, err := ro.AllModels()
	if err != nil || len(models) != 1 || models[0].ID != "org/model" {
		t.Fatalf("%v %v", models, err)
	}
}
