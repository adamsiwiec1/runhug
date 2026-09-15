package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adamsiwiec1/runhug/internal/config"
	"github.com/adamsiwiec1/runhug/internal/store"
)

func TestUsableRegistryEntriesFiltersAndOrders(t *testing.T) {
	reg := &store.Registry{
		Current: "org/b",
		Models: map[string]store.Model{
			"org/a": {HFRepo: "org/a", Backend: store.BackendRunpod, EndpointID: "ep-a"},
			"org/b": {HFRepo: "org/b", Backend: store.BackendLocal, BaseURL: "http://127.0.0.1:1/v1", Runtime: "ollama"},
			"org/c": {HFRepo: "org/c", Backend: store.BackendRunpod}, // no endpoint id → skip
			"org/d": {HFRepo: "org/d", Backend: store.BackendLocal, GGUFPath: "/tmp/x.gguf"}, // no base_url → skip
		},
	}
	got := usableRegistryEntries(reg)
	if len(got) != 2 {
		t.Fatalf("got %d: %+v", len(got), got)
	}
	if got[0].HFRepo != "org/b" {
		t.Fatalf("current should be first: %+v", got)
	}
	if got[1].HFRepo != "org/a" || got[1].Kind != "runpod" || got[1].Where != "ep-a" {
		t.Fatalf("%+v", got[1])
	}
	if got[0].Kind != "local" || got[0].Where != "http://127.0.0.1:1/v1" {
		t.Fatalf("%+v", got[0])
	}
}

func TestPickStartModelPassthrough(t *testing.T) {
	key, model, err := pickStartModel("org/m", "", "", "", true)
	if err != nil || key != "org/m" || model != "" {
		t.Fatalf("%q %q %v", key, model, err)
	}
	key, model, err = pickStartModel("", "http://x/v1", "served", "", true)
	if err != nil || key != "" || model != "served" {
		t.Fatalf("%q %q %v", key, model, err)
	}
}

func TestPickStartModelEmptyRegistryErrors(t *testing.T) {
	t.Setenv(config.EnvConfig, filepath.Join(t.TempDir(), "registry.json"))
	_, _, err := pickStartModel("", "", "", "", true)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no registry endpoints") {
		t.Fatalf("%v", err)
	}
}

func TestPickStartModelAutoPickSingle(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(config.EnvConfig, filepath.Join(dir, "registry.json"))
	reg := &store.Registry{
		Models: map[string]store.Model{
			"org/only": {HFRepo: "org/only", Backend: store.BackendRunpod, EndpointID: "ep1"},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatal(err)
	}
	key, model, err := pickStartModel("", "", "", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if key != "org/only" || model != "" {
		t.Fatalf("%q %q", key, model)
	}
}

func TestPickStartModelMultiNonInteractiveErrors(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(config.EnvConfig, filepath.Join(dir, "registry.json"))
	reg := &store.Registry{
		Models: map[string]store.Model{
			"org/a": {HFRepo: "org/a", Backend: store.BackendRunpod, EndpointID: "a"},
			"org/b": {HFRepo: "org/b", Backend: store.BackendRunpod, EndpointID: "b"},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatal(err)
	}
	_, _, err := pickStartModel("", "", "", "", true)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "multiple models") {
		t.Fatalf("%v", err)
	}
}

func TestPickStartModelBaseURLListsRemote(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{
				{"id": "alpha"},
				{"id": "beta"},
			},
		})
	}))
	defer srv.Close()

	_, _, err := pickStartModel("", srv.URL+"/v1", "", "", true)
	if err == nil || !strings.Contains(err.Error(), "multiple models") {
		t.Fatalf("expected multi error, got %v", err)
	}

	// Single remote model auto-picks under skipPrompt.
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"id": "solo"}},
		})
	}))
	defer srv1.Close()
	key, model, err := pickStartModel("", srv1.URL+"/v1", "", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if key != "" || model != "solo" {
		t.Fatalf("%q %q", key, model)
	}
}

func TestFormatRegistryEntryLine(t *testing.T) {
	line := formatRegistryEntryLine(1, registryPickEntry{
		HFRepo: "org/model", Kind: "runpod", Where: "abc", Extra: "loadbalancer",
	})
	if !strings.Contains(line, "1) org/model  runpod abc  loadbalancer") {
		t.Fatal(line)
	}
}
