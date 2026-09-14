package cli

import (
	"strings"
	"testing"
)

func TestBuildClaudeEnvScrubKeysAndValues(t *testing.T) {
	env := buildClaudeEnv("http://127.0.0.1:9999", "sk-runpod-abc", "org/model")
	if env["ANTHROPIC_BASE_URL"] != "http://127.0.0.1:9999" {
		t.Fatalf("%v", env)
	}
	if env["ANTHROPIC_AUTH_TOKEN"] != "sk-runpod-abc" {
		t.Fatal(env["ANTHROPIC_AUTH_TOKEN"])
	}
	if env["ANTHROPIC_MODEL"] != "org/model" || env["ANTHROPIC_DEFAULT_MODEL"] != "org/model" {
		t.Fatal(env)
	}
	if env["CLAUDE_CODE_ATTRIBUTION_HEADER"] != "0" {
		t.Fatal(env)
	}
	if env["CLAUDE_CODE_ENABLE_TELEMETRY"] != "0" {
		t.Fatal(env)
	}
	if env["CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"] != "1" {
		t.Fatal(env)
	}
	// Must not set ANTHROPIC_API_KEY
	if _, ok := env["ANTHROPIC_API_KEY"]; ok {
		t.Fatal("ANTHROPIC_API_KEY must be absent from set map")
	}
}

func TestScrubAndMergeEnvRemovesAnthropicLoginKeys(t *testing.T) {
	base := []string{
		"PATH=/bin",
		"ANTHROPIC_API_KEY=sk-ant-real",
		"CLAUDE_CODE_OAUTH_TOKEN=oauth",
		"HOME=/tmp",
	}
	set := buildClaudeEnv("http://127.0.0.1:1", "sk-runhug-local", "m")
	out := scrubAndMergeEnv(base, set, claudeEnvUnset)
	joined := strings.Join(out, "\n")
	if strings.Contains(joined, "ANTHROPIC_API_KEY=") {
		t.Fatalf("API key survived:\n%s", joined)
	}
	if strings.Contains(joined, "CLAUDE_CODE_OAUTH_TOKEN=") {
		t.Fatalf("oauth survived:\n%s", joined)
	}
	if !strings.Contains(joined, "ANTHROPIC_AUTH_TOKEN=sk-runhug-local") {
		t.Fatalf("missing auth token:\n%s", joined)
	}
	if !strings.Contains(joined, "ANTHROPIC_BASE_URL=http://127.0.0.1:1") {
		t.Fatal(joined)
	}
	if !strings.Contains(joined, "PATH=/bin") {
		t.Fatal(joined)
	}
}

func TestBuildClaudeArgsWithModernVersion(t *testing.T) {
	args := buildClaudeArgs("org/m", &claudeVersion{2, 1, 98})
	want := []string{
		"--model", "org/m",
		"--exclude-dynamic-system-prompt-sections",
		"--settings", claudeSettingsOverlay,
	}
	if strings.Join(args, " ") != strings.Join(want, " ") {
		t.Fatalf("got %v\nwant %v", args, want)
	}
}

func TestBuildClaudeArgsOldVersionSkipsFlags(t *testing.T) {
	args := buildClaudeArgs("org/m", &claudeVersion{2, 1, 97})
	if len(args) != 2 || args[0] != "--model" || args[1] != "org/m" {
		t.Fatalf("%v", args)
	}
}

func TestBuildClaudeArgsNoBinaryAssumesModern(t *testing.T) {
	args := buildClaudeArgs("m", nil)
	if !containsStr(args, "--settings") {
		t.Fatalf("expected settings overlay when version unknown: %v", args)
	}
}

func TestParseClaudeVersion(t *testing.T) {
	v := parseClaudeVersion("2.1.98 (Claude Code)")
	if v.Major != 2 || v.Minor != 1 || v.Patch != 98 {
		t.Fatalf("%+v", v)
	}
	if v.Less(2, 1, 98) {
		t.Fatal("should not be less")
	}
	old := parseClaudeVersion("claude version 2.0.1")
	if !old.Less(2, 1, 98) {
		t.Fatal(old)
	}
	bad := parseClaudeVersion("nope")
	if !bad.Less(2, 1, 98) {
		t.Fatal("unparseable should be treated as old")
	}
}

func containsStr(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
