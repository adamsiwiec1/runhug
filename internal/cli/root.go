package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/version"
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
	fmt.Fprintf(w, `%s %s — search Hugging Face, pull locally, deploy to Runpod.

Usage:
  runpod-vllm-proxy <command> [flags]

Setup
  init               Runtime + default local model (--model / --search)
  connect            Print API keys URL, then save a key locally (0600)
  disconnect         Forget the stored key

Search
  search [query]     Hugging Face (--sort relevance|likes|downloads; --engine; --license)
  inspect <model>    Hub card, params, VRAM estimate

Runpod
  deploy <model>     Serverless vLLM (workers min=0)
  list               Local registry + Runpod when connected  (alias: deployments)
  proxy              OpenAI proxy on 127.0.0.1:8080/v1  (alias: serve)
  gpus / import / delete / use / url / status

Local
  local add          Models already on this machine; --pick N searches Hub
  local setup        Show / install Ollama, llama.cpp, or MLX

Environment
  HF_TOKEN           Gated or private Hub models (not stored)
  RUNPOD_API_KEY     Used if set; otherwise the key from connect
  RVP_CONFIG         Override registry path
  NO_COLOR           Disable ANSI colors
`, version.Name, version.Version)
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
