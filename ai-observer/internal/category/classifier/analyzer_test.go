package classifier

import "testing"

func TestStripMarkdownFence(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "raw JSON unchanged",
			input: `[{"severity":"low"}]`,
			want:  `[{"severity":"low"}]`,
		},
		{
			name:  "json-fenced block",
			input: "```json\n[{\"severity\":\"low\"}]\n```",
			want:  `[{"severity":"low"}]`,
		},
		{
			name:  "plain-fenced block",
			input: "```\n[{\"severity\":\"low\"}]\n```",
			want:  `[{"severity":"low"}]`,
		},
		{
			name:  "surrounding whitespace trimmed",
			input: "  ```json\n[]\n```  ",
			want:  "[]",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripMarkdownFence(tt.input); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
