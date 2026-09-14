package packs

import (
	"os"
	"testing"
)

func TestResolveLimitUnlimitedByDefault(t *testing.T) {
	t.Setenv(EnvIndexLimit, "")
	_ = os.Unsetenv(EnvIndexLimit)
	if got := ResolveLimit(0); got != 0 {
		t.Fatalf("ResolveLimit(0)=%d want 0 (unlimited)", got)
	}
	if got := ResolveLimit(-1); got != 0 {
		t.Fatalf("ResolveLimit(-1)=%d want 0", got)
	}
	if got := ResolveLimit(100); got != 100 {
		t.Fatalf("ResolveLimit(100)=%d", got)
	}
}

func TestResolveLimitEnvCap(t *testing.T) {
	t.Setenv(EnvIndexLimit, "2500")
	if got := ResolveLimit(0); got != 2500 {
		t.Fatalf("env cap: got %d", got)
	}
	if got := ResolveLimit(10); got != 10 {
		t.Fatalf("explicit limit wins: got %d", got)
	}
}

func TestDefaultLimitNoHardcoded5000(t *testing.T) {
	t.Setenv(EnvIndexLimit, "")
	_ = os.Unsetenv(EnvIndexLimit)
	if got := DefaultLimit(); got != 0 {
		t.Fatalf("DefaultLimit()=%d want 0 (no 5000 default)", got)
	}
}
