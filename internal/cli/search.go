package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/config"
	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/family"
	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/hf"
)

func cmdSearch(args []string) error {
	fs := newFlagSet("search")
	author := fs.String("author", "", "filter by Hugging Face org or user")
	task := fs.String("task", "text-generation", "pipeline_tag (text-generation, any, …)")
	library := fs.String("library", "", "library filter (transformers, …)")
	filter := fs.String("filter", "", "extra Hub tag filter (safetensors, …)")
	license := fs.String("license", "", "license filter (apache-2.0, mit, …)")
	engine := fs.String("engine", "", "engine filter (vllm, gguf)")
	sort := fs.String("sort", "relevance", "relevance (default), likes, downloads")
	limit := fs.Int("limit", 15, "max results (1-100)")
	asJSON := fs.Bool("json", false, "print JSON")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	*limit = clampLimit(*limit)
	query := strings.Join(fs.Args(), " ")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := hf.New(config.Load().HFToken)
	models, err := client.Search(ctx, hf.SearchOpts{
		Query:   query,
		Author:  *author,
		Task:    *task,
		Library: *library,
		Filter:  *filter,
		License: *license,
		Engine:  *engine,
		Sort:    *sort,
		Limit:   *limit,
	})
	if err != nil {
		return err
	}
	if *asJSON {
		return writeJSON(models)
	}
	printHubResults(os.Stdout, hubView{
		Query:   query,
		Models:  models,
		Sort:    *sort,
		Limit:   *limit,
		Command: quotedCmd("search", query),
	})
	return nil
}

func searchAndPrint(query string, opts hubOpts) error {
	query = strings.TrimSpace(query)
	if query == "" {
		return fmt.Errorf("usage: runpod-vllm-proxy search <query>")
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
		Query:   query,
		Models:  models,
		Sort:    opts.Sort,
		Limit:   opts.Limit,
		Command: opts.Command,
	})
	return nil
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return hf.New(config.Load().HFToken).Search(ctx, hf.SearchOpts{
		Query:  query,
		Task:   task,
		Filter: filter,
		Sort:   sort,
		Limit:  limit,
	})
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
		return fmt.Errorf("usage: runpod-vllm-proxy inspect <org/model>")
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
		recText = "run `runpod-vllm-proxy connect` to pick a live serverless GPU pool"
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
		"runpod-vllm-proxy deploy " + model.RepoID(),
		"runpod-vllm-proxy connect",
	}
	if format.Engine == hf.EngineGGUF {
		next = []string{
			"runpod-vllm-proxy init --model " + model.RepoID(),
			"runpod-vllm-proxy search " + model.RepoID() + " --sort likes",
		}
	}
	commands(os.Stdout, "Next:", next...)
	return nil
}
