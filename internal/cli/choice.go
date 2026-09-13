package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func readLine(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	return scanLine()
}

func scanLine() (string, error) {
	sc := bufio.NewScanner(os.Stdin)
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return "", err
		}
		return "", io.EOF
	}
	return strings.TrimSpace(sc.Text()), nil
}

// readSecret reads a line. On a TTY it hides echo when the OS allows it.
// The value is never printed. If echo cannot be disabled, a warning is shown.
func readSecret(prompt string) (string, error) {
	restore, err := echoOff()
	hidden := err == nil
	if hidden {
		defer restore()
	} else {
		fmt.Fprintln(os.Stderr, yellow("input is visible on this terminal"))
	}
	fmt.Fprint(os.Stderr, prompt)
	line, err := scanLine()
	if hidden {
		fmt.Fprintln(os.Stderr)
	}
	return line, err
}

func confirmPref(prompt string, defYes bool) bool {
	ok, err := confirmPrefErr(prompt, defYes)
	return err == nil && ok
}

func confirmPrefErr(prompt string, defYes bool) (bool, error) {
	hint := "[y/N]"
	if defYes {
		hint = "[Y/n]"
	}
	s, err := readLine(prompt + " " + hint + " ")
	if err != nil {
		return false, err
	}
	switch strings.ToLower(s) {
	case "":
		return defYes, nil
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

// parseChoice reads a Hub pick, org/name, Ollama tag, or another search query.
func parseChoice(s string, max int) (pick int, query, repo string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, "", ""
	}
	if n, err := strconv.Atoi(s); err == nil {
		if n >= 1 && (max <= 0 || n <= max) {
			return n, "", ""
		}
		return 0, "", ""
	}
	if strings.Contains(s, "/") || strings.Contains(s, ":") {
		return 0, "", s
	}
	return 0, s, ""
}
