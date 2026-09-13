package family

import "testing"

func TestEnvFor(t *testing.T) {
	qwen3 := EnvFor("Qwen/Qwen3-8B")
	if qwen3["REASONING_PARSER"] != "qwen3" || qwen3["TOOL_CALL_PARSER"] != "hermes" {
		t.Fatalf("qwen3 %+v", qwen3)
	}
	qwen2 := EnvFor("Qwen/Qwen2.5-7B-Instruct")
	if qwen2["REASONING_PARSER"] != "" || qwen2["TOOL_CALL_PARSER"] != "hermes" {
		t.Fatalf("qwen2 %+v", qwen2)
	}
	mistral := EnvFor("mistralai/Ministral-8B-Instruct-2410")
	if mistral["TOKENIZER_MODE"] != "mistral" || mistral["TOOL_CALL_PARSER"] != "mistral" {
		t.Fatalf("mistral %+v", mistral)
	}
	if len(EnvFor("unknown/model")) != 0 {
		t.Fatal("expected no defaults")
	}
}
