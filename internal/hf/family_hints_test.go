package hf

import "testing"

func TestFamilyHintsBaseModelAndArch(t *testing.T) {
	m := Model{
		ID: "empero-ai/Qwythos-9B-Claude-Mythos-5-1M",
		Tags: []string{"function-calling", "tool-use", "qwen3_5"},
		CardData: map[string]any{"base_model": "Qwen/Qwen3.5-9B"},
		Config: map[string]any{
			"model_type":     "qwen3_5",
			"architectures":  []any{"Qwen3_5ForConditionalGeneration"},
		},
	}
	h := m.FamilyHints()
	joined := ""
	for _, s := range h {
		joined += " " + s
	}
	for _, want := range []string{"Qwen/Qwen3.5-9B", "qwen3_5", "Qwen3_5ForConditionalGeneration", "function-calling"} {
		found := false
		for _, s := range h {
			if s == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing %q in %v", want, h)
		}
	}
	_ = joined
}
