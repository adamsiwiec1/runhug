package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/adamsiwiec1/runhug-cli/internal/index"
	"github.com/adamsiwiec1/runhug-cli/internal/packs"
	"github.com/adamsiwiec1/runhug-cli/internal/version"
)

func cmdUpdate(args []string) error {
	if len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "self", "cli":
			return printCLIUpdateHelp(args[1:])
		case "index":
			args = args[1:]
		case "packs":
			args = append([]string{"--packs"}, args[1:]...)
		}
	}

	fs := newFlagSet("update")
	cliFlag := fs.Bool("cli", false, "print how to upgrade this CLI instead of refreshing the index")
	force := fs.Bool("force", false, "rebuild the local index from scratch (Hub scrape)")
	packsFlag := fs.Bool("packs", false, "re-download category packs from latest GitHub Release (full replace)")
	hubOnly := fs.Bool("hub", false, "refresh from Hub API only (ignore pack releases)")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if *cliFlag {
		return printCLIUpdateHelp(nil)
	}

	heading(os.Stdout, "Update search index")
	fmt.Fprintln(os.Stdout, dim("Local SQLite index + optional category packs. Prefer deltas over full re-download."))
	fmt.Fprintln(os.Stdout)

	indexPath := indexFilePath()
	instPath, _ := packs.InstalledPath()
	inst, _ := packs.LoadInstalled(instPath)
	hasPacks := inst != nil && len(inst.SelectedIDs()) > 0

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	if *force {
		return cmdIndexSetup([]string{"--force"})
	}

	if *packsFlag {
		if !hasPacks {
			fmt.Fprintln(os.Stdout, dim("No packs installed yet — prompting for categories."))
			ids, err := promptPackCategories(false)
			if err != nil {
				return err
			}
			return installPackCategories(ctx, ids)
		}
		return updateInstalledPacks(ctx, true)
	}

	if hasPacks && !*hubOnly {
		fmt.Fprintln(os.Stdout, dim("Refreshing installed category packs (delta / Hub since watermark)…"))
		fmt.Fprintln(os.Stdout)
		return updateInstalledPacks(ctx, false)
	}

	if !index.Exists(indexPath) {
		// Offer pack install first when nothing local exists
		if canPrompt() {
			ok, err := confirmPrefErr("No local index — install category packs from GitHub Releases?", true)
			if err != nil {
				return err
			}
			if ok {
				ids, err := promptPackCategories(false)
				if err != nil {
					return err
				}
				if len(ids) > 0 {
					return installPackCategories(ctx, ids)
				}
			}
		}
		return cmdIndexSetup(nil)
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
	fmt.Fprintln(os.Stdout, dim("Pack refresh from Releases: runhug-cli update --packs"))
	fmt.Fprintln(os.Stdout)
	return nil
}
