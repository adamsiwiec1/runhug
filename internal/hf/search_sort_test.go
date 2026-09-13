package hf

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeSort(t *testing.T) {
	got, err := NormalizeSort("")
	if err != nil || got != "relevance" {
		t.Fatalf("default: %q %v", got, err)
	}
	got, err = NormalizeSort("LIKES")
	if err != nil || got != "likes" {
		t.Fatalf("likes: %q %v", got, err)
	}
	if _, err := NormalizeSort("trendingScore"); err == nil {
		t.Fatal("expected error for trendingScore")
	}
}

func TestSearchRelevanceOmitsSort(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"a/b","likes":1,"downloads":1,"tags":["safetensors","license:apache-2.0"]}]`))
	}))
	t.Cleanup(srv.Close)
	c := New("")
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()
	_, err := c.Search(context.Background(), SearchOpts{Query: "qwen", Sort: "relevance", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "sort=") {
		t.Fatalf("relevance should omit sort: %s", got)
	}
	if !strings.Contains(got, "search=qwen") || !strings.Contains(got, "limit=5") {
		t.Fatalf("query %s", got)
	}
}

func TestSearchLikesFetches100ThenReranks(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":"a/low","likes":1,"downloads":100,"tags":["safetensors","license:mit"]},
			{"id":"a/high","likes":50,"downloads":10,"tags":["safetensors","license:apache-2.0"]},
			{"id":"a/mid","likes":10,"downloads":50,"tags":["gguf","license:apache-2.0"]}
		]`))
	}))
	t.Cleanup(srv.Close)
	c := New("")
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()
	models, err := c.Search(context.Background(), SearchOpts{Query: "x", Sort: "likes", Limit: 2, Engine: "vllm"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "limit=100") {
		t.Fatalf("expected fetch pool 100: %s", got)
	}
	if strings.Contains(got, "sort=") {
		t.Fatalf("likes path should query relevance (no sort): %s", got)
	}
	if !strings.Contains(got, "filter=safetensors") {
		t.Fatalf("engine vllm should filter safetensors: %s", got)
	}
	if len(models) != 2 {
		t.Fatalf("len %d", len(models))
	}
	if models[0].RepoID() != "a/high" || models[1].RepoID() != "a/low" {
		t.Fatalf("order %+v", models)
	}
}

func TestSearchLicenseAndEngineFilters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query()["filter"]; len(got) < 2 {
			t.Errorf("filters %v", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":"a/ok","likes":3,"tags":["safetensors","license:apache-2.0"]},
			{"id":"a/gguf","likes":9,"tags":["gguf","license:apache-2.0"]},
			{"id":"a/mit","likes":8,"tags":["safetensors","license:mit"]}
		]`))
	}))
	t.Cleanup(srv.Close)
	c := New("")
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()
	models, err := c.Search(context.Background(), SearchOpts{
		Query: "x", Sort: "relevance", Limit: 10, License: "apache-2.0", Engine: "vllm",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].RepoID() != "a/ok" {
		t.Fatalf("%+v", models)
	}
}

func TestSortModels(t *testing.T) {
	models := []Model{{ID: "a", Likes: 1, Downloads: 9}, {ID: "b", Likes: 5, Downloads: 1}}
	SortModels(models, "likes")
	if models[0].ID != "b" {
		t.Fatal(models[0].ID)
	}
	SortModels(models, "downloads")
	if models[0].ID != "a" {
		t.Fatal(models[0].ID)
	}
}
