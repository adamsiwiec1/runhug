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

func TestFormatGPUPickTableColumns(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	rows := sampleGPURows()
	out := formatGPUPickTable(rows, 0, false, 18)
	for _, want := range []string{"POOL", "VRAM", "EXAMPLE", "$/HR", "STOCK", "NOTE", "ADA_24", "AMPERE_48", "›", "recommended", "↑/↓"} {
		if !strings.Contains(out, want) {
			t.Fatalf("table missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "Cost estimate") || strings.Contains(out, "daily scenarios") {
		t.Fatalf("estimates must be opt-in; got cost dump:\n%s", out)
	}
	// Highlighted row only when estimate toggled.
	with := formatGPUPickTable(rows, 1, true, 18)
	if !strings.Contains(with, "Cost estimate for AMPERE_48") {
		t.Fatalf("expected estimate for highlighted AMPERE_48:\n%s", with)
	}
	if strings.Contains(with, "Cost estimate for ADA_24") {
		t.Fatalf("must not dump estimates for non-highlighted rows:\n%s", with)
	}
	plain := formatGPUPickTable(rows, -1, false, 0)
	if strings.Contains(plain, "›") {
		t.Fatalf("plain table should not highlight:\n%s", plain)
	}
}

func TestFormatGPUPickTableNoEstimateByDefaultForAll(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	rows := sampleGPURows()
	out := formatGPUPickTable(rows, 0, false, 18)
	if strings.Count(out, "est. $/cold") != 0 {
		t.Fatalf("unexpected cold cost lines:\n%s", out)
	}
}
