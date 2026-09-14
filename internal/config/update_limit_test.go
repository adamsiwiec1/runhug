package config

import (
	"path/filepath"
	"testing"
)

func TestResolveUpdateLimitPrecedence(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfig, filepath.Join(dir, "registry.json"))
	t.Setenv(EnvUpdateLimit, "")

	if got := ResolveUpdateLimit(-1); got != DefaultUpdateLimit {
		t.Fatalf("default: got %d want %d", got, DefaultUpdateLimit)
	}

	n := 500
	if err := SaveSettings(Settings{UpdateLimit: &n}); err != nil {
		t.Fatal(err)
	}
	if got := ResolveUpdateLimit(-1); got != 500 {
		t.Fatalf("settings: got %d", got)
	}

	t.Setenv(EnvUpdateLimit, "100")
	if got := ResolveUpdateLimit(-1); got != 100 {
		t.Fatalf("env: got %d", got)
	}

	if got := ResolveUpdateLimit(42); got != 42 {
		t.Fatalf("flag: got %d", got)
	}
	if got := ResolveUpdateLimit(0); got != 0 {
		t.Fatalf("flag unlimited: got %d", got)
	}
}

func TestSettingsAdvisorFieldsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfig, filepath.Join(dir, "registry.json"))
	s := Settings{
		AdvisorBaseURL: "http://127.0.0.1:11434/v1",
		AdvisorModel:   "llama3.2",
	}
	if err := SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	got := LoadSettings()
	if got.AdvisorBaseURL != s.AdvisorBaseURL || got.AdvisorModel != s.AdvisorModel {
		t.Fatalf("got %+v", got)
	}
}
