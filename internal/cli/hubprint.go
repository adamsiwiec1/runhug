package cli

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/adamsiwiec/runpod-vllm-proxy/internal/hf"
)

type hubView struct {
	Query      string
	Models     []hf.Model
	Pick       int
	Sort       string
	Limit      int
	Command    string
	SkipFooter bool
}

type hubOpts struct {
	Sort    string
	Limit   int
	Command string
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
			`runpod-vllm-proxy search instruct --sort likes --limit 20`,
			`runpod-vllm-proxy search qwen --filter gguf --limit 20`,
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
	idW := colWidth("MODEL", ids, 48)
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
		id := truncateRunes(ids[i], idW)
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
		"runpod-vllm-proxy inspect " + id,
		"runpod-vllm-proxy deploy " + id,
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
	case "rank":
		return "rank (likes, downloads, publisher, size)"
	case "downloads":
		return "downloads"
	case "lastmodified":
		return "lastModified"
	case "trendingscore":
		return "trendingScore"
	case "likes", "":
		return "likes"
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
	switch strings.ToLower(key) {
	case "downloads":
		sort.SliceStable(models, func(i, j int) bool {
			return models[i].Downloads > models[j].Downloads
		})
	case "likes":
		sort.SliceStable(models, func(i, j int) bool {
			return models[i].Likes > models[j].Likes
		})
	}
}

func quotedCmd(name, query string) string {
	if query == "" {
		return "runpod-vllm-proxy " + name
	}
	if strings.ContainsAny(query, " \t\"'") {
		return fmt.Sprintf("runpod-vllm-proxy %s %q", name, query)
	}
	return "runpod-vllm-proxy " + name + " " + query
}
