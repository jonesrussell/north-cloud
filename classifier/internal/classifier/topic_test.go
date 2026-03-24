//nolint:testpackage // Testing internal classifier requires same package access
package classifier

import (
	"context"
	"slices"
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

	classifier := NewTopicClassifier(&mockLogger{}, rules, 5)

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
	if !slices.Contains(result.Topics, "crime") {
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

	classifier := NewTopicClassifier(&mockLogger{}, rules, 5)

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
	if !slices.Contains(result.Topics, "crime") {
		t.Error("expected to find crime topic")
	}
	if !slices.Contains(result.Topics, "local_news") {
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

	classifier := NewTopicClassifier(&mockLogger{}, rules, 5)

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
	if slices.Contains(result.Topics, "crime") {
		t.Error("expected crime NOT to be in topics array")
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

	classifier := NewTopicClassifier(&mockLogger{}, rules, 5)

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

	classifier := NewTopicClassifier(&mockLogger{}, rules, 5)

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
	classifier := NewTopicClassifier(&mockLogger{}, nil, 5)

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
	classifier := NewTopicClassifier(&mockLogger{}, nil, 5)

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
	classifier := NewTopicClassifier(&mockLogger{}, nil, 5)

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
	classifier := NewTopicClassifier(&mockLogger{}, nil, 5)

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
	classifier := NewTopicClassifier(&mockLogger{}, nil, 5)

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
	classifier := NewTopicClassifier(&mockLogger{}, nil, 5)

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
	classifier := NewTopicClassifier(&mockLogger{}, nil, 5)

	rule := domain.ClassificationRule{
		Keywords: []string{"police", "arrest"},
	}

	score := classifier.scoreTextAgainstRule("", rule)

	if score != 0.0 {
		t.Errorf("expected 0.0 for empty text, got %f", score)
	}
}

func TestTopicClassifier_ScoreTextAgainstRule_NoMatches(t *testing.T) {
	classifier := NewTopicClassifier(&mockLogger{}, nil, 5)

	rule := domain.ClassificationRule{
		Keywords: []string{"police", "arrest"},
	}

	score := classifier.scoreTextAgainstRule("completely unrelated content here", rule)

	if score != 0.0 {
		t.Errorf("expected 0.0 for no matches, got %f", score)
	}
}

func TestTopicClassifier_ScoreTextAgainstRule_RCMPArticleCase(t *testing.T) {
	classifier := NewTopicClassifier(&mockLogger{}, nil, 5)

	// Simulate violent_crime rule with "shooting" keyword
	rule := domain.ClassificationRule{
		Keywords: []string{
			"shooting", "gunfire", "murder", "assault", "attack", "weapon", "armed",
			"gunman", "shooter", "fight", "fighting", "beating", "gang", "gang violence",
			"drive-by", "turf war", "gang member", "gang activity", "domestic violence",
			"sexual assault", "rape", "kidnapping", "abduction", "hostage",
		},
		MinConfidence: 0.3,
	}

	// RCMP article text with "shooting" appearing multiple times
	text := "RCMP investigate gunfire on First Nation in Saskatchewan after deadly shooting. " +
		"Mounties say they were called late Friday to Big Island Lake Cree Nation. " +
		"They say they didn't find anyone with injuries and are looking to determine " +
		"whether there is any connection to an early morning shooting Dec. 30. " +
		"That shooting left one person dead and three others with injuries. " +
		"Security has been scaled up as the search continues for a pair of suspects " +
		"wanted in connection with the shooting."

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

	classifier := NewTopicClassifier(&mockLogger{}, rules, 5)

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

	classifier := NewTopicClassifier(&mockLogger{}, initialRules, 5)

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

func TestTopicClassifier_ScoreTextAgainstRule_MultiWordKeyword(t *testing.T) {
	t.Helper()

	classifier := NewTopicClassifier(&mockLogger{}, nil, 5)

	rule := domain.ClassificationRule{
		Keywords: []string{"human trafficking", "organized crime"},
	}

	tests := []struct {
		name     string
		text     string
		wantZero bool
	}{
		{
			name:     "multi-word keyword matches",
			text:     "authorities investigate human trafficking ring in the city",
			wantZero: false,
		},
		{
			name:     "both multi-word keywords match",
			text:     "organized crime linked to human trafficking operations",
			wantZero: false,
		},
		{
			name:     "partial multi-word keyword does not match",
			text:     "the trafficking of goods across borders is organized",
			wantZero: true,
		},
		{
			name:     "empty text returns zero",
			text:     "",
			wantZero: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			score := classifier.scoreTextAgainstRule(tt.text, rule)
			if tt.wantZero && score != 0.0 {
				t.Errorf("expected 0.0, got %f", score)
			}
			if !tt.wantZero && score == 0.0 {
				t.Errorf("expected score > 0.0 for multi-word match, got 0.0")
			}
		})
	}
}

func TestTopicClassifier_ScoreTextAgainstRule_MixedSingleAndMultiWord(t *testing.T) {
	t.Helper()

	classifier := NewTopicClassifier(&mockLogger{}, nil, 5)

	rule := domain.ClassificationRule{
		Keywords:      []string{"drug", "drugs", "drug trafficking", "drug bust"},
		MinConfidence: 0.3,
	}

	tests := []struct {
		name        string
		text        string
		minExpected float64
		maxExpected float64
	}{
		{
			name:        "single-word only",
			text:        "police found drug and drugs at the scene",
			minExpected: 0.2,
			maxExpected: 0.8,
		},
		{
			name:        "multi-word only",
			text:        "a major drug trafficking operation led to a drug bust",
			minExpected: 0.3,
			maxExpected: 1.0,
		},
		{
			name:        "both single and multi-word",
			text:        "drug trafficking ring busted in major drug bust with drugs seized",
			minExpected: 0.5,
			maxExpected: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			score := classifier.scoreTextAgainstRule(tt.text, rule)
			if score < tt.minExpected || score > tt.maxExpected {
				t.Errorf("expected score between %f and %f, got %f", tt.minExpected, tt.maxExpected, score)
			}
		})
	}
}

func TestTopicClassifier_TestRule_MultiWordKeywords(t *testing.T) {
	t.Helper()

	classifier := NewTopicClassifier(&mockLogger{}, nil, 5)

	rule := &domain.ClassificationRule{
		Keywords:      []string{"human trafficking", "organized crime", "police"},
		MinConfidence: 0.3,
	}

	result := classifier.TestRule(rule, "Human Trafficking Ring", "police investigate organized crime linked to human trafficking")

	if !result.Matched {
		t.Error("expected rule to match")
	}

	if result.UniqueMatches != 3 {
		t.Errorf("expected 3 unique matches, got %d", result.UniqueMatches)
	}

	// Verify all keywords appear in matched list
	wantKeywords := map[string]bool{"human trafficking": false, "organized crime": false, "police": false}
	for _, kw := range result.MatchedKeywords {
		wantKeywords[kw] = true
	}
	for kw, found := range wantKeywords {
		if !found {
			t.Errorf("expected keyword %q in matched list", kw)
		}
	}
}

func TestTopicClassifier_DrugCrime_DoesNotMatchSexTrafficking(t *testing.T) {
	t.Helper()

	rules := []domain.ClassificationRule{
		{
			RuleName:  "drug_crime_detection",
			RuleType:  domain.RuleTypeTopic,
			TopicName: "drug_crime",
			Keywords: []string{
				"drug", "drugs", "narcotics", "dealer", "possession",
				"cocaine", "heroin", "fentanyl", "methamphetamine", "meth", "marijuana", "cannabis", "opioid",
				"drug bust", "drug ring", "cartel", "smuggling", "drug trafficking",
				"narcotics trafficking", "fentanyl trafficking", "cocaine trafficking", "meth trafficking",
				"overdose", "drug-related", "controlled substance",
			},
			MinConfidence: 0.3,
			Enabled:       true,
		},
	}

	classifier := NewTopicClassifier(&mockLogger{}, rules, 5)

	raw := &domain.RawContent{
		ID:    "sex-trafficking-article",
		Title: "Alexander brothers are convicted of sex trafficking in case that shocked real estate world",
		RawText: "Two brothers were convicted of sex trafficking charges after a lengthy trial. " +
			"The case involved multiple victims who were trafficked across state lines. " +
			"Prosecutors described the trafficking ring as one of the most organized in recent history.",
	}

	result, err := classifier.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if slices.Contains(result.Topics, "drug_crime") {
		t.Error("sex trafficking article should NOT be tagged as drug_crime")
	}
}

func TestTopicClassifier_DrugCrime_MatchesDrugTrafficking(t *testing.T) {
	t.Helper()

	rules := []domain.ClassificationRule{
		{
			RuleName:  "drug_crime_detection",
			RuleType:  domain.RuleTypeTopic,
			TopicName: "drug_crime",
			Keywords: []string{
				"drug", "drugs", "narcotics", "dealer", "possession",
				"cocaine", "heroin", "fentanyl", "methamphetamine", "meth", "marijuana", "cannabis", "opioid",
				"drug bust", "drug ring", "cartel", "smuggling", "drug trafficking",
				"narcotics trafficking", "fentanyl trafficking", "cocaine trafficking", "meth trafficking",
				"overdose", "drug-related", "controlled substance",
			},
			MinConfidence: 0.3,
			Enabled:       true,
		},
	}

	classifier := NewTopicClassifier(&mockLogger{}, rules, 5)

	raw := &domain.RawContent{
		ID:    "fentanyl-bust",
		Title: "Major fentanyl trafficking ring busted in downtown",
		RawText: "Police arrested several suspects in a major drug trafficking operation. " +
			"Officers seized large quantities of fentanyl and cocaine during the drug bust. " +
			"The narcotics trafficking ring had been under investigation for months.",
	}

	result, err := classifier.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !slices.Contains(result.Topics, "drug_crime") {
		t.Error("fentanyl trafficking article should be tagged as drug_crime")
	}
}

func TestTopicClassifier_Travel_DoesNotMatchTraffickingContext(t *testing.T) {
	t.Helper()

	rules := []domain.ClassificationRule{
		{
			RuleName:  "travel_detection",
			RuleType:  domain.RuleTypeTopic,
			TopicName: "travel",
			Keywords: []string{
				"vacation", "hotel", "flight", "tourism", "travel",
				"journey", "tour", "tourist",
				"resort", "airline", "airport", "luggage",
				"cruise", "beach", "sightseeing", "adventure", "backpacking",
				"travel guide", "itinerary", "booking", "reservation",
			},
			MinConfidence: 0.4,
			Enabled:       true,
		},
	}

	classifier := NewTopicClassifier(&mockLogger{}, rules, 5)

	raw := &domain.RawContent{
		ID:    "trafficking-context",
		Title: "Trafficking victims brought to destination country via forged passport",
		RawText: "Victims were given forged visas and passports. " +
			"The trafficking ring used a network of safe houses as destinations. " +
			"Authorities tracked the trip from origin to destination.",
	}

	result, err := classifier.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if slices.Contains(result.Topics, "travel") {
		t.Error("trafficking context article should NOT be tagged as travel")
	}
}

func TestTopicClassifier_ScoreTextAgainstRule_AccentedKeywords(t *testing.T) {
	t.Parallel()

	classifier := NewTopicClassifier(&mockLogger{}, nil, defaultMaxTopics)

	tests := []struct {
		name      string
		text      string
		keywords  []string
		wantMatch bool
	}{
		{
			name:      "single accented keyword matches",
			text:      "Les Métis du Manitoba se réunissent",
			keywords:  []string{"métis"},
			wantMatch: true,
		},
		{
			name:      "multi-word accented keyword matches",
			text:      "Les premières nations du Canada annoncent un accord",
			keywords:  []string{"premières nations"},
			wantMatch: true,
		},
		{
			name:      "mixed ASCII and accented keywords",
			text:      "Métis community celebrates résultats at the annual powwow",
			keywords:  []string{"métis", "powwow", "résultats"},
			wantMatch: true,
		},
		{
			name:      "uppercase accented text matches lowercase accented keyword",
			text:      "PREMIÈRES NATIONS DU QUÉBEC",
			keywords:  []string{"premières nations", "québec"},
			wantMatch: true,
		},
		{
			name:      "accented keyword does not match unaccented text",
			text:      "The premieres nations group met today",
			keywords:  []string{"premières nations"},
			wantMatch: false,
		},
		{
			name:      "unaccented keyword does not match accented text",
			text:      "Les premières nations du Canada",
			keywords:  []string{"premieres nations"},
			wantMatch: false,
		},
		{
			name:      "Spanish accented keywords match",
			text:      "Los pueblos indígenas de América celebran",
			keywords:  []string{"pueblos indígenas"},
			wantMatch: true,
		},
		{
			name:      "cedilla and circumflex characters match",
			text:      "Le français est parlé dans la forêt",
			keywords:  []string{"français", "forêt"},
			wantMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rule := domain.ClassificationRule{
				Keywords:      tt.keywords,
				MinConfidence: 0.1,
			}

			score := classifier.scoreTextAgainstRule(tt.text, rule)
			gotMatch := score > 0

			if gotMatch != tt.wantMatch {
				t.Errorf("scoreTextAgainstRule() score=%f, gotMatch=%v, wantMatch=%v",
					score, gotMatch, tt.wantMatch)
			}
		})
	}
}

func TestTopicClassifier_Classify_AccentedKeywordsInTopicRule(t *testing.T) {
	t.Parallel()

	rules := []domain.ClassificationRule{
		{
			RuleName:      "indigenous_detection",
			RuleType:      domain.RuleTypeTopic,
			TopicName:     "indigenous",
			Keywords:      []string{"premières nations", "métis", "pueblos indígenas", "autochtone"},
			MinConfidence: 0.5,
			Enabled:       true,
			Priority:      100,
		},
	}

	tests := []struct {
		name      string
		title     string
		rawText   string
		wantTopic string
	}{
		{
			name:      "French accented content matches indigenous topic",
			title:     "Les Premières Nations du Québec",
			rawText:   "Les premières nations et les Métis se réunissent pour discuter des droits autochtone",
			wantTopic: "indigenous",
		},
		{
			name:      "Spanish accented content matches indigenous topic",
			title:     "Pueblos Indígenas de América",
			rawText:   "Los pueblos indígenas y métis celebran su herencia autochtone en una conferencia global",
			wantTopic: "indigenous",
		},
		{
			name:      "unaccented text does not match accented keywords",
			title:     "Premieres Nations Meeting",
			rawText:   "The premieres nations group held a meeting about community matters today",
			wantTopic: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tc := NewTopicClassifier(&mockLogger{}, rules, defaultMaxTopics)

			raw := &domain.RawContent{
				ID:      "test-accent-" + tt.name,
				Title:   tt.title,
				RawText: tt.rawText,
			}

			result, err := tc.Classify(context.Background(), raw)
			if err != nil {
				t.Fatalf("Classify() error = %v", err)
			}

			if tt.wantTopic == "" {
				if len(result.Topics) != 0 {
					t.Errorf("expected no topics, got %v", result.Topics)
				}

				return
			}

			if result.HighestTopic != tt.wantTopic {
				t.Errorf("HighestTopic = %q, want %q (topics: %v, scores: %v)",
					result.HighestTopic, tt.wantTopic, result.Topics, result.TopicScores)
			}
		})
	}
}

func TestTopicClassifier_Travel_MatchesGenuineTravelContent(t *testing.T) {
	t.Helper()

	rules := []domain.ClassificationRule{
		{
			RuleName:  "travel_detection",
			RuleType:  domain.RuleTypeTopic,
			TopicName: "travel",
			Keywords: []string{
				"vacation", "hotel", "flight", "tourism", "travel",
				"journey", "tour", "tourist",
				"resort", "airline", "airport", "luggage",
				"cruise", "beach", "sightseeing", "adventure", "backpacking",
				"travel guide", "itinerary", "booking", "reservation",
			},
			MinConfidence: 0.4,
			Enabled:       true,
		},
	}

	classifier := NewTopicClassifier(&mockLogger{}, rules, 5)

	raw := &domain.RawContent{
		ID:    "vacation-article",
		Title: "Best beach resorts for your summer vacation",
		RawText: "Planning your next vacation? Check out these amazing beach resorts. " +
			"Book your hotel and flight together for the best deals. " +
			"Tourism is booming at these resort destinations.",
	}

	result, err := classifier.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !slices.Contains(result.Topics, "travel") {
		t.Error("genuine travel article should be tagged as travel")
	}
}
