package hf

import "strings"

type Engine string

const (
	EngineVLLM    Engine = "vllm"
	EngineGGUF    Engine = "gguf"
	EngineUnknown Engine = "unknown"
)

type Format struct {
	Engine         Engine
	HasSafetensors bool
	HasGGUF        bool
	Quant          string
}

func DetectFormat(m Model) Format {
	f := Format{Engine: EngineUnknown}
	for _, tag := range m.Tags {
		switch strings.ToLower(tag) {
		case "safetensors":
			f.HasSafetensors = true
		case "gguf":
			f.HasGGUF = true
		case "awq":
			f.Quant = "awq"
		case "gptq":
			f.Quant = "gptq"
		case "squeezellm":
			f.Quant = "squeezellm"
		}
	}
	if strings.EqualFold(m.LibraryName, "gguf") {
		f.HasGGUF = true
	}
	for _, s := range m.Siblings {
		name := strings.ToLower(s.RFilename)
		if strings.HasSuffix(name, ".safetensors") {
			f.HasSafetensors = true
		}
		if strings.HasSuffix(name, ".gguf") {
			f.HasGGUF = true
		}
	}
	if m.Safetensors != nil && m.Safetensors.Total > 0 {
		f.HasSafetensors = true
	}
	switch {
	case f.HasGGUF && !f.HasSafetensors:
		f.Engine = EngineGGUF
	case f.HasSafetensors:
		f.Engine = EngineVLLM
	case f.HasGGUF:
		f.Engine = EngineGGUF
	}
	return f
}
