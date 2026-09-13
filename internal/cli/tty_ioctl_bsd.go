//go:build darwin || freebsd || openbsd || netbsd || dragonfly

package cli

import "syscall"

const (
	ttyReadIoctl  = syscall.TIOCGETA
	ttyWriteIoctl = syscall.TIOCSETA
)
