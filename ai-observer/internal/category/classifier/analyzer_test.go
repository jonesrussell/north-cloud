package classifier

import (
	"testing"
)

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

func TestParseInsights_MultipleInsights(t *testing.T) {
	content := `[
		{"severity":"high","summary":"Domain X borderline rate 40%","details":{"domain":"x.com","rate":0.4},"suggested_actions":["Review domain X"]},
		{"severity":"medium","summary":"Label Y low confidence","details":{"label":"Y","avg_conf":0.55},"suggested_actions":["Retrain model"]},
		{"severity":"low","summary":"Minor label drift on Z","details":{"label":"Z"},"suggested_actions":["Monitor"]}
	]`

	const (
		expectedCount    = 3
		testTokensUsed   = 600
		tokensPerInsight = testTokensUsed / expectedCount
		testModel        = "claude-haiku-4-5-20251001"
	)

	insights, err := parseInsights(content, testTokensUsed, testModel)
	if err != nil {
		t.Fatalf("parseInsights() error = %v", err)
	}

	if len(insights) != expectedCount {
		t.Fatalf("expected %d insights, got %d", expectedCount, len(insights))
	}

	if insights[0].Severity != "high" {
		t.Errorf("expected first insight severity 'high', got %q", insights[0].Severity)
	}
	if insights[1].Severity != "medium" {
		t.Errorf("expected second insight severity 'medium', got %q", insights[1].Severity)
	}
	if insights[2].Severity != "low" {
		t.Errorf("expected third insight severity 'low', got %q", insights[2].Severity)
	}

	for i, ins := range insights {
		if ins.TokensUsed != tokensPerInsight {
			t.Errorf("insight[%d] tokens_used = %d, want %d", i, ins.TokensUsed, tokensPerInsight)
		}
		if ins.Model != testModel {
			t.Errorf("insight[%d] model = %q, want %q", i, ins.Model, testModel)
		}
		if ins.Category != categoryName {
			t.Errorf("insight[%d] category = %q, want %q", i, ins.Category, categoryName)
		}
	}
}

func TestParseInsights_TruncatedJSON(t *testing.T) {
	// Simulate truncated JSON response (what happens with low maxResponseTokens).
	content := `[{"severity":"high","summary":"Domain X borderline rate 40%","details":{"domain":"x.com"},"suggested_actions":["Re`

	_, err := parseInsights(content, 500, "test-model")
	if err == nil {
		t.Error("expected error for truncated JSON, got nil")
	}
}
