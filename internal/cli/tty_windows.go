//go:build windows

package cli

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const enableEchoInput = 0x0004

var (
	kernel32       = syscall.NewLazyDLL("kernel32.dll")
	getConsoleMode = kernel32.NewProc("GetConsoleMode")
	setConsoleMode = kernel32.NewProc("SetConsoleMode")
)

func stdinIsTTY() bool {
	return fdIsTTY(os.Stdin.Fd())
}

func stdoutIsTTY() bool {
	return fdIsTTY(os.Stdout.Fd())
}

func fdIsTTY(fd uintptr) bool {
	var mode uint32
	r, _, _ := getConsoleMode.Call(fd, uintptr(unsafe.Pointer(&mode)))
	return r != 0
}

func echoOff() (func(), error) {
	h := os.Stdin.Fd()
	var mode uint32
	r, _, err := getConsoleMode.Call(h, uintptr(unsafe.Pointer(&mode)))
	if r == 0 {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("not a console")
	}
	r, _, err = setConsoleMode.Call(h, uintptr(mode&^enableEchoInput))
	if r == 0 {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("could not hide input")
	}
	return func() {
		_, _, _ = setConsoleMode.Call(h, uintptr(mode))
	}, nil
}
