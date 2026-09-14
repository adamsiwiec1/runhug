package hf

import "testing"

func TestParseLinkNext(t *testing.T) {
	base := "https://huggingface.co"
	h := `<https://huggingface.co/api/models?limit=100&cursor=abc>; rel="next", <https://huggingface.co/api/models?limit=100&cursor=zzz>; rel="last"`
	got := parseLinkNext(h, base)
	if got != "/api/models?limit=100&cursor=abc" {
		t.Fatalf("got %q", got)
	}
	if parseLinkNext("", base) != "" {
		t.Fatal("expected empty")
	}
}

func TestParseLastModifiedUnix(t *testing.T) {
	if parseLastModifiedUnix("2026-01-02T03:04:05Z") == 0 {
		t.Fatal("expected non-zero")
	}
	if parseLastModifiedUnix("") != 0 {
		t.Fatal("expected zero")
	}
}
