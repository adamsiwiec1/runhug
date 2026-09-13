package cli

import (
	"testing"

	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/find"
	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/hf"
)

func TestSortHubModelsByLikes(t *testing.T) {
	models := []hf.Model{
		{ID: "low", Likes: 10, Downloads: 9_000},
		{ID: "high", Likes: 800, Downloads: 100},
	}
	sortHubModels(models, "likes")
	if models[0].ID != "high" {
		t.Fatalf("likes first %s", models[0].ID)
	}
	sortHubModels(models, "downloads")
	if models[0].ID != "low" {
		t.Fatalf("downloads first %s", models[0].ID)
	}
}

func TestClampLimitAndSortLabel(t *testing.T) {
	if clampLimit(0) != 1 || clampLimit(8) != 8 || clampLimit(200) != 100 {
		t.Fatalf("clamp %d %d %d", clampLimit(0), clampLimit(8), clampLimit(200))
	}
	if sortLabel("rank") != "rank (likes, downloads, publisher, size)" {
		t.Fatalf("label %s", sortLabel("rank"))
	}
}

func TestHubQueryFromName(t *testing.T) {
	cases := map[string]string{
		"gemma4:e4b":                         "gemma4",
		"qwen3:8b":                           "qwen3",
		"nomic-embed-text:latest":            "nomic-embed-text",
		"/tmp/Qwen2.5-1.5B-Instruct.gguf":    "Qwen2.5-1.5B-Instruct",
		"bartowski/Qwen2.5-3B-Instruct.GGUF": "Qwen2.5-3B-Instruct",
	}
	for in, want := range cases {
		if got := hubQueryFromName(in); got != want {
			t.Fatalf("%q → %q, want %q", in, got, want)
		}
	}
}

func TestExamplePickSkipsEmbedding(t *testing.T) {
	hits := []find.Found{
		{Name: "nomic-embed-text:latest"},
		{Name: "qwen3:8b"},
	}
	if examplePick(hits) != 2 {
		t.Fatalf("pick %d", examplePick(hits))
	}
	if exampleName(hits) != "qwen3" {
		t.Fatalf("name %s", exampleName(hits))
	}
}
