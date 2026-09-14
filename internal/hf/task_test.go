package hf

import "testing"

func TestInferPipelineTag(t *testing.T) {
	cases := []struct {
		q, want string
	}{
		{"top penetration testing models", "any"},
		{"heretic uncensored image models", "text-to-image"},
		{"animated cartoon generation models", "text-to-image"},
		{"flux image generation", "text-to-image"},
		{"text to video anime", "text-to-video"},
		{"whisper speech recognition", "automatic-speech-recognition"},
		{"qwen instruct chat", "any"},
		{"", "any"},
	}
	for _, tc := range cases {
		if got := InferPipelineTag(tc.q); got != tc.want {
			t.Fatalf("%q: got %q want %q", tc.q, got, tc.want)
		}
	}
}

func TestResolveTask(t *testing.T) {
	if got := ResolveTask("auto", "cartoon models"); got != "text-to-image" {
		t.Fatalf("auto: %q", got)
	}
	if got := ResolveTask("text-generation", "cartoon models"); got != "text-generation" {
		t.Fatalf("explicit: %q", got)
	}
	if got := ResolveTask("any", "x"); got != "any" {
		t.Fatalf("any: %q", got)
	}
}
