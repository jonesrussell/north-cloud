package drift

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/category"
	driftpkg "github.com/jonesrussell/north-cloud/ai-observer/internal/drift"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/provider"
)

const (
	driftAnalysisModel = "claude-haiku-4-5-20251001"
	// maxResponseTokens is the LLM response token limit for drift analysis.
	maxResponseTokens = 1500
)

const driftSystemPrompt = `You are an AI system observer analyzing statistical drift in a news content classifier.
You receive drift metrics (KL divergence, PSI, cross-matrix deviations) that have breached thresholds.
Your job is to:
1. Explain the likely cause of the drift in plain language
2. Suggest specific keyword rule changes (additions or removals) to address the drift
3. Rate your confidence in each suggestion (high/medium/low)

Respond ONLY with valid JSON matching this schema:
{
  "summary": "one sentence explanation",
  "suggested_actions": ["action 1", "action 2"],
  "suggested_rules": [
    {"operation": "add|remove", "topic": "topic_name", "keyword": "keyword", "confidence": "high|medium|low"}
  ]
}`

// promptSignal is a JSON-tagged projection of DriftSignal for LLM prompts.
type promptSignal struct {
	Metric    string  `json:"metric"`
	Scope     string  `json:"scope"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
}

func toPromptSignals(signals []driftpkg.DriftSignal) []promptSignal {
	out := make([]promptSignal, 0, len(signals))
	for _, s := range signals {
		if s.Breached {
			out = append(out, promptSignal{
				Metric:    s.Metric,
				Scope:     s.Scope,
				Value:     s.Value,
				Threshold: s.Threshold,
			})
		}
	}
	return out
}

func analyzeDrift(
	ctx context.Context,
	p provider.LLMProvider,
	signals []driftpkg.DriftSignal,
) (*category.Insight, error) {
	breachedSignals := toPromptSignals(signals)

	promptData, err := json.Marshal(breachedSignals)
	if err != nil {
		return nil, fmt.Errorf("marshal signals for prompt: %w", err)
	}

	userPrompt := fmt.Sprintf(
		"The following drift metrics have breached their thresholds:\n\n%s\n\nAnalyze the drift and suggest rule changes.",
		string(promptData),
	)

	resp, err := p.Generate(ctx, provider.GenerateRequest{
		SystemPrompt: driftSystemPrompt,
		UserPrompt:   userPrompt,
		MaxTokens:    maxResponseTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("llm generate: %w", err)
	}

	return parseDriftResponse(resp)
}

type driftLLMResponse struct {
	Summary          string   `json:"summary"`
	SuggestedActions []string `json:"suggested_actions"`
	SuggestedRules   []any    `json:"suggested_rules"`
}

func parseDriftResponse(resp provider.GenerateResponse) (*category.Insight, error) {
	content := stripFences(resp.Content)

	var parsed driftLLMResponse
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, fmt.Errorf("parse drift LLM response: %w", err)
	}

	details := map[string]any{}
	if len(parsed.SuggestedRules) > 0 {
		details["suggested_rules"] = parsed.SuggestedRules
		details["action_type"] = "rule_patch"
	}

	totalTokens := resp.InputTokens + resp.OutputTokens

	return &category.Insight{
		Category:         categoryName,
		Summary:          parsed.Summary,
		Details:          details,
		SuggestedActions: parsed.SuggestedActions,
		TokensUsed:       totalTokens,
		Model:            driftAnalysisModel,
	}, nil
}

// stripFences removes markdown code fences from LLM output.
// Handles: ```json\n{...}\n```, ```\n{...}\n```, and bare JSON.
// Also handles extra text after closing fences.
func stripFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	// Find end of opening fence line
	idx := strings.Index(s, "\n")
	if idx == -1 {
		return s
	}
	s = s[idx+1:]
	// Find closing fence
	if end := strings.LastIndex(s, "```"); end != -1 {
		s = s[:end]
	}
	return strings.TrimSpace(s)
}
