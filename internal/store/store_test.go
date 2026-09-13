package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLookup(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("RVP_CONFIG", filepath.Join(dir, "registry.json"))
	r := &Registry{Models: map[string]Model{
		"Qwen/Qwen2.5-7B-Instruct": {HFRepo: "Qwen/Qwen2.5-7B-Instruct", EndpointID: "ep1", CreatedAt: time.Now()},
		"google/gemma-3-1b-it":     {HFRepo: "google/gemma-3-1b-it", EndpointID: "ep2", CreatedAt: time.Now()},
	}, Current: "Qwen/Qwen2.5-7B-Instruct"}
	if err := r.Save(); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if m, ok := loaded.Lookup("default"); !ok || m.EndpointID != "ep1" {
		t.Fatalf("default %+v %v", m, ok)
	}
	if m, ok := loaded.Lookup("ep2"); !ok || m.HFRepo != "google/gemma-3-1b-it" {
		t.Fatalf("by id %+v %v", m, ok)
	}
	if m, ok := loaded.Lookup("gemma"); !ok || m.EndpointID != "ep2" {
		t.Fatalf("fuzzy %+v %v", m, ok)
	}
	if _, err := os.Stat(filepath.Join(dir, "registry.json")); err != nil {
		t.Fatal(err)
	}
}

func TestLookupLocalURLAndKind(t *testing.T) {
	r := &Registry{Models: map[string]Model{
		"local/tiny": {
			HFRepo:   "local/tiny",
			Backend:  BackendLocal,
			BaseURL:  "http://127.0.0.1:8081/v1",
			GGUFPath: "/tmp/tiny.gguf",
		},
	}, Current: "local/tiny"}
	m, ok := r.Lookup("http://127.0.0.1:8081/v1")
	if !ok || m.HFRepo != "local/tiny" {
		t.Fatalf("url lookup %+v %v", m, ok)
	}
	if m.Kind() != BackendLocal {
		t.Fatalf("kind %s", m.Kind())
	}
	if inferred := (Model{GGUFPath: "/x.gguf"}).Kind(); inferred != BackendLocal {
		t.Fatalf("inferred %s", inferred)
	}
	named := Model{HFRepo: "Qwen/x", ServeName: "qwen2.5:1.5b"}
	if named.UpstreamModel() != "qwen2.5:1.5b" {
		t.Fatal(named.UpstreamModel())
	}
}
