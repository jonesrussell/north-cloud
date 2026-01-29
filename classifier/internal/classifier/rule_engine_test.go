package classifier_test

import (
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/classifier/internal/classifier"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

func TestTrieRuleEngine_Match_KeywordsMatchContent(t *testing.T) {
	rules := []*domain.ClassificationRule{
		{
			ID:            1,
			RuleName:      "crime-detection",
			RuleType:      domain.RuleTypeTopic,
			TopicName:     "crime",
			Keywords:      []string{"murder", "robbery", "assault"},
			MinConfidence: 0.1,
			Enabled:       true,
			Priority:      10,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
		{
			ID:            2,
			RuleName:      "sports-detection",
			RuleType:      domain.RuleTypeTopic,
			TopicName:     "sports",
			Keywords:      []string{"hockey", "soccer", "basketball"},
			MinConfidence: 0.1,
			Enabled:       true,
			Priority:      5,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
	}

	engine := classifier.NewTrieRuleEngine(rules, nil, nil)

	testCases := []struct {
		name          string
		title         string
		body          string
		expectedRules []int // Expected rule IDs in order
	}{
		{
			name:          "crime keywords match",
			title:         "Local Murder Investigation",
			body:          "Police are investigating a robbery that led to an assault.",
			expectedRules: []int{1},
		},
		{
			name:          "sports keywords match",
			title:         "Hockey Season Begins",
			body:          "The hockey team is preparing for the soccer and basketball seasons.",
			expectedRules: []int{2},
		},
		{
			name:          "multiple rules match - sorted by priority",
			title:         "Crime at the Hockey Game",
			body:          "A robbery occurred during the hockey match. The assault victim was a soccer player.",
			expectedRules: []int{1, 2}, // Rule 1 has higher priority (10 vs 5)
		},
		{
			name:          "no match",
			title:         "Weather Report",
			body:          "Today will be sunny with clear skies.",
			expectedRules: []int{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matches := engine.Match(tc.title, tc.body)

			if len(matches) != len(tc.expectedRules) {
				t.Errorf("expected %d matches, got %d", len(tc.expectedRules), len(matches))
				return
			}

			for i, expectedID := range tc.expectedRules {
				if matches[i].Rule.ID != expectedID {
					t.Errorf("match %d: expected rule ID %d, got %d", i, expectedID, matches[i].Rule.ID)
				}
			}
		})
	}
}

func TestTrieRuleEngine_DisabledRulesNotMatched(t *testing.T) {
	rules := []*domain.ClassificationRule{
		{
			ID:            1,
			RuleName:      "enabled-rule",
			RuleType:      domain.RuleTypeTopic,
			TopicName:     "technology",
			Keywords:      []string{"computer", "software", "programming"},
			MinConfidence: 0.1,
			Enabled:       true,
			Priority:      10,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
		{
			ID:            2,
			RuleName:      "disabled-rule",
			RuleType:      domain.RuleTypeTopic,
			TopicName:     "science",
			Keywords:      []string{"research", "experiment", "laboratory"},
			MinConfidence: 0.1,
			Enabled:       false, // Disabled
			Priority:      10,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
	}

	engine := classifier.NewTrieRuleEngine(rules, nil, nil)

	// Verify only 1 rule is loaded (the enabled one)
	if engine.RuleCount() != 1 {
		t.Errorf("expected 1 enabled rule, got %d", engine.RuleCount())
	}

	// Test with content matching the disabled rule
	matches := engine.Match("Research Laboratory", "The experiment was conducted in the laboratory.")
	if len(matches) != 0 {
		t.Errorf("expected 0 matches for disabled rule, got %d", len(matches))
	}

	// Test with content matching the enabled rule
	matches = engine.Match("Programming Tutorial", "Learn computer software programming today.")
	if len(matches) != 1 {
		t.Errorf("expected 1 match for enabled rule, got %d", len(matches))
	}
	if len(matches) > 0 && matches[0].Rule.ID != 1 {
		t.Errorf("expected match for rule 1, got rule %d", matches[0].Rule.ID)
	}
}

func TestTrieRuleEngine_UpdateRulesDynamically(t *testing.T) {
	initialRules := []*domain.ClassificationRule{
		{
			ID:            1,
			RuleName:      "initial-rule",
			RuleType:      domain.RuleTypeTopic,
			TopicName:     "politics",
			Keywords:      []string{"election", "government", "policy"},
			MinConfidence: 0.1,
			Enabled:       true,
			Priority:      10,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
	}

	engine := classifier.NewTrieRuleEngine(initialRules, nil, nil)

	// Verify initial state
	if engine.RuleCount() != 1 {
		t.Errorf("expected 1 rule initially, got %d", engine.RuleCount())
	}

	// Content that should match initially
	matches := engine.Match("Election News", "The government announced a new policy today.")
	if len(matches) != 1 {
		t.Errorf("expected 1 match initially, got %d", len(matches))
	}

	// Update rules - add a new rule and modify existing
	updatedRules := []domain.ClassificationRule{
		{
			ID:            1,
			RuleName:      "initial-rule",
			RuleType:      domain.RuleTypeTopic,
			TopicName:     "politics",
			Keywords:      []string{"election", "government", "policy"},
			MinConfidence: 0.1,
			Enabled:       false, // Now disabled
			Priority:      10,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
		{
			ID:            2,
			RuleName:      "new-rule",
			RuleType:      domain.RuleTypeTopic,
			TopicName:     "business",
			Keywords:      []string{"market", "stock", "economy"},
			MinConfidence: 0.1,
			Enabled:       true,
			Priority:      10,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
	}

	engine.UpdateRules(updatedRules)

	// Verify updated state
	if engine.RuleCount() != 1 {
		t.Errorf("expected 1 enabled rule after update, got %d", engine.RuleCount())
	}

	// Old content should no longer match (rule disabled)
	matches = engine.Match("Election News", "The government announced a new policy today.")
	if len(matches) != 0 {
		t.Errorf("expected 0 matches after disabling rule, got %d", len(matches))
	}

	// New content should match new rule
	matches = engine.Match("Market Update", "The stock market shows positive economy growth.")
	if len(matches) != 1 {
		t.Errorf("expected 1 match for new rule, got %d", len(matches))
	}
	if len(matches) > 0 && matches[0].Rule.ID != 2 {
		t.Errorf("expected match for rule 2, got rule %d", matches[0].Rule.ID)
	}
}

func TestTrieRuleEngine_MatchScoring(t *testing.T) {
	rules := []*domain.ClassificationRule{
		{
			ID:            1,
			RuleName:      "broad-rule",
			RuleType:      domain.RuleTypeTopic,
			TopicName:     "news",
			Keywords:      []string{"news", "report", "update", "breaking", "live"},
			MinConfidence: 0.1,
			Enabled:       true,
			Priority:      5,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
	}

	engine := classifier.NewTrieRuleEngine(rules, nil, nil)

	// Test with multiple keyword hits
	matches := engine.Match("Breaking News Report", "This is a live update with the latest news and report.")
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}

	match := matches[0]

	// Verify match statistics
	if match.MatchCount < 4 {
		t.Errorf("expected at least 4 total keyword hits, got %d", match.MatchCount)
	}

	if match.UniqueMatches < 4 {
		t.Errorf("expected at least 4 unique matches, got %d", match.UniqueMatches)
	}

	if match.Coverage <= 0 {
		t.Errorf("expected positive coverage, got %f", match.Coverage)
	}

	if match.Score <= 0 {
		t.Errorf("expected positive score, got %f", match.Score)
	}

	if len(match.MatchedKeywords) < 4 {
		t.Errorf("expected at least 4 matched keywords, got %d", len(match.MatchedKeywords))
	}
}

func TestTrieRuleEngine_PrioritySorting(t *testing.T) {
	rules := []*domain.ClassificationRule{
		{
			ID:            1,
			RuleName:      "low-priority",
			RuleType:      domain.RuleTypeTopic,
			TopicName:     "general",
			Keywords:      []string{"test", "example"},
			MinConfidence: 0.1,
			Enabled:       true,
			Priority:      1,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
		{
			ID:            2,
			RuleName:      "high-priority",
			RuleType:      domain.RuleTypeTopic,
			TopicName:     "important",
			Keywords:      []string{"test", "sample"},
			MinConfidence: 0.1,
			Enabled:       true,
			Priority:      100,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
		{
			ID:            3,
			RuleName:      "medium-priority",
			RuleType:      domain.RuleTypeTopic,
			TopicName:     "medium",
			Keywords:      []string{"test", "demo"},
			MinConfidence: 0.1,
			Enabled:       true,
			Priority:      50,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
	}

	engine := classifier.NewTrieRuleEngine(rules, nil, nil)

	// All rules share "test" keyword
	matches := engine.Match("Test Content", "This is a test example sample demo.")

	if len(matches) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(matches))
	}

	// Verify sorted by priority (descending)
	expectedOrder := []int{2, 3, 1} // priority 100, 50, 1
	for i, expectedID := range expectedOrder {
		if matches[i].Rule.ID != expectedID {
			t.Errorf("position %d: expected rule ID %d, got %d", i, expectedID, matches[i].Rule.ID)
		}
	}
}

func TestTrieRuleEngine_EmptyRules(t *testing.T) {
	engine := classifier.NewTrieRuleEngine(nil, nil, nil)

	if engine.RuleCount() != 0 {
		t.Errorf("expected 0 rules, got %d", engine.RuleCount())
	}

	if engine.KeywordCount() != 0 {
		t.Errorf("expected 0 keywords, got %d", engine.KeywordCount())
	}

	matches := engine.Match("Any Title", "Any content here.")
	if len(matches) != 0 {
		t.Errorf("expected 0 matches with no rules, got %d", len(matches))
	}
}

func TestTrieRuleEngine_MinConfidenceFiltering(t *testing.T) {
	rules := []*domain.ClassificationRule{
		{
			ID:            1,
			RuleName:      "high-confidence-rule",
			RuleType:      domain.RuleTypeTopic,
			TopicName:     "strict",
			Keywords:      []string{"alpha", "beta", "gamma", "delta", "epsilon"},
			MinConfidence: 0.8, // Requires high coverage/score
			Enabled:       true,
			Priority:      10,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
		{
			ID:            2,
			RuleName:      "low-confidence-rule",
			RuleType:      domain.RuleTypeTopic,
			TopicName:     "relaxed",
			Keywords:      []string{"alpha", "beta", "gamma", "delta", "epsilon"},
			MinConfidence: 0.1, // Low threshold
			Enabled:       true,
			Priority:      10,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
	}

	engine := classifier.NewTrieRuleEngine(rules, nil, nil)

	// Match only one keyword - low coverage
	matches := engine.Match("Alpha Test", "The alpha particle was detected.")

	// Only rule 2 should match (low confidence threshold)
	hasRule1 := false
	hasRule2 := false
	for _, m := range matches {
		if m.Rule.ID == 1 {
			hasRule1 = true
		}
		if m.Rule.ID == 2 {
			hasRule2 = true
		}
	}

	if hasRule1 {
		t.Errorf("rule 1 should not match due to high min_confidence threshold")
	}

	if !hasRule2 {
		t.Errorf("rule 2 should match with low min_confidence threshold")
	}
}
