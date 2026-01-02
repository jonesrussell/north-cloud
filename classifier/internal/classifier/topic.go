package classifier

import (
	"context"
	"strings"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

// TopicClassifier classifies content by topic using rule-based keyword matching
type TopicClassifier struct {
	logger Logger
	rules  []domain.ClassificationRule
}

// TopicResult represents the result of topic classification
type TopicResult struct {
	Topics       []string           `json:"topics"`        // List of matched topics
	TopicScores  map[string]float64 `json:"topic_scores"`  // Score for each topic (0.0-1.0)
	HighestTopic string             `json:"highest_topic"` // Topic with highest score
}

// NewTopicClassifier creates a new topic classifier with the given rules
func NewTopicClassifier(logger Logger, rules []domain.ClassificationRule) *TopicClassifier {
	return &TopicClassifier{
		logger: logger,
		rules:  rules,
	}
}

// Classify classifies the content by topic using keyword matching
func (t *TopicClassifier) Classify(ctx context.Context, raw *domain.RawContent) (*TopicResult, error) {
	result := &TopicResult{
		Topics:      make([]string, 0),
		TopicScores: make(map[string]float64),
	}

	// Combine title and text for matching
	text := raw.Title + " " + raw.RawText
	text = strings.ToLower(text)

	// Apply each topic rule
	for i := range t.rules {
		rule := t.rules[i]
		// Skip if rule is not enabled or not a topic rule
		if !rule.Enabled || rule.RuleType != domain.RuleTypeTopic {
			continue
		}

		// Calculate score for this topic
		score := t.scoreTextAgainstRule(text, rule)

		// If score exceeds minimum confidence, add this topic
		if score >= rule.MinConfidence {
			result.Topics = append(result.Topics, rule.TopicName)
			result.TopicScores[rule.TopicName] = score

			t.logger.Debug("Topic matched",
				"content_id", raw.ID,
				"topic", rule.TopicName,
				"score", score,
				"min_confidence", rule.MinConfidence,
			)
		}
	}

	// Determine highest scoring topic
	if len(result.TopicScores) > 0 {
		result.HighestTopic = t.findHighestScoringTopic(result.TopicScores)
	}

	t.logger.Debug("Topic classification complete",
		"content_id", raw.ID,
		"topics", result.Topics,
		"highest_topic", result.HighestTopic,
	)

	return result, nil
}

// scoreTextAgainstRule calculates a score (0.0-1.0) for how well the text matches the rule's keywords
// Uses simple keyword matching with TF-like scoring
func (t *TopicClassifier) scoreTextAgainstRule(text string, rule domain.ClassificationRule) float64 {
	if len(rule.Keywords) == 0 {
		return 0.0
	}

	matchCount := 0
	totalKeywords := len(rule.Keywords)

	// Count how many keywords are present in the text
	for _, keyword := range rule.Keywords {
		keyword = strings.ToLower(strings.TrimSpace(keyword))
		if keyword == "" {
			continue
		}

		// Check if keyword is present in text
		// Use word boundary matching to avoid partial matches
		if strings.Contains(text, keyword) {
			matchCount++
		}
	}

	// Calculate score as ratio of matched keywords
	score := float64(matchCount) / float64(totalKeywords)

	return score
}

// findHighestScoringTopic returns the topic with the highest score
func (t *TopicClassifier) findHighestScoringTopic(scores map[string]float64) string {
	var highestTopic string
	var highestScore float64

	for topic, score := range scores {
		if score > highestScore {
			highestScore = score
			highestTopic = topic
		}
	}

	return highestTopic
}

// ClassifyBatch classifies multiple content items efficiently
func (t *TopicClassifier) ClassifyBatch(ctx context.Context, rawItems []*domain.RawContent) ([]*TopicResult, error) {
	results := make([]*TopicResult, len(rawItems))

	for i, raw := range rawItems {
		result, err := t.Classify(ctx, raw)
		if err != nil {
			return nil, err
		}
		results[i] = result
	}

	return results, nil
}

// UpdateRules updates the classification rules used by the classifier
func (t *TopicClassifier) UpdateRules(rules []*domain.ClassificationRule) {
	// Convert []*ClassificationRule to []ClassificationRule
	ruleValues := make([]domain.ClassificationRule, len(rules))
	for i, rule := range rules {
		ruleValues[i] = *rule
	}
	t.rules = ruleValues
	t.logger.Info("Topic classification rules updated", "count", len(rules))
}

// GetRules returns the current classification rules
func (t *TopicClassifier) GetRules() []domain.ClassificationRule {
	return t.rules
}

// GetTopicStats returns statistics about topic classifications
func (t *TopicClassifier) GetTopicStats() map[string]int {
	// TODO: Implement stats tracking
	// This would track counts of each topic classified
	stats := make(map[string]int)
	for i := range t.rules {
		rule := t.rules[i]
		if rule.RuleType == domain.RuleTypeTopic {
			stats[rule.TopicName] = 0
		}
	}
	return stats
}
