package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/adamsiwiec1/runhug-cli/internal/hf"
)

type hubView struct {
	Query      string
	Models     []hf.Model
	Pick       int
	Sort       string
	Limit      int
	Command    string
	SkipFooter bool
	RankSource string
	Queries    []string
	WordWrap   bool
}

type hubOpts struct {
	Sort     string
	Limit    int
	Command  string
	WordWrap bool
}

func printHubResults(w io.Writer, v hubView) {
	if w == nil {
		w = os.Stdout
	}
	title := "Hugging Face"
	if v.Query != "" {
		title = fmt.Sprintf("Hugging Face  %s", v.Query)
	}
	heading(w, fmt.Sprintf("%s  (%d)", title, len(v.Models)))
	if v.Sort != "" || v.Limit > 0 {
		fmt.Fprintf(w, "%s  %s", dim("sorted by"), sortLabel(v.Sort))
		if v.Limit > 0 {
			fmt.Fprintf(w, "   %s %d", dim("--limit"), v.Limit)
		}
		fmt.Fprintln(w)
		fmt.Fprintln(w)
	}
	if len(v.Models) == 0 {
		fmt.Fprintln(w, yellow("No models on the Hub matched."))
		fmt.Fprintln(w)
		commands(w, "Try a broader query:",
			`runhug-cli search instruct --sort likes --limit 20`,
			`runhug-cli search qwen --engine gguf --limit 20`,
			`runhug-cli search instruct --license apache-2.0 --engine vllm`,
		)
		return
	}

	ids := make([]string, len(v.Models))
	likes := make([]string, len(v.Models))
	dls := make([]string, len(v.Models))
	engines := make([]string, len(v.Models))
	licenses := make([]string, len(v.Models))
	formats := make([]hf.Format, len(v.Models))
	for i, m := range v.Models {
		ids[i] = m.RepoID()
		likes[i] = formatCount(int64(m.Likes))
		dls[i] = formatCount(m.Downloads)
		f := hf.DetectFormat(m)
		formats[i] = f
		engines[i] = string(f.Engine)
		licenses[i] = dash(m.License())
	}
	idMax := 48
	if v.WordWrap {
		idMax = 0
	}
	idW := colWidth("MODEL", ids, idMax)
	likeW := colWidth("LIKES", likes, 0)
	dlW := colWidth("DOWNLOADS", dls, 0)
	engW := colWidth("ENGINE", engines, 0)
	licW := colWidth("LICENSE", licenses, 16)

	fmt.Fprintf(w, "  %s  %s  %s  %s  %s  %s\n",
		dim(padRight("#", 2)),
		dim(padRight("MODEL", idW)),
		dim(padRight("LIKES", likeW)),
		dim(padRight("DOWNLOADS", dlW)),
		dim(padRight("ENGINE", engW)),
		dim(padRight("LICENSE", licW)),
	)
	for i := range v.Models {
		id := displayModel(ids[i], idW, v.WordWrap)
		fmt.Fprintf(w, "  %s  %s  %s  %s  %s  %s\n",
			padRight(fmt.Sprintf("%d", i+1), 2),
			bold(padRight(id, idW)),
			dim(padRight(likes[i], likeW)),
			dim(padRight(dls[i], dlW)),
			engineTag(formats[i].Engine, engW),
			dim(padRight(truncateRunes(licenses[i], licW), licW)),
		)
	}
	fmt.Fprintln(w)

	if v.SkipFooter {
		return
	}
	shown := 1
	if v.Pick > 0 {
		shown = v.Pick
	}
	if shown < 1 || shown > len(v.Models) {
		shown = 1
	}
	id := v.Models[shown-1].RepoID()
	open := []string{
		hubLink(id),
		"runhug-cli inspect " + id,
		"runhug-cli deploy " + id,
	}
	if v.Command != "" {
		open = append(open, v.Command+" --sort likes --limit 20")
	}
	commands(w, "Next:", open...)
}

func engineTag(engine hf.Engine, width int) string {
	s := padRight(string(engine), width)
	switch engine {
	case hf.EngineGGUF:
		return yellow(s)
	case hf.EngineVLLM:
		return green(s)
	default:
		return dim(s)
	}
}

func colWidth(header string, vals []string, max int) int {
	w := utf8.RuneCountInString(header)
	for _, v := range vals {
		n := utf8.RuneCountInString(v)
		if n > w {
			w = n
		}
	}
	if max > 0 && w > max {
		return max
	}
	return w
}

func sortLabel(sortKey string) string {
	switch strings.ToLower(strings.TrimSpace(sortKey)) {
	case "downloads":
		return "downloads"
	case "likes":
		return "likes"
	case "relevance", "relevant", "rank", "":
		return "relevance"
	default:
		return sortKey
	}
}

func clampLimit(n int) int {
	if n < 1 {
		return 1
	}
	if n > 100 {
		return 100
	}
	return n
}

func sortHubModels(models []hf.Model, key string) {
	hf.SortModels(models, key)
}

func quotedSearchCmd(query string) string {
	return quotedCmd("search", query)
}

func quotedCmd(name, query string) string {
	if query == "" {
		return "runhug-cli " + name
	}
	if strings.ContainsAny(query, " \t\"'") {
		return fmt.Sprintf("runhug-cli %s %q", name, query)
	}
	return "runhug-cli " + name + " " + query
}
