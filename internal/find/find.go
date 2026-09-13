package find

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/adamsiwiec1/runhug-cli/internal/hf"
)

type Found struct {
	Kind   string // gguf, ollama
	Name   string
	Path   string
	Size   int64
	Source string
}

func Roots() []string {
	var out []string
	if extra := strings.TrimSpace(os.Getenv("RVP_MODELS")); extra != "" {
		for _, p := range strings.Split(extra, string(os.PathListSeparator)) {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, expandHome(p))
			}
		}
	}
	home, _ := os.UserHomeDir()
	cache, _ := os.UserCacheDir()
	if dir, err := hf.CacheDir(); err == nil {
		out = append(out, dir)
	}
	if home != "" {
		out = append(out,
			filepath.Join(home, "models"),
			filepath.Join(home, "Models"),
			filepath.Join(home, "llm"),
			filepath.Join(home, "llms"),
			filepath.Join(home, "gguf"),
			filepath.Join(home, ".lmstudio", "models"),
			filepath.Join(home, "Library", "Application Support", "LM Studio", "models"),
		)
	}
	if cache != "" {
		out = append(out,
			filepath.Join(cache, "lm-studio", "models"),
			filepath.Join(cache, "llama.cpp"),
			filepath.Join(cache, "huggingface", "hub"),
		)
	}
	return uniqExisting(out)
}

func Scan() []Found {
	return append(scanRoots(Roots()), ollamaModels()...)
}

func scanRoots(roots []string) []Found {
	var out []Found
	seen := map[string]bool{}
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d == nil {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			if depth(rel) > 8 {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				name := d.Name()
				if name == ".git" || name == "node_modules" || name == "blobs" || strings.HasPrefix(name, "datasets--") {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(strings.ToLower(d.Name()), ".gguf") {
				return nil
			}
			low := strings.ToLower(d.Name())
			if strings.Contains(low, "mmproj") || strings.Contains(low, "imatrix") {
				return nil
			}
			abs, err := filepath.Abs(path)
			if err != nil {
				return nil
			}
			if seen[abs] {
				return nil
			}
			seen[abs] = true
			info, err := d.Info()
			var size int64
			if err == nil {
				size = info.Size()
			}
			out = append(out, Found{
				Kind:   "gguf",
				Name:   strings.TrimSuffix(d.Name(), ".gguf"),
				Path:   abs,
				Size:   size,
				Source: root,
			})
			return nil
		})
	}
	return out
}

func Filter(in []Found, q string) []Found {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return in
	}
	var out []Found
	for _, f := range in {
		if strings.Contains(strings.ToLower(f.Name), q) || strings.Contains(strings.ToLower(f.Path), q) {
			out = append(out, f)
		}
	}
	return out
}

func ollamaModels() []Found {
	seen := map[string]bool{}
	var out []Found
	add := func(name, source string) {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, Found{
			Kind:   "ollama",
			Name:   name,
			Path:   name,
			Source: source,
		})
	}
	for _, name := range ollamaManifestNames() {
		add(name, "ollama")
	}
	bin, err := exec.LookPath("ollama")
	if err != nil {
		return out
	}
	raw, err := exec.Command(bin, "list").Output()
	if err != nil {
		return out
	}
	for i, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || i == 0 && strings.HasPrefix(strings.ToUpper(line), "NAME") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		add(fields[0], "ollama")
	}
	return out
}

func ollamaHome() string {
	if p := strings.TrimSpace(os.Getenv("OLLAMA_MODELS")); p != "" {
		return expandHome(p)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".ollama", "models")
}

func ollamaManifestNames() []string {
	root := filepath.Join(ollamaHome(), "manifests")
	var out []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		if name, ok := ollamaNameFromRel(rel); ok {
			out = append(out, name)
		}
		return nil
	})
	return out
}

// registry.ollama.ai/library/qwen2.5-coder/14b → qwen2.5-coder:14b
// hf.co/org/repo/Q4_K_M → hf.co/org/repo:Q4_K_M
func ollamaNameFromRel(rel string) (string, bool) {
	rel = filepath.ToSlash(rel)
	parts := strings.Split(rel, "/")
	if len(parts) < 2 {
		return "", false
	}
	tag := parts[len(parts)-1]
	if tag == "" || strings.HasPrefix(tag, ".") {
		return "", false
	}
	if len(parts) >= 4 && parts[0] == "registry.ollama.ai" && parts[1] == "library" {
		return parts[2] + ":" + tag, true
	}
	return strings.Join(parts[:len(parts)-1], "/") + ":" + tag, true
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

func uniqExisting(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range in {
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		if seen[abs] {
			continue
		}
		if st, err := os.Stat(abs); err != nil || !st.IsDir() {
			continue
		}
		seen[abs] = true
		out = append(out, abs)
	}
	return out
}

func depth(rel string) int {
	if rel == "." || rel == "" {
		return 0
	}
	n := 1
	for _, c := range rel {
		if c == os.PathSeparator {
			n++
		}
	}
	return n
}
