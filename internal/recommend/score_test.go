package recommend

import (
	"testing"

	"github.com/adamsiwiec1/runhug/internal/hf"
)

func TestScoreDropsOversizedPrefersKnown(t *testing.T) {
	models := []hf.Model{
		{
			ID:          "anon/huge-70B-Instruct-GGUF",
			LibraryName: "gguf",
			Tags:        []string{"gguf"},
			Likes:       9000,
			Downloads:   1_000_000,
		},
		{
			ID:          "unknown/mystery-1.5B-base-GGUF",
			Author:      "unknown",
			LibraryName: "gguf",
			Tags:        []string{"gguf"},
			Likes:       20,
			Downloads:   400,
		},
		{
			ID:          "Qwen/Qwen2.5-1.5B-Instruct-GGUF",
			Author:      "Qwen",
			LibraryName: "gguf",
			Tags:        []string{"gguf", "license:apache-2.0"},
			Likes:       200,
			Downloads:   50_000,
		},
	}
	ranked := Score(models, Intent{PreferGGUF: true, MaxParamsB: 4, TargetParamsB: 1.7})
	if len(ranked) != 2 {
		t.Fatalf("want 2 after dropping 70B, got %d", len(ranked))
	}
	if ranked[0].Model.RepoID() != "Qwen/Qwen2.5-1.5B-Instruct-GGUF" {
		t.Fatalf("top %+v", ranked[0].Model.RepoID())
	}
}

func TestScoreLikesBeatUnknown(t *testing.T) {
	models := []hf.Model{
		{ID: "bartowski/Qwen2.5-3B-Instruct-GGUF", Author: "bartowski", LibraryName: "gguf", Tags: []string{"gguf"}, Likes: 80, Downloads: 10_000},
		{ID: "randouser/Qwen2.5-3B-Instruct-GGUF", Author: "randouser", LibraryName: "gguf", Tags: []string{"gguf"}, Likes: 80, Downloads: 10_000},
	}
	ranked := Score(models, Intent{PreferGGUF: true, MaxParamsB: 8, TargetParamsB: 3})
	if ranked[0].Model.Author != "bartowski" {
		t.Fatalf("known publisher should win, got %s", ranked[0].Model.Author)
	}
}
