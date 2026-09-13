package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/adamsiwiec/runpod-vllm-proxy/internal/runtime"
)

func cmdInit(args []string) error {
	fs := newFlagSet("init")
	yes := fs.Bool("yes", false, "install the recommended runtime and default model without a prompt")
	wantRuntime := fs.String("runtime", "", "ollama, llamacpp, or mlx (default: detect / Ollama)")
	model := fs.String("model", "", "Hub id or Ollama tag instead of the default")
	search := fs.String("search", "", "search the Hub and pick instead of the default")
	pick := fs.Int("pick", 0, "with --search, 1-based row (default: ask, or 1 with --yes)")
	limit := fs.Int("limit", 8, "Hub rows when choosing your own (1-100)")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	*limit = clampLimit(*limit)
	if *model == "" && fs.NArg() > 0 {
		*model = strings.TrimSpace(fs.Arg(0))
	}

	heading(os.Stdout, "Init")
	eng, err := resolveEngine(*wantRuntime, *yes, true)
	if err != nil {
		runtime.PrintConfig(os.Stderr, runtime.Detect())
		return err
	}
	printKV(os.Stdout, "runtime", eng.Kind+"  "+dash(eng.Binary))
	if eng.BaseURL != "" {
		if eng.Running {
			printKV(os.Stdout, "openai", cyan(eng.BaseURL)+"  "+green("(up)"))
		} else {
			printKV(os.Stdout, "openai", cyan(eng.BaseURL))
		}
	}
	fmt.Fprintln(os.Stdout)

	switch {
	case *model != "":
		return activateLocal(eng, specFromOverride(*model, eng.Kind))
	case *search != "":
		return initFromSearch(eng, *search, *pick, *limit, *yes)
	case *yes:
		def := runtime.Default()
		printDefaultModel(os.Stdout, eng.Kind, def)
		return activateLocal(eng, specFromDefault(def))
	case canPrompt():
		def := runtime.Default()
		printDefaultModel(os.Stdout, eng.Kind, def)
		ok, err := confirmPrefErr("Install this default locally?", true)
		if err != nil {
			return initNeedChoice()
		}
		if ok {
			return activateLocal(eng, specFromDefault(def))
		}
		fmt.Fprintln(os.Stdout)
		heading(os.Stdout, "Choose your own")
		return initChooseOwn(eng, *limit)
	default:
		def := runtime.Default()
		printDefaultModel(os.Stdout, eng.Kind, def)
		return initNeedChoice()
	}
}

func initNeedChoice() error {
	commands(os.Stdout, "Non-interactive — pick one:",
		"runpod-vllm-proxy init --yes",
		"runpod-vllm-proxy init --model qwen3:8b",
		`runpod-vllm-proxy init --search "coding assistant" --pick 1 --yes`,
	)
	return fmt.Errorf("pass --yes for the default, --model to override, or run init in a terminal")
}

func printDefaultModel(w io.Writer, kind string, d runtime.DefaultModel) {
	fmt.Fprintf(w, "%s  %s\n", dim("Recommended"), bold(d.HF))
	printKV(w, "why", d.Why)
	printKV(w, "pull", d.Pull(kind))
	printKV(w, "hub", cyan(hubLink(d.HF)))
	fmt.Fprintln(w)
}

func initFromSearch(eng runtime.Engine, query string, pick, limit int, yes bool) error {
	models, err := searchHub(query, "likes", "any", "", limit)
	if err != nil {
		return err
	}
	printHubResults(os.Stdout, hubView{
		Query:      query,
		Models:     models,
		Sort:       "likes",
		Limit:      limit,
		SkipFooter: true,
	})
	if len(models) == 0 {
		return fmt.Errorf("no Hub models matched %q", query)
	}
	n := pick
	if n == 0 && yes {
		n = 1
	}
	if n == 0 {
		if !canPrompt() {
			return fmt.Errorf("pass --pick 1..%d (example: --pick 1)", len(models))
		}
		line, err := readLine(fmt.Sprintf("Which row? [1-%d]  (or another query) ", len(models)))
		if err != nil {
			return err
		}
		p, q, repo := parseChoice(line, len(models))
		switch {
		case repo != "":
			return activateLocal(eng, specFromOverride(repo, eng.Kind))
		case q != "":
			return initFromSearch(eng, q, 0, limit, false)
		case p > 0:
			n = p
		default:
			return fmt.Errorf("expected 1..%d, a Hub id, or a search query", len(models))
		}
	}
	if n < 1 || n > len(models) {
		return fmt.Errorf("--pick must be 1..%d", len(models))
	}
	return activateLocal(eng, specFromHub(models[n-1], eng.Kind))
}

func initChooseOwn(eng runtime.Engine, limit int) error {
	line, err := readLine("Hub search, org/model, or Ollama tag: ")
	if err != nil {
		return err
	}
	if line == "" {
		return fmt.Errorf("aborted — no model chosen")
	}
	_, query, direct := parseChoice(line, 0)
	if direct != "" {
		return activateLocal(eng, specFromOverride(direct, eng.Kind))
	}
	if query == "" {
		query = line
	}
	return initFromSearch(eng, query, 0, limit, false)
}
