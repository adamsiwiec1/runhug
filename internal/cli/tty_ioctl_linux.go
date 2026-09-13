//go:build linux

package cli

import "syscall"

const (
	ttyReadIoctl  = syscall.TCGETS
	ttyWriteIoctl = syscall.TCSETS
)
