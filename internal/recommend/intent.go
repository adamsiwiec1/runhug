package recommend

import (
	"regexp"
	"strconv"
	"strings"
)

type Intent struct {
	Raw           string
	Task          string
	Query         string
	PreferGGUF    bool
	MaxParamsB    float64
	TargetParamsB float64
}

var ramRe = regexp.MustCompile(`(?i)\b(\d+)\s*gb\b`)

func ParseIntent(raw string, ramGB float64) Intent {
	s := strings.ToLower(strings.TrimSpace(raw))
	in := Intent{
		Raw:           raw,
		Task:          "chat",
		Query:         "instruct",
		PreferGGUF:    true,
		MaxParamsB:    4,
		TargetParamsB: 1.7,
	}
	if ramGB >= 12 {
		in.MaxParamsB = 8
	}

	switch {
	case containsAny(s, "code", "coding", "programmer", "developer", "script", "refactor", "debug"):
		in.Task = "code"
		in.Query = "instruct coder"
	case containsAny(s, "math", "reason", "logic", "think"):
		in.Task = "reason"
		in.Query = "instruct"
	case containsAny(s, "summar", "tldr", "notes"):
		in.Task = "summarize"
		in.Query = "instruct"
	case containsAny(s, "translat"):
		in.Task = "translate"
		in.Query = "instruct"
	}

	if m := ramRe.FindStringSubmatch(s); len(m) == 2 {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			in.MaxParamsB = float64(n) * 0.45
			if in.MaxParamsB < 2 {
				in.MaxParamsB = 2
			}
			if in.MaxParamsB > 8 {
				in.MaxParamsB = 8
			}
		}
	}
	if containsAny(s, "laptop", "macbook", "cpu", "lightweight", "phone", "cheap") {
		if in.MaxParamsB > 4 {
			in.MaxParamsB = 4
		}
		in.TargetParamsB = 1.7
	}
	if containsAny(s, "large", "biggest", "70b", "best quality", "maximum") {
		in.TargetParamsB = 7
		if in.MaxParamsB < 14 {
			in.MaxParamsB = 14
		}
	}
	if containsAny(s, "cloud", "runpod", "vllm", "gpu") {
		in.PreferGGUF = false
	}
	return in
}

func containsAny(s string, parts ...string) bool {
	for _, p := range parts {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}
