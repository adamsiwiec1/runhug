package sizing

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/adamsiwiec1/runhug-cli/internal/hf"
)

type Estimate struct {
	Params         int64
	ParamsSource   string
	BytesPerParam  float64
	Precision      string
	WeightGB       float64
	OverheadFactor float64
	RequiredGB     float64
	DiskGB         int
	Notes          []string
}

var (
	moeRe       = regexp.MustCompile(`(?i)(\d+)x(\d+(?:\.\d+)?)[bB](?:[-_]|$)`)
	moeActiveRe = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)[bB]-a(\d+(?:\.\d+)?)[bB]`)
	sizeRe      = regexp.MustCompile(`(?i)(?:^|[^0-9.])(\d+(?:\.\d+)?)[bB](?:[-_]|$)`)
	int4Tags    = []string{"awq", "gptq", "squeezellm", "4-bit", "int4"}
	int8Tags    = []string{"8-bit", "int8"}
)

func EstimateModel(m hf.Model, format hf.Format, maxModelLen int) Estimate {
	e := Estimate{OverheadFactor: 1.25, BytesPerParam: 2, Precision: "fp16/bf16"}
	if maxModelLen <= 0 {
		maxModelLen = 8192
	}

	if format.Quant != "" {
		e.BytesPerParam = 0.5
		e.Precision = format.Quant
	} else {
		for _, tag := range m.Tags {
			l := strings.ToLower(tag)
			for _, t := range int4Tags {
				if l == t {
					e.BytesPerParam = 0.5
					e.Precision = t
				}
			}
			for _, t := range int8Tags {
				if l == t {
					e.BytesPerParam = 1
					e.Precision = t
				}
			}
		}
	}

	if m.Safetensors != nil {
		if m.Safetensors.Total > 0 {
			e.Params = m.Safetensors.Total
			e.ParamsSource = "huggingface safetensors.total"
		}
		if dtype, n := dominantDtype(m.Safetensors.Parameters); n > 0 && format.Quant == "" {
			e.Precision = dtype
			e.BytesPerParam = bytesForDtype(dtype)
			if e.Params == 0 {
				e.Params = n
				e.ParamsSource = "huggingface safetensors.parameters"
			}
		}
	}

	if e.Params == 0 {
		if n, src, ok := parseParamsFromName(m.RepoID()); ok {
			e.Params = n
			e.ParamsSource = src
		}
	}

	if e.Params > 0 {
		e.WeightGB = float64(e.Params) * e.BytesPerParam / 1e9
	} else if m.UsedStorage > 0 {
		e.WeightGB = float64(m.UsedStorage) / 1e9
		e.ParamsSource = "huggingface usedStorage (no param count)"
		e.Notes = append(e.Notes, "parameter count unknown; sized from repo storage")
	}

	if maxModelLen > 8192 {
		extra := 0.10 * float64(maxModelLen-8192) / 8192
		e.OverheadFactor += extra
		e.Notes = append(e.Notes, fmt.Sprintf("added %.0f%% KV headroom for max_model_len=%d", extra*100, maxModelLen))
	}

	if e.WeightGB > 0 {
		e.RequiredGB = e.WeightGB * e.OverheadFactor
	}

	switch {
	case m.UsedStorage > 0:
		e.DiskGB = int(math.Ceil(float64(m.UsedStorage)/1e9)) + 30
	case e.WeightGB > 0:
		e.DiskGB = int(math.Ceil(e.WeightGB)) + 30
	default:
		e.DiskGB = 80
		e.Notes = append(e.Notes, "could not size weights; using 80 GB container disk")
	}
	if e.DiskGB < 40 {
		e.DiskGB = 40
	}
	if e.DiskGB > 250 {
		e.DiskGB = 250
	}
	return e
}

func (e Estimate) ParamsLabel() string {
	if e.Params <= 0 {
		return "unknown"
	}
	b := float64(e.Params) / 1e9
	if b >= 1 {
		return fmt.Sprintf("%.2fB", b)
	}
	return fmt.Sprintf("%.0fM", float64(e.Params)/1e6)
}

func dominantDtype(params map[string]int64) (string, int64) {
	var best string
	var n int64
	for k, v := range params {
		if v > n {
			best, n = k, v
		}
	}
	return best, n
}

func bytesForDtype(dtype string) float64 {
	switch strings.ToUpper(dtype) {
	case "F64", "FLOAT64":
		return 8
	case "F32", "FLOAT32", "FP32":
		return 4
	case "F16", "FLOAT16", "FP16", "BF16", "BFLOAT16":
		return 2
	case "F8_E4M3", "F8_E5M2", "FP8":
		return 1
	case "I8", "INT8", "U8":
		return 1
	case "I4", "INT4", "U4":
		return 0.5
	default:
		return 2
	}
}

func parseParamsFromName(id string) (int64, string, bool) {
	if m := moeRe.FindStringSubmatch(id); len(m) == 3 {
		experts, _ := strconv.ParseFloat(m[1], 64)
		size, _ := strconv.ParseFloat(m[2], 64)
		if experts > 0 && size > 0 {
			// Stored MoE weights ≈ experts × expert-size. Active params are lower.
			n := int64(experts * size * 1e9)
			return n, fmt.Sprintf("parsed %sx%sB MoE from model id", m[1], m[2]), true
		}
	}
	if m := moeActiveRe.FindStringSubmatch(id); len(m) == 3 {
		total, _ := strconv.ParseFloat(m[1], 64)
		if total > 0 {
			// Qwen3-Coder-30B-A3B: RAM follows the full 30B, not the 3B active path.
			return int64(total * 1e9), fmt.Sprintf("parsed %sB MoE (A%sB active) from model id", m[1], m[2]), true
		}
	}
	matches := sizeRe.FindAllStringSubmatch(id, -1)
	if len(matches) == 0 {
		return 0, "", false
	}
	last := matches[len(matches)-1]
	size, err := strconv.ParseFloat(last[1], 64)
	if err != nil || size <= 0 {
		return 0, "", false
	}
	return int64(size * 1e9), fmt.Sprintf("parsed %sB from model id", last[1]), true
}
