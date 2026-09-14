package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/adamsiwiec1/runhug-cli/internal/hf"
	"github.com/adamsiwiec1/runhug-cli/internal/version"
)

type replSession struct {
	lastSearchResults []hf.Model
	lastSearchQuery   string
}

func cmdInteractive(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("interactive mode takes no arguments")
	}
	return runREPL()
}

func runREPL() error {
	session := &replSession{}
	reader := bufio.NewReader(os.Stdin)

	fmt.Fprintf(os.Stdout, "%s %s — Interactive Mode\n\n", bold(version.Name), version.Version)
	fmt.Fprintln(os.Stdout, "Commands:")
	fmt.Fprintln(os.Stdout, "  "+cyan("search -q \"...\"")+"        Local index search (id, tags, description; --online for Hub)")
	fmt.Fprintln(os.Stdout, "  "+cyan("copy N")+"                Copy model id from last search (N = row number)")
	fmt.Fprintln(os.Stdout, "  "+cyan("inspect <model>")+"       Show model details")
	fmt.Fprintln(os.Stdout, "  "+cyan("deploy <model>")+"        Deploy model to Runpod")
	fmt.Fprintln(os.Stdout, "  "+cyan("connect")+"               Configure Runpod API key")
	fmt.Fprintln(os.Stdout, "  "+cyan("status")+"                Show current deployment status")
	fmt.Fprintln(os.Stdout, "  "+cyan("help")+"                  Show this help")
	fmt.Fprintln(os.Stdout, "  "+cyan("quit")+" / "+cyan("exit")+"          Exit interactive mode")
	fmt.Fprintln(os.Stdout)

	for {
		fmt.Fprint(os.Stdout, bold("runhug-cli> "))
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Fprintln(os.Stdout)
				return nil
			}
			return err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if err := session.handleCommand(line); err != nil {
			if err == errExitREPL {
				fmt.Fprintln(os.Stdout, "Goodbye!")
				return nil
			}
			fmt.Fprintf(os.Stderr, "%s  %s\n", red("error"), err)
		}
		fmt.Fprintln(os.Stdout)
	}
}

var errExitREPL = fmt.Errorf("exit REPL")

func (s *replSession) handleCommand(line string) error {
	parts := tokenize(line)
	if len(parts) == 0 {
		return nil
	}

	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	switch cmd {
	case "quit", "exit", "q":
		return errExitREPL

	case "help", "h", "?":
		return s.cmdHelp()

	case "search", "s":
		return s.cmdSearchREPL(args)

	case "copy", "c":
		return s.cmdCopyREPL(args)

	case "inspect", "i":
		if len(args) == 0 {
			return fmt.Errorf("usage: inspect <model>")
		}
		return cmdInspect(args)

	case "deploy", "d":
		if len(args) == 0 {
			return fmt.Errorf("usage: deploy <model>")
		}
		return cmdDeploy(args)

	case "connect":
		return cmdConnect(args)

	case "status":
		return cmdStatus(args)

	case "list", "ls":
		return cmdList(args)

	default:
		return fmt.Errorf("unknown command %q — type 'help' for available commands", cmd)
	}
}

func (s *replSession) cmdHelp() error {
	fmt.Fprintln(os.Stdout, "Available commands:")
	fmt.Fprintln(os.Stdout, "  "+cyan("search -q \"query\"")+"   Local SQLite index (optional embedding rerank); --online/--hub for live Hub (--engine, --license, --sort, --keyword/--no-semantic)")
	fmt.Fprintln(os.Stdout, "  "+cyan("copy N")+"             Copy model id from row N of last search")
	fmt.Fprintln(os.Stdout, "  "+cyan("inspect <model>")+"    Show model details and VRAM estimates")
	fmt.Fprintln(os.Stdout, "  "+cyan("deploy <model>")+"     Deploy model to Runpod serverless")
	fmt.Fprintln(os.Stdout, "  "+cyan("connect")+"            Configure Runpod API key")
	fmt.Fprintln(os.Stdout, "  "+cyan("status")+"             Show current deployment status")
	fmt.Fprintln(os.Stdout, "  "+cyan("list")+"               List deployments")
	fmt.Fprintln(os.Stdout, "  "+cyan("help")+"               Show this help")
	fmt.Fprintln(os.Stdout, "  "+cyan("quit")+" / "+cyan("exit")+"       Exit interactive mode")
	return nil
}

func (s *replSession) cmdSearchREPL(args []string) error {
	fs := newFlagSet("search")
	sf := registerSearchFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return err
	}

	query := resolveSearchQuery(sf.queryFlag, strings.Join(fs.Args(), " "))
	if query == "" {
		return fmt.Errorf("usage: search -q <query>   (or: search <query>)")
	}
	req := sf.request(query)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	models, meta, err := searchModels(ctx, req)
	if err != nil {
		return err
	}

	s.lastSearchResults = models
	s.lastSearchQuery = query

	printHubResults(os.Stdout, hubView{
		Query:      query,
		Models:     models,
		Sort:       req.Sort,
		Limit:      req.Limit,
		Command:    quotedSearchCmd(query),
		RankSource: meta.RankSource,
		Queries:    meta.Queries,
		WrapWidth:  resolveWrapWidth(*sf.wrap, *sf.wordWrap, *sf.ww),
	})

	return nil
}

func (s *replSession) cmdCopyREPL(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: copy N (where N is the row number from last search)")
	}
	if len(s.lastSearchResults) == 0 {
		return fmt.Errorf("no search results — run 'search -q \"...\"' first")
	}

	idx, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid row number %q", args[0])
	}

	if idx < 1 || idx > len(s.lastSearchResults) {
		return fmt.Errorf("row %d out of range (1-%d)", idx, len(s.lastSearchResults))
	}

	id := s.lastSearchResults[idx-1].RepoID()
	if err := copyToClipboard(id); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "%s  %s\n", green("copied"), bold(id))
	return nil
}

func tokenize(line string) []string {
	var parts []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, r := range line {
		switch {
		case r == '"' || r == '\'':
			if !inQuote {
				inQuote = true
				quoteChar = r
			} else if r == quoteChar {
				inQuote = false
				quoteChar = 0
			} else {
				current.WriteRune(r)
			}
		case r == ' ' || r == '\t':
			if inQuote {
				current.WriteRune(r)
			} else if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}
