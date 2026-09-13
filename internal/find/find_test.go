package find

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanFindsGGUF(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("RVP_MODELS", dir)
	t.Setenv("RVP_CACHE", filepath.Join(t.TempDir(), "empty-cache"))
	if err := os.WriteFile(filepath.Join(dir, "Qwen2.5-1.5B-Instruct-Q4_K_M.gguf"), []byte("GGUF"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mmproj-f16.gguf"), []byte("skip"), 0o644); err != nil {
		t.Fatal(err)
	}
	hits := Filter(scanRoots([]string{dir}), "qwen")
	if len(hits) != 1 || hits[0].Kind != "gguf" {
		t.Fatalf("%+v", hits)
	}
	if !filepath.IsAbs(hits[0].Path) {
		t.Fatal(hits[0].Path)
	}
}

func TestOllamaManifestsFromDisk(t *testing.T) {
	root := t.TempDir()
	man := filepath.Join(root, "manifests", "registry.ollama.ai", "library", "qwen2.5-coder")
	if err := os.MkdirAll(man, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(man, "1.5b"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OLLAMA_MODELS", root)
	names := ollamaManifestNames()
	if len(names) != 1 || names[0] != "qwen2.5-coder:1.5b" {
		t.Fatalf("%v", names)
	}
}

func TestOllamaNameFromRel(t *testing.T) {
	got, ok := ollamaNameFromRel("registry.ollama.ai/library/qwen2.5-coder/14b")
	if !ok || got != "qwen2.5-coder:14b" {
		t.Fatalf("%q %v", got, ok)
	}
	got, ok = ollamaNameFromRel("hf.co/bartowski/Foo-GGUF/Q4_K_M")
	if !ok || got != "hf.co/bartowski/Foo-GGUF:Q4_K_M" {
		t.Fatalf("%q %v", got, ok)
	}
}

func TestFilterEmptyQuery(t *testing.T) {
	in := []Found{{Name: "a"}, {Name: "b"}}
	if len(Filter(in, "")) != 2 {
		t.Fatal("empty query should keep all")
	}
}
