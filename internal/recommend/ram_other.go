//go:build !darwin && !linux

package recommend

func ramBytes() uint64 { return 0 }
