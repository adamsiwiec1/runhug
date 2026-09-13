package hf

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchQuery(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"Qwen/Qwen2.5-7B-Instruct","pipeline_tag":"text-generation","downloads":10}]`))
	}))
	t.Cleanup(srv.Close)

	c := New("tok")
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()
	models, err := c.Search(context.Background(), SearchOpts{Query: "qwen", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].RepoID() != "Qwen/Qwen2.5-7B-Instruct" {
		t.Fatalf("%+v", models)
	}
	if got == "" || !containsAll(got, "search=qwen", "pipeline_tag=text-generation", "limit=5") {
		t.Fatalf("query %s", got)
	}
}

func TestIsGated(t *testing.T) {
	if (Model{Gated: false}).IsGated() {
		t.Fatal("false")
	}
	if !(Model{Gated: true}).IsGated() {
		t.Fatal("true")
	}
	if !(Model{Gated: "auto"}).IsGated() {
		t.Fatal("auto")
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !contains(s, p) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || (len(s) > 0 && (indexOf(s, sub) >= 0)))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
