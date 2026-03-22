package drift

import (
	"testing"
	"time"

	driftpkg "github.com/jonesrussell/north-cloud/ai-observer/internal/drift"
)

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

func TestIsBaselineStale(t *testing.T) {
	cat := &Category{baselineDays: 7}

	tests := []struct {
		name string
		base *driftpkg.Baseline
		want bool
	}{
		{
			name: "fresh baseline",
			base: &driftpkg.Baseline{ComputedAt: time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)},
			want: false,
		},
		{
			name: "stale baseline",
			base: &driftpkg.Baseline{ComputedAt: time.Now().UTC().Add(-10 * 24 * time.Hour).Format(time.RFC3339)},
			want: true,
		},
		{
			name: "just inside boundary",
			base: &driftpkg.Baseline{ComputedAt: time.Now().UTC().Add(-6 * 24 * time.Hour).Format(time.RFC3339)},
			want: false,
		},
		{
			name: "unparseable timestamp",
			base: &driftpkg.Baseline{ComputedAt: "not-a-date"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cat.isBaselineStale(tt.base)
			if got != tt.want {
				t.Errorf("isBaselineStale() = %v, want %v (computed_at=%s)", got, tt.want, tt.base.ComputedAt)
			}
		})
	}
}
