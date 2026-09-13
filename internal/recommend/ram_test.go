package recommend

import "testing"

func TestRAMGB(t *testing.T) {
	if n := RAMGB(); n <= 0 {
		t.Fatalf("RAMGB() = %v", n)
	}
}
