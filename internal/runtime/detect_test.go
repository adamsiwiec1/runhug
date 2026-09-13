package runtime

import (
	"bytes"
	"strings"
	"testing"
)

func TestDetectFindsSomethingOrPrintsConfig(t *testing.T) {
	s := Detect()
	if len(s.Engines) != 3 {
		t.Fatalf("engines %d", len(s.Engines))
	}
	var buf bytes.Buffer
	PrintConfig(&buf, s)
	out := buf.String()
	for _, want := range []string{"ollama", "llamacpp", "mlx", "RVP_RUNTIME", "--url"} {
		if !strings.Contains(out, want) {
			t.Fatalf("config missing %q\n%s", want, out)
		}
	}
}

func TestPreferredWantMissing(t *testing.T) {
	s := Snapshot{Engines: []Engine{
		{Kind: Ollama, Present: true, Binary: "/bin/ollama"},
	}}
	if e := s.Preferred("mlx"); e != nil {
		t.Fatalf("expected nil, got %+v", e)
	}
	if e := s.Preferred("ollama"); e == nil || e.Binary != "/bin/ollama" {
		t.Fatalf("%+v", e)
	}
}

func TestInstallPlanOllama(t *testing.T) {
	p := InstallPlan("ollama")
	if p.Kind != Ollama || len(p.Manual) == 0 {
		t.Fatalf("%+v", p)
	}
}
