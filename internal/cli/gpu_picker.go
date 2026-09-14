package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

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

func formatGPUPickHelp() string {
	return dim("↑/↓ j/k n/p  cycle · 1–9 jump · e estimate (highlighted) · Enter select · Esc/q recommended")
}

const (
	gpuColNum     = 2
	gpuColPool    = 14
	gpuColVRAM    = 6
	gpuColExample = 16
	gpuColPrice   = 7
	gpuColStock   = 8
	gpuColNote    = 14
)

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

func formatGPUPickRow(i int, row gpuPickRow, selected int) string {
	p := row.Pool
	marker := " "
	if selected >= 0 && i == selected {
		marker = "›"
	}
	num := strconv.Itoa(i + 1)
	line := fmt.Sprintf("%s %s  %s  %s  %s  %s  %s  %s",
		marker,
		padRight(num, gpuColNum),
		padRight(truncateRunes(p.ID, gpuColPool), gpuColPool),
		padRight(fmt.Sprintf("%.0f GB", p.MemoryGB), gpuColVRAM),
		padRight(truncateRunes(dash(p.ExampleGPU), gpuColExample), gpuColExample),
		padRight(fmt.Sprintf("$%.2f", p.PricePerHour), gpuColPrice),
		padRight(truncateRunes(poolStockLabel(p), gpuColStock), gpuColStock),
		padRight(truncateRunes(row.Note, gpuColNote), gpuColNote),
	)
	if selected >= 0 && i == selected {
		if useColor() {
			return paint("\033[36m\033[7m", line)
		}
		return line
	}
	if i == 0 && row.Note == "recommended" {
		// Soft emphasis on the recommended pool id when not highlighted.
		parts := strings.SplitN(line, p.ID, 2)
		if len(parts) == 2 && useColor() {
			return parts[0] + green(p.ID) + parts[1]
		}
	}
	return line
}

// formatGPUPickTable renders the pool table. selected < 0 disables the › highlight.
// When showEstimate is true, appends the full cost block for the highlighted row only.
func formatGPUPickTable(rows []gpuPickRow, selected int, showEstimate bool, weightGB float64) string {
	if len(rows) == 0 {
		return ""
	}
	if selected >= len(rows) {
		selected = len(rows) - 1
	}

	var b strings.Builder
	b.WriteString(formatGPUPickHeader())
	b.WriteByte('\n')
	for i, row := range rows {
		b.WriteString(formatGPUPickRow(i, row, selected))
		b.WriteByte('\n')
	}
	b.WriteString(formatGPUPickHelp())
	b.WriteByte('\n')

	if showEstimate && selected >= 0 && selected < len(rows) {
		p := rows[selected].Pool
		cost := sizing.EstimateServerlessCost(p.PricePerHour, weightGB, 1, 5, true)
		b.WriteByte('\n')
		b.WriteString(bold(cost.FormatBlock(p.ID)))
		b.WriteByte('\n')
		b.WriteString(dim("(e again to hide estimate)"))
		b.WriteByte('\n')
	}
	return b.String()
}

func countPrintedLines(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}

func redrawGPUPick(w io.Writer, prevLines int, body string) int {
	if prevLines > 0 {
		fmt.Fprintf(w, "\033[%dA\033[J", prevLines)
	}
	fmt.Fprint(w, body)
	return countPrintedLines(body)
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
	fmt.Fprint(w, formatGPUPickTable(rows, -1, false, 0))
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

	selected := 0
	showEst := false
	prev := 0

	body := formatGPUPickTable(rows, selected, showEst, weightGB)
	prev = redrawGPUPick(w, 0, body)

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
		body = formatGPUPickTable(rows, selected, showEst, weightGB)
		prev = redrawGPUPick(w, prev, body)
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

// visibleWidth approximates printed width ignoring simple ANSI CSI m sequences.
func visibleWidth(s string) int {
	plain := s
	for {
		i := strings.IndexByte(plain, 0x1b)
		if i < 0 {
			break
		}
		j := i + 1
		if j < len(plain) && plain[j] == '[' {
			for j < len(plain) && plain[j] != 'm' {
				j++
			}
			if j < len(plain) {
				j++
			}
			plain = plain[:i] + plain[j:]
			continue
		}
		break
	}
	return utf8.RuneCountInString(plain)
}
