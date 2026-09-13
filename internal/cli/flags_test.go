package cli

import (
	"bytes"
	"flag"
	"strings"
	"testing"
)

func TestParseFlagsAfterArgs(t *testing.T) {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	limit := fs.Int("limit", 15, "")
	asJSON := fs.Bool("json", false, "")
	if err := parseFlags(fs, []string{"qwen2.5", "--limit", "3", "--json"}); err != nil {
		t.Fatal(err)
	}
	if *limit != 3 || !*asJSON {
		t.Fatalf("limit=%d json=%v", *limit, *asJSON)
	}
	if fs.NArg() != 1 || fs.Arg(0) != "qwen2.5" {
		t.Fatalf("args %v", fs.Args())
	}
}

func TestUsageMentionsSearch(t *testing.T) {
	var buf bytes.Buffer
	printUsage(&buf)
	s := buf.String()
	for _, want := range []string{"init", "search", "Hugging Face", "inspect", "connect", "API keys", "disconnect", "deploy", "list", "deployments", "proxy", "local add", "--pick", "--limit", "--sort", "relevance", "--license", "--engine"} {
		if !strings.Contains(s, want) {
			t.Fatalf("usage missing %q\n%s", want, s)
		}
	}
	for _, drop := range []string{"quickstart", "chat"} {
		if strings.Contains(s, drop) {
			t.Fatalf("usage should not mention %q\n%s", drop, s)
		}
	}
}

func TestRemovedCommands(t *testing.T) {
	for _, cmd := range []string{"chat", "quickstart"} {
		err := Run([]string{cmd})
		if err == nil || !strings.Contains(err.Error(), "unknown command") {
			t.Fatalf("%s: %v", cmd, err)
		}
	}
}
