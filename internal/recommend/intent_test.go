package recommend

import "testing"

func TestParseIntentCodeLaptop(t *testing.T) {
	in := ParseIntent("a coding assistant for scripts on a laptop", 16)
	if in.Task != "code" {
		t.Fatalf("task %s", in.Task)
	}
	if !in.PreferGGUF {
		t.Fatal("expected GGUF default")
	}
	if in.MaxParamsB > 4 {
		t.Fatalf("laptop should cap size, got %.1f", in.MaxParamsB)
	}
}

func TestParseIntentRAMPhrase(t *testing.T) {
	in := ParseIntent("chat on 8gb", 64)
	if in.Query != "instruct" {
		t.Fatalf("query should stay a Hub keyword, got %q", in.Query)
	}
	if in.MaxParamsB > 4.5 {
		t.Fatalf("8gb should stay small, got %.1f", in.MaxParamsB)
	}
}

func TestParseIntentCloud(t *testing.T) {
	in := ParseIntent("deploy on runpod gpu", 16)
	if in.PreferGGUF {
		t.Fatal("cloud wording should prefer safetensors")
	}
}
