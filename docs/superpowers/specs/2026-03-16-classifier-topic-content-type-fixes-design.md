# Classifier Topic & Content Type Fixes

**Date**: 2026-03-16
**Status**: Approved
**Scope**: classifier service (topic scorer, keyword rules, content type heuristics)

## Problem Statement

AI observer insights (1,036 total, latest 2026-03-16) reveal two distinct classifier problems:

1. **Topic misclassification** — `drug_crime` and `travel` topics are assigned to unrelated content due to ambiguous keywords. A sex trafficking conviction story was tagged `drug_crime` + `travel`.
2. **Event/article borderline confidence** — Sources like Battlefords News-Optimist (75-80% borderline), Asia Pacific Report (53.8% borderline, 8x baseline), and Toronto CityNews (42.9% borderline) show low confidence in event vs article classification.

### Root Causes

| Issue | Root Cause | Location |
|-------|-----------|----------|
| `drug_crime` false positives | Generic `"trafficking"` keyword matches sex/human/arms trafficking | Migration 007, line 99 |
| `travel` false positives | Ambiguous keywords (`destination`, `trip`, `visa`, `passport`) appear in crime/immigration context | Migration 005, lines 73-79 |
| Multi-word keywords never match | Scorer uses single-token `wordFreq[keyword]` lookup; multi-word keywords silently fail | `topic.go` lines 131-142 |
| Event/article borderline | Event heuristic requires 2+ event keywords ("register now", "tickets available") that news-about-events never contains | `content_type_event_heuristic.go` |

## Decisions

These were discussed and agreed during brainstorming:

1. **Delivery method**: C) Both SQL migration + updated seed data — migration fixes production, seed data fixes new environments, prevents drift.
2. **Event/article fix**: B) Add `event_report` as a subtype of `article` — fits existing subtype pattern, no ES/routing/consumer changes needed.
3. **Travel keyword fix**: A) Trim the keyword list — remove ambiguous words entirely. No scoring engine co-occurrence logic needed.

## Design

### Phase 1: Fix Topic Misclassification

#### 1A. Fix multi-word keyword matching in scorer

**File**: `classifier/internal/classifier/topic.go`

**Problem**: `scoreTextAgainstRule()` (lines 131-142) does exact single-token lookup via `wordFreq[keyword]`. Any keyword containing a space (e.g., `"human trafficking"`, `"drug bust"`, `"gang violence"`) silently returns 0 matches. This affects all existing rules from migrations 001, 005, and 007 — not just the new drug_crime fix.

**Fix**: In the keyword matching loop (which starts at line ~131 and uses existing variables `totalMatches` and `uniqueKeywordsMatched`), detect multi-word keywords (contains a space) and fall back to `strings.Contains()` on the cleaned text. Single-word keywords continue using the O(1) `wordFreq` lookup. This modifies the existing loop body — no new variables are introduced.

```go
// Replace the existing keyword loop body in scoreTextAgainstRule() (~lines 131-143):
for _, keyword := range rule.Keywords {
    keyword = strings.ToLower(strings.TrimSpace(keyword))
    if keyword == "" {
        continue
    }

    if strings.Contains(keyword, " ") {
        // Multi-word: substring match on cleaned text.
        // Uses strings.Contains (not token lookup) since multi-word
        // phrases span token boundaries. This is consistent with the
        // event heuristic (content_type_event_heuristic.go line 80).
        //
        // Trade-off: substring matching means "drug bust" matches
        // inside "drug buster". This is accepted as low-risk for the
        // compound terms in our rules, and matches existing behavior
        // in the event heuristic.
        //
        // Scoring note: multi-word keywords contribute 1 to totalMatches
        // regardless of how many times the phrase appears. This is
        // intentional — multi-word phrases are high-signal, and counting
        // them once avoids inflating TF for repeated phrases.
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

**Rationale**: Matches the existing pattern in `content_type_event_heuristic.go` line 80 which uses `strings.Contains()` for multi-word event keywords. Preserves log-TF scoring semantics for single-word keywords.

**Impact**: The same fix must be applied to `TestRule()` (~lines 277-288) which duplicates the scoring loop.

**Global behavior change**: Fixing multi-word matching will activate ~30+ previously-silent multi-word keywords across ALL topic rules (migrations 001, 005, 007), not just drug_crime and travel. Examples: `"human trafficking"`, `"gang violence"`, `"domestic violence"`, `"breaking news"`, `"climate change"`, `"real estate"`, etc. This is a net improvement — these keywords were always intended to match — but the deployment plan must include before/after scoring comparison to catch any unexpected threshold crossings.

#### 1B. Fix `drug_crime` keyword rule

**Migration 013** updates `drug_crime_detection` rule:

- **Remove**: `"trafficking"` (generic — matches sex trafficking, human trafficking, arms trafficking)
- **Add**: `"drug trafficking"`, `"narcotics trafficking"`, `"fentanyl trafficking"`, `"cocaine trafficking"`, `"meth trafficking"`

These compound terms are unambiguous and will now match correctly thanks to the multi-word scorer fix.

**Existing `organized_crime` rule** already has `"human trafficking"` and `"trafficking ring"` — no changes needed there.

#### 1C. Trim `travel` keyword rule

**Migration 013** updates `travel_detection` rule:

- **Remove**: `destination`, `trip`, `visa`, `passport` (appear in crime/immigration context)
- **Keep**: `vacation`, `hotel`, `flight`, `tourism`, `travel`, `journey`, `tour`, `tourist`, `resort`, `airline`, `airport`, `luggage`, `cruise`, `beach`, `sightseeing`, `adventure`, `backpacking`, `travel guide`, `itinerary`, `booking`, `reservation`

All retained terms are high-signal travel indicators. Multi-word terms (`travel guide`) benefit from the scorer fix in 1A.

### Phase 2: Add `article:event_report` Subtype

#### 2A. New content subtype constant

**File**: `classifier/internal/domain/classification.go` (after line 227)

```go
ContentSubtypeEventReport = "event_report"
```

#### 2B. New event report heuristic

**File**: `classifier/internal/classifier/content_type_event_heuristic.go`

Add a third detection path in `classifyFromEventKeywords()`. After the keyword path and date+location path both return nil, check for event coverage signals:

**Detection phrases** (1+ match required):
- `"scheduled for"`
- `"will take place"`
- `"lineup announced"`
- `"set to perform"`
- `"protest planned"`
- `"hearing set for"`
- `"festival announced"`
- `"tournament begins"`

**Returns**:
- `ContentType`: `article`
- `ContentSubtype`: `event_report`
- `Confidence`: 0.80
- `Method`: `"event_report_heuristic"`

These phrases are specific enough to avoid false positives. They capture "coverage of an event" vs "an event listing itself."

#### 2C. Routing table entry

**File**: `classifier/internal/config/config.go` `getDefaultRouting()` (after line 405)

```go
"article:event_report": {"location"},
```

Same routing as `article:event` — location classifier only. Event reports need location extraction but not crime/mining/entertainment/etc. sidecars.

### What Does NOT Change

- No ES mapping changes (subtype is already a string field)
- No publisher routing changes (publisher routes on topics, not subtypes)
- No consumer changes
- No new top-level content types
- `ResolveSidecars()` fallback still works (article:event_report → falls back to article if not in table)

## Migration Strategy

### Migration 013: `013_fix_topic_keywords.up.sql`

Single migration with two UPDATEs:

```sql
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

### Migration 013 down: `013_fix_topic_keywords.down.sql`

Restores original keyword arrays from migrations 005 and 007.

### Seed data update

Mirror the migration changes in migration 001/005/007 seed data (or wherever the canonical seed is) so new environments start with correct rules.

## Testing Strategy

### Unit tests

1. **Multi-word keyword matching** — verify `scoreTextAgainstRule()` matches `"drug trafficking"`, `"human trafficking"`, `"travel guide"` etc.
2. **drug_crime rule** — verify "sex trafficking" article does NOT match drug_crime, but "fentanyl trafficking ring busted" DOES.
3. **travel rule** — verify "trafficking destination country" does NOT match travel, but "vacation resort beach cruise" DOES.
4. **event_report heuristic** — verify "Concert scheduled for Saturday at the arena" returns `article:event_report`, but "Register now for tickets" returns `event`.
5. **Routing** — verify `ResolveSidecars("article", "event_report")` returns `["location"]`.

### Regression validation

Use AI observer's flagged sources as test fixtures:
- Battlefords News-Optimist content
- Asia Pacific Report event classifications
- Toronto CityNews borderline cases

## Risks

### Multi-word keyword activation is a global behavior change

The scorer fix (1A) will activate ~30+ multi-word keywords that have been silently failing since their introduction. This affects ALL topic rules, not just drug_crime and travel. While this is a net improvement (these keywords were always intended to match), it will change topic scores and potentially cross thresholds for documents that previously didn't match.

**Mitigation**: Before deploying, run the existing `TestRule()` diagnostic against a sample of recent classified content to produce a before/after comparison. Flag any documents that gain or lose topics. Review the delta before proceeding.

### Substring matching for multi-word keywords

`strings.Contains()` for multi-word keywords means `"drug bust"` matches inside `"drug buster"`. This is accepted as low-risk for the compound terms in our rules (most are 2+ word phrases that rarely appear as substrings of other words) and is consistent with the event heuristic's existing approach.

### Rules cached at startup

All keyword changes require a classifier restart to take effect. The migration runs on startup, so a deploy covers both. But if migration 013 is applied without a restart (e.g., manual SQL), rules won't reload.

## Deployment

1. Deploy migration 013 (runs automatically on startup)
2. Deploy classifier with scorer fix + event_report heuristic + routing update
3. Classifier restarts → loads updated rules from Postgres
4. Monitor AI observer insights for 24-48h to confirm:
   - drug_crime false positive rate drops
   - travel false positive rate drops
   - event/article borderline rate decreases for flagged sources

## Future Work

The current keyword rule system has reached its expressiveness limits. A GitHub issue will be created to track a longer-term redesign covering:

- Structured rule model (type, weight, requires, excludes fields)
- Rule versioning and drift detection
- Startup validation and linting of rules
- Optional YAML/JSON rule definitions
- Rule test harness for pre-deploy validation

This is tracked separately because the Phase 1/2 fixes are correct and sufficient for the current problems — the rule system redesign is a classifier evolution, not a bug fix.
