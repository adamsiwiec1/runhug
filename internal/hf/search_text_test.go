package hf

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHubSearchQueriesAliases(t *testing.T) {
	q := HubSearchQueries("hacking")
	if len(q) < 2 || q[0] != "hacking" {
		t.Fatalf("primary: %v", q)
	}
	joined := strings.Join(q, " | ")
	for _, want := range []string{"pentest", "offsec", "cybersecurity", "bug hunter"} {
		if !containsString(q, want) {
			t.Fatalf("missing alias %q in %s", want, joined)
		}
	}
	if containsString(q, "instruct") {
		t.Fatalf("must not invent instruct: %v", q)
	}

	q = HubSearchQueries("cybersec")
	if !containsString(q, "cybersecurity") {
		t.Fatalf("cybersec aliases: %v", q)
	}

	q = HubSearchQueries("offsec")
	if !containsString(q, "pentest") || !containsString(q, "red team") || !containsString(q, "cyber") {
		t.Fatalf("offsec aliases: %v", q)
	}

	q = HubSearchQueries("qwen2.5")
	if len(q) != 1 || q[0] != "qwen2.5" {
		t.Fatalf("no security aliases for model names: %v", q)
	}
}

func TestHubQueryPlanCapsExtras(t *testing.T) {
	plan := HubQueryPlan("hacking", "text-generation")
	if len(plan) != 1+maxExtraHubQueries {
		t.Fatalf("len %d want %d: %+v", len(plan), 1+maxExtraHubQueries, plan)
	}
	if plan[0].Search != "hacking" || plan[0].Task != "text-generation" {
		t.Fatalf("primary %+v", plan[0])
	}
	var extras int
	for _, c := range plan[1:] {
		if c.Search == "" && c.Filter == "" && c.Task != "any" {
			t.Fatalf("empty extra %+v", c)
		}
		extras++
	}
	if extras > maxExtraHubQueries {
		t.Fatalf("extras %d", extras)
	}

	plan = HubQueryPlan("qwen", "text-generation")
	if len(plan) < 2 {
		t.Fatalf("qwen should add task=any or tag filter: %+v", plan)
	}
	var sawAny, sawTag bool
	for _, c := range plan {
		if c.Task == "any" && c.Search == "qwen" {
			sawAny = true
		}
		if c.Filter == "qwen" {
			sawTag = true
		}
	}
	if !sawAny && !sawTag {
		t.Fatalf("expected task=any or filter=qwen: %+v", plan)
	}
}

func TestScoreRelevancePrefersDescription(t *testing.T) {
	models := []Model{
		{ID: "acme/general-chat-7b", Likes: 5000, Tags: []string{"safetensors"}},
		{
			ID:          "lab/secure-ops-agent",
			Likes:       12,
			Tags:        []string{"safetensors"},
			Description: "offsec and hacking assistant for authorized tests",
		},
	}
	ScoreRelevance(models, "hacking")
	if models[0].RepoID() != "lab/secure-ops-agent" {
		t.Fatalf("description match should beat unrelated id, got %s", models[0].RepoID())
	}
}

func TestScoreRelevanceAliasOnID(t *testing.T) {
	models := []Model{
		{ID: "other/random-instruct", Likes: 800, Tags: []string{"text-generation"}},
		{ID: "deadbydawn101/RavenX-CyberAgent-7B-Pentester-BugHunter", Likes: 152, Tags: []string{"text-generation"}},
	}
	ScoreRelevance(models, "hacking")
	if !strings.Contains(models[0].RepoID(), "RavenX") {
		t.Fatalf("alias overlap on id should win, got %s", models[0].RepoID())
	}
}

func TestScoreRelevanceTieBreakLikes(t *testing.T) {
	models := []Model{
		{ID: "a/hacking-small", Likes: 3},
		{ID: "b/hacking-big", Likes: 30},
	}
	ScoreRelevance(models, "hacking")
	if models[0].RepoID() != "b/hacking-big" {
		t.Fatalf("likes tie-break %s", models[0].RepoID())
	}
}

func TestSearchExpandUsesFullAndAliases(t *testing.T) {
	var searches []string
	var sawFull, sawSort bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/models/") && r.URL.Path != "/api/models/" {
			id := strings.TrimPrefix(r.URL.Path, "/api/models/")
			_ = json.NewEncoder(w).Encode(Model{
				ID:          id,
				Description: "card for " + id,
			})
			return
		}
		q := r.URL.Query()
		if q.Get("full") == "true" {
			sawFull = true
		}
		if q.Get("sort") != "" {
			sawSort = true
		}
		if s := q.Get("search"); s != "" {
			searches = append(searches, s)
		}
		w.Header().Set("Content-Type", "application/json")
		switch q.Get("search") {
		case "pentest":
			_, _ = w.Write([]byte(`[{"id":"deadbydawn101/RavenX-CyberAgent-7B-Pentester-BugHunter","likes":152,"downloads":10,"tags":["safetensors"]}]`))
		case "hacking":
			_, _ = w.Write([]byte(`[{"id":"acme/generic-hacking-notes","likes":9,"downloads":1,"tags":["safetensors"]}]`))
		default:
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	t.Cleanup(srv.Close)

	c := New("")
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()
	models, err := c.Search(context.Background(), SearchOpts{
		Query: "hacking", Sort: "relevance", Limit: 10, Expand: true, Full: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !sawFull {
		t.Fatal("expected full=true")
	}
	if sawSort {
		t.Fatal("relevance must omit sort=")
	}
	if !containsString(searches, "hacking") || !containsString(searches, "pentest") {
		t.Fatalf("searches %v", searches)
	}
	if len(models) == 0 || !strings.Contains(models[0].RepoID(), "RavenX") {
		t.Fatalf("RavenX should enter via pentest alias and rank first: %+v", idsOf(models))
	}
}

func TestSearchExpandLikesSortsExpandedPool(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/models/") && r.URL.Path != "/api/models/" {
			_ = json.NewEncoder(w).Encode(Model{ID: strings.TrimPrefix(r.URL.Path, "/api/models/")})
			return
		}
		if r.URL.Query().Get("sort") != "" {
			t.Errorf("must not Hub-sort: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("search") {
		case "pentest":
			_, _ = w.Write([]byte(`[{"id":"x/pentester","likes":2,"downloads":1,"tags":["safetensors"]}]`))
		default:
			_, _ = w.Write([]byte(`[{"id":"x/hacking","likes":50,"downloads":9,"tags":["safetensors"]}]`))
		}
	}))
	t.Cleanup(srv.Close)
	c := New("")
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()
	models, err := c.Search(context.Background(), SearchOpts{
		Query: "hacking", Sort: "likes", Limit: 10, Expand: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(models) < 2 {
		t.Fatalf("pool %v", idsOf(models))
	}
	if models[0].RepoID() != "x/hacking" {
		t.Fatalf("likes should rank expanded pool, got %s", models[0].RepoID())
	}
}

func TestHydrateDescriptionFromGet(t *testing.T) {
	var gets int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/models/") && r.URL.Path != "/api/models/" {
			gets++
			id := strings.TrimPrefix(r.URL.Path, "/api/models/")
			m := Model{ID: id}
			if id == "lab/secure-ops-agent" {
				m.CardData = map[string]any{
					"description": "authorized hacking and pentest agent",
				}
			}
			_ = json.NewEncoder(w).Encode(m)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"lab/secure-ops-agent","likes":12,"tags":["safetensors"]},{"id":"acme/general-chat-7b","likes":5000,"tags":["safetensors"]}]`))
	}))
	t.Cleanup(srv.Close)
	c := New("")
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()
	models, err := c.Search(context.Background(), SearchOpts{
		Query: "hacking", Sort: "relevance", Limit: 10, Expand: true, Full: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if gets == 0 {
		t.Fatal("expected card Get for missing description")
	}
	if len(models) == 0 || models[0].RepoID() != "lab/secure-ops-agent" {
		t.Fatalf("hydrated description should rank first: %+v", idsOf(models))
	}
	if models[0].CardDescription() == "" {
		t.Fatal("description should be stored after Get")
	}
}

func TestCardDescriptionSources(t *testing.T) {
	if (Model{Description: " top "}).CardDescription() != "top" {
		t.Fatal("top-level")
	}
	m := Model{CardData: map[string]any{"description": "from card"}}
	if m.CardDescription() != "from card" {
		t.Fatal(m.CardDescription())
	}
}

func containsString(in []string, want string) bool {
	for _, s := range in {
		if s == want {
			return true
		}
	}
	return false
}
