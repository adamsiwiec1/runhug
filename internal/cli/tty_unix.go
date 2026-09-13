//go:build unix

package cli

import (
	"os"
	"syscall"
	"unsafe"
)

func stdinIsTTY() bool {
	return fdIsTTY(os.Stdin.Fd())
}

func stdoutIsTTY() bool {
	return fdIsTTY(os.Stdout.Fd())
}

func fdIsTTY(fd uintptr) bool {
	var t syscall.Termios
	_, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, fd, ttyReadIoctl, uintptr(unsafe.Pointer(&t)), 0, 0, 0)
	return errno == 0
}

func echoOff() (func(), error) {
	fd := os.Stdin.Fd()
	var old syscall.Termios
	_, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, fd, ttyReadIoctl, uintptr(unsafe.Pointer(&old)), 0, 0, 0)
	if errno != 0 {
		return nil, errno
	}
	raw := old
	raw.Lflag &^= syscall.ECHO
	_, _, errno = syscall.Syscall6(syscall.SYS_IOCTL, fd, ttyWriteIoctl, uintptr(unsafe.Pointer(&raw)), 0, 0, 0)
	if errno != 0 {
		return nil, errno
	}
	return func() {
		_, _, _ = syscall.Syscall6(syscall.SYS_IOCTL, fd, ttyWriteIoctl, uintptr(unsafe.Pointer(&old)), 0, 0, 0)
	}, nil
}
