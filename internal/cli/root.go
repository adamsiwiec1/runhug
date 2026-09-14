package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/adamsiwiec1/runhug-cli/internal/version"
)

func Run(args []string) error {
	if len(args) == 0 {
		printUsage(os.Stdout)
		return flag.ErrHelp
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "init":
		return cmdInit(rest)
	case "search":
		return cmdSearch(rest)
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
	fmt.Fprintln(w, "  runhug-cli <command> [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, bold("Setup"))
	fmt.Fprintln(w, "  init               Search NLP setup (nomic-embed-text / connect hf + optional index)")
	fmt.Fprintln(w, "  connect            Save Runpod API key (0600)")
	fmt.Fprintln(w, "  connect hf         Save Hugging Face token (0600); aliases: login hf, hf login")
	fmt.Fprintln(w, "  disconnect [hf]    Forget stored Runpod key or HF token")
	fmt.Fprintln(w)
	fmt.Fprintln(w, bold("Search & index"))
	fmt.Fprintln(w, "  search [query]     Local SQLite index (optional embeddings); --online/--hub for live Hub")
	fmt.Fprintln(w, "  inspect <model>    Hub card, params, VRAM estimate")
	fmt.Fprintln(w, "  update             Refresh local model index from Hub")
	fmt.Fprintln(w, "  update --cli       Print how to upgrade this CLI (go install / releases)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, bold("Runpod"))
	fmt.Fprintln(w, "  deploy <model>     Serverless vLLM (workers min=0)")
	fmt.Fprintln(w, "  list               Local registry + Runpod when connected  (alias: deployments)")
	fmt.Fprintln(w, "  proxy              OpenAI proxy on 127.0.0.1:8080/v1  (alias: serve)")
	fmt.Fprintln(w, "  gpus / import / delete / use / url / status")
	fmt.Fprintln(w)
	fmt.Fprintln(w, bold("Local"))
	fmt.Fprintln(w, "  local add          Models already on this machine; --pick N searches the local index")
	fmt.Fprintln(w, "  local setup        Show / install Ollama, llama.cpp, or MLX")
	fmt.Fprintln(w)
	fmt.Fprintln(w, bold("Config"))
	fmt.Fprintln(w, "  config             Show config dir + settings")
	fmt.Fprintln(w, "  config get|set     Read/write no_color (and show paths)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, bold("Search stack (honest)"))
	fmt.Fprintln(w, "  Default search is local-only: SQLite index (data/models.db or ~/.config/runhug-cli/models.db).")
	fmt.Fprintln(w, "  update refreshes that index from the Hub. search never calls Hub unless --online / --hub.")
	fmt.Fprintln(w, "  Embedding rerank via Ollama nomic-embed-text (optional; local/cached).")
	fmt.Fprintln(w, "  --semantic on by default; --keyword / --no-semantic for lexical-only.")
	fmt.Fprintln(w, "  Not a Hub-wide vector database. No local instruct model required for search.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, bold("Environment"))
	fmt.Fprintln(w, "  HF_TOKEN           Wins over stored hf.token (gated Hub / embeddings)")
	fmt.Fprintln(w, "  RUNPOD_API_KEY     Wins over stored runpod.key")
	fmt.Fprintln(w, "  RUNHUG_CONFIG      Override registry path (RVP_CONFIG still accepted)")
	fmt.Fprintln(w, "  NO_COLOR           Disable ANSI colors (or: config set no_color true)")
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
