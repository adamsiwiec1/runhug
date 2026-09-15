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

func TestEnvForQwythosAlias(t *testing.T) {
	e := EnvFor("empero-ai/Qwythos-9B-Claude-Mythos-5-1M")
	if e["ENABLE_AUTO_TOOL_CHOICE"] != "true" || e["TOOL_CALL_PARSER"] != "hermes" || e["REASONING_PARSER"] != "qwen3" {
		t.Fatalf("qwythos alias %+v", e)
	}
}

func TestEnvForHintsBaseModel(t *testing.T) {
	// Repo name has no "qwen"; Hub base_model / tags should still match.
	e := EnvForHints(
		"empero-ai/SomeMythosFineTune",
		"base_model:Qwen/Qwen3.5-9B",
		"Qwen/Qwen3.5-9B",
		"qwen3_5",
		"Qwen3_5ForConditionalGeneration",
	)
	if e["ENABLE_AUTO_TOOL_CHOICE"] != "true" || e["TOOL_CALL_PARSER"] != "hermes" || e["REASONING_PARSER"] != "qwen3" {
		t.Fatalf("hints %+v", e)
	}
}

func TestApply(t *testing.T) {
	base := map[string]string{"MODEL_NAME": "empero-ai/Qwythos-9B-Claude-Mythos-5-1M"}
	e := Apply("empero-ai/Qwythos-9B-Claude-Mythos-5-1M", base)
	if e["MODEL_NAME"] == "" || e["ENABLE_AUTO_TOOL_CHOICE"] != "true" {
		t.Fatalf("%+v", e)
	}
}
