//go:build windows

package runtime

import "os/exec"

func detach(cmd *exec.Cmd) {}
