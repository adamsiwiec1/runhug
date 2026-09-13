package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/config"
	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/hf"
	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/index"
)

func cmdIndexSetup(args []string) error {
	fs := newFlagSet("index-setup")
	force := fs.Bool("force", false, "rebuild index even if it exists")
	if err := parseFlags(fs, args); err != nil {
		return err
	}

	indexPath := indexFilePath()
	if index.Exists(indexPath) && !*force {
		fmt.Fprintf(os.Stderr, "%s  Index already exists at %s\n", yellow("⚠"), indexPath)
		fmt.Fprintf(os.Stderr, "   Use --force to rebuild, or run: %s\n", cyan("runpod-vllm-proxy index-update"))
		return nil
	}

	fmt.Fprintf(os.Stderr, "%s Setting up local search index...\n", bold("⚡"))
	fmt.Fprintf(os.Stderr, "   This will fetch model metadata from Hugging Face API\n")
	fmt.Fprintf(os.Stderr, "   (this may take 2-3 minutes)\n\n")

	// Remove existing index if --force
	if *force && index.Exists(indexPath) {
		if err := os.Remove(indexPath); err != nil {
			return fmt.Errorf("remove existing index: %w", err)
		}
	}

	idx, err := index.Open(indexPath)
	if err != nil {
		return fmt.Errorf("open index: %w", err)
	}
	defer idx.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	client := hf.New(config.Load().HFToken)
	
	// Fetch models in batches
	var totalModels int
	limit := 1000
	offset := 0
	startTime := time.Now()

	for {
		fmt.Fprintf(os.Stderr, "\r📥 Fetching models... %d fetched", totalModels)
		
		models, err := client.Search(ctx, hf.SearchOpts{
			Task:   "text-generation",
			Sort:   "downloads",
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			return fmt.Errorf("fetch models: %w", err)
		}

		if len(models) == 0 {
			break
		}

		// Insert into index
		for _, m := range models {
			if err := idx.InsertModel(m); err != nil {
				fmt.Fprintf(os.Stderr, "\n%s  Failed to index %s: %v\n", red("✗"), m.ID, err)
			}
		}

		totalModels += len(models)
		offset += limit

		// Break if we got fewer results than requested (last page)
		if len(models) < limit {
			break
		}

		// Rate limiting
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Fprintf(os.Stderr, "\r")

	// Store metadata
	idx.SetMetadata("created_at", time.Now().Format(time.RFC3339))
	idx.SetMetadata("last_update", time.Now().Format(time.RFC3339))

	elapsed := time.Since(startTime)
	fileInfo, _ := os.Stat(indexPath)
	sizeKB := fileInfo.Size() / 1024

	fmt.Fprintf(os.Stderr, "%s Indexed %s models in %s\n", green("✓"), bold(fmt.Sprintf("%d", totalModels)), elapsed.Round(time.Second))
	fmt.Fprintf(os.Stderr, "%s Saved to %s (%d KB)\n\n", green("✓"), indexPath, sizeKB)
	fmt.Fprintf(os.Stderr, "Now you can search instantly with: %s\n", cyan("runpod-vllm-proxy search -q \"...\""))

	return nil
}

func cmdIndexUpdate(args []string) error {
	fs := newFlagSet("index-update")
	if err := parseFlags(fs, args); err != nil {
		return err
	}

	indexPath := indexFilePath()
	if !index.Exists(indexPath) {
		fmt.Fprintf(os.Stderr, "%s  No index found. Run: %s\n", yellow("⚠"), cyan("runpod-vllm-proxy index-setup"))
		return nil
	}

	idx, err := index.Open(indexPath)
	if err != nil {
		return fmt.Errorf("open index: %w", err)
	}
	defer idx.Close()

	lastUpdate, err := idx.LastUpdate()
	if err != nil {
		return fmt.Errorf("get last update: %w", err)
	}

	fmt.Fprintf(os.Stderr, "%s Updating index (last update: %s ago)...\n", 
		bold("⚡"), formatDuration(time.Since(lastUpdate)))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	client := hf.New(config.Load().HFToken)
	
	// Fetch recently modified models
	var totalUpdated int
	limit := 1000
	offset := 0
	startTime := time.Now()

	for {
		fmt.Fprintf(os.Stderr, "\r📥 Checking for updates... %d updated", totalUpdated)
		
		models, err := client.Search(ctx, hf.SearchOpts{
			Task:   "text-generation",
			Sort:   "downloads",
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			return fmt.Errorf("fetch models: %w", err)
		}

		if len(models) == 0 {
			break
		}

		foundOld := false
		for _, m := range models {
			lastMod, _ := time.Parse(time.RFC3339, m.LastModified)
			if lastMod.Before(lastUpdate) {
				foundOld = true
				break
			}

			if err := idx.InsertModel(m); err != nil {
				fmt.Fprintf(os.Stderr, "\n%s  Failed to update %s: %v\n", red("✗"), m.ID, err)
			} else {
				totalUpdated++
			}
		}

		// If we found models older than our last update, we've caught up
		if foundOld {
			break
		}

		offset += limit
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Fprintf(os.Stderr, "\r")

	idx.SetMetadata("last_update", time.Now().Format(time.RFC3339))

	elapsed := time.Since(startTime)
	fmt.Fprintf(os.Stderr, "%s Updated %s models in %s\n", 
		green("✓"), bold(fmt.Sprintf("%d", totalUpdated)), elapsed.Round(time.Second))

	return nil
}

func cmdIndexInfo(args []string) error {
	indexPath := indexFilePath()
	bundledPath := bundledIndexPath()
	
	// Check for user-local index
	hasLocal := index.Exists(indexPath)
	hasBundled := bundledPath != "" && index.Exists(bundledPath)
	
	if !hasLocal && !hasBundled {
		fmt.Fprintf(os.Stderr, "%s  No index found\n", yellow("⚠"))
		fmt.Fprintf(os.Stderr, "   Run: %s to create your own index\n", cyan("runpod-vllm-proxy index-setup"))
		return nil
	}

	// Show user-local index if exists
	if hasLocal {
		idx, err := index.Open(indexPath)
		if err != nil {
			return fmt.Errorf("open index: %w", err)
		}
		defer idx.Close()

		count, err := idx.Count()
		if err != nil {
			return fmt.Errorf("count models: %w", err)
		}

		lastUpdate, err := idx.LastUpdate()
		if err != nil {
			return fmt.Errorf("get last update: %w", err)
		}

		createdAt, _ := idx.GetMetadata("created_at")
		
		fileInfo, _ := os.Stat(indexPath)
		sizeKB := fileInfo.Size() / 1024

		heading(os.Stdout, "User-Local Search Index")
		fmt.Fprintf(os.Stdout, "  %s  %s\n", dim("Path"), indexPath)
		fmt.Fprintf(os.Stdout, "  %s  %d KB\n", dim("Size"), sizeKB)
		fmt.Fprintf(os.Stdout, "  %s  %s models\n", dim("Models"), bold(fmt.Sprintf("%d", count)))
		if createdAt != "" {
			created, _ := time.Parse(time.RFC3339, createdAt)
			fmt.Fprintf(os.Stdout, "  %s  %s\n", dim("Created"), formatTime(created))
		}
		fmt.Fprintf(os.Stdout, "  %s  %s ago\n", dim("Updated"), formatDuration(time.Since(lastUpdate)))
		fmt.Fprintln(os.Stdout)

		if time.Since(lastUpdate).Hours() > 24*7 {
			fmt.Fprintf(os.Stdout, "%s  Index is over a week old\n", yellow("⚠"))
			fmt.Fprintf(os.Stdout, "   Run: %s\n", cyan("runpod-vllm-proxy index-update"))
			fmt.Fprintln(os.Stdout)
		}
	}
	
	// Show bundled index info
	if hasBundled {
		idx, err := index.Open(bundledPath)
		if err == nil {
			count, _ := idx.Count()
			lastUpdate, _ := idx.LastUpdate()
			fileInfo, _ := os.Stat(bundledPath)
			sizeKB := fileInfo.Size() / 1024
			idx.Close()
			
			if hasLocal {
				fmt.Fprintln(os.Stdout)
			}
			heading(os.Stdout, "Bundled Search Index")
			fmt.Fprintf(os.Stdout, "  %s  %s\n", dim("Path"), bundledPath)
			fmt.Fprintf(os.Stdout, "  %s  %d KB\n", dim("Size"), sizeKB)
			fmt.Fprintf(os.Stdout, "  %s  %s models\n", dim("Models"), bold(fmt.Sprintf("%d", count)))
			fmt.Fprintf(os.Stdout, "  %s  %s ago\n", dim("Indexed"), formatDuration(time.Since(lastUpdate)))
			fmt.Fprintln(os.Stdout)
			
			if !hasLocal {
				fmt.Fprintf(os.Stdout, "%s  Using bundled index (ships with package)\n", dim("ℹ"))
				fmt.Fprintf(os.Stdout, "   Run %s for latest models\n", cyan("runpod-vllm-proxy index-setup"))
				fmt.Fprintln(os.Stdout)
			}
		}
	}

	commands(os.Stdout, "Commands:",
		"runpod-vllm-proxy search -q \"...\"",
		"runpod-vllm-proxy index-update",
		"runpod-vllm-proxy index-setup --force",
	)

	return nil
}

func indexFilePath() string {
	// Use platform-specific config directory for user-local index
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.TempDir()
	}
	return filepath.Join(configDir, "runpod-vllm-proxy", "models.db")
}

func bundledIndexPath() string {
	// Try multiple locations for bundled index
	
	// 1. Relative to executable (production: bin/runpod-vllm-proxy -> ../data/models.db)
	exePath, err := os.Executable()
	if err == nil {
		bundled := filepath.Join(filepath.Dir(exePath), "..", "data", "models.db")
		absPath, _ := filepath.Abs(bundled)
		if _, err := os.Stat(absPath); err == nil {
			return absPath
		}
	}
	
	// 2. Working directory (development: run from repo root)
	if wd, err := os.Getwd(); err == nil {
		bundled := filepath.Join(wd, "data", "models.db")
		if _, err := os.Stat(bundled); err == nil {
			return bundled
		}
	}
	
	// 3. Executable's directory (if data is alongside bin/)
	if exePath, err := os.Executable(); err == nil {
		bundled := filepath.Join(filepath.Dir(exePath), "data", "models.db")
		if _, err := os.Stat(bundled); err == nil {
			return bundled
		}
	}
	
	return ""
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute"
		}
		return fmt.Sprintf("%d minutes", mins)
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", hours)
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}

func formatTime(t time.Time) string {
	return t.Format("Jan 2, 2006 3:04 PM")
}
