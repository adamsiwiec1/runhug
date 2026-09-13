//go:build !darwin

package cli

import (
	"fmt"
	"os/exec"
	"runtime"
)

func copyToClipboard(text string) error {
	switch runtime.GOOS {
	case "linux":
		if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd := exec.Command("wl-copy")
			in, err := cmd.StdinPipe()
			if err != nil {
				return err
			}
			if err := cmd.Start(); err != nil {
				return err
			}
			_, _ = in.Write([]byte(text))
			_ = in.Close()
			return cmd.Wait()
		}
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd := exec.Command("xclip", "-selection", "clipboard")
			in, err := cmd.StdinPipe()
			if err != nil {
				return err
			}
			if err := cmd.Start(); err != nil {
				return err
			}
			_, _ = in.Write([]byte(text))
			_ = in.Close()
			return cmd.Wait()
		}
	}
	return fmt.Errorf("clipboard copy not supported on %s", runtime.GOOS)
}
