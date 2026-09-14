package cli

import (
	"strings"
	"testing"
)

func TestPaintRespectsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if bold("hi") != "hi" || green("ok") != "ok" || cyan("url") != "url" {
		t.Fatalf("NO_COLOR should strip codes: %q %q %q", bold("hi"), green("ok"), cyan("url"))
	}
}

func TestPaintPlainWhenNotTTY(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	if stdoutIsTTY() {
		t.Skip("stdout is a TTY")
	}
	if red("err") != "err" {
		t.Fatalf("non-TTY should be plain: %q", red("err"))
	}
}

func TestPadAndTruncate(t *testing.T) {
	if padRight("ab", 5) != "ab   " {
		t.Fatalf("pad %q", padRight("ab", 5))
	}
	if truncateRunes("abcdef", 4) != "abc…" {
		t.Fatalf("trunc %q", truncateRunes("abcdef", 4))
	}
}

func TestClampAndResolveWrapWidth(t *testing.T) {
	if clampWrapWidth(0) != 0 || clampWrapWidth(-1) != 0 {
		t.Fatal("0/neg should stay truncate")
	}
	if clampWrapWidth(5) != 12 {
		t.Fatalf("clamp low: %d", clampWrapWidth(5))
	}
	if clampWrapWidth(100) != 80 {
		t.Fatalf("clamp high: %d", clampWrapWidth(100))
	}
	if clampWrapWidth(28) != 28 {
		t.Fatalf("28: %d", clampWrapWidth(28))
	}
	if resolveWrapWidth(0, 0, 28) != 28 {
		t.Fatal("resolve picks nonzero")
	}
	if resolveWrapWidth(50, 28, 0) != 50 {
		t.Fatal("resolve picks max")
	}
}

func TestWrapModelLines(t *testing.T) {
	long := "org/very-long-name-that-continues-here"
	lines := wrapModelLines(long, 20)
	if len(lines) < 2 {
		t.Fatalf("expected multi-line, got %#v", lines)
	}
	for _, line := range lines {
		if len([]rune(line)) > 20 {
			t.Fatalf("line wider than 20: %q", line)
		}
	}
	if strings.Join(lines, "") != long {
		t.Fatalf("joined %#v != %q", lines, long)
	}
	if got := wrapModelLines("short", 20); len(got) != 1 || got[0] != "short" {
		t.Fatalf("short: %#v", got)
	}
}

func TestActionsCell(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got := actionsCell("org/model")
	if !strings.Contains(got, "🔗") || !strings.Contains(got, "📋") {
		t.Fatalf("actionsCell: %q", got)
	}
	// Without color/TTY, OSC-8 should not wrap.
	if strings.Contains(got, "]8;;") {
		t.Fatalf("NO_COLOR should not OSC-8: %q", got)
	}
}
