//nolint:testpackage // Testing internal classifier requires same package access
package classifier

import (
	"context"
	"strings"
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

func TestTopicClassifier_Classify_Crime(t *testing.T) {
	rules := []domain.ClassificationRule{
		{
			RuleName:      "crime_detection",
			RuleType:      domain.RuleTypeTopic,
			TopicName:     "crime",
			Keywords:      []string{"police", "arrest", "charged", "murder", "investigation"},
			MinConfidence: 0.3,
			Enabled:       true,
		},
	}

	classifier := NewTopicClassifier(&mockLogger{}, rules)

	raw := &domain.RawContent{
		ID:      "crime-article",
		Title:   "Police Arrest Suspect in Downtown Area",
		RawText: "Police have arrested a suspect following an investigation into the incident.",
	}

	result, err := classifier.Classify(context.Background(), raw)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should detect crime topic
	if len(result.Topics) == 0 {
		t.Fatal("expected at least one topic, got none")
	}

	if result.Topics[0] != "crime" {
		t.Errorf("expected crime topic, got %s", result.Topics[0])
	}

	// Verify crime is in topics array
	hasCrime := false
	for _, topic := range result.Topics {
		if topic == "crime" {
			hasCrime = true
			break
		}
	}
	if !hasCrime {
		t.Error("expected crime to be in topics array")
	}

	if result.HighestTopic != "crime" {
		t.Errorf("expected highest topic to be crime, got %s", result.HighestTopic)
	}

	// Should have a reasonable score (3/5 keywords matched = 0.6)
	crimeScore, ok := result.TopicScores["crime"]
	if !ok {
		t.Fatal("expected crime score in TopicScores")
	}

	if crimeScore < 0.3 {
		t.Errorf("expected crime score >= 0.3, got %f", crimeScore)
	}
}

func TestTopicClassifier_Classify_MultipleTopics(t *testing.T) {
	rules := []domain.ClassificationRule{
		{
			RuleName:      "crime_detection",
			RuleType:      domain.RuleTypeTopic,
			TopicName:     "crime",
			Keywords:      []string{"police", "arrest"},
			MinConfidence: 0.3,
			Enabled:       true,
		},
		{
			RuleName:      "local_news",
			RuleType:      domain.RuleTypeTopic,
			TopicName:     "local_news",
			Keywords:      []string{"community", "local", "downtown"},
			MinConfidence: 0.3,
			Enabled:       true,
		},
	}

	classifier := NewTopicClassifier(&mockLogger{}, rules)

	raw := &domain.RawContent{
		ID:      "multi-topic",
		Title:   "Police Arrest in Downtown Community",
		RawText: "Local police made an arrest in the downtown area affecting the community.",
	}

	result, err := classifier.Classify(context.Background(), raw)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should detect both topics
	if len(result.Topics) != 2 {
		t.Errorf("expected 2 topics, got %d", len(result.Topics))
	}

	// Check both topics are present
	foundCrime := false
	foundLocal := false
	for _, topic := range result.Topics {
		if topic == "crime" {
			foundCrime = true
		}
		if topic == "local_news" {
			foundLocal = true
		}
	}

	if !foundCrime {
		t.Error("expected to find crime topic")
	}
	if !foundLocal {
		t.Error("expected to find local_news topic")
	}

	// Crime should be highest scoring (2/2 = 1.0 vs 3/3 = 1.0, tie)
	if result.HighestTopic == "" {
		t.Error("expected a highest topic to be set")
	}
}

func TestTopicClassifier_Classify_NoMatch(t *testing.T) {
	rules := []domain.ClassificationRule{
		{
			RuleName:      "crime_detection",
			RuleType:      domain.RuleTypeTopic,
			TopicName:     "crime",
			Keywords:      []string{"police", "arrest", "murder"},
			MinConfidence: 0.5,
			Enabled:       true,
		},
	}

	classifier := NewTopicClassifier(&mockLogger{}, rules)

	raw := &domain.RawContent{
		ID:      "no-match",
		Title:   "Local Restaurant Opens",
		RawText: "A new restaurant has opened in the neighborhood, serving delicious food.",
	}

	result, err := classifier.Classify(context.Background(), raw)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not detect any topics
	if len(result.Topics) != 0 {
		t.Errorf("expected no topics, got %d", len(result.Topics))
	}

	// Verify crime is not in topics array
	for _, topic := range result.Topics {
		if topic == "crime" {
			t.Error("expected crime NOT to be in topics array")
			break
		}
	}

	if result.HighestTopic != "" {
		t.Errorf("expected no highest topic, got %s", result.HighestTopic)
	}
}

func TestTopicClassifier_Classify_BelowConfidenceThreshold(t *testing.T) {
	rules := []domain.ClassificationRule{
		{
			RuleName:      "crime_detection",
			RuleType:      domain.RuleTypeTopic,
			TopicName:     "crime",
			Keywords:      []string{"police", "arrest", "murder", "investigation", "detective"},
			MinConfidence: 0.8, // Very high threshold
			Enabled:       true,
		},
	}

	classifier := NewTopicClassifier(&mockLogger{}, rules)

	raw := &domain.RawContent{
		ID:      "below-threshold",
		Title:   "Police Report",
		RawText: "A police spokesperson made a brief statement.", // Only 1 of 5 keywords = 0.2
	}

	result, err := classifier.Classify(context.Background(), raw)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not match because score (0.2) is below confidence threshold (0.8)
	if len(result.Topics) != 0 {
		t.Errorf("expected no topics (below threshold), got %d", len(result.Topics))
	}
}

func TestTopicClassifier_Classify_DisabledRule(t *testing.T) {
	rules := []domain.ClassificationRule{
		{
			RuleName:      "crime_detection",
			RuleType:      domain.RuleTypeTopic,
			TopicName:     "crime",
			Keywords:      []string{"police", "arrest"},
			MinConfidence: 0.3,
			Enabled:       false, // Disabled
		},
	}

	classifier := NewTopicClassifier(&mockLogger{}, rules)

	raw := &domain.RawContent{
		ID:      "disabled-rule",
		Title:   "Police Arrest Suspect",
		RawText: "Police arrested a suspect today.",
	}

	result, err := classifier.Classify(context.Background(), raw)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not match because rule is disabled
	if len(result.Topics) != 0 {
		t.Errorf("expected no topics (rule disabled), got %d", len(result.Topics))
	}
}

func TestTopicClassifier_ScoreTextAgainstRule(t *testing.T) {
	classifier := NewTopicClassifier(&mockLogger{}, nil)

	rule := domain.ClassificationRule{
		Keywords: []string{"police", "arrest", "murder", "investigation"},
	}

	tests := []struct {
		name        string
		text        string
		minExpected float64
		maxExpected float64
		description string
	}{
		{
			name:        "all keywords match",
			text:        "police arrest murder investigation",
			minExpected: 0.8, // Should be high due to full coverage + TF
			maxExpected: 1.0,
			description: "All keywords present should score very high",
		},
		{
			name:        "half keywords match",
			text:        "police arrest other words",
			minExpected: 0.4, // Coverage 0.5, TF component varies
			maxExpected: 0.7,
			description: "Half keywords should score moderately",
		},
		{
			name:        "no keywords match",
			text:        "completely different content",
			minExpected: 0.0,
			maxExpected: 0.0,
			description: "No matches should return 0",
		},
		{
			name:        "one keyword match",
			text:        "the police were present",
			minExpected: 0.1, // Coverage 0.25, low TF
			maxExpected: 0.4,
			description: "Single keyword should score low but above 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := classifier.scoreTextAgainstRule(tt.text, rule)
			if score < tt.minExpected || score > tt.maxExpected {
				t.Errorf("%s: expected score between %f and %f, got %f", tt.description, tt.minExpected, tt.maxExpected, score)
			}
		})
	}
}

func TestTopicClassifier_ScoreTextAgainstRule_SubstringTrap(t *testing.T) {
	classifier := NewTopicClassifier(&mockLogger{}, nil)

	rule := domain.ClassificationRule{
		Keywords: []string{"shoot"},
	}

	// "shoot" keyword should NOT match "shooting" word
	text := "shooting shooting shooting"
	score := classifier.scoreTextAgainstRule(text, rule)

	// Should be 0.0 because "shoot" is not an exact word match
	if score > 0.0 {
		t.Errorf("expected 0.0 for substring trap (shoot vs shooting), got %f", score)
	}
}

func TestTopicClassifier_ScoreTextAgainstRule_RepeatedKeywords(t *testing.T) {
	classifier := NewTopicClassifier(&mockLogger{}, nil)

	rule := domain.ClassificationRule{
		Keywords: []string{"shooting"},
	}

	// "shooting" appearing multiple times should score higher than once
	textSingle := "there was a shooting incident"
	textMultiple := "shooting shooting shooting shooting shooting happened"

	scoreSingle := classifier.scoreTextAgainstRule(textSingle, rule)
	scoreMultiple := classifier.scoreTextAgainstRule(textMultiple, rule)

	// Multiple occurrences should score higher due to log-TF
	if scoreMultiple <= scoreSingle {
		t.Errorf("expected repeated keyword to score higher: single=%f, multiple=%f", scoreSingle, scoreMultiple)
	}

	// Multiple occurrences should exceed 0.3 threshold (for RCMP article case)
	if scoreMultiple < 0.3 {
		t.Errorf("expected repeated keyword to exceed 0.3 threshold, got %f", scoreMultiple)
	}
}

func TestTopicClassifier_ScoreTextAgainstRule_Punctuation(t *testing.T) {
	classifier := NewTopicClassifier(&mockLogger{}, nil)

	rule := domain.ClassificationRule{
		Keywords: []string{"shooting"},
	}

	tests := []struct {
		name     string
		text     string
		expected float64
	}{
		{
			name:     "with comma",
			text:     "there was a shooting, and it was serious",
			expected: 0.0, // Will be > 0 due to match
		},
		{
			name:     "with period",
			text:     "there was a shooting. it was serious",
			expected: 0.0, // Will be > 0 due to match
		},
		{
			name:     "with exclamation",
			text:     "there was a shooting! it was serious",
			expected: 0.0, // Will be > 0 due to match
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := classifier.scoreTextAgainstRule(tt.text, rule)
			// Should match despite punctuation
			if score == 0.0 {
				t.Errorf("expected score > 0.0 for text with punctuation, got %f", score)
			}
		})
	}
}

func TestTopicClassifier_ScoreTextAgainstRule_LongDocument(t *testing.T) {
	classifier := NewTopicClassifier(&mockLogger{}, nil)

	rule := domain.ClassificationRule{
		Keywords: []string{"shooting", "police", "arrest"},
	}

	// Create a long document (5000 words) with sparse matches
	var builder strings.Builder
	builder.WriteString("word ")
	for range 5000 {
		builder.WriteString("word ")
	}
	builder.WriteString("shooting police arrest") // Only 3 matches at the end
	longText := builder.String()

	score := classifier.scoreTextAgainstRule(longText, rule)

	// Should score but not be over-weighted due to log-TF normalization
	if score > 1.0 {
		t.Errorf("expected score <= 1.0 for long document, got %f", score)
	}

	// Should still score above 0 due to coverage (3/3 keywords = 1.0 coverage component)
	if score < 0.3 {
		t.Errorf("expected score >= 0.3 for full keyword coverage, got %f", score)
	}
}

func TestTopicClassifier_ScoreTextAgainstRule_ShortDocument(t *testing.T) {
	classifier := NewTopicClassifier(&mockLogger{}, nil)

	rule := domain.ClassificationRule{
		Keywords: []string{"shooting", "police", "arrest"},
	}

	// Short document with dense matches
	shortText := "shooting shooting police arrest shooting"

	score := classifier.scoreTextAgainstRule(shortText, rule)

	// Should score well due to high TF and coverage
	if score < 0.5 {
		t.Errorf("expected high score for dense matches in short document, got %f", score)
	}
}

func TestTopicClassifier_ScoreTextAgainstRule_EmptyText(t *testing.T) {
	classifier := NewTopicClassifier(&mockLogger{}, nil)

	rule := domain.ClassificationRule{
		Keywords: []string{"police", "arrest"},
	}

	score := classifier.scoreTextAgainstRule("", rule)

	if score != 0.0 {
		t.Errorf("expected 0.0 for empty text, got %f", score)
	}
}

func TestTopicClassifier_ScoreTextAgainstRule_NoMatches(t *testing.T) {
	classifier := NewTopicClassifier(&mockLogger{}, nil)

	rule := domain.ClassificationRule{
		Keywords: []string{"police", "arrest"},
	}

	score := classifier.scoreTextAgainstRule("completely unrelated content here", rule)

	if score != 0.0 {
		t.Errorf("expected 0.0 for no matches, got %f", score)
	}
}

func TestTopicClassifier_ScoreTextAgainstRule_RCMPArticleCase(t *testing.T) {
	classifier := NewTopicClassifier(&mockLogger{}, nil)

	// Simulate violent_crime rule with "shooting" keyword
	rule := domain.ClassificationRule{
		Keywords:      []string{"shooting", "gunfire", "murder", "assault", "attack", "weapon", "armed", "gunman", "shooter", "fight", "fighting", "beating", "gang", "gang violence", "drive-by", "turf war", "gang member", "gang activity", "domestic violence", "sexual assault", "rape", "kidnapping", "abduction", "hostage"},
		MinConfidence: 0.3,
	}

	// RCMP article text with "shooting" appearing multiple times
	text := "RCMP investigate gunfire on First Nation in Saskatchewan after deadly shooting. " +
		"Mounties say they were called late Friday to Big Island Lake Cree Nation. " +
		"They say they didn't find anyone with injuries and are looking to determine whether there is any connection to an early morning shooting Dec. 30. " +
		"That shooting left one person dead and three others with injuries. " +
		"Security has been scaled up as the search continues for a pair of suspects wanted in connection with the shooting."

	score := classifier.scoreTextAgainstRule(text, rule)

	// Should exceed 0.3 threshold due to repeated "shooting" keyword
	if score < 0.3 {
		t.Errorf("expected score >= 0.3 for RCMP article case (shooting appears multiple times), got %f", score)
	}

	// Verify "shooting" is being counted (not "shoot" substring)
	// If substring matching was used, we'd get false positives
	if score > 1.0 {
		t.Errorf("expected score <= 1.0, got %f", score)
	}
}

func TestTopicClassifier_ClassifyBatch(t *testing.T) {
	rules := []domain.ClassificationRule{
		{
			RuleName:      "crime_detection",
			RuleType:      domain.RuleTypeTopic,
			TopicName:     "crime",
			Keywords:      []string{"police", "arrest"},
			MinConfidence: 0.3,
			Enabled:       true,
		},
	}

	classifier := NewTopicClassifier(&mockLogger{}, rules)

	rawItems := []*domain.RawContent{
		{
			ID:      "batch-1",
			Title:   "Police Arrest",
			RawText: "Police made an arrest today.",
		},
		{
			ID:      "batch-2",
			Title:   "Restaurant News",
			RawText: "New restaurant opens.",
		},
	}

	results, err := classifier.ClassifyBatch(context.Background(), rawItems)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// First should match crime
	if len(results[0].Topics) == 0 || results[0].Topics[0] != "crime" {
		t.Error("expected first item to match crime topic")
	}

	// Second should not match
	if len(results[1].Topics) != 0 {
		t.Error("expected second item to not match any topic")
	}
}

func TestTopicClassifier_UpdateRules(t *testing.T) {
	initialRules := []domain.ClassificationRule{
		{
			RuleName:  "crime_detection",
			RuleType:  domain.RuleTypeTopic,
			TopicName: "crime",
			Enabled:   true,
		},
	}

	classifier := NewTopicClassifier(&mockLogger{}, initialRules)

	newRules := []domain.ClassificationRule{
		{
			RuleName:  "sports_detection",
			RuleType:  domain.RuleTypeTopic,
			TopicName: "sports",
			Enabled:   true,
		},
	}

	// Convert to pointers as UpdateRules expects []*domain.ClassificationRule
	rulePointers := make([]*domain.ClassificationRule, len(newRules))
	for i := range newRules {
		rulePointers[i] = &newRules[i]
	}

	classifier.UpdateRules(rulePointers)

	updatedRules := classifier.GetRules()

	if len(updatedRules) != 1 {
		t.Fatalf("expected 1 rule after update, got %d", len(updatedRules))
	}

	if updatedRules[0].TopicName != "sports" {
		t.Errorf("expected sports topic, got %s", updatedRules[0].TopicName)
	}
}
