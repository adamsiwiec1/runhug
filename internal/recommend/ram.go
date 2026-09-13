package recommend

import (
	"runtime"
)

// RAMGB is a conservative default when the OS cannot be queried.
func RAMGB() float64 {
	if n := ramBytes(); n > 0 {
		return float64(n) / (1024 * 1024 * 1024)
	}
	if runtime.GOARCH == "arm64" {
		return 8
	}
	return 8
}
