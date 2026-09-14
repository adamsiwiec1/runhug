// Command build-index-packs fetches Hub model metadata into category SQLite
// packs plus index-manifest.json for GitHub Release assets.
//
//	go run ./cmd/build-index-packs --out dist/index --limit 5000
//
// Uses HF_TOKEN / stored hf.token when available. Cap per pack with
// --limit or RUNHUG_INDEX_LIMIT (default 5000). Packs are top-N samples
// when limited; structure supports larger builds.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/adamsiwiec1/runhug-cli/internal/config"
	"github.com/adamsiwiec1/runhug-cli/internal/hf"
	"github.com/adamsiwiec1/runhug-cli/internal/packs"
)

func main() {
	out := flag.String("out", "dist/index", "output directory for dbs + manifest")
	limit := flag.Int("limit", 0, "max rows per category (0 = RUNHUG_INDEX_LIMIT or 5000)")
	cats := flag.String("categories", "", "comma-separated category ids (default: all)")
	full := flag.Bool("full", true, "request Hub expand fields (cardData, tags, …)")
	sleepMs := flag.Int("sleep-ms", 250, "sleep between Hub pages (rate limits)")
	listCats := flag.Bool("list-categories", false, "print category ids and exit")
	flag.Parse()

	if *listCats {
		for _, c := range packs.DefaultCategories() {
			fmt.Printf("%s\t%s\tpipeline=%s\tfilter=%s\n", c.ID, c.Title, c.Pipeline, c.Filter)
		}
		return
	}

	var catIDs []string
	if s := strings.TrimSpace(*cats); s != "" {
		for _, p := range strings.Split(s, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				catIDs = append(catIDs, p)
			}
		}
	}

	token := config.Load().HFToken
	client := hf.New(token)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 6*time.Hour)
	defer cancel()

	fmt.Fprintf(os.Stderr, "Building index packs → %s (limit=%d)\n", *out, effectiveLimit(*limit))
	if token == "" {
		fmt.Fprintf(os.Stderr, "warning: no HF_TOKEN — Hub rate limits will be lower\n")
	}

	manifest, err := packs.Build(ctx, client, packs.BuildOpts{
		OutDir:     *out,
		Limit:      *limit,
		Categories: catIDs,
		Sleep:      time.Duration(*sleepMs) * time.Millisecond,
		Full:       *full,
		SourceRepo: packs.ReleaseRepo(),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Wrote %s (%d packs)\n", *out+"/index-manifest.json", len(manifest.Packs))
	for _, p := range manifest.Packs {
		fmt.Fprintf(os.Stderr, "  %s  rows=%d  size=%d  sha256=%s…\n",
			p.ID, p.Rows, p.SizeBytes, shortSHA(p.SHA256))
	}
}

func effectiveLimit(n int) int {
	if n > 0 {
		return n
	}
	return packs.DefaultLimit()
}

func shortSHA(s string) string {
	if len(s) <= 12 {
		return s
	}
	return s[:12]
}
