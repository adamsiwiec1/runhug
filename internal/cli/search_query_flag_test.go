package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/hf"
)

func TestQuotedSearchCmd(t *testing.T) {
	if got := quotedSearchCmd("qwen"); got != "runpod-vllm-proxy search qwen" {
		t.Fatalf("%s", got)
	}
	if got := quotedSearchCmd("instruct coder"); !strings.Contains(got, `search "instruct coder"`) {
		t.Fatalf("%s", got)
	}
}

func TestPrintHubResultsNextUsesFullID(t *testing.T) {
	var buf bytes.Buffer
	long := "org-with-a-very-long-name/model-with-an-extremely-long-identifier-that-exceeds-forty-eight"
	printHubResults(&buf, hubView{
		Models: []hf.Model{{ID: long, Likes: 1, Downloads: 1, Tags: []string{"safetensors"}}},
		Sort:   "relevance",
		Limit:  5,
	})
	s := buf.String()
	if !strings.Contains(s, "inspect "+long) || !strings.Contains(s, "deploy "+long) {
		t.Fatalf("footer should contain full repo id\n%s", s)
	}
	if !strings.Contains(s, "https://huggingface.co/"+long) {
		t.Fatalf("footer should contain full Hub URL\n%s", s)
	}
}
