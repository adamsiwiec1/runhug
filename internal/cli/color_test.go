package cli

import (
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
