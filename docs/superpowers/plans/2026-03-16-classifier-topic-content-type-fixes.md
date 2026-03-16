# Classifier Topic & Content Type Fixes — Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix topic misclassification (drug_crime/travel false positives) and reduce event/article borderline confidence by adding multi-word keyword matching, updating keyword rules, and introducing the `article:event_report` subtype.

**Architecture:** Three independent changes: (1) scorer fix for multi-word keywords in `topic.go`, (2) SQL migration + seed data for drug_crime/travel rules, (3) new event_report heuristic + routing entry. All changes are in the classifier service. TDD throughout.

**Tech Stack:** Go 1.26+, PostgreSQL migrations, testify assertions

**Spec:** `docs/superpowers/specs/2026-03-16-classifier-topic-content-type-fixes-design.md`

---

## Chunk 1: Multi-word keyword matching in topic scorer

### Task 1: Add multi-word keyword matching tests

**Files:**
- Modify: `classifier/internal/classifier/topic_test.go`

- [ ] **Step 1: Write failing tests for multi-word keyword matching**

Add these tests after the existing `TestTopicClassifier_ScoreTextAgainstRule_*` tests (after line ~455):

```go
func TestTopicClassifier_ScoreTextAgainstRule_MultiWordKeyword(t *testing.T) {
	t.Helper()

	classifier := NewTopicClassifier(&mockLogger{}, nil)

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

	classifier := NewTopicClassifier(&mockLogger{}, nil)

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

func TestTopicClassifier_ScoreTextAgainstRule_MultiWordDoesNotSubstringMatch(t *testing.T) {
	t.Helper()

	classifier := NewTopicClassifier(&mockLogger{}, nil)

	// "drug bust" should NOT match "drug buster" — but with strings.Contains it will.
	// This test documents the accepted trade-off: substring matching for multi-word
	// keywords is consistent with event heuristic behavior and low-risk for our rules.
	rule := domain.ClassificationRule{
		Keywords: []string{"drug bust"},
	}

	// "drug bust" appears as substring of "drug buster" — this WILL match
	// (accepted trade-off per spec)
	text := "the drug buster team arrived"
	score := classifier.scoreTextAgainstRule(text, rule)

	// Document that this matches (accepted behavior)
	if score == 0.0 {
		t.Log("Note: multi-word substring matching did not trigger — if this test fails, " +
			"the implementation may have changed to exact phrase boundary matching (which is fine)")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd classifier && GOWORK=off go test ./internal/classifier/ -run "TestTopicClassifier_ScoreTextAgainstRule_MultiWord|TestTopicClassifier_ScoreTextAgainstRule_Mixed" -v`

Expected: FAIL — multi-word keywords return 0.0 because `wordFreq[keyword]` can't match phrases with spaces.

- [ ] **Step 3: Commit failing tests**

```bash
cd /home/fsd42/dev/north-cloud
git add classifier/internal/classifier/topic_test.go
git commit -m "test(classifier): add failing tests for multi-word keyword matching

Multi-word keywords like 'human trafficking' and 'drug bust' silently
return 0 matches because the scorer uses single-token wordFreq lookup.
These tests document the expected behavior after the fix."
```

### Task 2: Implement multi-word keyword matching in scoreTextAgainstRule

**Files:**
- Modify: `classifier/internal/classifier/topic.go:131-143`

- [ ] **Step 1: Update the keyword matching loop in scoreTextAgainstRule**

Replace the existing keyword loop body (lines ~131-143) in `scoreTextAgainstRule()`:

```go
	for _, keyword := range rule.Keywords {
		keyword = strings.ToLower(strings.TrimSpace(keyword))
		if keyword == "" {
			continue
		}

		if strings.Contains(keyword, " ") {
			// Multi-word: substring match on cleaned text
			if strings.Contains(text, keyword) {
				totalMatches++
				uniqueKeywordsMatched++
			}
		} else {
			// Single-word: exact token match via frequency map
			occurrences := wordFreq[keyword]
			if occurrences > 0 {
				totalMatches += occurrences
				uniqueKeywordsMatched++
			}
		}
	}
```

The old code to replace (lines ~131-143):

```go
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
```

- [ ] **Step 2: Apply the same fix to TestRule**

Replace the matching keyword loop body in `TestRule()` (lines ~277-288):

```go
	for _, keyword := range rule.Keywords {
		keyword = strings.ToLower(strings.TrimSpace(keyword))
		if keyword == "" {
			continue
		}

		if strings.Contains(keyword, " ") {
			// Multi-word: substring match on cleaned text
			if strings.Contains(text, keyword) {
				totalMatches++
				matchedKeywords = append(matchedKeywords, keyword)
			}
		} else {
			occurrences := wordFreq[keyword]
			if occurrences > 0 {
				totalMatches += occurrences
				matchedKeywords = append(matchedKeywords, keyword)
			}
		}
	}
```

The old code to replace (lines ~277-288):

```go
	for _, keyword := range rule.Keywords {
		keyword = strings.ToLower(strings.TrimSpace(keyword))
		if keyword == "" {
			continue
		}

		occurrences := wordFreq[keyword]
		if occurrences > 0 {
			totalMatches += occurrences
			matchedKeywords = append(matchedKeywords, keyword)
		}
	}
```

- [ ] **Step 3: Run tests to verify they pass**

Run: `cd classifier && GOWORK=off go test ./internal/classifier/ -run "TestTopicClassifier" -v`

Expected: ALL PASS — including the new multi-word tests and all existing tests.

- [ ] **Step 4: Run linter**

Run: `cd classifier && GOWORK=off golangci-lint run --config ../.golangci.yml ./internal/classifier/`

Expected: No new violations.

- [ ] **Step 5: Commit**

```bash
cd /home/fsd42/dev/north-cloud
git add classifier/internal/classifier/topic.go
git commit -m "fix(classifier): support multi-word keyword matching in topic scorer

Multi-word keywords (e.g. 'human trafficking', 'drug bust') were
silently ignored because the scorer used single-token wordFreq lookup.
Now detects space in keyword and falls back to strings.Contains on
cleaned text, matching the event heuristic's existing pattern."
```

### Task 3: Add TestRule multi-word test

**Files:**
- Modify: `classifier/internal/classifier/topic_test.go`

- [ ] **Step 1: Add a TestRule test for multi-word keywords**

Add after the existing tests:

```go
func TestTopicClassifier_TestRule_MultiWordKeywords(t *testing.T) {
	t.Helper()

	classifier := NewTopicClassifier(&mockLogger{}, nil)

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
```

- [ ] **Step 2: Run test**

Run: `cd classifier && GOWORK=off go test ./internal/classifier/ -run "TestTopicClassifier_TestRule_MultiWord" -v`

Expected: PASS

- [ ] **Step 3: Commit**

```bash
cd /home/fsd42/dev/north-cloud
git add classifier/internal/classifier/topic_test.go
git commit -m "test(classifier): add TestRule coverage for multi-word keywords"
```

---

## Chunk 2: Migration and seed data for drug_crime / travel keyword fixes

### Task 4: Write migration 013 up

**Files:**
- Create: `classifier/migrations/013_fix_topic_keywords.up.sql`

- [ ] **Step 1: Create the up migration**

```sql
-- Migration 013: Fix topic keyword rules
-- 1. drug_crime: replace generic "trafficking" with drug-specific compound terms
-- 2. travel: remove ambiguous soft keywords (destination, trip, visa, passport)
-- See: docs/superpowers/specs/2026-03-16-classifier-topic-content-type-fixes-design.md

BEGIN;

-- Fix drug_crime: remove generic "trafficking", add drug-specific compound terms
UPDATE classification_rules
SET keywords = ARRAY[
    'drug', 'drugs', 'narcotics', 'dealer', 'possession',
    'cocaine', 'heroin', 'fentanyl', 'methamphetamine', 'meth', 'marijuana', 'cannabis', 'opioid',
    'drug bust', 'drug ring', 'cartel', 'smuggling', 'drug trafficking',
    'narcotics trafficking', 'fentanyl trafficking', 'cocaine trafficking', 'meth trafficking',
    'overdose', 'drug-related', 'controlled substance'
],
    updated_at = CURRENT_TIMESTAMP
WHERE rule_name = 'drug_crime_detection';

-- Fix travel: remove ambiguous soft keywords
UPDATE classification_rules
SET keywords = ARRAY[
    'vacation', 'hotel', 'flight', 'tourism', 'travel',
    'journey', 'tour', 'tourist',
    'resort', 'airline', 'airport', 'luggage',
    'cruise', 'beach', 'sightseeing', 'adventure', 'backpacking',
    'travel guide', 'itinerary', 'booking', 'reservation'
],
    updated_at = CURRENT_TIMESTAMP
WHERE rule_name = 'travel_detection';

COMMIT;
```

- [ ] **Step 2: Commit**

```bash
cd /home/fsd42/dev/north-cloud
git add classifier/migrations/013_fix_topic_keywords.up.sql
git commit -m "fix(classifier): migration 013 — fix drug_crime and travel keyword rules

Remove generic 'trafficking' from drug_crime (caused false positives on
sex/human trafficking stories). Add drug-specific compounds instead.
Remove ambiguous travel keywords (destination, trip, visa, passport)
that appeared in crime/immigration context."
```

### Task 5: Write migration 013 down

**Files:**
- Create: `classifier/migrations/013_fix_topic_keywords.down.sql`

- [ ] **Step 1: Create the down migration**

Restore original keyword arrays from migrations 005 and 007:

```sql
-- Rollback migration 013: restore original keyword arrays

BEGIN;

-- Restore original drug_crime keywords (from migration 007)
UPDATE classification_rules
SET keywords = ARRAY[
    'drug', 'drugs', 'narcotics', 'trafficking', 'dealer', 'possession',
    'cocaine', 'heroin', 'fentanyl', 'methamphetamine', 'meth', 'marijuana', 'cannabis', 'opioid',
    'drug bust', 'drug ring', 'cartel', 'smuggling', 'drug trafficking',
    'overdose', 'drug-related', 'controlled substance'
],
    updated_at = CURRENT_TIMESTAMP
WHERE rule_name = 'drug_crime_detection';

-- Restore original travel keywords (from migration 005)
UPDATE classification_rules
SET keywords = ARRAY[
    'trip', 'vacation', 'hotel', 'flight', 'destination', 'tourism', 'travel',
    'travel', 'trip', 'vacation', 'journey', 'tour', 'tourist', 'destination',
    'hotel', 'resort', 'flight', 'airline', 'airport', 'luggage', 'passport',
    'visa', 'cruise', 'beach', 'sightseeing', 'adventure', 'backpacking',
    'tourism', 'travel guide', 'itinerary', 'booking', 'reservation'
],
    updated_at = CURRENT_TIMESTAMP
WHERE rule_name = 'travel_detection';

COMMIT;
```

- [ ] **Step 2: Commit**

```bash
cd /home/fsd42/dev/north-cloud
git add classifier/migrations/013_fix_topic_keywords.down.sql
git commit -m "fix(classifier): migration 013 down — restore original keyword arrays"
```

### Task 6: Update seed data in migration 005 and 007

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/classifier/migrations/005_add_comprehensive_categories.up.sql:73-79`
- Modify: `/home/fsd42/dev/north-cloud/classifier/migrations/007_add_crime_subcategories.up.sql:97-106`

- [ ] **Step 1: Update travel keywords in migration 005**

Replace the travel_detection INSERT keywords (lines 73-79) with the fixed list:

Old:
```sql
    ('travel_detection', 'topic', 'travel', ARRAY[
        'trip', 'vacation', 'hotel', 'flight', 'destination', 'tourism', 'travel',
        'travel', 'trip', 'vacation', 'journey', 'tour', 'tourist', 'destination',
        'hotel', 'resort', 'flight', 'airline', 'airport', 'luggage', 'passport',
        'visa', 'cruise', 'beach', 'sightseeing', 'adventure', 'backpacking',
        'tourism', 'travel guide', 'itinerary', 'booking', 'reservation'
    ], 0.4, 5, TRUE),
```

New:
```sql
    ('travel_detection', 'topic', 'travel', ARRAY[
        'vacation', 'hotel', 'flight', 'tourism', 'travel',
        'journey', 'tour', 'tourist',
        'resort', 'airline', 'airport', 'luggage',
        'cruise', 'beach', 'sightseeing', 'adventure', 'backpacking',
        'travel guide', 'itinerary', 'booking', 'reservation'
    ], 0.4, 5, TRUE),
```

- [ ] **Step 2: Update drug_crime keywords in migration 007**

Replace the drug_crime_detection INSERT keywords (lines 97-106) with the fixed list:

Old:
```sql
    ARRAY[
        -- Core drug crime
        'drug', 'drugs', 'narcotics', 'trafficking', 'dealer', 'possession',
        -- Specific substances
        'cocaine', 'heroin', 'fentanyl', 'methamphetamine', 'meth', 'marijuana', 'cannabis', 'opioid',
        -- Operations
        'drug bust', 'drug ring', 'cartel', 'smuggling', 'drug trafficking',
        -- Related
        'overdose', 'drug-related', 'controlled substance'
    ],
```

New:
```sql
    ARRAY[
        -- Core drug crime
        'drug', 'drugs', 'narcotics', 'dealer', 'possession',
        -- Specific substances
        'cocaine', 'heroin', 'fentanyl', 'methamphetamine', 'meth', 'marijuana', 'cannabis', 'opioid',
        -- Operations
        'drug bust', 'drug ring', 'cartel', 'smuggling', 'drug trafficking',
        'narcotics trafficking', 'fentanyl trafficking', 'cocaine trafficking', 'meth trafficking',
        -- Related
        'overdose', 'drug-related', 'controlled substance'
    ],
```

- [ ] **Step 3: Commit**

```bash
cd /home/fsd42/dev/north-cloud
git add classifier/migrations/005_add_comprehensive_categories.up.sql classifier/migrations/007_add_crime_subcategories.up.sql
git commit -m "fix(classifier): update seed data for drug_crime and travel keywords

Mirror migration 013 changes in seed migrations so new environments
start with correct rules. Prevents environment drift."
```

### Task 7: Add topic classification regression tests

**Files:**
- Modify: `classifier/internal/classifier/topic_test.go`

- [ ] **Step 1: Add regression tests for drug_crime and travel false positives**

```go
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

	classifier := NewTopicClassifier(&mockLogger{}, rules)

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

	for _, topic := range result.Topics {
		if topic == "drug_crime" {
			t.Error("sex trafficking article should NOT be tagged as drug_crime")
		}
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

	classifier := NewTopicClassifier(&mockLogger{}, rules)

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

	found := false
	for _, topic := range result.Topics {
		if topic == "drug_crime" {
			found = true
			break
		}
	}

	if !found {
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

	classifier := NewTopicClassifier(&mockLogger{}, rules)

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

	for _, topic := range result.Topics {
		if topic == "travel" {
			t.Error("trafficking context article should NOT be tagged as travel")
		}
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

	classifier := NewTopicClassifier(&mockLogger{}, rules)

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

	found := false
	for _, topic := range result.Topics {
		if topic == "travel" {
			found = true
			break
		}
	}

	if !found {
		t.Error("genuine travel article should be tagged as travel")
	}
}
```

- [ ] **Step 2: Run regression tests**

Run: `cd classifier && GOWORK=off go test ./internal/classifier/ -run "TestTopicClassifier_DrugCrime|TestTopicClassifier_Travel" -v`

Expected: ALL PASS

- [ ] **Step 3: Commit**

```bash
cd /home/fsd42/dev/north-cloud
git add classifier/internal/classifier/topic_test.go
git commit -m "test(classifier): add regression tests for drug_crime and travel false positives

Tests verify: sex trafficking does NOT trigger drug_crime, fentanyl
trafficking DOES, trafficking context does NOT trigger travel, genuine
vacation content DOES. Based on AI observer insights."
```

---

## Chunk 3: Event report subtype and routing

### Task 8: Add ContentSubtypeEventReport constant

**Files:**
- Modify: `classifier/internal/domain/classification.go:227`

- [ ] **Step 1: Add the constant**

After line 227 (`ContentSubtypeCompanyAnnouncement = "company_announcement"`), add:

```go
	ContentSubtypeEventReport         = "event_report"
```

- [ ] **Step 2: Verify it compiles**

Run: `cd classifier && GOWORK=off go build ./...`

Expected: Build succeeds.

- [ ] **Step 3: Commit**

```bash
cd /home/fsd42/dev/north-cloud
git add classifier/internal/domain/classification.go
git commit -m "feat(classifier): add ContentSubtypeEventReport constant"
```

### Task 9: Add event_report heuristic tests

**Files:**
- Modify: `classifier/internal/classifier/content_type_event_heuristic_test.go`

- [ ] **Step 1: Write failing tests for event_report detection**

Add after the existing tests:

```go
func TestClassifyFromEventKeywords_EventReport_ScheduledFor(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})

	raw := &domain.RawContent{
		ID:      "test-event-report-scheduled",
		Title:   "Annual Music Festival Returns to Sudbury",
		RawText: "The popular music festival is scheduled for next weekend at the waterfront park.",
	}

	result := c.classifyFromEventKeywords(raw)
	require.NotNil(t, result)
	assert.Equal(t, domain.ContentTypeArticle, result.Type)
	assert.Equal(t, domain.ContentSubtypeEventReport, result.Subtype)
	assert.Equal(t, "event_report_heuristic", result.Method)
	assert.InDelta(t, keywordHeuristicConfidence, result.Confidence, 0.001)
}

func TestClassifyFromEventKeywords_EventReport_WillTakePlace(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})

	raw := &domain.RawContent{
		ID:      "test-event-report-takeplace",
		Title:   "Protest March Planned for Downtown",
		RawText: "The demonstration will take place Saturday morning starting at city hall.",
	}

	result := c.classifyFromEventKeywords(raw)
	require.NotNil(t, result)
	assert.Equal(t, domain.ContentTypeArticle, result.Type)
	assert.Equal(t, domain.ContentSubtypeEventReport, result.Subtype)
}

func TestClassifyFromEventKeywords_EventReport_DoesNotOverrideEvent(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})

	// This has 2+ event keywords, so it should be classified as event, not event_report
	raw := &domain.RawContent{
		ID:      "test-event-not-report",
		Title:   "Register Now for the Festival",
		RawText: "Tickets available at the door. The event is scheduled for Saturday.",
	}

	result := c.classifyFromEventKeywords(raw)
	require.NotNil(t, result)
	assert.Equal(t, domain.ContentTypeEvent, result.Type)
}

func TestClassifyFromEventKeywords_EventReport_NoSignal_ReturnsNil(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})

	raw := &domain.RawContent{
		ID:      "test-no-event-report",
		Title:   "City Council Approves New Budget",
		RawText: "The council voted unanimously to approve the annual budget for the city.",
	}

	result := c.classifyFromEventKeywords(raw)
	assert.Nil(t, result)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd classifier && GOWORK=off go test ./internal/classifier/ -run "TestClassifyFromEventKeywords_EventReport" -v`

Expected: FAIL — `classifyFromEventKeywords` doesn't have the event_report path yet. Tests will fail on nil result or wrong type.

**Note:** If `ContentTypeResult` does not have a `Subtype` field yet, the tests will fail to compile. Check the struct definition first. If `Subtype` is missing, add it to `ContentTypeResult` before the tests:

```go
// In content_type.go, add Subtype to ContentTypeResult struct:
type ContentTypeResult struct {
    Type       string
    Subtype    string  // Add this field
    Confidence float64
    Method     string
    Reason     string
}
```

- [ ] **Step 3: Commit failing tests**

```bash
cd /home/fsd42/dev/north-cloud
git add classifier/internal/classifier/content_type_event_heuristic_test.go
git commit -m "test(classifier): add failing tests for event_report heuristic

Tests for article:event_report subtype detection via event coverage
phrases like 'scheduled for' and 'will take place'."
```

### Task 10: Implement event_report heuristic

**Files:**
- Modify: `classifier/internal/classifier/content_type_event_heuristic.go`
- Possibly modify: `classifier/internal/classifier/content_type.go` (if `ContentTypeResult.Subtype` field is needed)

- [ ] **Step 1: Add Subtype field to ContentTypeResult if not present**

Check `content_type.go` for the `ContentTypeResult` struct. If it doesn't have a `Subtype` field, add one. The field should be used in `classifyFromEventKeywords` and propagated to `ClassificationResult.ContentSubtype` in the main orchestrator.

- [ ] **Step 2: Add event report phrases and detection function**

In `content_type_event_heuristic.go`, add after the `hasLocationSignal` function:

```go
// eventReportPhrases are linguistic patterns that indicate news coverage
// of an event (as opposed to an event listing). Only 1 match is required
// because these phrases are specific and low-ambiguity.
var eventReportPhrases = []string{
	"scheduled for",
	"will take place",
	"lineup announced",
	"set to perform",
	"protest planned",
	"hearing set for",
	"festival announced",
	"tournament begins",
}

// matchEventReport checks for event coverage signals that indicate the
// content is a news article about an event, not an event listing itself.
// Returns article with event_report subtype, or nil if no signal found.
func (c *ContentTypeClassifier) matchEventReport(
	raw *domain.RawContent, text string,
) *ContentTypeResult {
	for _, phrase := range eventReportPhrases {
		if strings.Contains(text, phrase) {
			c.logger.Debug("Event report detected via coverage phrase",
				infralogger.String("content_id", raw.ID),
				infralogger.String("phrase", phrase),
			)
			return &ContentTypeResult{
				Type:       domain.ContentTypeArticle,
				Subtype:    domain.ContentSubtypeEventReport,
				Confidence: keywordHeuristicConfidence,
				Method:     "event_report_heuristic",
				Reason:     "Event coverage phrase detected in content",
			}
		}
	}
	return nil
}
```

- [ ] **Step 3: Wire into classifyFromEventKeywords**

Update `classifyFromEventKeywords()` to add the third path:

```go
func (c *ContentTypeClassifier) classifyFromEventKeywords(
	raw *domain.RawContent,
) *ContentTypeResult {
	combinedText := strings.ToLower(raw.Title + " " + raw.RawText)

	// Path 1: keyword counting (returns event)
	if result := c.matchEventKeywords(raw, combinedText); result != nil {
		return result
	}

	// Path 2: date + location heuristic (returns event)
	if result := c.matchDateLocation(raw, combinedText); result != nil {
		return result
	}

	// Path 3: event coverage phrases (returns article:event_report)
	return c.matchEventReport(raw, combinedText)
}
```

- [ ] **Step 4: Ensure Subtype is propagated in the orchestrator**

In `classifier.go`, verify that `ContentTypeResult.Subtype` is used when building the `ClassificationResult`. Look for where `contentResult.Type` is assigned and ensure `contentResult.Subtype` is also assigned to `result.ContentSubtype`.

- [ ] **Step 5: Run tests**

Run: `cd classifier && GOWORK=off go test ./internal/classifier/ -run "TestClassifyFromEventKeywords" -v`

Expected: ALL PASS — including new event_report tests and all existing event tests.

- [ ] **Step 6: Run full test suite**

Run: `cd classifier && GOWORK=off go test ./...`

Expected: ALL PASS

- [ ] **Step 7: Run linter**

Run: `cd classifier && GOWORK=off golangci-lint run --config ../.golangci.yml ./...`

Expected: No violations.

- [ ] **Step 8: Commit**

```bash
cd /home/fsd42/dev/north-cloud
git add classifier/internal/classifier/content_type_event_heuristic.go classifier/internal/classifier/content_type.go classifier/internal/classifier/classifier.go
git commit -m "feat(classifier): add article:event_report subtype detection

News articles about events (concerts, protests, court dates) now
classify as article:event_report instead of falling through to generic
article with low confidence. Uses specific coverage phrases like
'scheduled for' and 'will take place'."
```

### Task 11: Add event_report routing table entry

**Files:**
- Modify: `classifier/internal/config/config.go:405`
- Modify: `classifier/internal/classifier/classifier_routing_test.go`

- [ ] **Step 1: Write failing routing test**

Add to `classifier_routing_test.go`:

```go
func TestResolveSidecars_EventReport_LocationOnly(t *testing.T) {
	t.Helper()

	rec := &recordingLogger{}
	cfg := Config{
		RoutingTable: map[string][]string{
			"article":              {"crime", "mining", "coforge", "entertainment", "indigenous", "location"},
			"article:event":        {"location"},
			"article:event_report": {"location"},
			"article:blotter":      {"crime"},
			"article:report":       {},
		},
	}
	clf := NewClassifier(rec, nil, nil, nil, cfg)

	sidecars := clf.ResolveSidecars(domain.ContentTypeArticle, domain.ContentSubtypeEventReport)

	if len(sidecars) != 1 || sidecars[0] != "location" {
		t.Errorf("expected [location], got %v", sidecars)
	}
}
```

- [ ] **Step 2: Run test to verify it passes** (routing table is passed in config, so this should pass immediately)

Run: `cd classifier && GOWORK=off go test ./internal/classifier/ -run "TestResolveSidecars_EventReport" -v`

Expected: PASS

- [ ] **Step 3: Add event_report to default routing**

In `config.go`, update `getDefaultRouting()` — add after the `"article:event"` entry:

```go
"article:event_report": {"location"},
```

- [ ] **Step 4: Run full test suite**

Run: `cd classifier && GOWORK=off go test ./...`

Expected: ALL PASS

- [ ] **Step 5: Run linter**

Run: `cd classifier && GOWORK=off golangci-lint run --config ../.golangci.yml ./...`

Expected: No violations.

- [ ] **Step 6: Commit**

```bash
cd /home/fsd42/dev/north-cloud
git add classifier/internal/config/config.go classifier/internal/classifier/classifier_routing_test.go
git commit -m "feat(classifier): add event_report routing — location sidecar only

Event report articles route to location classifier only, same as
event listings. Prevents unnecessary crime/mining/etc. classification
on event coverage content."
```

---

## Chunk 4: Final verification and GitHub issue

### Task 12: Full test suite and lint verification

**Files:** None (verification only)

- [ ] **Step 1: Run all classifier tests**

Run: `cd classifier && GOWORK=off go test ./... -count=1`

Expected: ALL PASS

- [ ] **Step 2: Run linter with cache bypass**

Run: `cd /home/fsd42/dev/north-cloud && task lint:force`

Expected: No violations across any service.

- [ ] **Step 3: Verify no regressions in existing tests**

Run: `cd /home/fsd42/dev/north-cloud && task test`

Expected: ALL PASS

### Task 13: Before/after comparison for multi-word keyword activation

This task mitigates the global behavior change risk identified in the spec. Fixing multi-word matching activates ~30+ previously-silent keywords across ALL topic rules.

**Files:** None (analysis only)

- [ ] **Step 1: Sample recent classified content from production**

Run against production ES to get 50 recently classified documents with their current topics:

```bash
ssh jones@northcloud.one "docker exec north-cloud-elasticsearch-1 curl -s 'http://localhost:9200/*_classified_content/_search' -H 'Content-Type: application/json' -d '{\"size\":50,\"sort\":[{\"classified_at\":\"desc\"}],\"_source\":[\"title\",\"topics\",\"source_name\",\"content_type\",\"classification_confidence\"]}'"
```

Save the output as the "before" baseline.

- [ ] **Step 2: Run TestRule diagnostic against sampled content**

Using the classifier's `TestRule()` API or a local test, evaluate the sampled documents against all topic rules with both the old scorer (single-word only) and the new scorer (multi-word enabled). Compare which topics each document would gain or lose.

Focus on documents that would gain new topics due to multi-word keywords like `"breaking news"`, `"climate change"`, `"real estate"`, `"domestic violence"`, `"gang violence"`, etc.

- [ ] **Step 3: Review the delta**

Check whether any documents cross a threshold they shouldn't. If the delta is clean (only intended improvements), proceed. If unexpected topics appear, adjust keyword rules before deploying.

- [ ] **Step 4: Document the comparison results**

Add a brief summary of findings to the PR description so reviewers can see the impact scope.

### Task 14: Create GitHub issue for topic rule system redesign

**Files:** None (GitHub issue)

- [ ] **Step 1: Create the issue**

```bash
gh issue create --title "Redesign topic rule system for maintainability & expressiveness" --body "$(cat <<'EOF'
## Problem

The current topic classification system relies on simple keyword arrays stored in Postgres. This worked initially but has reached its expressiveness limits.

### Symptoms discovered during AI observer review (2026-03-16)
- Multi-word keywords silently fail (tokenizer uses single-token lookup)
- Ambiguous keywords cause cross-topic false positives (e.g., "trafficking" in drug_crime)
- No ability to express co-occurrence, weighted logic, or exclusions
- Rules are opaque and hard to audit
- Migrations required for every rule change
- No validation or linting of rule quality
- No versioning or rollback mechanism

### Impact
- Topic misclassification affecting downstream routing
- Low confidence scores flagged by AI observer
- Increasing maintenance burden as topic count grows

## Proposed Direction

Move from flat keyword lists to a structured rule model:

1. **Rule metadata** — type (single/phrase/cooccurrence/negative), weight, requires, excludes
2. **Rule versioning** — canonical rules in code, migrations for production deltas, checksum drift detection
3. **Startup validation** — lint rules on load, warn on ambiguous/overlapping keywords
4. **Optional YAML/JSON definitions** — easier to review, diff, and test
5. **Rule test harness** — sample texts, show which rules fire, catch regressions pre-deploy

## Context

Phase 1 fixes (migration 013 + scorer multi-word support) address the immediate symptoms. This issue tracks the structural redesign.

See: `docs/superpowers/specs/2026-03-16-classifier-topic-content-type-fixes-design.md`
EOF
)" --label "enhancement"
```

- [ ] **Step 2: Note the issue number in the spec's Future Work section**
