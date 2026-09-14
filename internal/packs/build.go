package packs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/adamsiwiec1/runhug-cli/internal/hf"
	"github.com/adamsiwiec1/runhug-cli/internal/index"
)

// BuildOpts configures pack generation.
type BuildOpts struct {
	OutDir     string
	Limit      int           // per category (0 → env RUNHUG_INDEX_LIMIT or 5000)
	Categories []string      // empty = all defaults
	Sleep      time.Duration // between Hub pages
	Full       bool          // expand cardData etc.
	SourceRepo string
}

// DefaultLimit reads RUNHUG_INDEX_LIMIT or returns 5000.
func DefaultLimit() int {
	if v := strings.TrimSpace(os.Getenv(EnvIndexLimit)); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n > 0 {
			return n
		}
	}
	return 5000
}

// Build writes index-<cat>.db files and index-manifest.json into opts.OutDir.
func Build(ctx context.Context, client *hf.Client, opts BuildOpts) (*Manifest, error) {
	if opts.OutDir == "" {
		return nil, fmt.Errorf("out dir required")
	}
	if opts.Limit <= 0 {
		opts.Limit = DefaultLimit()
	}
	if opts.Sleep <= 0 {
		opts.Sleep = 250 * time.Millisecond
	}
	if err := os.MkdirAll(opts.OutDir, 0755); err != nil {
		return nil, err
	}

	cats := selectCategories(opts.Categories)
	manifest := &Manifest{
		Version:     ManifestVersion,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		SourceRepo:  opts.SourceRepo,
		Packs:       make([]PackInfo, 0, len(cats)),
	}

	for _, cat := range cats {
		if err := ctx.Err(); err != nil {
			return manifest, err
		}
		info, err := buildOne(ctx, client, opts, cat)
		if err != nil {
			return manifest, fmt.Errorf("build %s: %w", cat.ID, err)
		}
		manifest.Packs = append(manifest.Packs, info)
	}

	manPath := filepath.Join(opts.OutDir, ManifestFilename)
	if err := WriteManifest(manPath, manifest); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func selectCategories(ids []string) []Category {
	if len(ids) == 0 {
		return DefaultCategories()
	}
	var out []Category
	for _, id := range ids {
		c, ok := LookupCategory(id)
		if !ok {
			continue
		}
		out = append(out, c)
	}
	return out
}

func buildOne(ctx context.Context, client *hf.Client, opts BuildOpts, cat Category) (PackInfo, error) {
	dbName := DBFilenameFor(cat.ID)
	dbPath := filepath.Join(opts.OutDir, dbName)
	_ = os.Remove(dbPath)

	idx, err := index.Open(dbPath)
	if err != nil {
		return PackInfo{}, err
	}
	defer idx.Close()

	pipelines := []string{}
	if cat.Pipeline != "" {
		pipelines = append(pipelines, cat.Pipeline)
	}
	pipelines = append(pipelines, cat.ExtraPipelines...)
	if len(pipelines) == 0 && cat.Filter == "" {
		pipelines = []string{"any"}
	}

	seen := map[string]bool{}
	var maxLM time.Time
	total := 0
	perPipeLimit := opts.Limit
	if len(pipelines) > 1 {
		perPipeLimit = (opts.Limit + len(pipelines) - 1) / len(pipelines)
	}

	fetch := func(task string) error {
		remaining := opts.Limit - total
		if remaining <= 0 {
			return nil
		}
		lim := perPipeLimit
		if lim > remaining {
			lim = remaining
		}
		models, err := client.ListModels(ctx, hf.ListOpts{
			Task:     task,
			Filter:   cat.Filter,
			Sort:     "downloads",
			Limit:    lim,
			PageSize: 100,
			Sleep:    opts.Sleep,
			Full:     opts.Full,
		})
		if err != nil {
			return err
		}
		for _, m := range models {
			id := m.RepoID()
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			if err := idx.InsertModel(m); err != nil {
				return err
			}
			total++
			if lm := parseLM(m.LastModified); lm.After(maxLM) {
				maxLM = lm
			}
			if total >= opts.Limit {
				break
			}
		}
		return nil
	}

	if len(pipelines) == 0 {
		if err := fetch(""); err != nil {
			return PackInfo{}, err
		}
	} else {
		for _, p := range pipelines {
			if total >= opts.Limit {
				break
			}
			task := p
			if task == "any" {
				task = ""
			}
			if err := fetch(task); err != nil {
				return PackInfo{}, err
			}
		}
	}

	wm := formatWatermark(maxLM)
	_ = idx.SetMetadata("created_at", time.Now().UTC().Format(time.RFC3339))
	_ = idx.SetMetadata("last_update", time.Now().UTC().Format(time.RFC3339))
	_ = idx.SetMetadata("category", cat.ID)
	if wm != "" {
		_ = idx.SetMetadata(MetadataKeyWatermark(cat.ID), wm)
		_ = idx.SetMetadata("watermark", wm)
	}
	if err := idx.Close(); err != nil {
		return PackInfo{}, err
	}

	sum, err := FileSHA256(dbPath)
	if err != nil {
		return PackInfo{}, err
	}
	fi, err := os.Stat(dbPath)
	if err != nil {
		return PackInfo{}, err
	}

	pipeline := cat.Pipeline
	if pipeline == "" && len(cat.ExtraPipelines) > 0 {
		pipeline = strings.Join(append([]string{cat.Pipeline}, cat.ExtraPipelines...), ",")
	} else if len(cat.ExtraPipelines) > 0 {
		pipeline = strings.Join(append([]string{cat.Pipeline}, cat.ExtraPipelines...), ",")
	}

	return PackInfo{
		ID:         cat.ID,
		Title:      cat.Title,
		Pipeline:   strings.Trim(pipeline, ","),
		Filter:     cat.Filter,
		Rows:       total,
		SizeBytes:  fi.Size(),
		SHA256:     sum,
		DBFilename: dbName,
		Watermark:  wm,
	}, nil
}
