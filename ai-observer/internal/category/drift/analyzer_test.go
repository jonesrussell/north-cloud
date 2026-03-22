package drift

import "testing"

func TestStripFences(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain JSON",
			input: `{"summary":"ok"}`,
			want:  `{"summary":"ok"}`,
		},
		{
			name:  "json fence",
			input: "```json\n{\"summary\":\"ok\"}\n```",
			want:  `{"summary":"ok"}`,
		},
		{
			name:  "bare fence",
			input: "```\n{\"summary\":\"ok\"}\n```",
			want:  `{"summary":"ok"}`,
		},
		{
			name:  "with leading whitespace",
			input: "  ```json\n{\"summary\":\"ok\"}\n```  ",
			want:  `{"summary":"ok"}`,
		},
		{
			name:  "no fences with backticks inside",
			input: `{"summary":"use ` + "`" + `foo` + "`" + ` command"}`,
			want:  `{"summary":"use ` + "`" + `foo` + "`" + ` command"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripFences(tt.input)
			if got != tt.want {
				t.Errorf("stripFences() = %q, want %q", got, tt.want)
			}
		})
	}
}
