# Crime Pipeline Precision Fix — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix the crime content pipeline so only genuine street-crime events reach Streetcode.net, by fixing publisher field mapping, tightening classifier rules, and hardening the Streetcode subscriber.

**Architecture:** Three-phase fix across two repos. Phase 1 fixes the publisher's Article struct to unmarshal nested ES fields (following the existing Mining pattern). Phase 2 adds exclusion patterns and tightens `core_street_crime` requirements in the classifier. Phase 3 hardens the Streetcode Laravel subscriber with quality thresholds, crime-relevance validation, and filtered tag assignment.

**Tech Stack:** Go 1.25+ (publisher, classifier), PHP/Laravel (streetcode-laravel), Elasticsearch, Redis Pub/Sub

**Design Doc:** `docs/plans/2026-02-05-crime-pipeline-precision-design.md`

---

## Phase 1: Fix Publisher Field Mapping

The classifier stores crime data nested under `crime.street_crime_relevance` in ES, but the publisher's `Article` struct has flat `json:"crime_relevance"` tags. Go's `json.Unmarshal` silently leaves these as zero values. The fix follows the same pattern already used for `Mining *MiningData`.

---

### Task 1: Add CrimeData and LocationData nested structs to Article

**Files:**
- Modify: `publisher/internal/router/service.go` (lines 213-266, Article struct)

**Step 1: Write the failing test**

Create a test that unmarshals a realistic ES document (with nested `crime` and `location` objects) into an Article and asserts the nested fields are populated.

File: `publisher/internal/router/service_test.go`

```go
func TestArticle_UnmarshalNestedCrimeFields(t *testing.T) {
	t.Helper() // Not needed in test function, but keep for consistency

	esJSON := `{
		"title": "Man charged with murder in downtown Vancouver",
		"body": "A man has been charged...",
		"canonical_url": "https://example.com/article",
		"source": "example_com",
		"quality_score": 75,
		"content_type": "article",
		"is_crime_related": true,
		"crime": {
			"street_crime_relevance": "core_street_crime",
			"sub_label": "",
			"crime_types": ["violent_crime"],
			"location_specificity": "local_canada",
			"final_confidence": 0.95,
			"homepage_eligible": true,
			"category_pages": ["violent-crime", "crime"],
			"review_required": false
		},
		"location": {
			"city": "vancouver",
			"province": "BC",
			"country": "canada",
			"specificity": "city",
			"confidence": 0.85
		}
	}`

	var article Article
	err := json.Unmarshal([]byte(esJSON), &article)
	require.NoError(t, err)

	// Nested structs populated
	require.NotNil(t, article.Crime)
	assert.Equal(t, "core_street_crime", article.Crime.Relevance)
	assert.True(t, article.Crime.Homepage)
	assert.Equal(t, []string{"violent-crime", "crime"}, article.Crime.Categories)

	require.NotNil(t, article.Location)
	assert.Equal(t, "vancouver", article.Location.City)
	assert.Equal(t, "BC", article.Location.Province)
	assert.Equal(t, "canada", article.Location.Country)
}
```

Add `"encoding/json"` to the test file imports if not already present.

**Step 2: Run test to verify it fails**

Run: `cd publisher && go test ./internal/router/ -run TestArticle_UnmarshalNestedCrimeFields -v`
Expected: FAIL — `Article` has no `Crime` or `Location` field.

**Step 3: Add nested structs to Article**

File: `publisher/internal/router/service.go`

Add these structs near the existing `MiningData` struct:

```go
// CrimeData matches the classifier's nested crime object in Elasticsearch.
type CrimeData struct {
	Relevance      string   `json:"street_crime_relevance"`
	SubLabel       string   `json:"sub_label,omitempty"`
	CrimeTypes     []string `json:"crime_types"`
	Specificity    string   `json:"location_specificity"`
	Confidence     float64  `json:"final_confidence"`
	Homepage       bool     `json:"homepage_eligible"`
	Categories     []string `json:"category_pages"`
	ReviewRequired bool     `json:"review_required"`
}

// LocationData matches the classifier's nested location object in Elasticsearch.
type LocationData struct {
	City        string  `json:"city,omitempty"`
	Province    string  `json:"province,omitempty"`
	Country     string  `json:"country"`
	Specificity string  `json:"specificity"`
	Confidence  float64 `json:"confidence"`
}
```

Add to the Article struct (next to the existing `Mining *MiningData` field):

```go
Crime    *CrimeData    `json:"crime,omitempty"`
Location *LocationData `json:"location,omitempty"`
```

**Step 4: Run test to verify it passes**

Run: `cd publisher && go test ./internal/router/ -run TestArticle_UnmarshalNestedCrimeFields -v`
Expected: PASS

**Step 5: Commit**

```bash
git add publisher/internal/router/service.go publisher/internal/router/service_test.go
git commit -m "feat(publisher): add CrimeData and LocationData nested structs to Article"
```

---

### Task 2: Add extractNestedFields() to populate flat fields from nested structs

**Files:**
- Modify: `publisher/internal/router/service.go`

**Step 1: Write the failing test**

File: `publisher/internal/router/service_test.go`

```go
func TestArticle_ExtractNestedFields_Crime(t *testing.T) {
	t.Helper()

	article := Article{
		Crime: &CrimeData{
			Relevance:  "core_street_crime",
			SubLabel:   "",
			CrimeTypes: []string{"violent_crime"},
			Confidence: 0.95,
			Homepage:   true,
			Categories: []string{"violent-crime", "crime"},
		},
		Location: &LocationData{
			City:        "vancouver",
			Province:    "BC",
			Country:     "canada",
			Specificity: "city",
			Confidence:  0.85,
		},
	}

	article.extractNestedFields()

	// Crime flat fields populated
	assert.Equal(t, "core_street_crime", article.CrimeRelevance)
	assert.Equal(t, "", article.CrimeSubLabel)
	assert.Equal(t, []string{"violent_crime"}, article.CrimeTypes)
	assert.True(t, article.HomepageEligible)
	assert.Equal(t, []string{"violent-crime", "crime"}, article.CategoryPages)

	// Location flat fields populated
	assert.Equal(t, "vancouver", article.LocationCity)
	assert.Equal(t, "BC", article.LocationProvince)
	assert.Equal(t, "canada", article.LocationCountry)
	assert.Equal(t, "city", article.LocationSpecificity)
	assert.InDelta(t, 0.85, article.LocationConfidence, 0.001)
}

func TestArticle_ExtractNestedFields_NilCrime(t *testing.T) {
	t.Helper()

	article := Article{
		CrimeRelevance: "should-not-change",
	}

	article.extractNestedFields()

	// Flat fields untouched when nested structs are nil
	assert.Equal(t, "should-not-change", article.CrimeRelevance)
}
```

**Step 2: Run test to verify it fails**

Run: `cd publisher && go test ./internal/router/ -run TestArticle_ExtractNestedFields -v`
Expected: FAIL — `extractNestedFields` method does not exist.

**Step 3: Implement extractNestedFields()**

File: `publisher/internal/router/service.go`

Add this method to Article:

```go
// extractNestedFields copies values from the nested Crime and Location
// structs into the flat Article fields used by GenerateCrimeChannels()
// and GenerateLocationChannels(). Call after unmarshaling from Elasticsearch.
func (a *Article) extractNestedFields() {
	if a.Crime != nil {
		a.CrimeRelevance = a.Crime.Relevance
		a.CrimeSubLabel = a.Crime.SubLabel
		a.CrimeTypes = a.Crime.CrimeTypes
		a.LocationSpecificity = a.Crime.Specificity
		a.HomepageEligible = a.Crime.Homepage
		a.CategoryPages = a.Crime.Categories
		a.ReviewRequired = a.Crime.ReviewRequired
	}

	if a.Location != nil {
		a.LocationCity = a.Location.City
		a.LocationProvince = a.Location.Province
		a.LocationCountry = a.Location.Country
		a.LocationConfidence = a.Location.Confidence
		if a.Location.Specificity != "" {
			a.LocationSpecificity = a.Location.Specificity
		}
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd publisher && go test ./internal/router/ -run TestArticle_ExtractNestedFields -v`
Expected: PASS

**Step 5: Commit**

```bash
git add publisher/internal/router/service.go publisher/internal/router/service_test.go
git commit -m "feat(publisher): add extractNestedFields() to populate flat crime/location fields"
```

---

### Task 3: Call extractNestedFields() in fetchArticles() unmarshal loop

**Files:**
- Modify: `publisher/internal/router/service.go` (fetchArticles method, ~line 330)

**Step 1: Write the failing test**

This is an integration-level change. Write a test that verifies the full unmarshal+extract pipeline from raw ES JSON.

File: `publisher/internal/router/service_test.go`

```go
func TestArticle_FullUnmarshalPipeline(t *testing.T) {
	t.Helper()

	esJSON := `{
		"title": "Drug bust in Toronto",
		"body": "Toronto police seized...",
		"canonical_url": "https://example.com/drug-bust",
		"source": "example_com",
		"quality_score": 80,
		"content_type": "article",
		"is_crime_related": true,
		"crime": {
			"street_crime_relevance": "core_street_crime",
			"sub_label": "",
			"crime_types": ["drug_crime"],
			"location_specificity": "local_canada",
			"final_confidence": 0.90,
			"homepage_eligible": true,
			"category_pages": ["drug-crime", "crime"],
			"review_required": false
		},
		"location": {
			"city": "toronto",
			"province": "ON",
			"country": "canada",
			"specificity": "city",
			"confidence": 0.90
		}
	}`

	var article Article
	err := json.Unmarshal([]byte(esJSON), &article)
	require.NoError(t, err)
	article.extractNestedFields()

	// Crime channels should now generate correctly
	crimeChannels := GenerateCrimeChannels(&article)
	assert.Contains(t, crimeChannels, "crime:homepage")
	assert.Contains(t, crimeChannels, "crime:category:drug-crime")
	assert.Contains(t, crimeChannels, "crime:category:crime")

	// Location channels should now generate correctly
	locationChannels := GenerateLocationChannels(&article)
	assert.Contains(t, locationChannels, "crime:local:toronto")
	assert.Contains(t, locationChannels, "crime:province:on")
	assert.Contains(t, locationChannels, "crime:canada")
}
```

**Step 2: Run test to verify it passes already (since extract is called manually)**

Run: `cd publisher && go test ./internal/router/ -run TestArticle_FullUnmarshalPipeline -v`
Expected: PASS (this test calls extractNestedFields manually, confirming the pipeline works)

**Step 3: Wire extractNestedFields() into fetchArticles()**

File: `publisher/internal/router/service.go`

In the `fetchArticles()` method, find the unmarshal loop (approximately line 330). After `article.ID = hit.ID` and `article.Sort = hit.Sort`, add:

```go
article.extractNestedFields()
```

The loop should look like:

```go
for _, hit := range esResponse.Hits.Hits {
    var article Article
    if unmarshalErr := json.Unmarshal(hit.Source, &article); unmarshalErr != nil {
        s.logger.Error("Error unmarshaling article", ...)
        continue
    }
    article.ID = hit.ID
    article.Sort = hit.Sort
    article.extractNestedFields()
    articles = append(articles, article)
}
```

**Step 4: Run full test suite**

Run: `cd publisher && go test ./internal/router/ -v`
Expected: All tests PASS

**Step 5: Lint**

Run: `cd publisher && golangci-lint run`
Expected: No errors

**Step 6: Commit**

```bash
git add publisher/internal/router/service.go publisher/internal/router/service_test.go
git commit -m "feat(publisher): wire extractNestedFields into fetchArticles unmarshal loop

Layer 3 crime channels and Layer 4 location channels will now fire
correctly. Previously, nested ES fields were silently ignored."
```

---

## Phase 2: Tighten Crime Classifier Rules

The current classifier assigns `core_street_crime` too loosely — opinion pieces, travel articles, and gaming content that mention crime vocabulary get through. We need stronger exclusions and tighter core requirements.

---

### Task 4: Add exclusion patterns for opinion/editorial content

**Files:**
- Modify: `classifier/internal/classifier/crime_rules.go`
- Modify: `classifier/internal/classifier/crime_rules_test.go`

**Step 1: Write the failing tests**

File: `classifier/internal/classifier/crime_rules_test.go`

Add a new test function for the expanded exclusion patterns:

```go
func TestCrimeRules_ClassifyByRules_OpinionExclusions(t *testing.T) {
	t.Helper()

	tests := []struct {
		name  string
		title string
	}{
		{"opinion prefix", "Opinion: Crime rates are a political tool"},
		{"editorial prefix", "Editorial: Why policing needs reform"},
		{"commentary prefix", "Commentary: The murder rate debate"},
		{"column prefix", "Column: My thoughts on gang violence"},
		{"op-ed prefix", "Op-Ed: Drug policy has failed us"},
		{"letters prefix", "Letters: Readers respond to shooting coverage"},
		{"first person opinion", "I think the police response was inadequate"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyByRules(tt.title, "")
			assert.Equal(t, "not_crime", result.relevance,
				"title %q should be excluded as opinion", tt.title)
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd classifier && go test ./internal/classifier/ -run TestCrimeRules_ClassifyByRules_OpinionExclusions -v`
Expected: FAIL — several titles match crime patterns (murder, gang violence, drug, shooting) and return `core_street_crime` instead of `not_crime`.

**Step 3: Add exclusion patterns**

File: `classifier/internal/classifier/crime_rules.go`

Add these patterns to the `excludePatterns` slice:

```go
// Opinion/editorial content
`(?i)^(opinion|editorial|commentary|letters?|column|op-ed)\s*:`,
`(?i)\b(i think|in my view|in our view|we believe|my view)\b`,
```

**Step 4: Run test to verify it passes**

Run: `cd classifier && go test ./internal/classifier/ -run TestCrimeRules_ClassifyByRules_OpinionExclusions -v`
Expected: PASS

**Step 5: Commit**

```bash
git add classifier/internal/classifier/crime_rules.go classifier/internal/classifier/crime_rules_test.go
git commit -m "feat(classifier): add opinion/editorial exclusion patterns to crime rules"
```

---

### Task 5: Add exclusion patterns for lifestyle/non-crime content

**Files:**
- Modify: `classifier/internal/classifier/crime_rules.go`
- Modify: `classifier/internal/classifier/crime_rules_test.go`

**Step 1: Write the failing tests**

File: `classifier/internal/classifier/crime_rules_test.go`

```go
func TestCrimeRules_ClassifyByRules_LifestyleExclusions(t *testing.T) {
	t.Helper()

	tests := []struct {
		name  string
		title string
	}{
		{"renovation", "7 best house renovation contractors in the area"},
		{"tournament", "PUBG online tournament finals this weekend"},
		{"travel guide", "A new lifeline for anyone travelling through BC"},
		{"recipe", "Best recipe for a killer BBQ sauce"},
		{"best-of list", "Best contractors in the Vancouver area"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyByRules(tt.title, "")
			assert.Equal(t, "not_crime", result.relevance,
				"title %q should be excluded as lifestyle", tt.title)
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd classifier && go test ./internal/classifier/ -run TestCrimeRules_ClassifyByRules_LifestyleExclusions -v`
Expected: FAIL — some titles don't match existing exclusions.

**Step 3: Add exclusion patterns**

File: `classifier/internal/classifier/crime_rules.go`

Add these patterns to the `excludePatterns` slice:

```go
// Lifestyle/non-crime content
`(?i)\b(renovation|contractor|tournament|recipe|travel guide|lifeline)\b`,
`(?i)\bbest\s+.+\s+in\s+the\s+.+\s+area\b`,
```

**Step 4: Run test to verify it passes**

Run: `cd classifier && go test ./internal/classifier/ -run TestCrimeRules_ClassifyByRules_LifestyleExclusions -v`
Expected: PASS

**Step 5: Run full test suite to ensure no regressions**

Run: `cd classifier && go test ./internal/classifier/ -v`
Expected: All tests PASS

**Step 6: Commit**

```bash
git add classifier/internal/classifier/crime_rules.go classifier/internal/classifier/crime_rules_test.go
git commit -m "feat(classifier): add lifestyle/non-crime exclusion patterns"
```

---

### Task 6: Tighten core_street_crime to require authority indicators

Currently, patterns like `murder`, `shooting`, `stabbing` match without requiring an authority indicator (police, court, etc.). This lets fiction, opinion, and historical references through.

**Files:**
- Modify: `classifier/internal/classifier/crime_rules.go`
- Modify: `classifier/internal/classifier/crime_rules_test.go`

**Step 1: Write the failing tests**

File: `classifier/internal/classifier/crime_rules_test.go`

```go
func TestCrimeRules_ClassifyByRules_RequiresAuthority(t *testing.T) {
	t.Helper()

	// These should STILL be core_street_crime (have authority indicators)
	coreTests := []struct {
		name  string
		title string
	}{
		{"murder with police", "Police investigate murder in downtown Toronto"},
		{"shooting with RCMP", "RCMP respond to shooting at mall"},
		{"stabbing with arrest", "Man arrested after stabbing outside bar"},
		{"drug bust", "Police drug bust seizes fentanyl in Vancouver"},
		{"sexual assault charged", "Suspect charged with sexual assault"},
		{"found dead investigation", "Woman found dead, police launch investigation"},
		{"sentenced murder", "Man sentenced to life for murder of wife"},
	}

	for _, tt := range coreTests {
		t.Run("core: "+tt.name, func(t *testing.T) {
			result := classifyByRules(tt.title, "")
			assert.Equal(t, "core_street_crime", result.relevance,
				"title %q should be core_street_crime", tt.title)
		})
	}

	// These should NOT be core_street_crime (no authority indicator)
	nonCoreTests := []struct {
		name  string
		title string
	}{
		{"murder fiction", "Murder on the Orient Express returns to stage"},
		{"shooting metaphor", "Shooting for the stars: local athlete's journey"},
		{"stabbing game", "Stabbing mechanics in new action RPG reviewed"},
	}

	for _, tt := range nonCoreTests {
		t.Run("not-core: "+tt.name, func(t *testing.T) {
			result := classifyByRules(tt.title, "")
			assert.NotEqual(t, "core_street_crime", result.relevance,
				"title %q should NOT be core_street_crime", tt.title)
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd classifier && go test ./internal/classifier/ -run TestCrimeRules_ClassifyByRules_RequiresAuthority -v`
Expected: FAIL — the "not-core" tests will return `core_street_crime` because current patterns don't require authority indicators for murder/shooting/stabbing.

**Step 3: Modify violent crime patterns to require authority indicators**

File: `classifier/internal/classifier/crime_rules.go`

Refactor the violent crime patterns. The current patterns like:

```go
`(?i)(murder|homicide|manslaughter)`
`(?i)(shooting|shootout|shot dead|gunfire)`
`(?i)(stab|stabbing|stabbed)`
```

Need to become bidirectional patterns requiring an authority indicator nearby. Use the same pattern style already used for assault:

```go
// Authority indicators used across violent crime patterns
// police|rcmp|opp|sq|court|judge|investigation|suspect|accused|officer|
// constable|detective|prosecution|charged|arrest|sentenced|convicted|
// found guilty|pleaded guilty

// Violent crime: action + authority (either order)
`(?i)(murder|homicide|manslaughter).*(police|rcmp|opp|court|judge|investigation|suspect|accused|officer|charged|arrest|sentenced|convicted|prosecution)`,
`(?i)(police|rcmp|opp|court|judge|investigation|suspect|accused|officer|charged|arrest|sentenced|convicted|prosecution).*(murder|homicide|manslaughter)`,
`(?i)(shooting|shootout|shot dead|gunfire).*(police|rcmp|opp|court|judge|investigation|suspect|accused|officer|charged|arrest|sentenced|convicted|prosecution)`,
`(?i)(police|rcmp|opp|court|judge|investigation|suspect|accused|officer|charged|arrest|sentenced|convicted|prosecution).*(shooting|shootout|shot dead|gunfire)`,
`(?i)(stab|stabbing|stabbed).*(police|rcmp|opp|court|judge|investigation|suspect|accused|officer|charged|arrest|sentenced|convicted|prosecution)`,
`(?i)(police|rcmp|opp|court|judge|investigation|suspect|accused|officer|charged|arrest|sentenced|convicted|prosecution).*(stab|stabbing|stabbed)`,
```

Keep the existing `assault` and `sexual assault` patterns unchanged (they already require context). Keep `found dead|human remains` unchanged (inherently crime-related). Keep drug patterns unchanged (already specific enough).

**Important:** The existing tests in `TestCrimeRules_ClassifyByRules_ViolentCrime` use titles like "Man charged with murder after stabbing" and "Police respond to downtown shooting" — these already contain authority indicators and should still pass.

**Step 4: Run test to verify it passes**

Run: `cd classifier && go test ./internal/classifier/ -run TestCrimeRules_ClassifyByRules_RequiresAuthority -v`
Expected: PASS

**Step 5: Run full test suite**

Run: `cd classifier && go test ./internal/classifier/ -v`
Expected: All tests PASS (existing tests contain authority indicators)

**Step 6: Lint**

Run: `cd classifier && golangci-lint run`
Expected: No errors

**Step 7: Commit**

```bash
git add classifier/internal/classifier/crime_rules.go classifier/internal/classifier/crime_rules_test.go
git commit -m "feat(classifier): require authority indicators for core_street_crime

Murder, shooting, and stabbing patterns now require a nearby authority
indicator (police, court, charged, etc.) to prevent fiction, metaphors,
and opinion pieces from being classified as core_street_crime."
```

---

### Task 7: Add sentencing/verdict patterns as core_street_crime

Court outcomes (sentenced, convicted, found guilty, pleaded guilty) with an authority indicator should qualify as `core_street_crime` and route to `crime:category:court-news`.

**Files:**
- Modify: `classifier/internal/classifier/crime_rules.go`
- Modify: `classifier/internal/classifier/crime_rules_test.go`

**Step 1: Write the failing tests**

File: `classifier/internal/classifier/crime_rules_test.go`

```go
func TestCrimeRules_ClassifyByRules_CourtOutcomes(t *testing.T) {
	t.Helper()

	tests := []struct {
		name  string
		title string
	}{
		{"sentenced", "Man sentenced to 15 years in prison for armed robbery"},
		{"convicted", "Jury convicts accused in deadly shooting case"},
		{"found guilty", "Woman found guilty of fraud by judge"},
		{"pleaded guilty", "Teen pleaded guilty to assault charges in court"},
		{"prison term", "Judge hands down prison term for drug trafficking ring leader"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyByRules(tt.title, "")
			assert.Equal(t, "core_street_crime", result.relevance,
				"title %q should be core_street_crime", tt.title)
			assert.Contains(t, result.crimeTypes, "criminal_justice",
				"title %q should have criminal_justice crime type", tt.title)
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd classifier && go test ./internal/classifier/ -run TestCrimeRules_ClassifyByRules_CourtOutcomes -v`
Expected: FAIL — sentencing patterns may not all match current rules.

**Step 3: Add court outcome patterns**

File: `classifier/internal/classifier/crime_rules.go`

Add these patterns to the violent crime section (they represent real criminal outcomes):

```go
// Court outcomes with authority context
`(?i)(sentenced|convicted|found guilty|pleaded guilty|prison term).*(court|judge|prison|jail|penitentiary|charges)`,
`(?i)(court|judge|jury).*(sentenced|convicted|found guilty|pleaded guilty|prison term)`,
```

These should use the same confidence as `confidenceAssault` (0.85) since they're confirmed crime events but less urgent than active violence.

**Step 4: Run test to verify it passes**

Run: `cd classifier && go test ./internal/classifier/ -run TestCrimeRules_ClassifyByRules_CourtOutcomes -v`
Expected: PASS

**Step 5: Run full test suite**

Run: `cd classifier && go test ./internal/classifier/ -v`
Expected: All tests PASS

**Step 6: Commit**

```bash
git add classifier/internal/classifier/crime_rules.go classifier/internal/classifier/crime_rules_test.go
git commit -m "feat(classifier): add sentencing/verdict patterns as core_street_crime"
```

---

### Task 8: Verify classifier lint and run full test suite

**Files:** None (verification only)

**Step 1: Run full test suite**

Run: `cd classifier && go test ./... -v`
Expected: All tests PASS

**Step 2: Run linter**

Run: `cd classifier && golangci-lint run`
Expected: No errors. If there are line length or complexity issues from the new patterns, refactor by extracting the authority indicator regex into a named constant.

**Step 3: Fix any lint issues (if needed)**

If patterns make lines too long, extract the authority indicators into a constant:

```go
const authorityIndicators = `police|rcmp|opp|sq|court|judge|investigation|suspect|accused|officer|charged|arrest|sentenced|convicted|prosecution`
```

Then build patterns programmatically or use shorter inline versions.

**Step 4: Commit fixes (if any)**

```bash
git add classifier/
git commit -m "fix(classifier): resolve lint issues in crime rules"
```

---

## Phase 3: Streetcode Subscriber Changes

Harden the Streetcode Laravel subscriber to reject non-crime content even if upstream misclassifies.

---

### Task 9: Remove peripheral crime channels from Streetcode subscription

**Files:**
- Modify: `/home/fsd42/dev/streetcode-laravel/config/database.php` (lines 200-209)

**Step 1: Review current channel list**

Current channels include `crime:courts` and `crime:context` which are peripheral crime catch-alls. Remove them.

**Step 2: Update channel list**

File: `/home/fsd42/dev/streetcode-laravel/config/database.php`

Change the `crime_channels` array from:

```php
'crime_channels' => [
    'crime:homepage',
    'crime:category:violent-crime',
    'crime:category:property-crime',
    'crime:category:drug-crime',
    'crime:category:gang-violence',
    'crime:category:organized-crime',
    'crime:category:court-news',
    'crime:category:crime',
    'crime:courts',
    'crime:context',
],
```

To:

```php
// Crime-only Redis channels (North Cloud Layer 3).
// Only core_street_crime articles reach these channels.
// Peripheral crime (crime:courts, crime:context) deliberately excluded.
'crime_channels' => [
    'crime:homepage',
    'crime:category:violent-crime',
    'crime:category:property-crime',
    'crime:category:drug-crime',
    'crime:category:gang-violence',
    'crime:category:organized-crime',
    'crime:category:court-news',
    'crime:category:crime',
],
```

**Step 3: Commit**

```bash
cd /home/fsd42/dev/streetcode-laravel
git add config/database.php
git commit -m "fix: remove peripheral crime channels (crime:courts, crime:context)

These channels include opinion, context, and non-event articles.
Only subscribe to core_street_crime Layer 3 channels."
```

---

### Task 10: Set quality threshold to 50

**Files:**
- Modify: `/home/fsd42/dev/streetcode-laravel/.env.example`

**Step 1: Update .env.example**

File: `/home/fsd42/dev/streetcode-laravel/.env.example`

Change:
```
ARTICLES_MIN_QUALITY_SCORE=0
```

To:
```
ARTICLES_MIN_QUALITY_SCORE=50
```

**Step 2: Also update the live .env file (if it exists and has the old value)**

Check `/home/fsd42/dev/streetcode-laravel/.env` and update `ARTICLES_MIN_QUALITY_SCORE=50` if present.

**Step 3: Commit**

```bash
cd /home/fsd42/dev/streetcode-laravel
git add .env.example
git commit -m "fix: set ARTICLES_MIN_QUALITY_SCORE=50 to filter low-quality content"
```

---

### Task 11: Add crime-relevance validation to ProcessIncomingArticle

**Files:**
- Modify: `/home/fsd42/dev/streetcode-laravel/app/Jobs/ProcessIncomingArticle.php`

**Step 1: Write the test**

File: `/home/fsd42/dev/streetcode-laravel/tests/Unit/Jobs/ProcessIncomingArticleTest.php`

Create or update the test file. If it doesn't exist, create it:

```php
<?php

namespace Tests\Unit\Jobs;

use App\Jobs\ProcessIncomingArticle;
use App\Models\Article;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Support\Facades\Log;
use Tests\TestCase;

class ProcessIncomingArticleTest extends TestCase
{
    use RefreshDatabase;

    public function test_rejects_non_core_crime_article(): void
    {
        Log::shouldReceive('info')
            ->once()
            ->withArgs(function ($message) {
                return str_contains($message, 'Skipping non-core-crime article');
            });

        $data = [
            'id' => 'test-article-123',
            'title' => 'Opinion about crime rates',
            'canonical_url' => 'https://example.com/opinion',
            'source' => 'example_com',
            'quality_score' => 75,
            'content_type' => 'article',
            'crime_relevance' => 'peripheral_crime',
            'publisher' => [
                'route_id' => 'route-1',
                'published_at' => now()->toISOString(),
                'channel' => 'crime:context',
            ],
        ];

        $job = new ProcessIncomingArticle($data);
        $job->handle();

        $this->assertDatabaseMissing('articles', [
            'external_id' => 'test-article-123',
        ]);
    }

    public function test_accepts_core_street_crime_article(): void
    {
        $data = [
            'id' => 'test-article-456',
            'title' => 'Man arrested for robbery',
            'canonical_url' => 'https://example.com/robbery',
            'source' => 'example_com',
            'quality_score' => 75,
            'content_type' => 'article',
            'crime_relevance' => 'core_street_crime',
            'topics' => ['violent_crime'],
            'publisher' => [
                'route_id' => 'route-1',
                'published_at' => now()->toISOString(),
                'channel' => 'crime:homepage',
            ],
        ];

        $job = new ProcessIncomingArticle($data);
        $job->handle();

        $this->assertDatabaseHas('articles', [
            'external_id' => 'test-article-456',
        ]);
    }
}
```

**Step 2: Run test to verify it fails**

Run: `cd /home/fsd42/dev/streetcode-laravel && php artisan test --filter=ProcessIncomingArticleTest`
Expected: FAIL — the non-core crime article gets stored (no validation exists yet).

**Step 3: Add crime-relevance validation**

File: `/home/fsd42/dev/streetcode-laravel/app/Jobs/ProcessIncomingArticle.php`

In the `handle()` method, add this check early (after the publisher format validation but before the duplicate check):

```php
// Defense-in-depth: only accept core_street_crime articles
$crimeRelevance = $this->articleData['crime_relevance'] ?? '';
if ($crimeRelevance !== 'core_street_crime') {
    Log::info('Skipping non-core-crime article', [
        'crime_relevance' => $crimeRelevance,
        'title' => $this->articleData['title'] ?? $this->articleData['og_title'] ?? 'unknown',
        'external_id' => $this->articleData['id'] ?? 'unknown',
    ]);

    return;
}
```

**Step 4: Run test to verify it passes**

Run: `cd /home/fsd42/dev/streetcode-laravel && php artisan test --filter=ProcessIncomingArticleTest`
Expected: PASS

**Step 5: Commit**

```bash
cd /home/fsd42/dev/streetcode-laravel
git add app/Jobs/ProcessIncomingArticle.php tests/Unit/Jobs/ProcessIncomingArticleTest.php
git commit -m "feat: add crime-relevance validation to ProcessIncomingArticle

Defense-in-depth: reject articles without core_street_crime relevance
even if they arrive on a subscribed channel."
```

---

### Task 12: Filter topic tags to only crime sub-types

**Files:**
- Modify: `/home/fsd42/dev/streetcode-laravel/app/Jobs/ProcessIncomingArticle.php`

Currently, all topics from the publisher are attached as `crime_category` tags. This means generic topics like `news`, `politics` get tagged as crime categories.

**Step 1: Write the failing test**

File: `/home/fsd42/dev/streetcode-laravel/tests/Unit/Jobs/ProcessIncomingArticleTest.php`

Add to the existing test class:

```php
public function test_only_attaches_crime_topic_tags(): void
{
    $data = [
        'id' => 'test-article-789',
        'title' => 'Drug bust in Vancouver',
        'canonical_url' => 'https://example.com/drug-bust',
        'source' => 'example_com',
        'quality_score' => 80,
        'content_type' => 'article',
        'crime_relevance' => 'core_street_crime',
        'topics' => ['drug_crime', 'news', 'politics', 'violent_crime'],
        'publisher' => [
            'route_id' => 'route-1',
            'published_at' => now()->toISOString(),
            'channel' => 'crime:category:drug-crime',
        ],
    ];

    $job = new ProcessIncomingArticle($data);
    $job->handle();

    $article = Article::where('external_id', 'test-article-789')->first();
    $this->assertNotNull($article);

    $tagSlugs = $article->tags->pluck('slug')->toArray();

    // Crime topics should be attached
    $this->assertContains('drug-crime', $tagSlugs);
    $this->assertContains('violent-crime', $tagSlugs);

    // Non-crime topics should NOT be attached
    $this->assertNotContains('news', $tagSlugs);
    $this->assertNotContains('politics', $tagSlugs);
}
```

**Step 2: Run test to verify it fails**

Run: `cd /home/fsd42/dev/streetcode-laravel && php artisan test --filter=test_only_attaches_crime_topic_tags`
Expected: FAIL — `news` and `politics` tags are attached.

**Step 3: Filter topics before attaching tags**

File: `/home/fsd42/dev/streetcode-laravel/app/Jobs/ProcessIncomingArticle.php`

Find the tag attachment code (approximately lines 86-89):

```php
if (! empty($this->articleData['topics'])) {
    $this->attachTags($article, $this->articleData['topics']);
}
```

Change to:

```php
if (! empty($this->articleData['topics'])) {
    $crimeTopics = ['violent_crime', 'property_crime', 'drug_crime', 'organized_crime', 'criminal_justice', 'gang_violence'];
    $filteredTopics = array_intersect($this->articleData['topics'], $crimeTopics);
    if (! empty($filteredTopics)) {
        $this->attachTags($article, array_values($filteredTopics));
    }
}
```

**Step 4: Run test to verify it passes**

Run: `cd /home/fsd42/dev/streetcode-laravel && php artisan test --filter=test_only_attaches_crime_topic_tags`
Expected: PASS

**Step 5: Run full test suite**

Run: `cd /home/fsd42/dev/streetcode-laravel && php artisan test`
Expected: All tests PASS

**Step 6: Commit**

```bash
cd /home/fsd42/dev/streetcode-laravel
git add app/Jobs/ProcessIncomingArticle.php tests/Unit/Jobs/ProcessIncomingArticleTest.php
git commit -m "fix: filter topic tags to only attach crime sub-types

Only violent_crime, property_crime, drug_crime, organized_crime,
criminal_justice, and gang_violence get crime_category tags.
Generic topics like news/politics are excluded."
```

---

## Phase 4: Verification and Cleanup

---

### Task 13: Run publisher full test suite and lint

**Step 1: Run tests**

Run: `cd /home/fsd42/dev/north-cloud/publisher && go test ./... -v`
Expected: All tests PASS

**Step 2: Run linter**

Run: `cd /home/fsd42/dev/north-cloud/publisher && golangci-lint run`
Expected: No errors

**Step 3: Fix any issues and commit**

---

### Task 14: Run classifier full test suite and lint

**Step 1: Run tests**

Run: `cd /home/fsd42/dev/north-cloud/classifier && go test ./... -v`
Expected: All tests PASS

**Step 2: Run linter**

Run: `cd /home/fsd42/dev/north-cloud/classifier && golangci-lint run`
Expected: No errors

**Step 3: Fix any issues and commit**

---

### Task 15: Run Streetcode full test suite

**Step 1: Run tests**

Run: `cd /home/fsd42/dev/streetcode-laravel && php artisan test`
Expected: All tests PASS

**Step 2: Fix any issues and commit**

---

### Task 16: Create Streetcode Artisan command to clean existing non-crime articles

**Files:**
- Create: `/home/fsd42/dev/streetcode-laravel/app/Console/Commands/SoftDeleteNonCrimeArticles.php`

**Step 1: Create the command**

```php
<?php

namespace App\Console\Commands;

use App\Models\Article;
use Illuminate\Console\Command;

class SoftDeleteNonCrimeArticles extends Command
{
    protected $signature = 'articles:soft-delete-non-crime {--dry-run : Show what would be deleted without deleting}';
    protected $description = 'Soft-delete articles that are not core_street_crime';

    public function handle(): int
    {
        $dryRun = $this->option('dry-run');

        $query = Article::whereRaw(
            "JSON_UNQUOTE(JSON_EXTRACT(metadata, '$.crime_relevance')) != ? OR JSON_EXTRACT(metadata, '$.crime_relevance') IS NULL",
            ['core_street_crime']
        );

        $count = $query->count();

        if ($count === 0) {
            $this->info('No non-crime articles found.');
            return Command::SUCCESS;
        }

        if ($dryRun) {
            $this->info("Would soft-delete {$count} non-crime articles.");
            $query->limit(10)->get()->each(function ($article) {
                $this->line("  - [{$article->external_id}] {$article->title}");
            });
            if ($count > 10) {
                $this->line("  ... and " . ($count - 10) . " more");
            }
            return Command::SUCCESS;
        }

        if (! $this->confirm("Soft-delete {$count} non-crime articles?")) {
            return Command::SUCCESS;
        }

        $deleted = $query->delete();
        $this->info("Soft-deleted {$deleted} non-crime articles.");

        return Command::SUCCESS;
    }
}
```

**Step 2: Test with dry-run**

Run: `cd /home/fsd42/dev/streetcode-laravel && php artisan articles:soft-delete-non-crime --dry-run`
Expected: Shows count and sample of articles that would be deleted.

**Step 3: Commit**

```bash
cd /home/fsd42/dev/streetcode-laravel
git add app/Console/Commands/SoftDeleteNonCrimeArticles.php
git commit -m "feat: add artisan command to soft-delete non-crime articles

Run with --dry-run to preview, then without to execute.
Uses soft-delete so articles can be restored if needed."
```

---

## Deployment Order

1. **Publisher** (Tasks 1-3) — Deploy first. Layer 3 crime channels start firing.
2. **Classifier** (Tasks 4-8) — Deploy second. New articles get stricter classification.
3. **Streetcode** (Tasks 9-12, 16) — Deploy last. Updated channels, quality threshold, validation.
4. **Cleanup** — Run `php artisan articles:soft-delete-non-crime` to remove existing junk.

---

## Success Criteria

- [ ] `publish_history` shows entries for `crime:homepage` and `crime:category:*` channels
- [ ] Streetcode homepage shows only real crime events (incidents, arrests, investigations, sentencing)
- [ ] No travel, lifestyle, opinion, or political articles on Streetcode
- [ ] Quality score threshold prevents spam/low-quality content
- [ ] Defense-in-depth: even if upstream mis-classifies, Streetcode rejects non-crime articles
