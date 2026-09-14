package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/adamsiwiec1/runhug-cli/internal/hf"
)

func TestResolveSearchQueryPrefersFlag(t *testing.T) {
	if got := resolveSearchQuery("cybersec", "positional"); got != "cybersec" {
		t.Fatalf("flag wins: %q", got)
	}
	if got := resolveSearchQuery("", "positional"); got != "positional" {
		t.Fatalf("positional: %q", got)
	}
	if got := resolveSearchQuery("  x  ", " y "); got != "x" {
		t.Fatalf("trim flag: %q", got)
	}
	if got := resolveSearchQuery("", ""); got != "" {
		t.Fatalf("empty %q", got)
	}
}

func TestSearchQueryFlagQ(t *testing.T) {
	fs := newFlagSet("search")
	var q string
	fs.StringVar(&q, "query", "", "")
	fs.StringVar(&q, "q", "", "")
	if err := parseFlags(fs, []string{"-q", "cybersec"}); err != nil {
		t.Fatal(err)
	}
	if q != "cybersec" {
		t.Fatalf("-q %q", q)
	}
	if fs.NArg() != 0 {
		t.Fatalf("args %v", fs.Args())
	}

	fs = newFlagSet("search")
	fs.StringVar(&q, "query", "", "")
	fs.StringVar(&q, "q", "", "")
	q = ""
	if err := parseFlags(fs, []string{"positional", "--query", "from-flag"}); err != nil {
		t.Fatal(err)
	}
	if q != "from-flag" {
		t.Fatalf("--query %q", q)
	}
	got := resolveSearchQuery(q, strings.Join(fs.Args(), " "))
	if got != "from-flag" {
		t.Fatalf("resolve %q", got)
	}
}

func TestSearchHelpMentionsQueryAndDescriptions(t *testing.T) {
	var buf bytes.Buffer
	fs := newFlagSet("search")
	fs.SetOutput(&buf)
	var queryFlag string
	fs.StringVar(&queryFlag, "query", "", "search query (same as positional; wins if both set)")
	fs.StringVar(&queryFlag, "q", "", "search query (same as --query)")
	_ = fs.String("sort", "relevance", "relevance (default; id/tags/description, sort omitted on Hub), likes, or downloads (re-rank a 100-hit expanded pool)")
	_ = fs.Bool("semantic", true, "rerank with embeddings when nomic-embed-text (Ollama) or HF Inference is available")
	_ = fs.Bool("no-semantic", false, "disable embedding rerank (lexical Hub search only)")
	_ = fs.Bool("keyword", false, "alias for --no-semantic (lexical-only)")
	addWordWrapFlags(fs)
	fs.PrintDefaults()
	s := buf.String()
	for _, want := range []string{"-q", "-query", "description", "positional", "semantic", "nomic-embed-text", "no-semantic", "keyword", "word-wrap", "-ww"} {
		if !strings.Contains(s, want) {
			t.Fatalf("search -h missing %q\n%s", want, s)
		}
	}
}

func TestQuotedSearchCmd(t *testing.T) {
	if got := quotedSearchCmd("qwen"); got != "runhug-cli search qwen" {
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
	if !strings.Contains(s, truncateRunes(long, 48)) {
		t.Fatalf("default MODEL should ellipsize\n%s", s)
	}
}

func TestPrintHubResultsWordWrapShowsFullModel(t *testing.T) {
	long := "org-with-a-very-long-name/model-with-an-extremely-long-identifier-that-exceeds-forty-eight"
	trunc := truncateRunes(long, 48)
	m := []hf.Model{{ID: long, Likes: 1, Downloads: 1, Tags: []string{"safetensors"}}}
	var buf bytes.Buffer
	printHubResults(&buf, hubView{Models: m, Sort: "relevance", Limit: 5, WordWrap: true})
	s := buf.String()
	if !strings.Contains(s, long) {
		t.Fatalf("word-wrap should print full MODEL\n%s", s)
	}
	if strings.Contains(s, trunc) {
		t.Fatalf("word-wrap should not ellipsize MODEL\n%s", s)
	}
	if !strings.Contains(s, "inspect "+long) || !strings.Contains(s, "deploy "+long) {
		t.Fatalf("Next: should stay full with wrap\n%s", s)
	}
}

func TestWordWrapFlagsEitherEnables(t *testing.T) {
	for _, args := range [][]string{{"--word-wrap"}, {"-ww"}, {"--ww"}, {"--word-wrap", "-ww"}} {
		fs := newFlagSet("search")
		wordWrap, ww := addWordWrapFlags(fs)
		if err := parseFlags(fs, args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if !*wordWrap && !*ww {
			t.Fatalf("%v: expected wrap", args)
		}
	}
	fs := newFlagSet("search")
	wordWrap, ww := addWordWrapFlags(fs)
	if err := parseFlags(fs, nil); err != nil {
		t.Fatal(err)
	}
	if *wordWrap || *ww {
		t.Fatal("default wrap should be off")
	}
}
