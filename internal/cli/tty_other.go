//go:build !unix && !windows

package cli

import (
	"fmt"
	"os"
)

func stdinIsTTY() bool {
	return fdIsChar(os.Stdin)
}

func stdoutIsTTY() bool {
	return fdIsChar(os.Stdout)
}

func fdIsChar(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func echoOff() (func(), error) {
	return nil, fmt.Errorf("hidden input is not supported on this OS")
}
