# Topic-Agnostic Location Routing Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Refactor publisher location routing from crime-only to topic-agnostic, so all classified topics (crime, mining, entertainment) get geographic channels.

**Architecture:** Extract `GenerateLocationChannels` from `crime.go` into a new `location.go`. The function detects which domain classifiers are active on an article and generates `{prefix}:local:{city}`, `{prefix}:province:{code}`, `{prefix}:canada`, `{prefix}:international` for each. Same function signature — `service.go` Layer 4 call unchanged.

**Tech Stack:** Go 1.25+, testify (assert/require)

---

### Task 1: Create `location.go` with topic-agnostic location routing

**Files:**
- Create: `publisher/internal/router/location.go`
- Reference: `publisher/internal/router/crime.go` (for constants and current logic)
- Reference: `publisher/internal/router/service.go:274-310` (Article struct with all classifier fields)

**Step 1: Create `location.go`**

```go
// publisher/internal/router/location.go
package router

import (
	"fmt"
	"strings"
)

// Location constants.
const (
	LocationCountryCanada   = "canada"
	LocationCountryUnknown  = "unknown"
	LocationSpecificityCity = "city"
)

// GenerateLocationChannels creates geographic channels based on article location.
// For each active domain classifier (crime, mining, entertainment), generates:
//   - {prefix}:local:{city} for city-specific Canadian content
//   - {prefix}:province:{code} for province-level Canadian content
//   - {prefix}:canada for national Canadian content
//   - {prefix}:international for non-Canadian content
func GenerateLocationChannels(article *Article) []string {
	// Skip unknown or empty locations
	if article.LocationCountry == LocationCountryUnknown || article.LocationCountry == "" {
		return nil
	}

	prefixes := activeTopicPrefixes(article)
	if len(prefixes) == 0 {
		return nil
	}

	// International (non-Canadian) — one channel per prefix
	if article.LocationCountry != LocationCountryCanada {
		channels := make([]string, 0, len(prefixes))
		for _, prefix := range prefixes {
			channels = append(channels, prefix+":international")
		}
		return channels
	}

	// Canadian locations — build from most specific to least specific per prefix
	return generateCanadianChannels(article, prefixes)
}

// generateCanadianChannels builds location channels for Canadian content.
func generateCanadianChannels(article *Article, prefixes []string) []string {
	// Estimate capacity: up to 3 channels (local, province, canada) per prefix
	const maxChannelsPerPrefix = 3
	channels := make([]string, 0, len(prefixes)*maxChannelsPerPrefix)

	for _, prefix := range prefixes {
		if article.LocationSpecificity == LocationSpecificityCity && article.LocationCity != "" {
			channels = append(channels, fmt.Sprintf("%s:local:%s", prefix, article.LocationCity))
		}
		if article.LocationProvince != "" {
			channels = append(channels, fmt.Sprintf("%s:province:%s", prefix, strings.ToLower(article.LocationProvince)))
		}
		channels = append(channels, prefix+":canada")
	}

	return channels
}

// activeTopicPrefixes returns the channel prefixes for domain classifiers
// that are active on this article.
func activeTopicPrefixes(article *Article) []string {
	const maxPrefixes = 3
	prefixes := make([]string, 0, maxPrefixes)

	if article.CrimeRelevance != CrimeRelevanceNotCrime && article.CrimeRelevance != "" {
		prefixes = append(prefixes, "crime")
	}
	if article.Mining != nil && article.Mining.Relevance != MiningRelevanceNotMining && article.Mining.Relevance != "" {
		prefixes = append(prefixes, "mining")
	}
	if article.Entertainment != nil && article.Entertainment.Relevance != EntertainmentRelevanceNot && article.Entertainment.Relevance != "" {
		prefixes = append(prefixes, "entertainment")
	}

	return prefixes
}
```

**Step 2: Run linter to verify**

Run: `cd /home/fsd42/dev/north-cloud/publisher && golangci-lint run ./internal/router/`

Expected: Compilation errors because `LocationCountryCanada`, `LocationCountryUnknown`, `LocationSpecificityCity` are still defined in `crime.go`. This is expected — we fix it in Task 2.

---

### Task 2: Remove location code from `crime.go`

**Files:**
- Modify: `publisher/internal/router/crime.go` (remove `GenerateLocationChannels` function and location constants)

**Step 1: Remove location constants and function from `crime.go`**

Remove these constants from the const block (lines 16-18):
- `LocationCountryCanada`
- `LocationCountryUnknown`
- `LocationSpecificityCity`

Remove the entire `GenerateLocationChannels` function (lines 63-99).

Also remove the `"strings"` import since `GenerateCrimeChannels` doesn't use it (only `"fmt"` is needed).

After editing, `crime.go` should contain only:
- Crime relevance constants (`CrimeRelevanceNotCrime`, etc.)
- Sub-label constants (`SubLabelCriminalJustice`, `SubLabelCrimeContext`)
- `GenerateCrimeChannels` function

**Step 2: Run linter**

Run: `cd /home/fsd42/dev/north-cloud/publisher && golangci-lint run ./internal/router/`

Expected: PASS (no compilation errors, no duplicate symbols)

**Step 3: Commit**

```bash
git add publisher/internal/router/location.go publisher/internal/router/crime.go
git commit -m "refactor(publisher): extract location routing from crime.go to location.go

Location channels are now topic-agnostic. For each active domain classifier
(crime, mining, entertainment), generates {prefix}:local:{city},
{prefix}:province:{code}, {prefix}:canada, {prefix}:international."
```

---

### Task 3: Create `location_test.go`

**Files:**
- Create: `publisher/internal/router/location_test.go`
- Reference: `publisher/internal/router/crime_test.go:283-406` (existing location tests to port)

**Step 1: Write location tests**

Port the existing crime-only location tests and add new multi-topic tests:

```go
// publisher/internal/router/location_test.go
//
//nolint:testpackage // Testing internal router requires same package access
package router

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateLocationChannels_CrimeCanadianCity(t *testing.T) {
	t.Helper()

	article := &Article{
		CrimeRelevance:      "core_street_crime",
		LocationCity:        "sudbury",
		LocationProvince:    "ON",
		LocationCountry:     "canada",
		LocationSpecificity: "city",
	}

	channels := GenerateLocationChannels(article)

	assert.Contains(t, channels, "crime:local:sudbury")
	assert.Contains(t, channels, "crime:province:on")
	assert.Contains(t, channels, "crime:canada")
	assert.Len(t, channels, 3)
}

func TestGenerateLocationChannels_CrimeInternational(t *testing.T) {
	t.Helper()

	article := &Article{
		CrimeRelevance:  "core_street_crime",
		LocationCountry: "united_states",
	}

	channels := GenerateLocationChannels(article)

	assert.Equal(t, []string{"crime:international"}, channels)
}

func TestGenerateLocationChannels_CrimeProvinceOnly(t *testing.T) {
	t.Helper()

	article := &Article{
		CrimeRelevance:      "peripheral_crime",
		LocationProvince:    "BC",
		LocationCountry:     "canada",
		LocationSpecificity: "province",
	}

	channels := GenerateLocationChannels(article)

	assert.Contains(t, channels, "crime:province:bc")
	assert.Contains(t, channels, "crime:canada")
	assert.NotContains(t, channels, "crime:local:")
	assert.Len(t, channels, 2)
}

func TestGenerateLocationChannels_MiningCanadianCity(t *testing.T) {
	t.Helper()

	article := &Article{
		Mining:              &MiningData{Relevance: "core_mining"},
		LocationCity:        "timmins",
		LocationProvince:    "ON",
		LocationCountry:     "canada",
		LocationSpecificity: "city",
	}

	channels := GenerateLocationChannels(article)

	assert.Contains(t, channels, "mining:local:timmins")
	assert.Contains(t, channels, "mining:province:on")
	assert.Contains(t, channels, "mining:canada")
	assert.Len(t, channels, 3)
}

func TestGenerateLocationChannels_MiningInternational(t *testing.T) {
	t.Helper()

	article := &Article{
		Mining:          &MiningData{Relevance: "peripheral_mining"},
		LocationCountry: "australia",
	}

	channels := GenerateLocationChannels(article)

	assert.Equal(t, []string{"mining:international"}, channels)
}

func TestGenerateLocationChannels_EntertainmentCanadianCity(t *testing.T) {
	t.Helper()

	article := &Article{
		Entertainment:       &EntertainmentData{Relevance: "core_entertainment"},
		LocationCity:        "toronto",
		LocationProvince:    "ON",
		LocationCountry:     "canada",
		LocationSpecificity: "city",
	}

	channels := GenerateLocationChannels(article)

	assert.Contains(t, channels, "entertainment:local:toronto")
	assert.Contains(t, channels, "entertainment:province:on")
	assert.Contains(t, channels, "entertainment:canada")
	assert.Len(t, channels, 3)
}

func TestGenerateLocationChannels_MultiTopic(t *testing.T) {
	t.Helper()

	article := &Article{
		CrimeRelevance:      "core_street_crime",
		Mining:              &MiningData{Relevance: "peripheral_mining"},
		LocationCity:        "sudbury",
		LocationProvince:    "ON",
		LocationCountry:     "canada",
		LocationSpecificity: "city",
	}

	channels := GenerateLocationChannels(article)

	// Crime channels
	assert.Contains(t, channels, "crime:local:sudbury")
	assert.Contains(t, channels, "crime:province:on")
	assert.Contains(t, channels, "crime:canada")
	// Mining channels
	assert.Contains(t, channels, "mining:local:sudbury")
	assert.Contains(t, channels, "mining:province:on")
	assert.Contains(t, channels, "mining:canada")
	assert.Len(t, channels, 6)
}

func TestGenerateLocationChannels_NoActiveTopic(t *testing.T) {
	t.Helper()

	article := &Article{
		CrimeRelevance:      "not_crime",
		LocationCity:        "toronto",
		LocationProvince:    "ON",
		LocationCountry:     "canada",
		LocationSpecificity: "city",
	}

	channels := GenerateLocationChannels(article)

	assert.Empty(t, channels)
}

func TestGenerateLocationChannels_UnknownLocation(t *testing.T) {
	t.Helper()

	article := &Article{
		CrimeRelevance: "core_street_crime",
		LocationCountry: "unknown",
	}

	channels := GenerateLocationChannels(article)

	assert.Empty(t, channels)
}

func TestGenerateLocationChannels_EmptyLocation(t *testing.T) {
	t.Helper()

	article := &Article{
		CrimeRelevance:  "core_street_crime",
		LocationCountry: "",
	}

	channels := GenerateLocationChannels(article)

	assert.Empty(t, channels)
}

func TestGenerateLocationChannels_NotMining(t *testing.T) {
	t.Helper()

	article := &Article{
		Mining:          &MiningData{Relevance: "not_mining"},
		LocationCountry: "canada",
	}

	channels := GenerateLocationChannels(article)

	assert.Empty(t, channels)
}

func TestGenerateLocationChannels_NotEntertainment(t *testing.T) {
	t.Helper()

	article := &Article{
		Entertainment:   &EntertainmentData{Relevance: "not_entertainment"},
		LocationCountry: "canada",
	}

	channels := GenerateLocationChannels(article)

	assert.Empty(t, channels)
}
```

**Step 2: Run tests**

Run: `cd /home/fsd42/dev/north-cloud/publisher && go test ./internal/router/ -run TestGenerateLocationChannels -v`

Expected: All location tests PASS

---

### Task 4: Update `crime_test.go` — remove location tests

**Files:**
- Modify: `publisher/internal/router/crime_test.go`

**Step 1: Remove location tests from `crime_test.go`**

Remove the following tests (lines 281-406) which have been ported to `location_test.go`:
- `TestGenerateLocationChannels_NotCrime`
- `TestGenerateLocationChannels_EmptyCrimeRelevance`
- `TestGenerateLocationChannels_CanadianCity`
- `TestGenerateLocationChannels_International`
- `TestGenerateLocationChannels_UnknownLocation`
- `TestGenerateLocationChannels_ProvinceOnly`

Also remove the `// Location channel tests` comment on line 281.

**Step 2: Run all router tests**

Run: `cd /home/fsd42/dev/north-cloud/publisher && go test ./internal/router/ -v`

Expected: All tests PASS (crime tests still pass, new location tests pass)

**Step 3: Run linter**

Run: `cd /home/fsd42/dev/north-cloud/publisher && golangci-lint run ./internal/router/`

Expected: PASS

**Step 4: Commit**

```bash
git add publisher/internal/router/location_test.go publisher/internal/router/crime_test.go
git commit -m "test(publisher): add location routing tests for all topics

Port crime-only location tests to location_test.go and add coverage for
mining, entertainment, multi-topic, and no-topic scenarios."
```

---

### Task 5: Update `publisher/CLAUDE.md`

**Files:**
- Modify: `publisher/CLAUDE.md:63` (Layer 4 description)
- Modify: `publisher/CLAUDE.md:189` (Router flow step 5)

**Step 1: Update Layer 4 description**

Change line 63 from:
```
- **Layer 4 (location channels)**: Automatic channels for geographic routing of crime content (`crime:local:{city}`, `crime:province:{code}`, `crime:canada`, `crime:international`).
```

To:
```
- **Layer 4 (location channels)**: Automatic geographic channels for all topics with active domain classifiers. For each active topic (crime, mining, entertainment), generates `{topic}:local:{city}`, `{topic}:province:{code}`, `{topic}:canada`, `{topic}:international`.
```

**Step 2: Update Router flow step 5**

Change line 189 from:
```
5. **Route Layer 4**: Generates location-based channels
```

To:
```
5. **Route Layer 4**: Generates topic-prefixed location channels for all active classifiers
```

**Step 3: Commit**

```bash
git add publisher/CLAUDE.md
git commit -m "docs(publisher): update Layer 4 description for topic-agnostic location routing"
```
