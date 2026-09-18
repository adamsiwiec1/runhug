package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCodeModelKey(t *testing.T) {
	cases := map[string]string{
		"Qwen/Qwen2.5-0.5B-Instruct":              "qwen-qwen2-5-0-5b-instruct",
		"empero-ai/Qwythos-9B-Claude-Mythos-5-1M": "empero-ai-qwythos-9b-claude-mythos-5-1m",
		"default":        "default",
		"":               "default",
		"  gemma4:e4b  ": "gemma4-e4b",
	}
	for in, want := range cases {
		if got := openCodeModelKey(in); got != want {
			t.Errorf("openCodeModelKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOpenCodeModelKeyTruncates(t *testing.T) {
	in := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if got := openCodeModelKey(in); len(got) > 40 {
		t.Errorf("slug too long: %q (%d)", got, len(got))
	}
}

func TestOpenCodeConfigShape(t *testing.T) {
	spec := openCodeSpec{BaseURL: "http://127.0.0.1:8080/v1", APIKey: "sk-x", ModelID: "empero-ai/Qwythos-9B-Claude-Mythos-5-1M", Source: "local proxy 127.0.0.1:8080"}
	cfg := openCodeConfig(spec, openCodeModelKey(spec.ModelID))

	if cfg["model"] != "runhug/empero-ai-qwythos-9b-claude-mythos-5-1m" {
		t.Errorf("model = %v", cfg["model"])
	}
	p := cfg["provider"].(map[string]any)["runhug"].(map[string]any)
	if p["npm"] != "@ai-sdk/openai-compatible" {
		t.Errorf("npm = %v", p["npm"])
	}
	opts := p["options"].(map[string]any)
	if opts["baseURL"] != "http://127.0.0.1:8080/v1" {
		t.Errorf("baseURL = %v", opts["baseURL"])
	}
	models := p["models"].(map[string]any)
	slug := openCodeModelKey(spec.ModelID)
	m := models[slug].(map[string]any)
	if m["id"] != spec.ModelID {
		t.Errorf("sent model id = %v, want %q", m["id"], spec.ModelID)
	}
	if m["tool_call"] != true {
		t.Errorf("tool_call should be enabled for opencode tool use")
	}
}

func TestWriteOpenCodeConfigPreservesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "opencode.json")

	existing := `{"$schema":"https://opencode.ai/config.json","model":"anthropic/claude-sonnet-4-6","provider":{"anthropic":{"options":{"apiKey":"real"}}},"autoupdate":true}`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	spec := openCodeSpec{BaseURL: "http://127.0.0.1:8080/v1", APIKey: "sk-x", ModelID: "qwen3:8b", Source: "local proxy"}
	cfg := openCodeConfig(spec, openCodeModelKey(spec.ModelID))
	if err := writeOpenCodeConfig(path, cfg); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("written config invalid JSON: %v\n%s", err, raw)
	}
	if got["autoupdate"] != true {
		t.Errorf("existing autoupdate lost: %v", got["autoupdate"])
	}
	if got["model"] != "runhug/qwen3-8b" {
		t.Errorf("model = %v", got["model"])
	}
	prov := got["provider"].(map[string]any)
	if _, ok := prov["anthropic"]; !ok {
		t.Errorf("existing anthropic provider lost")
	}
	runhug := prov["runhug"].(map[string]any)
	if runhug["npm"] != "@ai-sdk/openai-compatible" {
		t.Errorf("runhug npm = %v", runhug["npm"])
	}
}

func TestWriteOpenCodeConfigNewFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.json")
	spec := openCodeSpec{BaseURL: "http://127.0.0.1:8080/v1", APIKey: "sk-x", ModelID: "default", Source: "local proxy"}
	cfg := openCodeConfig(spec, openCodeModelKey(spec.ModelID))
	if err := writeOpenCodeConfig(path, cfg); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(raw) {
		t.Fatalf("invalid JSON written:\n%s", raw)
	}
}
