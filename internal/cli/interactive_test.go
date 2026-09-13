package cli

import (
	"testing"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{
			input:    `search -q "chat heretic"`,
			expected: []string{"search", "-q", "chat heretic"},
		},
		{
			input:    "copy 1",
			expected: []string{"copy", "1"},
		},
		{
			input:    `search -q 'coding assistant' --limit 10`,
			expected: []string{"search", "-q", "coding assistant", "--limit", "10"},
		},
		{
			input:    "",
			expected: []string{},
		},
		{
			input:    "help",
			expected: []string{"help"},
		},
		{
			input:    `deploy "Qwen/Qwen2.5-1.5B-Instruct"`,
			expected: []string{"deploy", "Qwen/Qwen2.5-1.5B-Instruct"},
		},
	}

	for _, tt := range tests {
		got := tokenize(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("tokenize(%q) returned %d tokens, want %d\ngot:  %v\nwant: %v",
				tt.input, len(got), len(tt.expected), got, tt.expected)
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("tokenize(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.expected[i])
			}
		}
	}
}
