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
	// Help lives in the status frame, not the table.
	if strings.Contains(out, "↑/↓") {
		t.Fatalf("help should be in status frame, not table:\n%s", out)
	}
}

func TestFormatGPUPickStatus(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	rows := sampleGPURows()
	out := formatGPUPickStatus(rows, 0)
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != gpuStatusLines {
		t.Fatalf("status lines=%d want %d\n%s", len(lines), gpuStatusLines, out)
	}
	if !strings.Contains(out, "›") || !strings.Contains(out, "ADA_24") {
		t.Fatalf("status missing selection marker:\n%s", out)
	}
	if !strings.Contains(out, "1 of 3") || !strings.Contains(out, "recommended") {
		t.Fatalf("status missing position/note:\n%s", out)
	}
	if !strings.Contains(out, "↑/↓") || !strings.Contains(out, "Enter") {
		t.Fatalf("status missing help:\n%s", out)
	}
	out2 := formatGPUPickStatus(rows, 1)
	if !strings.Contains(out2, "AMPERE_48") || !strings.Contains(out2, "2 of 3") {
		t.Fatalf("status for row1:\n%s", out2)
	}
	if strings.Contains(out2, "\033[7m") {
		t.Fatalf("status must not use reverse-video")
	}
}

func TestFormatGPUPickEstimateCompact(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	rows := sampleGPURows()
	out := formatGPUPickEstimate(rows[1], 18)
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != gpuEstimateLines {
		t.Fatalf("estimate lines=%d want %d\n%s", len(lines), gpuEstimateLines, out)
	}
	if !strings.Contains(out, "~/hr") && !strings.Contains(out, "/hr") {
		t.Fatalf("expected CompactLine-style hint:\n%s", out)
	}
	if strings.Contains(out, "Cost estimate for") || strings.Contains(out, "daily scenarios") || strings.Contains(out, "assumptions") {
		t.Fatalf("must not use huge FormatBlock:\n%s", out)
	}
	if !strings.Contains(out, "~100 req/day") {
		t.Fatalf("expected short daily line:\n%s", out)
	}
}

func TestGPUFrameLinesStable(t *testing.T) {
	if gpuFrameLines(false) != gpuStatusLines {
		t.Fatalf("hidden est frame=%d", gpuFrameLines(false))
	}
	if gpuFrameLines(true) != gpuStatusLines+gpuEstimateLines {
		t.Fatalf("shown est frame=%d", gpuFrameLines(true))
	}
}
