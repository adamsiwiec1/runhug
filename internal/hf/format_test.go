package hf

import "testing"

func TestDetectFormat(t *testing.T) {
	safetensors := Model{
		ID:       "Qwen/Qwen2.5-7B-Instruct",
		Tags:     []string{"safetensors", "text-generation"},
		Siblings: []Sibling{{RFilename: "model.safetensors"}},
	}
	if got := DetectFormat(safetensors); got.Engine != EngineVLLM {
		t.Fatalf("safetensors engine %s", got.Engine)
	}

	gguf := Model{
		ID:          "org/model-gguf",
		LibraryName: "gguf",
		Tags:        []string{"gguf"},
		Siblings:    []Sibling{{RFilename: "model-q4.gguf"}},
	}
	if got := DetectFormat(gguf); got.Engine != EngineGGUF {
		t.Fatalf("gguf engine %s", got.Engine)
	}

	awq := Model{Tags: []string{"safetensors", "awq", "4-bit"}}
	if got := DetectFormat(awq); got.Quant != "awq" || got.Engine != EngineVLLM {
		t.Fatalf("awq %+v", got)
	}
}
