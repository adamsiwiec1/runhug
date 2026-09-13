package family

import "strings"

// EnvFor returns vLLM worker env defaults for a Hugging Face repo id.
// Values are conservative and match the official worker-vllm Hub schema.
func EnvFor(modelID string) map[string]string {
	id := strings.ToLower(modelID)
	env := map[string]string{}
	switch {
	case containsAny(id, "qwen3"):
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

func containsAny(s string, parts ...string) bool {
	for _, p := range parts {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}
