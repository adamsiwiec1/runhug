package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/adamsiwiec1/runhug-cli/internal/index"
	"github.com/adamsiwiec1/runhug-cli/internal/version"
)

func cmdUpdate(args []string) error {
	if len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "self", "cli":
			return printCLIUpdateHelp(args[1:])
		case "index":
			args = args[1:]
		}
	}

	fs := newFlagSet("update")
	cliFlag := fs.Bool("cli", false, "print how to upgrade this CLI instead of refreshing the index")
	force := fs.Bool("force", false, "rebuild the local index from scratch")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if *cliFlag {
		return printCLIUpdateHelp(nil)
	}

	heading(os.Stdout, "Update search index")
	fmt.Fprintln(os.Stdout, dim("Semantic search uses a local SQLite model index plus optional embeddings."))
	fmt.Fprintln(os.Stdout, dim("This refreshes that index from Hugging Face (not a Hub-wide vector DB)."))
	fmt.Fprintln(os.Stdout)

	indexPath := indexFilePath()
	if *force || !index.Exists(indexPath) {
		return cmdIndexSetup([]string{"--force"})
	}
	return cmdIndexUpdate(nil)
}

func printCLIUpdateHelp(args []string) error {
	_ = args
	heading(os.Stdout, "Update CLI")
	fmt.Fprintf(os.Stdout, "Current: %s %s\n\n", version.Name, version.Version)
	fmt.Fprintln(os.Stdout, "Install / upgrade with Go:")
	fmt.Fprintln(os.Stdout, "  "+cyan("go install github.com/adamsiwiec1/runhug-cli/cmd/runhug-cli@latest"))
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Or grab a release:")
	fmt.Fprintln(os.Stdout, "  "+cyan("https://github.com/adamsiwiec1/runhug-cli/releases"))
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, dim("Index refresh (separate): runhug-cli update"))
	fmt.Fprintln(os.Stdout)
	return nil
}
