package cli

import "testing"

func TestParseChoice(t *testing.T) {
	cases := []struct {
		in         string
		max        int
		wantPick   int
		wantQuery  string
		wantDirect string
	}{
		{"3", 8, 3, "", ""},
		{"0", 8, 0, "", ""},
		{"9", 8, 0, "", ""},
		{"Qwen/Qwen2.5-7B-Instruct", 8, 0, "", "Qwen/Qwen2.5-7B-Instruct"},
		{"qwen3:8b", 8, 0, "", "qwen3:8b"},
		{"coding assistant", 8, 0, "coding assistant", ""},
		{"", 8, 0, "", ""},
	}
	for _, tc := range cases {
		p, q, d := parseChoice(tc.in, tc.max)
		if p != tc.wantPick || q != tc.wantQuery || d != tc.wantDirect {
			t.Fatalf("%q: pick=%d query=%q direct=%q", tc.in, p, q, d)
		}
	}
}
