package runpod

import "fmt"

// Suggest picks a pool for requiredGB using live GPUs when non-empty,
// otherwise OfflineCatalog. preferPool forces a pool id when set.
func Suggest(live []GPU, requiredGB float64, preferPool string) (Choice, bool, error) {
	gpus := live
	offline := false
	if len(gpus) == 0 {
		gpus = OfflineCatalog()
		offline = true
	}
	c, err := Pick(gpus, requiredGB, preferPool, 0)
	if err != nil {
		return Choice{}, offline, err
	}
	if offline {
		c.Reason = fmt.Sprintf("%s (offline catalog estimate)", c.Reason)
	}
	return c, offline, nil
}
