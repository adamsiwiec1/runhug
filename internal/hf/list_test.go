package hf

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseLinkNext(t *testing.T) {
	base := "https://huggingface.co"
	h := `<https://huggingface.co/api/models?limit=100&cursor=abc>; rel="next", <https://huggingface.co/api/models?limit=100&cursor=zzz>; rel="last"`
	got := parseLinkNext(h, base)
	if got != "/api/models?limit=100&cursor=abc" {
		t.Fatalf("got %q", got)
	}
	if parseLinkNext("", base) != "" {
		t.Fatal("expected empty")
	}
}

func TestParseLastModifiedUnix(t *testing.T) {
	if parseLastModifiedUnix("2026-01-02T03:04:05Z") == 0 {
		t.Fatal("expected non-zero")
	}
	if parseLastModifiedUnix("") != 0 {
		t.Fatal("expected zero")
	}
}

func TestListModelsMinFiltersAndEarlyStop(t *testing.T) {
	pages := 0
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages++
		w.Header().Set("Content-Type", "application/json")
		switch pages {
		case 1:
			w.Header().Set("Link", `<`+srv.URL+`/api/models?cursor=2>; rel="next"`)
			_, _ = w.Write([]byte(`[
				{"id":"a/hi","likes":10,"downloads":500},
				{"id":"a/low-likes","likes":1,"downloads":400},
				{"id":"a/ok","likes":5,"downloads":200}
			]`))
		case 2:
			// Entire page below MinDownloads → early stop (no further pages).
			w.Header().Set("Link", `<`+srv.URL+`/api/models?cursor=3>; rel="next"`)
			_, _ = w.Write([]byte(`[
				{"id":"b/below","likes":10,"downloads":50},
				{"id":"b/also","likes":10,"downloads":10}
			]`))
		default:
			t.Fatalf("unexpected page %d (early stop should prevent this)", pages)
		}
	}))
	t.Cleanup(srv.Close)

	c := New("")
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()

	var skipped int
	models, err := c.ListModels(context.Background(), ListOpts{
		Sort:          "downloads",
		Direction:     "-1",
		PageSize:      10,
		MaxPages:      10,
		Sleep:         1,
		MinLikes:      3,
		MinDownloads:  100,
		FilterSkipped: &skipped,
	})
	if err != nil {
		t.Fatal(err)
	}
	if pages != 2 {
		t.Fatalf("pages=%d want 2 (page1 + early-stop page2)", pages)
	}
	if len(models) != 2 {
		t.Fatalf("models=%d want 2; %+v", len(models), models)
	}
	if models[0].RepoID() != "a/hi" || models[1].RepoID() != "a/ok" {
		t.Fatalf("got %s %s", models[0].RepoID(), models[1].RepoID())
	}
	if skipped != 1 {
		t.Fatalf("filter skipped=%d want 1 (low-likes on page1)", skipped)
	}
}

func TestPageAllBelowDownloads(t *testing.T) {
	if !pageAllBelowDownloads([]Model{{Downloads: 10}, {Downloads: 20}}, 100) {
		t.Fatal("expected true")
	}
	if pageAllBelowDownloads([]Model{{Downloads: 10}, {Downloads: 200}}, 100) {
		t.Fatal("expected false")
	}
	if pageAllBelowDownloads(nil, 100) {
		t.Fatal("empty should be false")
	}
}
