package cli

import (
	"strings"
	"testing"
)

func TestSearchRequiresQueryFlag(t *testing.T) {
	err := cmdSearch([]string{"qwen", "--limit", "1"})
	if err == nil || !strings.Contains(err.Error(), "-q") {
		t.Fatalf("want -q error, got %v", err)
	}
}

func TestQuotedSearchCmd(t *testing.T) {
	if got := quotedSearchCmd("qwen"); got != "runpod-vllm-proxy search -q qwen" {
		t.Fatalf("%s", got)
	}
	if got := quotedSearchCmd("instruct coder"); !strings.Contains(got, `search -q "instruct coder"`) {
		t.Fatalf("%s", got)
	}
}
