package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
)

func useColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return stdoutIsTTY()
}

func paint(code, s string) string {
	if s == "" || !useColor() {
		return s
	}
	return code + s + ansiReset
}

func bold(s string) string   { return paint(ansiBold, s) }
func dim(s string) string    { return paint(ansiDim, s) }
func red(s string) string    { return paint(ansiRed, s) }
func green(s string) string  { return paint(ansiGreen, s) }
func yellow(s string) string { return paint(ansiYellow, s) }
func cyan(s string) string   { return paint(ansiCyan, s) }

// FormatError colors an error for stderr. Never used for secrets.
func FormatError(msg string) string {
	return red(msg)
}

func heading(w io.Writer, title string) {
	fmt.Fprintln(w, bold(title))
	fmt.Fprintln(w)
}

func printKV(w io.Writer, label, value string) {
	fmt.Fprintf(w, "  %s  %s\n", dim(padRight(label, 9)), value)
}

func commands(w io.Writer, title string, lines ...string) {
	if title != "" {
		fmt.Fprintln(w, title)
	}
	for _, line := range lines {
		if line == "" {
			fmt.Fprintln(w)
			continue
		}
		fmt.Fprintln(w, "  "+cyan(line))
	}
	fmt.Fprintln(w)
}

func hubLink(id string) string {
	return "https://huggingface.co/" + id
}

// osc8 wraps text in an OSC 8 hyperlink when stdout is a color TTY.
func osc8(url, text string) string {
	if url == "" || text == "" || !useColor() {
		return text
	}
	esc := string(rune(0x1b))
	return esc + "]8;;" + url + esc + "\\" + text + esc + "]8;;" + esc + "\\"
}

func actionsCell(id string) string {
	url := hubLink(id)
	// 🔗 opens Hub via OSC-8 hyperlink. 📋 is plain text (copy via --copy N or interactive 'copy N').
	return osc8(url, "🔗") + " 📋"
}

func padRight(s string, n int) string {
	if n <= 0 {
		return s
	}
	w := utf8.RuneCountInString(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}

func truncateRunes(s string, n int) string {
	if n <= 0 {
		return s
	}
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}

// displayModel ellipsizes repo ids for aligned Hub tables unless --word-wrap/-ww.
func displayModel(s string, n int, wrap bool) string {
	if wrap {
		return s
	}
	return truncateRunes(s, n)
}
