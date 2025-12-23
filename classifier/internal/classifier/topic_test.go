package classifier

import (
	"context"
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

	if !result.IsCrimeRelated {
		t.Error("expected IsCrimeRelated to be true")
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

	if result.IsCrimeRelated {
		t.Error("expected IsCrimeRelated to be false")
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
		name     string
		text     string
		expected float64
	}{
		{
			name:     "all keywords match",
			text:     "police arrest murder investigation",
			expected: 1.0,
		},
		{
			name:     "half keywords match",
			text:     "police arrest other words",
			expected: 0.5,
		},
		{
			name:     "no keywords match",
			text:     "completely different content",
			expected: 0.0,
		},
		{
			name:     "one keyword match",
			text:     "the police were present",
			expected: 0.25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := classifier.scoreTextAgainstRule(tt.text, rule)
			if score != tt.expected {
				t.Errorf("expected %f, got %f", tt.expected, score)
			}
		})
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

	classifier.UpdateRules(newRules)

	updatedRules := classifier.GetRules()

	if len(updatedRules) != 1 {
		t.Fatalf("expected 1 rule after update, got %d", len(updatedRules))
	}

	if updatedRules[0].TopicName != "sports" {
		t.Errorf("expected sports topic, got %s", updatedRules[0].TopicName)
	}
}
