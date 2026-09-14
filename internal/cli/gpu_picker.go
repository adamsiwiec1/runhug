package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/adamsiwiec1/runhug-cli/internal/runpod"
	"github.com/adamsiwiec1/runhug-cli/internal/sizing"
	"golang.org/x/term"
)

// gpuPickAction is a parsed keypress for the GPU pool picker.
type gpuPickAction int

const (
	gpuPickNone gpuPickAction = iota
	gpuPickUp
	gpuPickDown
	gpuPickJump
	gpuPickToggleEstimate
	gpuPickSelect
	gpuPickCancel // Esc / q → keep recommended (#1)
)

// gpuPickRow is one selectable pool in the wizard GPU step.
type gpuPickRow struct {
	Pool runpod.Pool
	Note string
}

func buildGPUPickRows(opts []runpod.Pool) []gpuPickRow {
	rows := make([]gpuPickRow, 0, len(opts))
	for i, p := range opts {
		note := ""
		if i == 0 {
			note = "recommended"
		} else if p.MemoryGB > opts[0].MemoryGB+0.01 {
			note = "larger / safer"
		}
		rows = append(rows, gpuPickRow{Pool: p, Note: note})
	}
	return rows
}

func poolStockLabel(p runpod.Pool) string {
	if p.Availability != "" {
		return p.Availability
	}
	if p.InStock {
		return "in stock"
	}
	return "none"
}

// parseGPUPickKey interprets raw stdin bytes from a TTY picker.
// jump is 1-based when action is gpuPickJump.
func parseGPUPickKey(b []byte) (action gpuPickAction, jump int) {
	if len(b) == 0 {
		return gpuPickNone, 0
	}
	if b[0] == 0x1b {
		if len(b) >= 3 && b[1] == '[' {
			switch b[2] {
			case 'A':
				return gpuPickUp, 0
			case 'B':
				return gpuPickDown, 0
			}
		}
		return gpuPickCancel, 0
	}
	switch b[0] {
	case '\r', '\n':
		return gpuPickSelect, 0
	case 3: // Ctrl-C
		return gpuPickCancel, 0
	case 'q', 'Q':
		return gpuPickCancel, 0
	case 'e', 'E':
		return gpuPickToggleEstimate, 0
	case 'j', 'J', 'n', 'N':
		return gpuPickDown, 0
	case 'k', 'K', 'p', 'P':
		return gpuPickUp, 0
	}
	if b[0] >= '1' && b[0] <= '9' {
		return gpuPickJump, int(b[0] - '0')
	}
	return gpuPickNone, 0
}

// applyGPUPickAction updates selection / estimate visibility.
// Returns (selectedIndex, showEstimate, done, usedRecommended).
func applyGPUPickAction(action gpuPickAction, jump, selected, n int, showEst bool) (sel int, est bool, done, usedRecommended bool) {
	sel, est = selected, showEst
	if n <= 0 {
		return 0, false, true, true
	}
	switch action {
	case gpuPickUp:
		sel = (sel - 1 + n) % n
	case gpuPickDown:
		sel = (sel + 1) % n
	case gpuPickJump:
		if jump >= 1 && jump <= n {
			sel = jump - 1
		}
	case gpuPickToggleEstimate:
		est = !est
	case gpuPickSelect:
		done = true
	case gpuPickCancel:
		sel = 0
		done = true
		usedRecommended = true
	}
	return sel, est, done, usedRecommended
}

// Short columns so the static table fits ~80 cols without wrapping.
const (
	gpuColNum     = 2
	gpuColPool    = 12
	gpuColVRAM    = 5
	gpuColExample = 12
	gpuColPrice   = 6
	gpuColStock   = 6
	gpuColNote    = 12

	// Fixed status frame (always redrawn). Estimate adds a stable block below.
	gpuStatusLines   = 3
	gpuEstimateLines = 3
)

func formatGPUPickHelp() string {
	return dim("↑/↓ or j/k cycle · e estimate · Enter select · Esc/q recommended")
}

func formatGPUPickHeader() string {
	return dim(fmt.Sprintf("  %s  %s  %s  %s  %s  %s  %s",
		padRight("#", gpuColNum),
		padRight("POOL", gpuColPool),
		padRight("VRAM", gpuColVRAM),
		padRight("EXAMPLE", gpuColExample),
		padRight("$/HR", gpuColPrice),
		padRight("STOCK", gpuColStock),
		padRight("NOTE", gpuColNote),
	))
}

// formatGPUPickRow renders one static table row (no selection highlight).
func formatGPUPickRow(i int, row gpuPickRow) string {
	p := row.Pool
	num := strconv.Itoa(i + 1)
	line := fmt.Sprintf("  %s  %s  %s  %s  %s  %s  %s",
		padRight(num, gpuColNum),
		padRight(truncateRunes(p.ID, gpuColPool), gpuColPool),
		padRight(fmt.Sprintf("%.0fGB", p.MemoryGB), gpuColVRAM),
		padRight(truncateRunes(dash(p.ExampleGPU), gpuColExample), gpuColExample),
		padRight(fmt.Sprintf("$%.2f", p.PricePerHour), gpuColPrice),
		padRight(truncateRunes(poolStockLabel(p), gpuColStock), gpuColStock),
		padRight(truncateRunes(row.Note, gpuColNote), gpuColNote),
	)
	if i == 0 && row.Note == "recommended" && useColor() {
		parts := strings.SplitN(line, p.ID, 2)
		if len(parts) == 2 {
			return parts[0] + green(p.ID) + parts[1]
		}
	}
	return line
}

// formatGPUPickTable renders the static pool table (no per-row highlight, no estimate).
func formatGPUPickTable(rows []gpuPickRow) string {
	if len(rows) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(formatGPUPickHeader())
	b.WriteByte('\n')
	for i, row := range rows {
		b.WriteString(formatGPUPickRow(i, row))
		b.WriteByte('\n')
	}
	return b.String()
}

// formatGPUPickStatus is the fixed 3-line frame under the table (selection lives here).
func formatGPUPickStatus(rows []gpuPickRow, selected int) string {
	if len(rows) == 0 {
		return strings.Repeat("\n", gpuStatusLines-1) + "\n"
	}
	if selected < 0 {
		selected = 0
	}
	if selected >= len(rows) {
		selected = len(rows) - 1
	}
	row := rows[selected]
	p := row.Pool
	marker := "›"
	detail := fmt.Sprintf("%s %s  %.0f GB  %s  $%.2f/hr  %s",
		marker,
		p.ID,
		p.MemoryGB,
		dash(p.ExampleGPU),
		p.PricePerHour,
		poolStockLabel(p),
	)
	if useColor() {
		detail = cyan(detail)
	}
	note := row.Note
	if note == "" {
		note = "—"
	}
	pos := fmt.Sprintf("  %d of %d · %s", selected+1, len(rows), note)
	help := "  " + formatGPUPickHelp()
	return detail + "\n" + dim(pos) + "\n" + help + "\n"
}

// formatGPUPickEstimate is a compact estimate block (exactly gpuEstimateLines lines).
func formatGPUPickEstimate(row gpuPickRow, weightGB float64) string {
	cost := sizing.EstimateServerlessCost(row.Pool.PricePerHour, weightGB, 1, 5, true)
	line1 := "  " + cost.CompactLine()
	warm := cost.DailyScenarioUSD(100, 0)
	mixed := cost.DailyScenarioUSD(100, 0.10)
	cold := cost.DailyScenarioUSD(100, 1)
	line2 := fmt.Sprintf("  ~100 req/day  warm ≈ $%.4f · 10%% cold ≈ $%.4f · all-cold ≈ $%.4f", warm, mixed, cold)
	line3 := "  " + dim("(e again to hide)")
	// Pad/truncate to exactly gpuEstimateLines for stable clear.
	lines := []string{line1, line2, line3}
	for len(lines) < gpuEstimateLines {
		lines = append(lines, "")
	}
	if len(lines) > gpuEstimateLines {
		lines = lines[:gpuEstimateLines]
	}
	return strings.Join(lines, "\n") + "\n"
}

func gpuFrameLines(showEstimate bool) int {
	n := gpuStatusLines
	if showEstimate {
		n += gpuEstimateLines
	}
	return n
}

// redrawGPUPickFrame moves up by prevLines, clears, prints status (+ optional estimate).
func redrawGPUPickFrame(w io.Writer, prevLines int, rows []gpuPickRow, selected int, showEst bool, weightGB float64) int {
	if prevLines > 0 {
		fmt.Fprintf(w, "\033[%dA\033[J", prevLines)
	}
	fmt.Fprint(w, formatGPUPickStatus(rows, selected))
	if showEst && selected >= 0 && selected < len(rows) {
		fmt.Fprint(w, formatGPUPickEstimate(rows[selected], weightGB))
	}
	return gpuFrameLines(showEst)
}

// pickGPUPool runs the interactive TTY picker when possible; otherwise a plain numbered list.
// Esc/q keeps recommended (#1). Estimates are opt-in via `e` (TTY only).
func pickGPUPool(w io.Writer, opts []runpod.Pool, requiredGB, weightGB float64) (string, error) {
	if len(opts) == 0 {
		return "", fmt.Errorf("no GPU pools to pick")
	}
	rows := buildGPUPickRows(opts)

	headroom := opts[0].MemoryGB - requiredGB
	if headroom >= 0 && headroom < 2 {
		fmt.Fprintln(w, yellow("⚠")+"  "+dim(fmt.Sprintf(
			"%s fits (~%.1f GB need on %.0f GB) but is tight — a larger pool is optional and safer.",
			opts[0].ID, requiredGB, opts[0].MemoryGB)))
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, bold("GPU pools")+"  "+dim("recommended = cheapest in-stock fit; higher # = more VRAM headroom"))
	fmt.Fprintln(w)

	if canPrompt() && stdinIsTTY() && stdoutIsTTY() {
		return pickGPUPoolTTY(w, rows, weightGB)
	}
	return pickGPUPoolPlain(w, rows)
}

func pickGPUPoolPlain(w io.Writer, rows []gpuPickRow) (string, error) {
	fmt.Fprint(w, formatGPUPickTable(rows))
	fmt.Fprintln(w)
	hint := fmt.Sprintf("Pick GPU 1-%d (Enter = recommended)", len(rows))
	line, err := readLine(hint + ": ")
	if err != nil {
		return "", err
	}
	pick := 1
	if strings.TrimSpace(line) != "" {
		n, _, _ := parseChoice(line, len(rows))
		if n <= 0 {
			want := strings.TrimSpace(line)
			found := -1
			for i, r := range rows {
				if strings.EqualFold(r.Pool.ID, want) {
					found = i + 1
					break
				}
			}
			if found < 0 {
				return "", fmt.Errorf("expected a number 1-%d or a pool id", len(rows))
			}
			pick = found
		} else {
			pick = n
		}
	}
	chosen := rows[pick-1].Pool.ID
	fmt.Fprintln(w, green("✓")+"  "+dim("Using GPU pool "+chosen))
	fmt.Fprintln(w)
	return chosen, nil
}

func pickGPUPoolTTY(w io.Writer, rows []gpuPickRow, weightGB float64) (string, error) {
	fd := int(os.Stdin.Fd())
	old, err := term.MakeRaw(fd)
	if err != nil {
		return pickGPUPoolPlain(w, rows)
	}
	defer func() { _ = term.Restore(fd, old) }()

	// Print the pool table once — never redrawn while cycling.
	fmt.Fprint(w, formatGPUPickTable(rows))
	fmt.Fprintln(w)

	selected := 0
	showEst := false
	prev := redrawGPUPickFrame(w, 0, rows, selected, showEst, weightGB)

	for {
		key, err := readTTYKey(os.Stdin)
		if err != nil {
			fmt.Fprintln(w)
			return "", err
		}
		action, jump := parseGPUPickKey(key)
		if action == gpuPickNone {
			continue
		}
		var done, usedRec bool
		selected, showEst, done, usedRec = applyGPUPickAction(action, jump, selected, len(rows), showEst)
		if done {
			_ = term.Restore(fd, old)
			if prev > 0 {
				fmt.Fprintf(w, "\033[%dA\033[J", prev)
			}
			chosen := rows[selected].Pool.ID
			if usedRec {
				fmt.Fprintln(w, green("✓")+"  "+dim("Using recommended GPU pool "+chosen))
			} else {
				fmt.Fprintln(w, green("✓")+"  "+dim("Using GPU pool "+chosen))
			}
			fmt.Fprintln(w)
			return chosen, nil
		}
		prev = redrawGPUPickFrame(w, prev, rows, selected, showEst, weightGB)
	}
}

// readTTYKey reads one logical keypress (including ESC / CSI arrows) from a raw TTY.
func readTTYKey(f *os.File) ([]byte, error) {
	var b [8]byte
	n, err := f.Read(b[:1])
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, io.EOF
	}
	if b[0] != 0x1b {
		return b[:1], nil
	}
	_ = f.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	n2, err2 := f.Read(b[1:3])
	_ = f.SetReadDeadline(time.Time{})
	if err2 != nil && !os.IsTimeout(err2) {
		return b[:1], nil
	}
	if n2 <= 0 {
		return b[:1], nil
	}
	return b[:1+n2], nil
}
