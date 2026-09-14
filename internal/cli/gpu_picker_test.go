package cli

import (
	"strings"
	"testing"

	"github.com/adamsiwiec1/runhug-cli/internal/runpod"
)

func sampleGPURows() []gpuPickRow {
	return buildGPUPickRows([]runpod.Pool{
		{ID: "ADA_24", MemoryGB: 24, PricePerHour: 0.44, Availability: "HIGH", ExampleGPU: "RTX 4090", InStock: true},
		{ID: "AMPERE_48", MemoryGB: 48, PricePerHour: 0.79, Availability: "MEDIUM", ExampleGPU: "A6000", InStock: true},
		{ID: "AMPERE_80", MemoryGB: 80, PricePerHour: 1.39, Availability: "LOW", ExampleGPU: "A100", InStock: true},
	})
}

func TestBuildGPUPickRowsNotes(t *testing.T) {
	rows := sampleGPURows()
	if rows[0].Note != "recommended" {
		t.Fatalf("row0 note=%q", rows[0].Note)
	}
	if rows[1].Note != "larger / safer" {
		t.Fatalf("row1 note=%q", rows[1].Note)
	}
}

func TestParseGPUPickKey(t *testing.T) {
	cases := []struct {
		in     []byte
		action gpuPickAction
		jump   int
	}{
		{[]byte{0x1b, '[', 'A'}, gpuPickUp, 0},
		{[]byte{0x1b, '[', 'B'}, gpuPickDown, 0},
		{[]byte{0x1b}, gpuPickCancel, 0},
		{[]byte{'k'}, gpuPickUp, 0},
		{[]byte{'j'}, gpuPickDown, 0},
		{[]byte{'p'}, gpuPickUp, 0},
		{[]byte{'n'}, gpuPickDown, 0},
		{[]byte{'e'}, gpuPickToggleEstimate, 0},
		{[]byte{'\r'}, gpuPickSelect, 0},
		{[]byte{'q'}, gpuPickCancel, 0},
		{[]byte{'3'}, gpuPickJump, 3},
		{[]byte{'z'}, gpuPickNone, 0},
	}
	for _, tc := range cases {
		a, j := parseGPUPickKey(tc.in)
		if a != tc.action || j != tc.jump {
			t.Fatalf("key %q → action=%d jump=%d want %d/%d", tc.in, a, j, tc.action, tc.jump)
		}
	}
}

func TestApplyGPUPickAction(t *testing.T) {
	sel, est, done, rec := applyGPUPickAction(gpuPickDown, 0, 0, 3, false)
	if sel != 1 || est || done || rec {
		t.Fatalf("down: sel=%d est=%v done=%v rec=%v", sel, est, done, rec)
	}
	sel, est, done, rec = applyGPUPickAction(gpuPickUp, 0, 0, 3, false)
	if sel != 2 || done {
		t.Fatalf("up wrap: sel=%d", sel)
	}
	sel, est, done, rec = applyGPUPickAction(gpuPickJump, 2, 0, 3, false)
	if sel != 1 || done {
		t.Fatalf("jump: sel=%d", sel)
	}
	sel, est, done, rec = applyGPUPickAction(gpuPickToggleEstimate, 0, 1, 3, false)
	if !est || done {
		t.Fatalf("toggle on: est=%v", est)
	}
	sel, est, done, rec = applyGPUPickAction(gpuPickToggleEstimate, 0, 1, 3, true)
	if est {
		t.Fatalf("toggle off")
	}
	sel, est, done, rec = applyGPUPickAction(gpuPickSelect, 0, 2, 3, true)
	if !done || sel != 2 || rec {
		t.Fatalf("select: sel=%d done=%v rec=%v", sel, done, rec)
	}
	sel, est, done, rec = applyGPUPickAction(gpuPickCancel, 0, 2, 3, true)
	if !done || sel != 0 || !rec {
		t.Fatalf("cancel → recommended: sel=%d done=%v rec=%v", sel, done, rec)
	}
}

func TestFormatGPUPickTableStatic(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	rows := sampleGPURows()
	out := formatGPUPickTable(rows)
	for _, want := range []string{"POOL", "VRAM", "EXAMPLE", "$/HR", "STOCK", "NOTE", "ADA_24", "AMPERE_48", "recommended"} {
		if !strings.Contains(out, want) {
			t.Fatalf("table missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "›") {
		t.Fatalf("static table must not highlight selection:\n%s", out)
	}
	if strings.Contains(out, "\033[7m") {
		t.Fatalf("must not use reverse-video:\n%s", out)
	}
	if strings.Contains(out, "Cost estimate") || strings.Contains(out, "daily scenarios") || strings.Contains(out, "CompactLine") {
		t.Fatalf("table must not include estimates:\n%s", out)
	}
	// Help is printed separately under the table in TTY, not inside the table.
	if strings.Contains(out, "↑/↓") {
		t.Fatalf("help should not be in table:\n%s", out)
	}
}

func TestFormatGPUPickSelection(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	rows := sampleGPURows()
	out := formatGPUPickSelection(rows, 0)
	if strings.Contains(out, "\n") {
		t.Fatalf("selection must be a single line:\n%q", out)
	}
	if !strings.HasPrefix(out, "› ") {
		t.Fatalf("expected › marker:\n%s", out)
	}
	for _, want := range []string{"ADA_24", "24GB", "RTX 4090", "$0.44/hr", "1/3"} {
		if !strings.Contains(out, want) {
			t.Fatalf("selection missing %q:\n%s", want, out)
		}
	}
	// Must stay short enough not to wrap on 80-col terminals.
	if len(out) > 72 {
		t.Fatalf("selection too long (%d): %q", len(out), out)
	}
	if strings.Contains(out, "\033[7m") {
		t.Fatalf("must not use reverse-video")
	}
	out2 := formatGPUPickSelection(rows, 1)
	for _, want := range []string{"AMPERE_48", "48GB", "A6000", "$0.79/hr", "2/3"} {
		if !strings.Contains(out2, want) {
			t.Fatalf("row1 selection missing %q:\n%s", want, out2)
		}
	}
}

func TestFormatGPUPickEstimateCompact(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	rows := sampleGPURows()
	lines := formatGPUPickEstimate(rows[1], 18)
	if len(lines) != 2 {
		t.Fatalf("estimate lines=%d want 2\n%v", len(lines), lines)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "~/hr") && !strings.Contains(joined, "/hr") {
		t.Fatalf("expected CompactLine-style hint:\n%s", joined)
	}
	if strings.Contains(joined, "Cost estimate for") || strings.Contains(joined, "daily scenarios") || strings.Contains(joined, "assumptions") {
		t.Fatalf("must not use huge FormatBlock:\n%s", joined)
	}
	if !strings.Contains(joined, "~100 req/day") {
		t.Fatalf("expected short daily line:\n%s", joined)
	}
	for _, line := range lines {
		if strings.Contains(line, "\n") {
			t.Fatalf("estimate line itself must not contain newline: %q", line)
		}
	}
}

func TestWriteRawCRLF(t *testing.T) {
	var b strings.Builder
	writeRawCRLF(&b, "a\nb\n")
	if b.String() != "a\r\nb\r\n" {
		t.Fatalf("got %q", b.String())
	}
	b.Reset()
	writeRawCRLF(&b, "a\r\nb\n")
	if b.String() != "a\r\nb\r\n" {
		t.Fatalf("idempotent normalize got %q", b.String())
	}
}

func TestFormatGPUPickHelpMentionsEstimate(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	h := formatGPUPickHelp()
	if !strings.Contains(h, "e estimate") {
		t.Fatalf("help should mention e: %q", h)
	}
	if !strings.Contains(h, "Enter") || !strings.Contains(h, "Esc") {
		t.Fatalf("help missing Enter/Esc: %q", h)
	}
}
