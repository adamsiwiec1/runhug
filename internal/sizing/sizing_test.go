package sizing

import (
	"testing"

	"github.com/adamsiwiec/runpod-vllm-proxy/internal/hf"
)

func TestParseParamsFromName(t *testing.T) {
	cases := []struct {
		id   string
		want float64
	}{
		{"Qwen/Qwen2.5-7B-Instruct", 7},
		{"bartowski/Qwen2.5-1.5B-Instruct-GGUF", 1.5},
		{"unsloth/Qwen3-Coder-30B-A3B-Instruct-GGUF", 30},
		{"meta-llama/Llama-3.1-8B-Instruct", 8},
		{"google/gemma-3-27b-it", 27},
		{"mistralai/Mixtral-8x7B-Instruct-v0.1", 56},
	}
	for _, tc := range cases {
		n, _, ok := parseParamsFromName(tc.id)
		if !ok {
			t.Fatalf("%s: not parsed", tc.id)
		}
		got := float64(n) / 1e9
		if got < tc.want*0.99 || got > tc.want*1.01 {
			t.Fatalf("%s: got %.2fB want %.2fB", tc.id, got, tc.want)
		}
	}
}

func TestEstimateFromSafetensors(t *testing.T) {
	m := hf.Model{
		ID: "Qwen/Qwen2.5-7B-Instruct",
		Safetensors: &hf.Safetensors{
			Total:      7615616512,
			Parameters: map[string]int64{"BF16": 7615616512},
		},
	}
	e := EstimateModel(m, hf.Format{Engine: hf.EngineVLLM}, 8192)
	if e.Params != 7615616512 {
		t.Fatalf("params %d", e.Params)
	}
	if e.BytesPerParam != 2 {
		t.Fatalf("bytes/param %v", e.BytesPerParam)
	}
	if e.RequiredGB < 15 || e.RequiredGB > 22 {
		t.Fatalf("required GB %v (want ~19)", e.RequiredGB)
	}
	if e.DiskGB < 40 {
		t.Fatalf("disk %d", e.DiskGB)
	}
}

func TestQuantHalvesVRAM(t *testing.T) {
	m := hf.Model{ID: "org/model-7B", Tags: []string{"awq", "4-bit"}}
	e := EstimateModel(m, hf.DetectFormat(m), 8192)
	if e.BytesPerParam != 0.5 {
		t.Fatalf("quant bytes %v", e.BytesPerParam)
	}
}
