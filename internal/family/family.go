package family

import "strings"

// EnvFor returns vLLM worker env defaults for a Hugging Face repo id.
// Values are conservative and match the official worker-vllm Hub schema.
// Prefer EnvForHints / Apply when Hub card base_model, tags, or config are available
// — many fine-tunes omit "qwen" from the repo name (e.g. Qwythos on Qwen3.5).
func EnvFor(modelID string) map[string]string {
	return EnvForHints(modelID)
}

// EnvForHints is like EnvFor but also matches against extra Hub signals
// (base_model, tags, architectures, model_type). Any matching part wins.
func EnvForHints(parts ...string) map[string]string {
	id := strings.ToLower(strings.Join(parts, " "))
	env := map[string]string{}
	switch {
	case containsAny(id, "qwen3", "qwen3_5", "qwen3.5", "qwythos"):
		env["ENABLE_AUTO_TOOL_CHOICE"] = "true"
		env["TOOL_CALL_PARSER"] = "hermes"
		env["REASONING_PARSER"] = "qwen3"
	case containsAny(id, "qwen"):
		env["ENABLE_AUTO_TOOL_CHOICE"] = "true"
		env["TOOL_CALL_PARSER"] = "hermes"
	case containsAny(id, "ministral", "mistral"):
		env["TOKENIZER_MODE"] = "mistral"
		env["CONFIG_FORMAT"] = "mistral"
		env["LOAD_FORMAT"] = "mistral"
		env["ENABLE_AUTO_TOOL_CHOICE"] = "true"
		env["TOOL_CALL_PARSER"] = "mistral"
	case containsAny(id, "llama-3", "llama3", "meta-llama/llama-3"):
		env["ENABLE_AUTO_TOOL_CHOICE"] = "true"
		env["TOOL_CALL_PARSER"] = "llama3_json"
	case containsAny(id, "deepseek-r1", "deepseek-ai/deepseek-r1"):
		env["REASONING_PARSER"] = "deepseek_r1"
	}
	return env
}

// Apply merges family env defaults for modelID (+ optional Hub hints) into base.
// Family keys overwrite base so DefaultImage tool defaults win over empty plans;
// callers should apply user --env after Apply.
func Apply(modelID string, base map[string]string, hints ...string) map[string]string {
	out := make(map[string]string, len(base)+8)
	for k, v := range base {
		out[k] = v
	}
	parts := make([]string, 0, 1+len(hints))
	parts = append(parts, modelID)
	parts = append(parts, hints...)
	for k, v := range EnvForHints(parts...) {
		out[k] = v
	}
	return out
}

func containsAny(s string, parts ...string) bool {
	for _, p := range parts {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}
