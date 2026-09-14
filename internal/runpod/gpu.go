package runpod

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

type Pool struct {
	ID           string
	MemoryGB     float64
	PricePerHour float64
	Availability string
	ExampleGPU   string
	InStock      bool
}

type Choice struct {
	Pool      Pool
	GPUCount  int
	HourlyUSD float64
	Reason    string
	Next      *Pool
}

var availRank = map[string]int{
	"HIGH":   3,
	"MEDIUM": 2,
	"LOW":    1,
	"NONE":   0,
	"":       0,
}

func (g GPU) PoolID() string {
	if g.Pool == nil {
		return ""
	}
	return strings.TrimSpace(*g.Pool)
}

func (g GPU) ServerlessPrice() float64 {
	if g.Price.Serverless > 0 {
		return g.Price.Serverless
	}
	if g.Price.Secure > 0 {
		return g.Price.Secure
	}
	return g.Price.Community
}

func (g GPU) InStock() bool {
	return availRank[strings.ToUpper(g.Availability)] > 0
}

func SummarizePools(gpus []GPU) []Pool {
	byID := map[string]*Pool{}
	for _, g := range gpus {
		id := g.PoolID()
		if id == "" || g.Memory <= 0 {
			continue
		}
		p, ok := byID[id]
		if !ok {
			byID[id] = &Pool{
				ID:           id,
				MemoryGB:     g.Memory,
				PricePerHour: g.ServerlessPrice(),
				Availability: g.Availability,
				ExampleGPU:   displayName(g),
				InStock:      g.InStock(),
			}
			continue
		}
		if g.Memory < p.MemoryGB {
			p.MemoryGB = g.Memory
		}
		price := g.ServerlessPrice()
		if price > 0 && (p.PricePerHour == 0 || price < p.PricePerHour) {
			p.PricePerHour = price
			p.ExampleGPU = displayName(g)
		}
		if availRank[strings.ToUpper(g.Availability)] > availRank[strings.ToUpper(p.Availability)] {
			p.Availability = g.Availability
		}
		if g.InStock() {
			p.InStock = true
		}
	}
	out := make([]Pool, 0, len(byID))
	for _, p := range byID {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MemoryGB != out[j].MemoryGB {
			return out[i].MemoryGB < out[j].MemoryGB
		}
		if out[i].PricePerHour != out[j].PricePerHour {
			return out[i].PricePerHour < out[j].PricePerHour
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func Pick(gpus []GPU, requiredGB float64, preferPool string, gpuCount int) (Choice, error) {
	if requiredGB <= 0 {
		requiredGB = 16
	}
	if gpuCount <= 0 {
		gpuCount = 1
	}
	pools := SummarizePools(gpus)
	if preferPool != "" {
		p, ok := findPool(pools, preferPool)
		if !ok {
			return Choice{}, fmt.Errorf("unknown GPU pool %q (run `gpus` to list pools)", preferPool)
		}
		if !p.InStock {
			return Choice{}, fmt.Errorf("GPU pool %s has no serverless stock right now", p.ID)
		}
		c := Choice{
			Pool:      p,
			GPUCount:  gpuCount,
			HourlyUSD: p.PricePerHour * float64(gpuCount),
			Reason:    "requested --gpu " + p.ID,
		}
		c.Next = nextUp(pools, p, requiredGB)
		return c, nil
	}

	fit := ListFitting(gpus, requiredGB)
	if len(fit) > 0 {
		best := fit[0]
		c := Choice{
			Pool:      best,
			GPUCount:  1,
			HourlyUSD: best.PricePerHour,
			Reason:    fmt.Sprintf("smallest in-stock pool that fits %.1f GB (weights + 25%% KV/activations)", requiredGB),
		}
		c.Next = nextUp(pools, best, requiredGB)
		return c, nil
	}

	// Multi-GPU: largest in-stock card, enough copies to hold the weights.
	var largest *Pool
	for i := range pools {
		p := &pools[i]
		if !p.InStock || p.MemoryGB <= 0 {
			continue
		}
		if largest == nil || p.MemoryGB > largest.MemoryGB || (p.MemoryGB == largest.MemoryGB && p.PricePerHour < largest.PricePerHour) {
			largest = p
		}
	}
	if largest == nil {
		return Choice{}, fmt.Errorf("no in-stock serverless GPU pools in the catalog")
	}
	need := int(math.Ceil(requiredGB / largest.MemoryGB))
	if need < 2 {
		need = 2
	}
	return Choice{
		Pool:      *largest,
		GPUCount:  need,
		HourlyUSD: largest.PricePerHour * float64(need),
		Reason:    fmt.Sprintf("%.1f GB does not fit one in-stock card; %dx %s (%.0f GB)", requiredGB, need, largest.ID, largest.MemoryGB),
	}, nil
}

// ListFitting returns in-stock serverless pools whose MemoryGB can hold
// requiredGB, sorted by $/hr ascending then VRAM ascending (cheapest fit first).
func ListFitting(gpus []GPU, requiredGB float64) []Pool {
	if requiredGB <= 0 {
		requiredGB = 16
	}
	pools := SummarizePools(gpus)
	var fit []Pool
	for _, p := range pools {
		if p.InStock && p.MemoryGB+0.01 >= requiredGB && p.PricePerHour > 0 {
			fit = append(fit, p)
		}
	}
	sort.Slice(fit, func(i, j int) bool {
		if fit[i].PricePerHour != fit[j].PricePerHour {
			return fit[i].PricePerHour < fit[j].PricePerHour
		}
		if fit[i].MemoryGB != fit[j].MemoryGB {
			return fit[i].MemoryGB < fit[j].MemoryGB
		}
		return fit[i].ID < fit[j].ID
	})
	return fit
}

// FittingOptions returns up to limit pools for interactive pickers: the
// cheapest fitting pool first (recommended), then larger-VRAM alternatives
// sorted by price. limit <= 0 defaults to 5.
func FittingOptions(gpus []GPU, requiredGB float64, limit int) []Pool {
	if limit <= 0 {
		limit = 5
	}
	fit := ListFitting(gpus, requiredGB)
	if len(fit) == 0 {
		return nil
	}
	if len(fit) <= limit {
		return fit
	}
	out := []Pool{fit[0]}
	// Prefer larger/safer cards after the recommended cheapest fit.
	var larger []Pool
	for _, p := range fit[1:] {
		if p.MemoryGB > fit[0].MemoryGB+0.01 {
			larger = append(larger, p)
		}
	}
	sort.Slice(larger, func(i, j int) bool {
		if larger[i].MemoryGB != larger[j].MemoryGB {
			return larger[i].MemoryGB < larger[j].MemoryGB
		}
		return larger[i].PricePerHour < larger[j].PricePerHour
	})
	for _, p := range larger {
		if len(out) >= limit {
			break
		}
		out = append(out, p)
	}
	// Fill remaining slots with other fitting pools (same VRAM, etc.).
	if len(out) < limit {
		seen := map[string]bool{}
		for _, p := range out {
			seen[p.ID] = true
		}
		for _, p := range fit[1:] {
			if len(out) >= limit {
				break
			}
			if seen[p.ID] {
				continue
			}
			out = append(out, p)
			seen[p.ID] = true
		}
	}
	return out
}

func findPool(pools []Pool, id string) (Pool, bool) {
	want := strings.ToUpper(strings.TrimSpace(id))
	for _, p := range pools {
		if strings.EqualFold(p.ID, want) {
			return p, true
		}
	}
	return Pool{}, false
}

func nextUp(pools []Pool, chosen Pool, requiredGB float64) *Pool {
	var best *Pool
	for i := range pools {
		p := &pools[i]
		if !p.InStock || p.ID == chosen.ID {
			continue
		}
		if p.MemoryGB <= chosen.MemoryGB || p.MemoryGB < requiredGB {
			continue
		}
		if best == nil || p.PricePerHour < best.PricePerHour || (p.PricePerHour == best.PricePerHour && p.MemoryGB < best.MemoryGB) {
			best = p
		}
	}
	return best
}

func displayName(g GPU) string {
	if g.Name != "" {
		return g.Name
	}
	return g.ID
}
