package runtime

import (
	"fmt"
	"os"
	"os/exec"
	goruntime "runtime"
	"strings"
	"time"
)

type Engine struct {
	Kind    string
	Name    string
	Binary  string
	BaseURL string
	Port    int
	Running bool
	Present bool
}

type Snapshot struct {
	Engines []Engine
}

func Detect() Snapshot {
	var s Snapshot
	s.Engines = append(s.Engines, detectOllama(), detectLlama(), detectMLX())
	return s
}

func (s Snapshot) Find(kind string) *Engine {
	kind = Normalize(kind)
	for i := range s.Engines {
		if s.Engines[i].Kind == kind {
			e := s.Engines[i]
			return &e
		}
	}
	return nil
}

func (s Snapshot) Installed() []Engine {
	var out []Engine
	for _, e := range s.Engines {
		if e.Present {
			out = append(out, e)
		}
	}
	return out
}

// Preferred picks an already-installed engine. Running servers win, then
// RVP_RUNTIME, then Ollama, llama.cpp, MLX.
func (s Snapshot) Preferred(want string) *Engine {
	if want = Normalize(want); want != "" {
		if e := s.Find(want); e != nil && e.Present {
			return e
		}
		return nil
	}
	if env := FromEnv(); env != "" {
		if e := s.Find(env); e != nil && e.Present {
			return e
		}
	}
	order := []string{Ollama, LlamaCPP, MLX}
	var first *Engine
	for _, k := range order {
		e := s.Find(k)
		if e == nil || !e.Present {
			continue
		}
		cp := *e
		if e.Running {
			return &cp
		}
		if first == nil {
			first = &cp
		}
	}
	return first
}

func detectOllama() Engine {
	e := Engine{Kind: Ollama, Name: "Ollama", Port: OllamaPort, BaseURL: OllamaURL}
	if p := strings.TrimSpace(os.Getenv("OLLAMA_BIN")); p != "" {
		e.Binary, e.Present = p, true
	} else if p, err := exec.LookPath("ollama"); err == nil {
		e.Binary, e.Present = p, true
	}
	e.Running = PortOpen(OllamaPort)
	return e
}

func detectLlama() Engine {
	e := Engine{Kind: LlamaCPP, Name: "llama.cpp", Port: LlamaPort, BaseURL: fmt.Sprintf("http://127.0.0.1:%d/v1", LlamaPort)}
	if p := strings.TrimSpace(os.Getenv("LLAMA_SERVER")); p != "" {
		e.Binary, e.Present = p, true
	} else {
		for _, name := range []string{"llama-server", "llama-cpp-server"} {
			if p, err := exec.LookPath(name); err == nil {
				e.Binary, e.Present = p, true
				break
			}
		}
	}
	e.Running = PortOpen(LlamaPort)
	return e
}

func detectMLX() Engine {
	e := Engine{Kind: MLX, Name: "MLX", Port: MLXPort, BaseURL: MLXURL}
	if goruntime.GOOS == "darwin" && goruntime.GOARCH != "arm64" {
		return e
	}
	if p := strings.TrimSpace(os.Getenv("MLX_SERVER")); p != "" {
		e.Binary, e.Present = p, true
	} else if p, err := exec.LookPath("mlx_lm.server"); err == nil {
		e.Binary, e.Present = p, true
	} else if pythonHasMLX() {
		if p, err := exec.LookPath("python3"); err == nil {
			e.Binary = p + " -m mlx_lm.server"
			e.Present = true
		}
	}
	e.Running = PortOpen(MLXPort)
	return e
}

func pythonHasMLX() bool {
	if goruntime.GOOS == "darwin" && goruntime.GOARCH != "arm64" {
		return false
	}
	cmd := exec.Command("python3", "-c", "import mlx_lm")
	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()
	select {
	case err := <-done:
		return err == nil
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
		return false
	}
}

func AppleSilicon() bool {
	return goruntime.GOOS == "darwin" && goruntime.GOARCH == "arm64"
}

func RecommendedKind() string {
	return Ollama
}
