package classifier

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/category"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/provider"
)

const (
	systemPrompt = `You are an AI system observer analyzing classifier output from a news content pipeline.
Your job is to identify label drift, borderline clusters, and domains that consistently produce low-confidence classifications.
Respond ONLY with valid JSON matching the schema provided. Be concise and actionable.`

	// insightJSONSchema is the JSON schema for the LLM response array.
	// Passed via GenerateRequest.JSONSchema so the provider can enforce it in the system prompt.
	insightJSONSchema = `{
  "type": "array",
  "items": {
    "type": "object",
    "required": ["severity", "summary", "details", "suggested_actions"],
    "properties": {
      "severity":          { "type": "string", "enum": ["low", "medium", "high"] },
      "summary":           { "type": "string" },
      "details":           { "type": "object" },
      "suggested_actions": { "type": "array", "items": { "type": "string" } }
    }
  }
}`

	maxResponseTokens = 2048
	// maxStatPairs is the max domain+label pairs to include in the LLM prompt.
	maxStatPairs = 30
	// categoryName is the category identifier for insights produced here.
	categoryName = "classifier"
)

// domainStats holds aggregated stats per domain+label pair.
type domainStats struct {
	Domain          string  `json:"domain"`
	Label           string  `json:"label"`
	Count           int     `json:"count"`
	BorderlineCount int     `json:"borderline_count"`
	AvgConfidence   float64 `json:"avg_confidence"`
}

// analyze runs the AI pass on sampled events.
func analyze(ctx context.Context, events []category.Event, pop PopulationStats, p provider.LLMProvider, model string) ([]category.Insight, error) {
	if len(events) == 0 {
		return nil, nil
	}

	stats := aggregateStats(events)
	userPrompt := buildPrompt(stats, pop)

	resp, err := p.Generate(ctx, provider.GenerateRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		MaxTokens:    maxResponseTokens,
		JSONSchema:   insightJSONSchema,
	})
	if err != nil {
		return nil, fmt.Errorf("generate: %w", err)
	}

	return parseInsights(resp.Content, resp.InputTokens+resp.OutputTokens, model)
}

func aggregateStats(events []category.Event) []domainStats {
	type key struct{ domain, label string }
	statsMap := make(map[key]*domainStats)

	for _, e := range events {
		k := key{e.Source, e.Label}
		s, ok := statsMap[k]
		if !ok {
			s = &domainStats{Domain: e.Source, Label: e.Label}
			statsMap[k] = s
		}
		s.Count++
		s.AvgConfidence += e.Confidence
		if e.Confidence < borderlineThreshold {
			s.BorderlineCount++
		}
	}

	result := make([]domainStats, 0, len(statsMap))
	for _, s := range statsMap {
		if s.Count > 0 {
			s.AvgConfidence /= float64(s.Count)
		}
		result = append(result, *s)
	}

	sort.Slice(result, func(i, j int) bool {
		ri := float64(result[i].BorderlineCount) / float64(result[i].Count)
		rj := float64(result[j].BorderlineCount) / float64(result[j].Count)
		return ri > rj
	})

	return result
}

func buildPrompt(stats []domainStats, pop PopulationStats) string {
	if len(stats) > maxStatPairs {
		stats = stats[:maxStatPairs]
	}

	data, err := json.Marshal(stats)
	if err != nil {
		// stats is always a []domainStats which is always marshallable — treat as impossible
		data = []byte("[]")
	}

	return fmt.Sprintf(`Analyze these classifier output statistics (domain+label aggregates from the last polling window).
Identify concerning patterns: high borderline rates, consistent low confidence, or unexpected label distributions.

Population context (full window, before sampling):
- total_docs: %d
- avg_confidence: %.4f
- borderline_rate: %.4f (fraction of docs below %.2f confidence)

The sample below is a RANDOM representative subset of the above population (not sorted by confidence).
Use the population context — not just the sample — to calibrate severity.
A high borderline_rate in the sample is only concerning if it substantially exceeds the population borderline_rate.

Sample statistics (JSON):
%s

Respond with a JSON array of insights. Each must have: severity (low/medium/high), summary (one sentence),
details (object with relevant metrics), suggested_actions (array of strings).
If no issues found, return [].`, pop.TotalDocs, pop.AvgConfidence, pop.BorderlineRate, borderlineThreshold, string(data))
}

// stripMarkdownFence removes optional ```json ... ``` or ``` ... ``` wrappers
// that some LLMs add despite being instructed to return raw JSON.
func stripMarkdownFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	return s
}

// rawInsight is the expected JSON shape from the LLM.
type rawInsight struct {
	Severity         string         `json:"severity"`
	Summary          string         `json:"summary"`
	Details          map[string]any `json:"details"`
	SuggestedActions []string       `json:"suggested_actions"`
}

func parseInsights(content string, tokensUsed int, model string) ([]category.Insight, error) {
	var raw []rawInsight
	if err := json.Unmarshal([]byte(stripMarkdownFence(content)), &raw); err != nil {
		return nil, fmt.Errorf("unmarshal LLM response: %w (content: %.200s)", err, content)
	}

	if len(raw) == 0 {
		return nil, nil
	}

	tokensPerInsight := tokensUsed / len(raw)
	insights := make([]category.Insight, 0, len(raw))
	for _, r := range raw {
		insights = append(insights, category.Insight{
			Category:         categoryName,
			Severity:         r.Severity,
			Summary:          r.Summary,
			Details:          r.Details,
			SuggestedActions: r.SuggestedActions,
			TokensUsed:       tokensPerInsight,
			Model:            model,
		})
	}
	return insights, nil
}
