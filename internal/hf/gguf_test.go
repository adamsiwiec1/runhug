package hf

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPickGGUFPrefersQ4KM(t *testing.T) {
	m := Model{
		Siblings: []Sibling{
			{RFilename: "model.Q8_0.gguf"},
			{RFilename: "model.Q4_K_M.gguf"},
			{RFilename: "model.Q3_K_M.gguf"},
			{RFilename: "mmproj.gguf"},
		},
	}
	file, quant, ok := PickGGUF(m)
	if !ok || file != "model.Q4_K_M.gguf" {
		t.Fatalf("file %s quant %s ok %v", file, quant, ok)
	}
	if quant != "Q4_K_M" {
		t.Fatalf("quant %s", quant)
	}
}

func TestDownload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/org/m/resolve/main/a.gguf" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("GGUF"))
	}))
	t.Cleanup(srv.Close)

	dest := filepath.Join(t.TempDir(), "a.gguf")
	c := New("")
	c.BaseURL = srv.URL
	if err := c.Download(context.Background(), "org/m", "a.gguf", dest); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "GGUF" {
		t.Fatalf("got %q", raw)
	}
}
