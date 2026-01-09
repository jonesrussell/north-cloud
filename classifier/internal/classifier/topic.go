package classifier

import (
	"context"
	"math"
	"strings"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

const (
	// tfNormalizationFactor normalizes log-TF component to prevent runaway scores
	// log(1+4) ≈ 1.6, so dividing by 2.5 gives ~0.64 for typical matches
	// This allows TF to contribute meaningfully while still capping at 1.0 for high counts
	tfNormalizationFactor = 2.5

	// tfWeight is the weight for term frequency component in score calculation
	tfWeight = 0.5

	// coverageWeight is the weight for coverage component in score calculation
	coverageWeight = 0.5
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

// scoreTextAgainstRule calculates a score (0.0-1.0) using log-Term Frequency + coverage
// Uses token-based matching to avoid substring false positives and log-TF for long document handling
func (t *TopicClassifier) scoreTextAgainstRule(text string, rule domain.ClassificationRule) float64 {
	if len(rule.Keywords) == 0 {
		return 0.0
	}

	// Step 1: Tokenize text (lowercase, strip punctuation, split on whitespace)
	text = strings.ToLower(text)
	// Remove common punctuation for word boundary matching
	text = strings.ReplaceAll(text, ",", " ")
	text = strings.ReplaceAll(text, ".", " ")
	text = strings.ReplaceAll(text, "!", " ")
	text = strings.ReplaceAll(text, "?", " ")
	text = strings.ReplaceAll(text, ";", " ")
	text = strings.ReplaceAll(text, ":", " ")

	textWords := strings.Fields(text)
	wordCount := len(textWords)

	if wordCount == 0 {
		return 0.0
	}

	// Step 2: Build word frequency map for O(1) lookup
	wordFreq := make(map[string]int)
	for _, word := range textWords {
		wordFreq[word]++
	}

	// Step 3: Count exact keyword matches (token-based, not substring)
	totalMatches := 0
	uniqueKeywordsMatched := 0
	totalKeywords := len(rule.Keywords)

	for _, keyword := range rule.Keywords {
		keyword = strings.ToLower(strings.TrimSpace(keyword))
		if keyword == "" {
			continue
		}

		// Check for exact word match (not substring)
		occurrences := wordFreq[keyword]
		if occurrences > 0 {
			totalMatches += occurrences
			uniqueKeywordsMatched++
		}
	}

	if totalMatches == 0 {
		return 0.0
	}

	// Step 4: Compute log-TF + coverage score
	// Log-TF: log(1 + occurrences) prevents runaway scores in long documents
	tf := math.Log(1 + float64(totalMatches))

	// Coverage: ratio of unique keywords matched
	coverage := float64(uniqueKeywordsMatched) / float64(totalKeywords)

	// Normalize TF component (log(1+10) ≈ 2.4, so /10 gives ~0.24 max)
	tfComponent := tf / tfNormalizationFactor
	if tfComponent > 1.0 {
		tfComponent = 1.0
	}

	// Weighted combination: TF (50%) + Coverage (50%)
	score := (tfComponent * tfWeight) + (coverage * coverageWeight)

	// Cap at 1.0
	if score > 1.0 {
		score = 1.0
	}

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
