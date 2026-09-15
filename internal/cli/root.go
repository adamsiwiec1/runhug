package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/adamsiwiec1/runhug/internal/version"
)

func Run(args []string) error {
	if len(args) == 0 {
		printUsage(os.Stdout)
		return flag.ErrHelp
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "wizard", "guide", "guided", "setup":
		return cmdWizard(rest)
	case "init":
		return cmdInit(rest)
	case "search":
		return cmdSearch(rest)
	case "recommend":
		return cmdRecommend(rest)
	case "inspect":
		return cmdInspect(rest)
	case "connect":
		return cmdConnect(rest)
	case "disconnect":
		return cmdDisconnect(rest)
	case "login":
		return cmdLogin(rest)
	case "hf":
		return cmdHF(rest)
	case "config":
		return cmdConfig(rest)
	case "update":
		return cmdUpdate(rest)
	case "index-setup":
		return cmdIndexSetup(rest)
	case "index-update":
		return cmdIndexUpdate(rest)
	case "index-info", "index":
		return cmdIndexInfo(rest)
	case "deploy":
		return cmdDeploy(rest)
	case "local":
		return cmdLocal(rest)
	case "list", "deployments":
		return cmdList(rest)
	case "use":
		return cmdUse(rest)
	case "url":
		return cmdURL(rest)
	case "proxy", "serve":
		return cmdProxy(rest)
	case "run":
		return cmdRun(rest)
	case "start":
		return cmdStart(rest)
	case "delete":
		return cmdDelete(rest)
	case "status":
		return cmdStatus(rest)
	case "gpus":
		return cmdGPUs(rest)
	case "import":
		return cmdImport(rest)
	case "version", "-v", "--version":
		fmt.Printf("%s %s\n", version.Name, version.Version)
		return nil
	case "help", "-h", "--help":
		printUsage(os.Stdout)
		return nil
	default:
		return fmt.Errorf("unknown command %q\n\nRun `%s help` for usage", cmd, version.Name)
	}
}

func printUsage(w io.Writer) {
	printBanner(w)
	printTagline(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  runhug <command> [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, bold("Setup"))
	fmt.Fprintln(w, "  wizard             Interactive guided setup (aliases: guide, guided, setup)")
	fmt.Fprintln(w, "  init               Search NLP setup (embedder + category index packs)")
	fmt.Fprintln(w, "  connect            Save Runpod API key (0600)")
	fmt.Fprintln(w, "  connect hf         Save Hugging Face token (0600); aliases: login hf, hf login")
	fmt.Fprintln(w, "  disconnect [hf]    Forget stored Runpod key or HF token")
	fmt.Fprintln(w)
	fmt.Fprintln(w, bold("Search & index"))
	fmt.Fprintln(w, "  search [query]     Local SQLite index (optional embeddings); --online/--hub for live Hub")
	fmt.Fprintln(w, "  recommend [query]  Shortlist (+ models on this machine) + optional LLM advisor; GPU pool hints")
	fmt.Fprintln(w, "  recommend gpu <m>  Suggest Runpod GPU pool / VRAM for one model")
	fmt.Fprintln(w, "  inspect <model>    Hub card, params, VRAM estimate")
	fmt.Fprintln(w, "  update             Refresh index (pack deltas / Hub since watermark)")
	fmt.Fprintln(w, "  update --limit N   Cap Hub delta upserts (0=unlimited; default 2000)")
	fmt.Fprintln(w, "  update --packs     Re-download category packs from latest GitHub Release")
	fmt.Fprintln(w, "  update --cli       Print how to upgrade this CLI (go install / releases)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, bold("Runpod"))
	fmt.Fprintln(w, "  deploy <model>     Serverless vLLM (workers min=0)")
	fmt.Fprintln(w, "  list               Local registry + Runpod when connected  (alias: deployments)")
	fmt.Fprintln(w, "  proxy              OpenAI proxy on 127.0.0.1:8080/v1  (alias: serve)")
	fmt.Fprintln(w, "  gpus / import / delete / use / url / status")
	fmt.Fprintln(w)
	fmt.Fprintln(w, bold("Chat & agents"))
	fmt.Fprintln(w, "  run [model]        Interactive chat REPL against an OpenAI endpoint (-q for one-shot)")
	fmt.Fprintln(w, "  start <agent>      Launch coding agent via local Anthropic→OpenAI bridge (claude; codex stub)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, bold("Local"))
	fmt.Fprintln(w, "  local add          Models already on this machine; --pick N searches the local index")
	fmt.Fprintln(w, "  local setup        Show / install Ollama, llama.cpp, or MLX")
	fmt.Fprintln(w)
	fmt.Fprintln(w, bold("Config"))
	fmt.Fprintln(w, "  config             Show config dir + settings")
	fmt.Fprintln(w, "  config get|set     no_color, update_limit, advisor_base_url, advisor_model")
	fmt.Fprintln(w)
	fmt.Fprintln(w, bold("Tips"))
	fmt.Fprintln(w, "  New here?        runhug wizard")
	fmt.Fprintln(w, "  Keys / tokens    runhug connect · runhug connect hf  (env overrides stored)")
	fmt.Fprintln(w, "  Search           local SQLite index by default; --online/--hub for live Hub")
	fmt.Fprintln(w, "  Docs             https://adamsiwiec1.github.io/runhug-cli/")
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

func parseFlags(fs *flag.FlagSet, args []string) error {
	bools := map[string]bool{}
	fs.VisitAll(func(f *flag.Flag) {
		type boolFlag interface{ IsBoolFlag() bool }
		if v, ok := f.Value.(boolFlag); ok && v.IsBoolFlag() {
			bools[f.Name] = true
		}
	})
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			flags = append(flags, a)
			name := strings.TrimLeft(a, "-")
			if strings.Contains(name, "=") || bools[name] {
				continue
			}
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		pos = append(pos, a)
	}
	return fs.Parse(append(flags, pos...))
}

func writeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func confirm(prompt string) bool {
	fmt.Fprint(os.Stderr, prompt+" [y/N] ")
	var s string
	_, _ = fmt.Fscanln(os.Stdin, &s)
	s = strings.TrimSpace(strings.ToLower(s))
	return s == "y" || s == "yes"
}

func parseKV(pairs []string) (map[string]string, error) {
	out := map[string]string{}
	for _, p := range pairs {
		k, v, ok := strings.Cut(p, "=")
		if !ok || strings.TrimSpace(k) == "" {
			return nil, fmt.Errorf("expected KEY=VALUE, got %q", p)
		}
		out[strings.TrimSpace(k)] = v
	}
	return out, nil
}

type stringsFlag []string

func (s *stringsFlag) String() string { return strings.Join(*s, ",") }
func (s *stringsFlag) Set(v string) error {
	*s = append(*s, v)
	return nil
}
