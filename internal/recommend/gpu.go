package recommend

import (
	"fmt"

	"github.com/adamsiwiec1/runhug/internal/hf"
	"github.com/adamsiwiec1/runhug/internal/runpod"
	"github.com/adamsiwiec1/runhug/internal/sizing"
)

// GPUAdvice is a VRAM estimate + suggested serverless pool.
type GPUAdvice struct {
	RequiredGB float64
	WeightGB   float64
	Params     string
	Choice     runpod.Choice
	Offline    bool
	Text       string
}

// AdviseGPU sizes the model and picks a Runpod-style pool (live or offline catalog).
func AdviseGPU(m hf.Model, live []runpod.GPU, preferPool string, maxLen int) (GPUAdvice, error) {
	if maxLen <= 0 {
		maxLen = 8192
	}
	format := hf.DetectFormat(m)
	if format.Engine == hf.EngineGGUF {
		if _, q, ok := hf.PickGGUF(m); ok {
			format.Quant = q
		}
	}
	est := sizing.EstimateModel(m, format, maxLen)
	req := est.RequiredGB
	if req <= 0 {
		req = 16
	}
	c, offline, err := runpod.Suggest(live, req, preferPool)
	if err != nil {
		return GPUAdvice{RequiredGB: req, WeightGB: est.WeightGB, Params: est.ParamsLabel()}, err
	}
	text := fmt.Sprintf("%s ×%d (%s, %.0f GB) ≈ $%.2f/hr  need ~%.1f GB VRAM",
		c.Pool.ID, c.GPUCount, c.Pool.ExampleGPU, c.Pool.MemoryGB, c.HourlyUSD, req)
	if offline {
		text += "  [offline estimate]"
	}
	if c.Next != nil {
		text += fmt.Sprintf("  | next %s $%.2f/hr", c.Next.ID, c.Next.PricePerHour)
	}
	return GPUAdvice{
		RequiredGB: req,
		WeightGB:   est.WeightGB,
		Params:     est.ParamsLabel(),
		Choice:     c,
		Offline:    offline,
		Text:       text,
	}, nil
}
