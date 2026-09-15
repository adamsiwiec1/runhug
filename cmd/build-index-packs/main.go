// Command build-index-packs fetches Hub model metadata into category SQLite
// packs plus index-manifest.json for GitHub Release assets.
//
//	go run ./cmd/build-index-packs --out dist/index --limit 0
//
// Uses HF_TOKEN / stored hf.token when available.
// --limit 0 (default) = unlimited rows per category (as many Hub models as
// pass --min-likes / --min-downloads). Set --limit N or RUNHUG_INDEX_LIMIT
// only when you want an explicit cap.
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

	"github.com/adamsiwiec1/runhug/internal/config"
	"github.com/adamsiwiec1/runhug/internal/hf"
	"github.com/adamsiwiec1/runhug/internal/packs"
)

func main() {
	out := flag.String("out", "dist/index", "output directory for dbs + manifest")
	limit := flag.Int("limit", 0, "max rows per category (0 = unlimited, unless RUNHUG_INDEX_LIMIT is set)")
	minLikes := flag.Int("min-likes", packs.DefaultMinLikes, "skip models with fewer likes (0 disables)")
	minDownloads := flag.Int("min-downloads", packs.DefaultMinDownloads, "skip models with fewer downloads; early-stop when sorting by downloads (0 disables)")
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
	ctx, cancel := context.WithTimeout(ctx, 12*time.Hour)
	defer cancel()

	effLimit := packs.ResolveLimit(*limit)
	fmt.Fprintf(os.Stderr, "Building index packs → %s (limit=%s min_likes=%d min_downloads=%d)\n",
		*out, limitLabel(effLimit), *minLikes, *minDownloads)
	if token == "" {
		fmt.Fprintf(os.Stderr, "warning: no HF_TOKEN — Hub rate limits will be lower\n")
	}

	opts := packs.BuildOpts{
		OutDir:       *out,
		Limit:        *limit,
		Categories:   catIDs,
		Sleep:        time.Duration(*sleepMs) * time.Millisecond,
		Full:         *full,
		SourceRepo:   packs.ReleaseRepo(),
		MinLikes:     *minLikes,
		MinDownloads: *minDownloads,
	}
	// Build treats (0,0) as package defaults. Explicit --min-likes 0
	// --min-downloads 0 disables filters via MinLikes < 0 sentinel.
	if *minLikes == 0 && *minDownloads == 0 {
		opts.MinLikes = -1
		opts.MinDownloads = 0
	}

	manifest, err := packs.Build(ctx, client, opts)
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

func limitLabel(n int) string {
	if n <= 0 {
		return "unlimited"
	}
	return fmt.Sprintf("%d", n)
}

func shortSHA(s string) string {
	if len(s) <= 12 {
		return s
	}
	return s[:12]
}
