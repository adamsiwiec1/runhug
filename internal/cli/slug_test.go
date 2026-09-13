package cli

import "testing"

func TestSlug(t *testing.T) {
	got := slug("Qwen/Qwen2.5-7B-Instruct")
	if got != "vllm-qwen-qwen2-5-7b-instruct" {
		t.Fatalf("%s", got)
	}
}
