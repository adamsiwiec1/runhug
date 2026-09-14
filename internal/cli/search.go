package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/adamsiwiec1/runhug-cli/internal/config"
	"github.com/adamsiwiec1/runhug-cli/internal/family"
	"github.com/adamsiwiec1/runhug-cli/internal/hf"
)

func cmdSearch(args []string) error {
	fs := newFlagSet("search")
	var queryFlag string
	fs.StringVar(&queryFlag, "query", "", "search query (same as positional; wins if both set)")
	fs.StringVar(&queryFlag, "q", "", "search query (same as --query)")
	author := fs.String("author", "", "filter by Hugging Face org or user")
	task := fs.String("task", "auto", "pipeline_tag: auto (detect image/audio/… else any), any, text-generation, text-to-image, …")
	library := fs.String("library", "", "library filter (transformers, …)")
	filter := fs.String("filter", "", "extra Hub tag filter (safetensors, gguf, …)")
	license := fs.String("license", "", "license filter (apache-2.0, mit, gemma, other, …)")
	engine := fs.String("engine", "", "engine filter (vllm, gguf, …)")
	sort := fs.String("sort", "relevance", "relevance (default; embedding rerank when available, else id/tags/description), likes, or downloads (re-rank the same 100-hit pool)")
	limit := fs.Int("limit", 15, "rows to show (1-100)")
	semanticOn := fs.Bool("semantic", true, "rerank with embeddings when nomic-embed-text (Ollama) or HF Inference is available")
	noSemantic := fs.Bool("no-semantic", false, "disable embedding rerank (lexical Hub/index search only)")
	keyword := fs.Bool("keyword", false, "alias for --no-semantic (lexical-only)")
	wordWrap, ww := addWordWrapFlags(fs)
	asJSON := fs.Bool("json", false, "print JSON")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	*limit = clampLimit(*limit)
	query := resolveSearchQuery(queryFlag, strings.Join(fs.Args(), " "))
	resolvedTask := hf.ResolveTask(*task, query)
	wantSemantic := *semanticOn && !*noSemantic && !*keyword

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	client := hf.New(config.Load().HFToken)
	models, note, err := searchRanked(ctx, client, hf.SearchOpts{
		Query:   query,
		Author:  *author,
		Task:    resolvedTask,
		Library: *library,
		Filter:  *filter,
		License: *license,
		Engine:  *engine,
	}, *sort, *limit, wantSemantic)
	if err != nil {
		return err
	}
	if note != "" {
		fmt.Fprintln(os.Stderr, dim(note))
	}
	if *asJSON {
		return writeJSON(models)
	}
	printHubResults(os.Stdout, hubView{
		Query:    query,
		Models:   models,
		Sort:     *sort,
		Limit:    *limit,
		Command:  quotedCmd("search", query),
		WordWrap: *wordWrap || *ww,
	})
	return nil
}

func searchAndPrint(query string, opts hubOpts) error {
	query = strings.TrimSpace(query)
	if query == "" {
		return fmt.Errorf("usage: runhug-cli search <query>")
	}
	if opts.Sort == "" {
		opts.Sort = "relevance"
	}
	opts.Limit = clampLimit(opts.Limit)
	if opts.Command == "" {
		opts.Command = quotedCmd("search", query)
	}
	models, err := searchHub(query, opts.Sort, "any", "", opts.Limit)
	if err != nil {
		return err
	}
	printHubResults(os.Stdout, hubView{
		Query:    query,
		Models:   models,
		Sort:     opts.Sort,
		Limit:    opts.Limit,
		Command:  opts.Command,
		WordWrap: opts.WordWrap,
	})
	return nil
}

func addWordWrapFlags(fs *flag.FlagSet) (wordWrap, ww *bool) {
	wordWrap = fs.Bool("word-wrap", false, "print full MODEL names (no ellipsis)")
	ww = fs.Bool("ww", false, "same as --word-wrap")
	return wordWrap, ww
}

func searchHub(query, sort, task, filter string, limit int) ([]hf.Model, error) {
	if sort == "" {
		sort = "relevance"
	}
	if task == "" {
		task = "any"
	}
	if limit <= 0 {
		limit = 10
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	client := hf.New(config.Load().HFToken)
	models, note, err := searchRanked(ctx, client, hf.SearchOpts{
		Query:  query,
		Task:   task,
		Filter: filter,
	}, sort, limit, true)
	if note != "" {
		fmt.Fprintln(os.Stderr, dim(note))
	}
	return models, err
}

// resolveSearchQuery prefers --query/-q when set, otherwise the positional words.
func resolveSearchQuery(flagQuery, positional string) string {
	flagQuery = strings.TrimSpace(flagQuery)
	positional = strings.TrimSpace(positional)
	if flagQuery != "" {
		return flagQuery
	}
	return positional
}

func formatCount(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func yn(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func cmdInspect(args []string) error {
	fs := newFlagSet("inspect")
	maxLen := fs.Int("max-len", 8192, "context length used for the VRAM estimate")
	gpu := fs.String("gpu", "", "force this GPU pool in the recommendation")
	asJSON := fs.Bool("json", false, "print JSON")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: runhug-cli inspect <org/model>")
	}
	modelID := fs.Arg(0)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	env := config.Load()
	model, err := hf.New(env.HFToken).Get(ctx, modelID)
	if err != nil {
		return err
	}
	format := hf.DetectFormat(*model)
	est := inspectEstimate(*model, format, *maxLen)

	var plan any
	var recText string
	if env.RunpodAPIKey != "" {
		recText, plan, err = inspectGPU(ctx, env.RunpodAPIKey, est.RequiredGB, *gpu)
		if err != nil {
			recText = "GPU catalog: " + err.Error()
		}
	} else {
		recText = "run `runhug-cli connect` to pick a live serverless GPU pool"
	}

	if *asJSON {
		return writeJSON(map[string]any{
			"model":    model,
			"format":   format,
			"estimate": est,
			"gpu":      plan,
		})
	}

	heading(os.Stdout, "Inspect")
	fmt.Fprintf(os.Stdout, "%s  %s\n", dim("Model"), bold(model.RepoID()))
	printKV(os.Stdout, "url", cyan(hubLink(model.RepoID())))
	printKV(os.Stdout, "task", dash(model.PipelineTag))
	printKV(os.Stdout, "library", dash(model.LibraryName))
	printKV(os.Stdout, "license", dash(model.License()))
	printKV(os.Stdout, "gated", yn(model.IsGated()))
	printKV(os.Stdout, "downloads", fmt.Sprintf("%s    likes %d", formatCount(model.Downloads), model.Likes))
	fmt.Fprintf(os.Stdout, "  %s  %s", dim(padRight("format", 9)), format.Engine)
	if format.Quant != "" {
		fmt.Fprintf(os.Stdout, "  quantization=%s", format.Quant)
	}
	fmt.Println()
	printKV(os.Stdout, "params", fmt.Sprintf("%s  (%s)", est.ParamsLabel(), dash(est.ParamsSource)))
	printKV(os.Stdout, "precision", fmt.Sprintf("%s  (%.2g bytes/param)", est.Precision, est.BytesPerParam))
	if est.WeightGB > 0 {
		printKV(os.Stdout, "weights", fmt.Sprintf("%.1f GB", est.WeightGB))
		printKV(os.Stdout, "vram est", fmt.Sprintf("%.1f GB  (×%.2f overhead, max_len=%d)", est.RequiredGB, est.OverheadFactor, *maxLen))
	} else {
		printKV(os.Stdout, "vram est", "unknown")
	}
	printKV(os.Stdout, "disk", fmt.Sprintf("%d GB container (ephemeral)", est.DiskGB))
	if format.Engine == hf.EngineGGUF {
		printKV(os.Stdout, "engine", yellow("GGUF — do not deploy on worker-vllm; use a llama.cpp worker"))
	} else {
		printKV(os.Stdout, "engine", green("vLLM"))
	}
	printKV(os.Stdout, "gpu", recText)
	if extras := family.EnvFor(model.RepoID()); len(extras) > 0 {
		var bits []string
		for k, v := range extras {
			bits = append(bits, k+"="+v)
		}
		printKV(os.Stdout, "family", strings.Join(bits, "  "))
	}
	for _, n := range est.Notes {
		fmt.Fprintf(os.Stdout, "  %s  %s\n", yellow(padRight("note", 9)), n)
	}
	fmt.Fprintln(os.Stdout)
	next := []string{
		"runhug-cli deploy " + model.RepoID(),
		"runhug-cli connect",
	}
	if format.Engine == hf.EngineGGUF {
		next = []string{
			"runhug-cli init --model " + model.RepoID(),
			"runhug-cli search " + model.RepoID() + " --sort likes",
		}
	}
	commands(os.Stdout, "Next:", next...)
	return nil
}
