package recommend

import (
	"strings"
	"testing"

	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/hf"
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

func TestScoreDistinctiveTermBeatsPopular(t *testing.T) {
	models := []hf.Model{
		{
			ID:          "Qwen/Qwen2.5-1.5B-Instruct",
			Author:      "Qwen",
			LibraryName: "transformers",
			Tags:        []string{"safetensors", "license:apache-2.0"},
			Likes:       826,
			Downloads:   7_100_000,
		},
		{
			ID:          "0bserverx/Heretic-Coder-8B-GGUF",
			Author:      "0bserverx",
			LibraryName: "gguf",
			Tags:        []string{"gguf"},
			Likes:       5,
			Downloads:   120,
		},
		{
			ID:          "p-e-w/gpt-oss-20b-heretic",
			Author:      "p-e-w",
			LibraryName: "transformers",
			Tags:        []string{"safetensors"},
			Likes:       12,
			Downloads:   850,
		},
	}
	
	// Search for "heretic" - distinctive term should beat popularity
	intent := Intent{
		Raw:           "heretic",
		Query:         "instruct",
		Task:          "chat",
		PreferGGUF:    true,
		MaxParamsB:    0,
		TargetParamsB: 3,
	}
	
	ranked := Score(models, intent)
	
	// The heretic models should rank above the popular Qwen model
	top2 := []string{ranked[0].Model.RepoID(), ranked[1].Model.RepoID()}
	
	foundHeretic1 := false
	foundHeretic2 := false
	for _, id := range top2 {
		if strings.Contains(strings.ToLower(id), "heretic") {
			if !foundHeretic1 {
				foundHeretic1 = true
			} else {
				foundHeretic2 = true
			}
		}
	}
	
	if !foundHeretic1 || !foundHeretic2 {
		t.Fatalf("expected both heretic models in top 2, got: %+v", []string{ranked[0].Model.RepoID(), ranked[1].Model.RepoID(), ranked[2].Model.RepoID()})
	}
}

func TestScoreCybersecurityDistinctiveTerm(t *testing.T) {
	models := []hf.Model{
		{
			ID:          "meta-llama/Llama-3.2-3B-Instruct",
			Author:      "meta-llama",
			LibraryName: "transformers",
			Tags:        []string{"safetensors"},
			Likes:       2600,
			Downloads:   1_600_000,
		},
		{
			ID:          "dealignai/CyberSecurityLLM-v2-GGUF",
			Author:      "dealignai",
			LibraryName: "gguf",
			Tags:        []string{"gguf"},
			Likes:       3,
			Downloads:   200,
		},
		{
			ID:          "Lily-Cybersecurity/Lily-Cybersecurity-7B-v0.2",
			Author:      "Lily-Cybersecurity",
			LibraryName: "transformers",
			Tags:        []string{"safetensors"},
			Likes:       8,
			Downloads:   450,
		},
	}
	
	intent := Intent{
		Raw:           "cybersecurity",
		Query:         "instruct",
		Task:          "chat",
		PreferGGUF:    true,
		MaxParamsB:    0,
		TargetParamsB: 5,
	}
	
	ranked := Score(models, intent)
	
	// Both cybersecurity models should rank above the popular Llama model
	if !strings.Contains(strings.ToLower(ranked[0].Model.RepoID()), "cybersecurity") {
		t.Fatalf("expected cybersecurity model at top, got: %s", ranked[0].Model.RepoID())
	}
}
