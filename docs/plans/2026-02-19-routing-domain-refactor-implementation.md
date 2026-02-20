# Routing Domain Refactor + Coforge Layer 8 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Introduce a `RoutingDomain` interface, migrate all 7 existing routing layers into it, add Coforge as Layer 8, and update tests and docs throughout.

**Architecture:** Everything stays in `package router` (`publisher/internal/router/`). Domain structs replace exported free functions. The router iterates a fixed-order `[]RoutingDomain` slice. Intermediate states always compile and pass tests.

**Tech Stack:** Go 1.26, `github.com/google/uuid`, `github.com/stretchr/testify`

---

## Before You Start

Run the existing tests to establish a green baseline:

```bash
cd publisher && GOWORK=off go test ./internal/router/... -v
```

Expected: all tests pass.

---

## Task 1: Add `RoutingDomain` Interface (`domain.go`)

**Files:**
- Create: `publisher/internal/router/domain.go`

**Step 1: Create the file**

```go
package router

import "github.com/google/uuid"

// ChannelRoute represents a routing decision: a Redis channel name and an optional
// DB channel ID. ChannelID is nil for all auto-generated channels; only
// DBChannelDomain sets it (to link back to the publisher.channels table row).
type ChannelRoute struct {
	Channel   string
	ChannelID *uuid.UUID
}

// RoutingDomain is implemented by each routing layer.
// Routes returns the channels this domain produces for the given article.
// Returning nil or empty means the domain does not apply to this article.
type RoutingDomain interface {
	Name() string
	Routes(a *Article) []ChannelRoute
}

// channelRoutesFromSlice converts a slice of channel name strings to []ChannelRoute
// with nil ChannelIDs. Use this in all domains except DBChannelDomain.
func channelRoutesFromSlice(names []string) []ChannelRoute {
	if len(names) == 0 {
		return nil
	}
	routes := make([]ChannelRoute, len(names))
	for i, name := range names {
		routes[i] = ChannelRoute{Channel: name}
	}
	return routes
}
```

**Step 2: Verify it compiles**

```bash
cd publisher && GOWORK=off go build ./internal/router/...
```

Expected: no errors.

**Step 3: Commit**

```bash
git add publisher/internal/router/domain.go
git commit -m "feat(publisher/router): add RoutingDomain interface and ChannelRoute type"
```

---

## Task 2: Extract `Article` Struct to `article.go`

**Files:**
- Create: `publisher/internal/router/article.go`
- Modify: `publisher/internal/router/service.go`

This is a pure refactor — move types verbatim, no logic changes.

**Step 1: Create `article.go`**

Move these items from `service.go` into `article.go`. Add `CoforgeData` (new) and
`Coforge *CoforgeData` field to `Article`. Everything else is verbatim.

```go
package router

import "time"

// CoforgeData holds Coforge classification fields from Elasticsearch.
type CoforgeData struct {
	Relevance           string   `json:"relevance"`
	RelevanceConfidence float64  `json:"relevance_confidence"`
	Audience            string   `json:"audience"`
	AudienceConfidence  float64  `json:"audience_confidence"`
	Topics              []string `json:"topics"`
	Industries          []string `json:"industries"`
	FinalConfidence     float64  `json:"final_confidence"`
	ReviewRequired      bool     `json:"review_required"`
	ModelVersion        string   `json:"model_version,omitempty"`
}

// MiningData holds mining classification fields from Elasticsearch.
type MiningData struct {
	Relevance       string   `json:"relevance"`
	MiningStage     string   `json:"mining_stage"`
	Commodities     []string `json:"commodities"`
	Location        string   `json:"location"`
	FinalConfidence float64  `json:"final_confidence"`
	ReviewRequired  bool     `json:"review_required"`
	ModelVersion    string   `json:"model_version,omitempty"`
}

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

// AnishinaabeData holds Anishinaabe classification fields from Elasticsearch.
type AnishinaabeData struct {
	Relevance       string   `json:"relevance"`
	Categories      []string `json:"categories"`
	FinalConfidence float64  `json:"final_confidence"`
	ReviewRequired  bool     `json:"review_required"`
	ModelVersion    string   `json:"model_version,omitempty"`
}

// EntertainmentData holds entertainment classification fields from Elasticsearch.
type EntertainmentData struct {
	Relevance        string   `json:"relevance"`
	Categories       []string `json:"categories"`
	FinalConfidence  float64  `json:"final_confidence"`
	HomepageEligible bool     `json:"homepage_eligible"`
	ReviewRequired   bool     `json:"review_required"`
	ModelVersion     string   `json:"model_version,omitempty"`
}

// Article represents an article from Elasticsearch classified_content index.
type Article struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Body          string    `json:"body"`
	RawText       string    `json:"raw_text"`
	RawHTML       string    `json:"raw_html"`
	URL           string    `json:"canonical_url"`
	Source        string    `json:"source"`
	PublishedDate time.Time `json:"published_date"`

	// Classification metadata
	QualityScore     int      `json:"quality_score"`
	Topics           []string `json:"topics"`
	ContentType      string   `json:"content_type"`
	ContentSubtype   string   `json:"content_subtype,omitempty"`
	SourceReputation int      `json:"source_reputation"`
	Confidence       float64  `json:"confidence"`

	// Crime classification (hybrid rule + ML) — flat fields
	CrimeRelevance      string   `json:"crime_relevance"`
	CrimeSubLabel       string   `json:"crime_sub_label,omitempty"`
	CrimeTypes          []string `json:"crime_types"`
	LocationSpecificity string   `json:"location_specificity"`
	HomepageEligible    bool     `json:"homepage_eligible"`
	CategoryPages       []string `json:"category_pages"`
	ReviewRequired      bool     `json:"review_required"`

	// Location detection (content-based) — flat fields
	LocationCity       string  `json:"location_city,omitempty"`
	LocationProvince   string  `json:"location_province,omitempty"`
	LocationCountry    string  `json:"location_country"`
	LocationConfidence float64 `json:"location_confidence"`

	// Nested classifier objects from Elasticsearch
	Crime         *CrimeData         `json:"crime,omitempty"`
	Location      *LocationData      `json:"location,omitempty"`
	Mining        *MiningData        `json:"mining,omitempty"`
	Anishinaabe   *AnishinaabeData   `json:"anishinaabe,omitempty"`
	Entertainment *EntertainmentData `json:"entertainment,omitempty"`
	Coforge       *CoforgeData       `json:"coforge,omitempty"`

	// Entertainment flat fields (populated from nested Entertainment object)
	EntertainmentRelevance        string   `json:"entertainment_relevance"`
	EntertainmentCategories       []string `json:"entertainment_categories"`
	EntertainmentHomepageEligible bool     `json:"entertainment_homepage_eligible"`

	// Open Graph metadata
	OGTitle       string `json:"og_title"`
	OGDescription string `json:"og_description"`
	OGImage       string `json:"og_image"`
	OGURL         string `json:"og_url"`

	// Additional fields
	WordCount int `json:"word_count"`

	// Sort values for search_after pagination
	Sort []any `json:"-"`
}

// extractNestedFields copies values from nested Elasticsearch objects into the
// flat Article fields used by domain routing functions.
// Call after unmarshaling from Elasticsearch.
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

	if a.Entertainment != nil {
		a.EntertainmentRelevance = a.Entertainment.Relevance
		a.EntertainmentCategories = a.Entertainment.Categories
		a.EntertainmentHomepageEligible = a.Entertainment.HomepageEligible
	}
}
```

**Step 2: Remove the moved items from `service.go`**

Delete from `service.go`:
- `MiningData` struct
- `CrimeData` struct
- `LocationData` struct
- `AnishinaabeData` struct
- `EntertainmentData` struct
- `Article` struct
- `extractNestedFields()` method

Do NOT remove `GenerateLayer1Channels` or `layer1SkipTopics` yet — those go in Task 8.

**Step 3: Verify it compiles and tests pass**

```bash
cd publisher && GOWORK=off go test ./internal/router/... -v
```

Expected: all tests pass (no behavioral change, just file reorganisation).

**Step 4: Commit**

```bash
git add publisher/internal/router/article.go publisher/internal/router/service.go
git commit -m "refactor(publisher/router): extract Article struct and data types to article.go"
```

---

## Task 3: Migrate `CrimeDomain` + Add Test Helper

**Files:**
- Create: `publisher/internal/router/testhelpers_test.go`
- Modify: `publisher/internal/router/crime.go`
- Modify: `publisher/internal/router/crime_test.go`

**Step 1: Create shared test helper**

```go
// publisher/internal/router/testhelpers_test.go
//
//nolint:testpackage // Testing internal router requires same package access
package router

// routeChannelNames extracts the Channel field from each ChannelRoute.
// Use in tests that need to assert on channel name strings after a domain.Routes() call.
func routeChannelNames(routes []ChannelRoute) []string {
	names := make([]string, len(routes))
	for i, r := range routes {
		names[i] = r.Channel
	}
	return names
}
```

**Step 2: Update ONE test to use the new domain API (make it fail)**

In `crime_test.go`, update `TestCrimeRouter_Route_HomepageEligible` to call
`NewCrimeDomain().Routes()` instead of `GenerateCrimeChannels()`:

```go
func TestCrimeRouter_Route_HomepageEligible(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:               "test-1",
		Title:            "Murder suspect arrested",
		HomepageEligible: true,
		CrimeRelevance:   "core_street_crime",
		CategoryPages:    []string{"violent-crime", "crime"},
	}

	routes := NewCrimeDomain().Routes(article)
	channels := routeChannelNames(routes)

	if !containsChannel(channels, "crime:homepage") {
		t.Error("expected crime:homepage channel")
	}
	if !containsChannel(channels, "crime:category:violent-crime") {
		t.Error("expected crime:category:violent-crime channel")
	}
}
```

**Step 3: Run the test — it MUST fail**

```bash
cd publisher && GOWORK=off go test ./internal/router/... -run TestCrimeRouter_Route_HomepageEligible -v
```

Expected: `FAIL — NewCrimeDomain undefined`

**Step 4: Add `CrimeDomain` to `crime.go`**

Append to the end of `crime.go`:

```go
// CrimeDomain routes crime-classified articles to crime:* channels.
type CrimeDomain struct{}

// NewCrimeDomain creates a CrimeDomain.
func NewCrimeDomain() *CrimeDomain { return &CrimeDomain{} }

// Name returns the domain identifier used in routing decision logs.
func (d *CrimeDomain) Name() string { return "crime" }

// Routes returns crime channels for the article. Returns nil if the article
// is not crime-classified. Delegates to GenerateCrimeChannels.
func (d *CrimeDomain) Routes(a *Article) []ChannelRoute {
	return channelRoutesFromSlice(GenerateCrimeChannels(a))
}
```

**Step 5: Run the updated test — it MUST pass**

```bash
cd publisher && GOWORK=off go test ./internal/router/... -run TestCrimeRouter_Route_HomepageEligible -v
```

Expected: PASS

**Step 6: Update remaining crime tests to use domain API**

Update all other `GenerateCrimeChannels(article)` call sites in `crime_test.go`:

```go
// Pattern for every remaining test — replace:
//   channels := GenerateCrimeChannels(article)
// with:
//   routes := NewCrimeDomain().Routes(article)
//   channels := routeChannelNames(routes)
```

Also update `TestArticle_FullUnmarshalPipeline` which calls both
`GenerateCrimeChannels` and `GenerateLocationChannels`:

```go
// Replace GenerateCrimeChannels call:
crimeRoutes := NewCrimeDomain().Routes(&article)
crimeChannels := routeChannelNames(crimeRoutes)
assert.Contains(t, crimeChannels, "crime:homepage")
assert.Contains(t, crimeChannels, "crime:category:drug-crime")
assert.Contains(t, crimeChannels, "crime:category:crime")

// Leave GenerateLocationChannels for now — LocationDomain migrated in Task 6
locationChannels := GenerateLocationChannels(&article)
assert.Contains(t, locationChannels, "crime:local:toronto")
```

**Step 7: Run all router tests**

```bash
cd publisher && GOWORK=off go test ./internal/router/... -v
```

Expected: all pass.

**Step 8: Commit**

```bash
git add publisher/internal/router/crime.go \
        publisher/internal/router/crime_test.go \
        publisher/internal/router/testhelpers_test.go
git commit -m "feat(publisher/router): migrate CrimeDomain to RoutingDomain interface"
```

---

## Task 4: Migrate `MiningDomain`

**Files:**
- Modify: `publisher/internal/router/mining.go`
- Modify: `publisher/internal/router/mining_test.go`

**Step 1: Update ONE test (make it fail)**

In `mining_test.go`, update `TestGenerateMiningChannels_NotMining` to use the domain:

```go
func TestGenerateMiningChannels_NotMining(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:    "test-mining-3",
		Title: "Weather forecast",
		Mining: &MiningData{
			Relevance: MiningRelevanceNotMining,
		},
	}

	routes := NewMiningDomain().Routes(article)

	if len(routes) != 0 {
		t.Errorf("expected no channels for not_mining, got %v", routeChannelNames(routes))
	}
}
```

**Step 2: Run test — must fail**

```bash
cd publisher && GOWORK=off go test ./internal/router/... -run TestGenerateMiningChannels_NotMining -v
```

Expected: `FAIL — NewMiningDomain undefined`

**Step 3: Add `MiningDomain` to `mining.go`**

Append to the end of `mining.go`:

```go
// MiningDomain routes mining-classified articles to mining:* channels.
type MiningDomain struct{}

// NewMiningDomain creates a MiningDomain.
func NewMiningDomain() *MiningDomain { return &MiningDomain{} }

// Name returns the domain identifier.
func (d *MiningDomain) Name() string { return "mining" }

// Routes returns mining channels for the article. Returns nil if the article
// is not mining-classified. Delegates to GenerateMiningChannels.
func (d *MiningDomain) Routes(a *Article) []ChannelRoute {
	return channelRoutesFromSlice(GenerateMiningChannels(a))
}
```

**Step 4: Run test — must pass**

```bash
cd publisher && GOWORK=off go test ./internal/router/... -run TestGenerateMiningChannels_NotMining -v
```

**Step 5: Update all remaining `mining_test.go` tests**

Replace every `GenerateMiningChannels(article)` with:
```go
routes := NewMiningDomain().Routes(article)
channels := routeChannelNames(routes)
```

Then update all assertions that previously used `channels []string` to use the
new `channels` variable. The helpers `assertChannelsEqual`, `assertContains`,
`assertNotContains` still work on `[]string` — no changes needed to those.

**Step 6: Run all router tests**

```bash
cd publisher && GOWORK=off go test ./internal/router/... -v
```

**Step 7: Commit**

```bash
git add publisher/internal/router/mining.go publisher/internal/router/mining_test.go
git commit -m "feat(publisher/router): migrate MiningDomain to RoutingDomain interface"
```

---

## Task 5: Migrate `EntertainmentDomain`

**Files:**
- Modify: `publisher/internal/router/entertainment.go`
- Modify: `publisher/internal/router/entertainment_test.go`

**Step 1: Update ONE test (make it fail)**

```go
func TestGenerateEntertainmentChannels_Peripheral(t *testing.T) {
	t.Helper()
	article := &Article{
		Title: "Arts news",
		Entertainment: &EntertainmentData{
			Relevance: EntertainmentRelevancePeripheral,
		},
	}

	routes := NewEntertainmentDomain().Routes(article)
	channels := routeChannelNames(routes)

	if len(channels) != 1 || channels[0] != "entertainment:peripheral" {
		t.Errorf("expected [entertainment:peripheral], got %v", channels)
	}
}
```

**Step 2: Run test — must fail**

```bash
cd publisher && GOWORK=off go test ./internal/router/... -run TestGenerateEntertainmentChannels_Peripheral -v
```

**Step 3: Add `EntertainmentDomain` to `entertainment.go`**

```go
// EntertainmentDomain routes entertainment-classified articles to entertainment:* channels.
type EntertainmentDomain struct{}

// NewEntertainmentDomain creates an EntertainmentDomain.
func NewEntertainmentDomain() *EntertainmentDomain { return &EntertainmentDomain{} }

// Name returns the domain identifier.
func (d *EntertainmentDomain) Name() string { return "entertainment" }

// Routes returns entertainment channels for the article. Returns nil if the article
// is not entertainment-classified. Delegates to GenerateEntertainmentChannels.
func (d *EntertainmentDomain) Routes(a *Article) []ChannelRoute {
	return channelRoutesFromSlice(GenerateEntertainmentChannels(a))
}
```

**Step 4: Run test — must pass, then update all remaining tests**

Same pattern: replace `GenerateEntertainmentChannels(article)` with
`routeChannelNames(NewEntertainmentDomain().Routes(article))`.

**Step 5: Run all router tests, then commit**

```bash
cd publisher && GOWORK=off go test ./internal/router/... -v
git add publisher/internal/router/entertainment.go publisher/internal/router/entertainment_test.go
git commit -m "feat(publisher/router): migrate EntertainmentDomain to RoutingDomain interface"
```

---

## Task 6: Migrate `LocationDomain`

**Files:**
- Modify: `publisher/internal/router/location.go`
- Modify: `publisher/internal/router/location_test.go`
- Modify: `publisher/internal/router/crime_test.go` (finishes the remaining call)

**Step 1: Update ONE test (make it fail)**

```go
func TestGenerateLocationChannels_CrimeInternational(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:                  "loc-crime-intl",
		CrimeRelevance:      "core_street_crime",
		LocationCountry:     "united_states",
		LocationSpecificity: "country",
	}

	routes := NewLocationDomain().Routes(article)
	channels := routeChannelNames(routes)

	assert.Equal(t, []string{"crime:international"}, channels)
}
```

**Step 2: Run test — must fail**

```bash
cd publisher && GOWORK=off go test ./internal/router/... -run TestGenerateLocationChannels_CrimeInternational -v
```

**Step 3: Add `LocationDomain` to `location.go`**

```go
// LocationDomain routes articles to geographic channels for active domain classifiers.
// Active classifiers are crime and entertainment; mining is excluded because
// MiningDomain already generates mining:canada / mining:international.
type LocationDomain struct{}

// NewLocationDomain creates a LocationDomain.
func NewLocationDomain() *LocationDomain { return &LocationDomain{} }

// Name returns the domain identifier.
func (d *LocationDomain) Name() string { return "location" }

// Routes returns geographic channels for articles with an active domain classifier
// and a known location. Delegates to GenerateLocationChannels.
func (d *LocationDomain) Routes(a *Article) []ChannelRoute {
	return channelRoutesFromSlice(GenerateLocationChannels(a))
}
```

**Step 4: Run test — must pass, then update all remaining tests**

In `location_test.go`: replace `GenerateLocationChannels(article)` with
`routeChannelNames(NewLocationDomain().Routes(article))`.

In `crime_test.go` (`TestArticle_FullUnmarshalPipeline`): finish the update
by replacing the remaining `GenerateLocationChannels(&article)` call:

```go
locationRoutes := NewLocationDomain().Routes(&article)
locationChannels := routeChannelNames(locationRoutes)
assert.Contains(t, locationChannels, "crime:local:toronto")
assert.Contains(t, locationChannels, "crime:province:on")
assert.Contains(t, locationChannels, "crime:canada")
```

**Step 5: Run all router tests, then commit**

```bash
cd publisher && GOWORK=off go test ./internal/router/... -v
git add publisher/internal/router/location.go \
        publisher/internal/router/location_test.go \
        publisher/internal/router/crime_test.go
git commit -m "feat(publisher/router): migrate LocationDomain to RoutingDomain interface"
```

---

## Task 7: Migrate `AnishinaabeeDomain`

**Files:**
- Modify: `publisher/internal/router/anishinaabe.go`
- Modify: `publisher/internal/router/anishinaabe_test.go`

**Step 1: Update ONE test (make it fail)**

```go
func TestGenerateAnishinaabeChannels_NotAnishinaabe(t *testing.T) {
	t.Helper()
	article := &Article{
		Title: "Weather report",
		Anishinaabe: &AnishinaabeData{
			Relevance: AnishinaabeRelevanceNot,
		},
	}

	routes := NewAnishinaabeeDomain().Routes(article)
	if len(routes) != 0 {
		t.Errorf("expected no channels, got %v", routeChannelNames(routes))
	}
}
```

**Step 2: Run test — must fail**

```bash
cd publisher && GOWORK=off go test ./internal/router/... -run TestGenerateAnishinaabeChannels_NotAnishinaabe -v
```

**Step 3: Add `AnishinaabeeDomain` to `anishinaabe.go`**

```go
// AnishinaabeeDomain routes Anishinaabe-classified articles to anishinaabe:* channels.
type AnishinaabeeDomain struct{}

// NewAnishinaabeeDomain creates an AnishinaabeeDomain.
func NewAnishinaabeeDomain() *AnishinaabeeDomain { return &AnishinaabeeDomain{} }

// Name returns the domain identifier.
func (d *AnishinaabeeDomain) Name() string { return "anishinaabe" }

// Routes returns Anishinaabe channels for the article. Returns nil if the article
// is not Anishinaabe-classified. Delegates to GenerateAnishinaabeChannels.
func (d *AnishinaabeeDomain) Routes(a *Article) []ChannelRoute {
	return channelRoutesFromSlice(GenerateAnishinaabeChannels(a))
}
```

**Step 4: Run test — must pass, then update all remaining tests**

Replace `GenerateAnishinaabeChannels(article)` with
`routeChannelNames(NewAnishinaabeeDomain().Routes(article))` throughout
`anishinaabe_test.go`.

**Step 5: Run all router tests, then commit**

```bash
cd publisher && GOWORK=off go test ./internal/router/... -v
git add publisher/internal/router/anishinaabe.go publisher/internal/router/anishinaabe_test.go
git commit -m "feat(publisher/router): migrate AnishinaabeeDomain to RoutingDomain interface"
```

---

## Task 8: Add `TopicDomain` (Layer 1)

**Files:**
- Create: `publisher/internal/router/domain_topic.go`
- Create: `publisher/internal/router/domain_topic_test.go`
- Modify: `publisher/internal/router/service.go` (remove `layer1SkipTopics` var only)
- Modify: `publisher/internal/router/service_test.go`
- Modify: `publisher/internal/router/integration_test.go`

**Step 1: Write failing test**

```go
// publisher/internal/router/domain_topic_test.go
package router_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/publisher/internal/router"
	"github.com/stretchr/testify/assert"
)

func TestTopicDomain_Name(t *testing.T) {
	t.Helper()
	assert.Equal(t, "topic", router.NewTopicDomain().Name())
}

func TestTopicDomain_Routes(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		topics   []string
		expected []string
	}{
		{
			name:     "multiple topics produce articles: channels",
			topics:   []string{"violent_crime", "local_news"},
			expected: []string{"articles:violent_crime", "articles:local_news"},
		},
		{
			name:     "empty topics produce no channels",
			topics:   []string{},
			expected: nil,
		},
		{
			name:     "mining topic is skipped",
			topics:   []string{"news", "mining", "technology"},
			expected: []string{"articles:news", "articles:technology"},
		},
		{
			name:     "anishinaabe topic is skipped",
			topics:   []string{"news", "anishinaabe"},
			expected: []string{"articles:news"},
		},
		{
			name:     "coforge topic is skipped",
			topics:   []string{"news", "coforge"},
			expected: []string{"articles:news"},
		},
		{
			name:     "all skip topics produce no channels",
			topics:   []string{"mining", "anishinaabe", "coforge"},
			expected: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			article := &router.Article{Topics: tc.topics}
			routes := router.NewTopicDomain().Routes(article)
			var names []string
			for _, r := range routes {
				names = append(names, r.Channel)
			}
			assert.Equal(t, tc.expected, names)
		})
	}
}
```

**Step 2: Run test — must fail**

```bash
cd publisher && GOWORK=off go test ./internal/router/... -run TestTopicDomain -v
```

Expected: `FAIL — NewTopicDomain undefined`

**Step 3: Create `domain_topic.go`**

```go
package router

// layer1SkipTopics lists topics handled by dedicated routing layers.
// These topics are excluded from Layer 1 auto-routing to prevent bypassing
// their specialised classifiers (e.g., coforge → Layer 8).
var layer1SkipTopics = map[string]bool{
	"mining":      true,
	"anishinaabe": true,
	"coforge":     true,
}

// TopicDomain routes articles to articles:{topic} for each non-skipped topic tag.
// This is Layer 1 in the routing pipeline.
type TopicDomain struct{}

// NewTopicDomain creates a TopicDomain.
func NewTopicDomain() *TopicDomain { return &TopicDomain{} }

// Name returns the domain identifier.
func (d *TopicDomain) Name() string { return "topic" }

// Routes returns an articles:{topic} channel for each topic not in layer1SkipTopics.
func (d *TopicDomain) Routes(a *Article) []ChannelRoute {
	names := make([]string, 0, len(a.Topics))
	for _, topic := range a.Topics {
		if layer1SkipTopics[topic] {
			continue
		}
		names = append(names, "articles:"+topic)
	}
	return channelRoutesFromSlice(names)
}
```

**Step 4: Remove `layer1SkipTopics` from `service.go`**

Delete only the `layer1SkipTopics` var block from `service.go`. Leave
`GenerateLayer1Channels` — it now references `layer1SkipTopics` from `domain_topic.go`
(same package, so this compiles immediately).

**Step 5: Run test — must pass**

```bash
cd publisher && GOWORK=off go test ./internal/router/... -run TestTopicDomain -v
```

**Step 6: Update `service_test.go` Layer 1 calls**

In `service_test.go`, replace `router.GenerateLayer1Channels(tc.article)` with
`router.NewTopicDomain().Routes(tc.article)`. Add a local helper at the bottom of
the file to convert routes:

```go
// channelNames extracts Channel strings from a []router.ChannelRoute.
func channelNames(routes []router.ChannelRoute) []string {
	names := make([]string, len(routes))
	for i, r := range routes {
		names[i] = r.Channel
	}
	return names
}
```

Then update all assertions from `channels[i]` → `channelNames(routes)[i]`.

**Step 7: Update `integration_test.go` Layer 1 calls**

Same helper pattern. Replace `router.GenerateLayer1Channels(tc.article)` with
`router.NewTopicDomain().Routes(tc.article)` throughout. Add `channelNames` helper
at the bottom if it's not already there.

**Step 8: Run all router tests**

```bash
cd publisher && GOWORK=off go test ./internal/router/... -v
```

Expected: all pass.

**Step 9: Commit**

```bash
git add publisher/internal/router/domain_topic.go \
        publisher/internal/router/domain_topic_test.go \
        publisher/internal/router/service.go \
        publisher/internal/router/service_test.go \
        publisher/internal/router/integration_test.go
git commit -m "feat(publisher/router): add TopicDomain (Layer 1), add coforge to skip list"
```

---

## Task 9: Add `DBChannelDomain` (Layer 2)

**Files:**
- Create: `publisher/internal/router/domain_dbchannel.go`
- Create: `publisher/internal/router/domain_dbchannel_test.go`

**Step 1: Write failing test**

```go
// publisher/internal/router/domain_dbchannel_test.go
package router_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/publisher/internal/models"
	"github.com/jonesrussell/north-cloud/publisher/internal/router"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDBChannelDomain_Name(t *testing.T) {
	t.Helper()
	assert.Equal(t, "db_channel", router.NewDBChannelDomain(nil).Name())
}

func TestDBChannelDomain_Routes(t *testing.T) {
	t.Helper()

	crimeChannel := models.Channel{
		ID:           uuid.New(),
		RedisChannel: "articles:crime:all",
		Rules: models.Rules{
			IncludeTopics:   []string{"violent_crime", "property_crime"},
			MinQualityScore: 50,
			ContentTypes:    []string{"article"},
		},
		Enabled: true,
	}
	premiumChannel := models.Channel{
		ID:           uuid.New(),
		RedisChannel: "articles:premium",
		Rules:        models.Rules{MinQualityScore: 80},
		Enabled:      true,
	}

	tests := []struct {
		name             string
		article          *router.Article
		channels         []models.Channel
		expectedChannels []string
		expectChannelIDs bool
	}{
		{
			name: "matching channel produces route with ChannelID set",
			article: &router.Article{
				Topics:       []string{"violent_crime"},
				QualityScore: 75,
				ContentType:  "article",
			},
			channels:         []models.Channel{crimeChannel},
			expectedChannels: []string{"articles:crime:all"},
			expectChannelIDs: true,
		},
		{
			name: "no match returns nil",
			article: &router.Article{
				Topics:       []string{"technology"},
				QualityScore: 40,
				ContentType:  "article",
			},
			channels:         []models.Channel{crimeChannel, premiumChannel},
			expectedChannels: nil,
		},
		{
			name: "multiple matching channels",
			article: &router.Article{
				Topics:       []string{"violent_crime"},
				QualityScore: 90,
				ContentType:  "article",
			},
			channels:         []models.Channel{crimeChannel, premiumChannel},
			expectedChannels: []string{"articles:crime:all", "articles:premium"},
			expectChannelIDs: true,
		},
		{
			name:             "nil channel list returns nil",
			article:          &router.Article{Topics: []string{"news"}},
			channels:         nil,
			expectedChannels: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			domain := router.NewDBChannelDomain(tc.channels)
			routes := domain.Routes(tc.article)

			if tc.expectedChannels == nil {
				assert.Nil(t, routes)
				return
			}

			require.Len(t, routes, len(tc.expectedChannels))
			for i, r := range routes {
				assert.Equal(t, tc.expectedChannels[i], r.Channel)
				if tc.expectChannelIDs {
					assert.NotNil(t, r.ChannelID, "ChannelID must be set by DBChannelDomain")
				}
			}
		})
	}
}
```

**Step 2: Run test — must fail**

```bash
cd publisher && GOWORK=off go test ./internal/router/... -run TestDBChannelDomain -v
```

Expected: `FAIL — NewDBChannelDomain undefined`

**Step 3: Create `domain_dbchannel.go`**

```go
package router

import (
	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/publisher/internal/models"
)

// DBChannelDomain routes articles using database-configured custom channels (Layer 2).
// It is the only domain that produces ChannelRoute values with non-nil ChannelIDs,
// which link publish_history records back to the publisher.channels table.
type DBChannelDomain struct {
	channels []models.Channel
}

// NewDBChannelDomain creates a DBChannelDomain loaded with the current channel
// configuration. channels should be refreshed from the database at each poll cycle.
func NewDBChannelDomain(channels []models.Channel) *DBChannelDomain {
	return &DBChannelDomain{channels: channels}
}

// Name returns the domain identifier.
func (d *DBChannelDomain) Name() string { return "db_channel" }

// Routes returns ChannelRoutes for each custom channel whose rules match the article.
// Each route carries the DB channel ID so that publish_history can reference it.
func (d *DBChannelDomain) Routes(a *Article) []ChannelRoute {
	routes := make([]ChannelRoute, 0, len(d.channels))
	for i := range d.channels {
		ch := &d.channels[i]
		if ch.Rules.Matches(a.QualityScore, a.ContentType, a.Topics) {
			id := ch.ID // copy UUID to avoid loop variable address reuse
			routes = append(routes, ChannelRoute{
				Channel:   ch.RedisChannel,
				ChannelID: &id,
			})
		}
	}
	if len(routes) == 0 {
		return nil
	}
	return routes
}

// Ensure DBChannelDomain implements RoutingDomain at compile time.
var _ RoutingDomain = (*DBChannelDomain)(nil)

// Ensure uuid import is used (ChannelID field type).
var _ *uuid.UUID
```

Wait — the `var _ *uuid.UUID` trick is ugly. Drop it; uuid is already used through the `ChannelRoute` type in `domain.go`. Actually `DBChannelDomain` imports uuid only for the `id := ch.ID` copy. `ch.ID` is `uuid.UUID` from the models package — we need uuid imported because `ChannelRoute.ChannelID` is `*uuid.UUID` and we take `&id`. Actually uuid is imported in `domain.go`, not `domain_dbchannel.go`. In `domain_dbchannel.go`, `id` is `uuid.UUID` inferred from `ch.ID`, and `&id` is `*uuid.UUID`. So uuid IS needed. Drop the fake var:

```go
package router

import (
	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/publisher/internal/models"
)

// DBChannelDomain routes articles using database-configured custom channels (Layer 2).
// It is the only domain that produces ChannelRoute values with non-nil ChannelIDs,
// linking publish_history records back to the publisher.channels table.
type DBChannelDomain struct {
	channels []models.Channel
}

// NewDBChannelDomain creates a DBChannelDomain with the current channel configuration.
// channels should be refreshed from the database at each poll cycle.
func NewDBChannelDomain(channels []models.Channel) *DBChannelDomain {
	return &DBChannelDomain{channels: channels}
}

// Name returns the domain identifier.
func (d *DBChannelDomain) Name() string { return "db_channel" }

// Routes returns ChannelRoutes for each custom channel whose rules match the article.
// Each route carries the DB channel UUID so publish_history can reference it.
func (d *DBChannelDomain) Routes(a *Article) []ChannelRoute {
	routes := make([]ChannelRoute, 0, len(d.channels))
	for i := range d.channels {
		ch := &d.channels[i]
		if ch.Rules.Matches(a.QualityScore, a.ContentType, a.Topics) {
			id := ch.ID
			routes = append(routes, ChannelRoute{
				Channel:   ch.RedisChannel,
				ChannelID: &id,
			})
		}
	}
	if len(routes) == 0 {
		return nil
	}
	return routes
}

// compile-time interface check
var _ RoutingDomain = (*DBChannelDomain)(nil)

// imported for uuid.UUID type used in id copy above
var _ uuid.UUID
```

Actually the `var _ uuid.UUID` line will be flagged by the linter as an unused blank identifier. Cleaner: just let the compiler infer. Remove the blank identifiers and keep just the interface check:

```go
package router

import (
	"github.com/jonesrussell/north-cloud/publisher/internal/models"
)

// DBChannelDomain routes articles using database-configured custom channels (Layer 2).
// It is the only domain that produces ChannelRoute values with non-nil ChannelIDs.
type DBChannelDomain struct {
	channels []models.Channel
}

// NewDBChannelDomain creates a DBChannelDomain with the current channel configuration.
func NewDBChannelDomain(channels []models.Channel) *DBChannelDomain {
	return &DBChannelDomain{channels: channels}
}

// Name returns the domain identifier.
func (d *DBChannelDomain) Name() string { return "db_channel" }

// Routes returns ChannelRoutes for each custom channel whose rules match the article.
// Each route carries a non-nil ChannelID referencing the publisher.channels DB row.
func (d *DBChannelDomain) Routes(a *Article) []ChannelRoute {
	routes := make([]ChannelRoute, 0, len(d.channels))
	for i := range d.channels {
		ch := &d.channels[i]
		if ch.Rules.Matches(a.QualityScore, a.ContentType, a.Topics) {
			id := ch.ID // copy to avoid loop variable address reuse
			routes = append(routes, ChannelRoute{
				Channel:   ch.RedisChannel,
				ChannelID: &id,
			})
		}
	}
	if len(routes) == 0 {
		return nil
	}
	return routes
}

// compile-time interface check
var _ RoutingDomain = (*DBChannelDomain)(nil)
```

`ch.ID` is `uuid.UUID` from the models package — the `uuid` package is imported
transitively through `models.Channel`. No direct import needed here.

**Step 4: Run test — must pass**

```bash
cd publisher && GOWORK=off go test ./internal/router/... -run TestDBChannelDomain -v
```

**Step 5: Run all router tests**

```bash
cd publisher && GOWORK=off go test ./internal/router/... -v
```

**Step 6: Commit**

```bash
git add publisher/internal/router/domain_dbchannel.go \
        publisher/internal/router/domain_dbchannel_test.go
git commit -m "feat(publisher/router): add DBChannelDomain (Layer 2)"
```

---

## Task 10: Add `CoforgeDomain` (Layer 8)

**Files:**
- Create: `publisher/internal/router/domain_coforge.go`
- Create: `publisher/internal/router/domain_coforge_test.go`

**Step 1: Write failing test**

```go
// publisher/internal/router/domain_coforge_test.go
package router_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/publisher/internal/router"
	"github.com/stretchr/testify/assert"
)

func TestCoforgeDomain_Name(t *testing.T) {
	t.Helper()
	assert.Equal(t, "coforge", router.NewCoforgeDomain().Name())
}

func TestCoforgeDomain_Routes(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		article  *router.Article
		expected []string
	}{
		{
			name:     "nil coforge data returns nil",
			article:  &router.Article{},
			expected: nil,
		},
		{
			name: "not_relevant returns nil",
			article: &router.Article{
				Coforge: &router.CoforgeData{Relevance: "not_relevant"},
			},
			expected: nil,
		},
		{
			name: "empty relevance returns nil",
			article: &router.Article{
				Coforge: &router.CoforgeData{Relevance: ""},
			},
			expected: nil,
		},
		{
			name: "core_coforge produces coforge:core",
			article: &router.Article{
				Coforge: &router.CoforgeData{
					Relevance: "core_coforge",
					Audience:  "developer",
				},
			},
			expected: []string{"coforge:core", "coforge:audience:developer"},
		},
		{
			name: "peripheral produces coforge:peripheral",
			article: &router.Article{
				Coforge: &router.CoforgeData{
					Relevance: "peripheral",
					Audience:  "entrepreneur",
				},
			},
			expected: []string{"coforge:peripheral", "coforge:audience:entrepreneur"},
		},
		{
			name: "hybrid audience",
			article: &router.Article{
				Coforge: &router.CoforgeData{
					Relevance: "core_coforge",
					Audience:  "hybrid",
				},
			},
			expected: []string{"coforge:core", "coforge:audience:hybrid"},
		},
		{
			name: "topics produce slugified channels",
			article: &router.Article{
				Coforge: &router.CoforgeData{
					Relevance: "core_coforge",
					Audience:  "developer",
					Topics:    []string{"framework_release", "open_source"},
				},
			},
			expected: []string{
				"coforge:core",
				"coforge:audience:developer",
				"coforge:topic:framework-release",
				"coforge:topic:open-source",
			},
		},
		{
			name: "industries produce slugified channels",
			article: &router.Article{
				Coforge: &router.CoforgeData{
					Relevance:  "core_coforge",
					Audience:   "hybrid",
					Industries: []string{"ai_ml", "saas"},
				},
			},
			expected: []string{
				"coforge:core",
				"coforge:audience:hybrid",
				"coforge:industry:ai-ml",
				"coforge:industry:saas",
			},
		},
		{
			name: "full classification produces all channels",
			article: &router.Article{
				Coforge: &router.CoforgeData{
					Relevance:  "core_coforge",
					Audience:   "hybrid",
					Topics:     []string{"funding_round", "devtools"},
					Industries: []string{"saas", "ai_ml"},
				},
			},
			expected: []string{
				"coforge:core",
				"coforge:audience:hybrid",
				"coforge:topic:funding-round",
				"coforge:topic:devtools",
				"coforge:industry:saas",
				"coforge:industry:ai-ml",
			},
		},
		{
			name: "no articles:coforge catch-all is produced",
			article: &router.Article{
				Coforge: &router.CoforgeData{Relevance: "core_coforge", Audience: "developer"},
			},
			// assert that articles:coforge is NOT in the output
			expected: []string{"coforge:core", "coforge:audience:developer"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			routes := router.NewCoforgeDomain().Routes(tc.article)
			if tc.expected == nil {
				assert.Nil(t, routes)
				return
			}
			var names []string
			for _, r := range routes {
				names = append(names, r.Channel)
			}
			assert.Equal(t, tc.expected, names)
			// Verify no catch-all channel is generated
			for _, name := range names {
				assert.NotEqual(t, "articles:coforge", name,
					"CoforgeDomain must not produce a catch-all articles:coforge channel")
			}
		})
	}
}
```

**Step 2: Run test — must fail**

```bash
cd publisher && GOWORK=off go test ./internal/router/... -run TestCoforgeDomain -v
```

**Step 3: Create `domain_coforge.go`**

```go
package router

import "strings"

// Coforge relevance constants.
const (
	CoforgeRelevanceNotRelevant = "not_relevant"
	CoforgeRelevanceCore        = "core_coforge"
	CoforgeRelevancePeripheral  = "peripheral"
)

// CoforgeDomain routes Coforge-classified articles to coforge:* channels.
//
// Coforge is a product-specific routing domain — not a public topic domain.
// It does NOT produce a catch-all articles:coforge channel. Entry points are
// coforge:core and coforge:peripheral, plus audience, topic, and industry sub-channels.
type CoforgeDomain struct{}

// NewCoforgeDomain creates a CoforgeDomain.
func NewCoforgeDomain() *CoforgeDomain { return &CoforgeDomain{} }

// Name returns the domain identifier.
func (d *CoforgeDomain) Name() string { return "coforge" }

// Routes returns Coforge channels for the article.
// Returns nil if Coforge data is absent or relevance is not_relevant.
func (d *CoforgeDomain) Routes(a *Article) []ChannelRoute {
	if a.Coforge == nil {
		return nil
	}

	rel := a.Coforge.Relevance
	if rel == CoforgeRelevanceNotRelevant || rel == "" {
		return nil
	}

	names := make([]string, 0)

	// Relevance channel — coforge:core or coforge:peripheral
	switch rel {
	case CoforgeRelevanceCore:
		names = append(names, "coforge:core")
	case CoforgeRelevancePeripheral:
		names = append(names, "coforge:peripheral")
	}

	// Audience channel
	if a.Coforge.Audience != "" {
		names = append(names, "coforge:audience:"+a.Coforge.Audience)
	}

	// Topic channels — underscores converted to hyphens for slug format
	for _, topic := range a.Coforge.Topics {
		slug := strings.ToLower(strings.ReplaceAll(topic, "_", "-"))
		if slug != "" {
			names = append(names, "coforge:topic:"+slug)
		}
	}

	// Industry channels — underscores converted to hyphens for slug format
	for _, industry := range a.Coforge.Industries {
		slug := strings.ToLower(strings.ReplaceAll(industry, "_", "-"))
		if slug != "" {
			names = append(names, "coforge:industry:"+slug)
		}
	}

	return channelRoutesFromSlice(names)
}

// compile-time interface check
var _ RoutingDomain = (*CoforgeDomain)(nil)
```

**Step 4: Run test — must pass**

```bash
cd publisher && GOWORK=off go test ./internal/router/... -run TestCoforgeDomain -v
```

**Step 5: Run all router tests**

```bash
cd publisher && GOWORK=off go test ./internal/router/... -v
```

**Step 6: Commit**

```bash
git add publisher/internal/router/domain_coforge.go \
        publisher/internal/router/domain_coforge_test.go
git commit -m "feat(publisher/router): add CoforgeDomain (Layer 8)"
```

---

## Task 11: Refactor `service.go` to Use Domain Slice

This is the central wiring task. It:
- Replaces the 7-layer explicit calls in `routeArticle` with a domain slice loop
- Adds `publishRoutes` (replaces `publishToChannels`)
- Adds routing decision log and channel count guardrail
- Removes all exported free functions from domain files
- Adds `"coforge"` to the Redis payload in `publishToChannel`

**Files:**
- Modify: `publisher/internal/router/service.go`
- Modify: `publisher/internal/router/crime.go`
- Modify: `publisher/internal/router/mining.go`
- Modify: `publisher/internal/router/entertainment.go`
- Modify: `publisher/internal/router/location.go`
- Modify: `publisher/internal/router/anishinaabe.go`
- Modify: `publisher/internal/router/integration_test.go`

**Step 1: Update `routeArticle` in `service.go`**

Replace the entire `routeArticle` function with:

```go
// maxChannelsPerArticle is the guardrail threshold for routing fanout warnings.
const maxChannelsPerArticle = 30

// routeArticle routes a single article through all routing domains (Layers 1–8)
// and returns the list of Redis channels where publishing succeeded.
func (s *Service) routeArticle(ctx context.Context, article *Article, channels []models.Channel) []string {
	domains := []RoutingDomain{
		NewTopicDomain(),           // Layer 1: articles:{topic}
		NewDBChannelDomain(channels), // Layer 2: custom DB-backed channels
		NewCrimeDomain(),           // Layer 3: crime:homepage, crime:category:*
		NewLocationDomain(),        // Layer 4: {topic}:local:*, province:*, canada, international
		NewMiningDomain(),          // Layer 5: mining:core, mining:commodity:*, etc.
		NewEntertainmentDomain(),   // Layer 6: entertainment:homepage, category:*, peripheral
		NewAnishinaabeeDomain(),    // Layer 7: articles:anishinaabe, category:*
		NewCoforgeDomain(),         // Layer 8: coforge:core, coforge:audience:*, etc.
	}

	var published []string
	for _, domain := range domains {
		routes := domain.Routes(article)
		s.logger.Debug("routing decision",
			infralogger.String("domain", domain.Name()),
			infralogger.String("article_id", article.ID),
			infralogger.Int("channels", len(routes)),
		)
		published = append(published, s.publishRoutes(ctx, article, routes)...)
	}

	if len(published) > maxChannelsPerArticle {
		s.logger.Warn("article produced excessive channel count",
			infralogger.String("article_id", article.ID),
			infralogger.Int("count", len(published)),
		)
	}

	s.emitPublishedEvent(ctx, article, published)
	return published
}
```

**Step 2: Replace `publishToChannels` with `publishRoutes` in `service.go`**

Remove the old `publishToChannels` method. Add:

```go
// publishRoutes publishes an article to each route and returns the names of channels
// where publishing succeeded.
func (s *Service) publishRoutes(ctx context.Context, article *Article, routes []ChannelRoute) []string {
	published := make([]string, 0, len(routes))
	for _, route := range routes {
		if s.publishToChannel(ctx, article, route.Channel, route.ChannelID) {
			published = append(published, route.Channel)
		}
	}
	return published
}
```

**Step 3: Add `"coforge"` to the Redis payload in `publishToChannel`**

In the `payload` map inside `publishToChannel`, add alongside the existing
`"mining"`, `"anishinaabe"`, and `"entertainment"` entries:

```go
// Coforge classification
"coforge": article.Coforge,
```

**Step 4: Remove `GenerateLayer1Channels` from `service.go`**

Delete the `GenerateLayer1Channels` function — it was moved to `TopicDomain.Routes()`.

**Step 5: Remove exported free functions from domain files**

For each domain file, remove the exported free function and update the domain's
`Routes()` method to inline the logic directly (no delegation):

**`crime.go`** — remove `GenerateCrimeChannels`, inline in `CrimeDomain.Routes()`:

```go
func (d *CrimeDomain) Routes(a *Article) []ChannelRoute {
	if a.CrimeRelevance == CrimeRelevanceNotCrime || a.CrimeRelevance == "" {
		return nil
	}

	channels := make([]string, 0)

	if a.CrimeRelevance == CrimeRelevancePeripheral {
		switch a.CrimeSubLabel {
		case SubLabelCriminalJustice:
			channels = append(channels, "crime:courts")
		case SubLabelCrimeContext:
			channels = append(channels, "crime:context")
		default:
			channels = append(channels, "crime:context")
		}
		return channelRoutesFromSlice(channels)
	}

	if a.HomepageEligible {
		channels = append(channels, "crime:homepage")
	}
	for _, category := range a.CategoryPages {
		channels = append(channels, fmt.Sprintf("crime:category:%s", category))
	}
	return channelRoutesFromSlice(channels)
}
```

**`mining.go`** — remove `GenerateMiningChannels`, delegate to the existing private helpers:

```go
func (d *MiningDomain) Routes(a *Article) []ChannelRoute {
	if a.Mining == nil {
		return nil
	}
	rel := a.Mining.Relevance
	if rel == MiningRelevanceNotMining || rel == "" {
		return nil
	}
	channels := []string{"articles:mining"}
	channels = appendRelevanceChannel(channels, rel)
	channels = appendCommodityChannels(channels, a.Mining.Commodities)
	channels = appendStageChannel(channels, a.Mining.MiningStage)
	channels = appendMiningLocationChannel(channels, a.Mining.Location)
	return channelRoutesFromSlice(channels)
}
```

**`entertainment.go`** — remove `GenerateEntertainmentChannels`, inline in `Routes()`:

```go
func (d *EntertainmentDomain) Routes(a *Article) []ChannelRoute {
	if a.Entertainment == nil {
		return nil
	}
	rel := a.Entertainment.Relevance
	if rel == EntertainmentRelevanceNot || rel == "" {
		return nil
	}
	var channels []string
	if rel == EntertainmentRelevanceCore && a.Entertainment.HomepageEligible {
		channels = append(channels, "entertainment:homepage")
	}
	for _, cat := range a.Entertainment.Categories {
		slug := strings.ToLower(strings.ReplaceAll(cat, " ", "-"))
		if slug != "" {
			channels = append(channels, "entertainment:category:"+slug)
		}
	}
	if rel == EntertainmentRelevancePeripheral {
		channels = append(channels, "entertainment:peripheral")
	}
	return channelRoutesFromSlice(channels)
}
```

**`location.go`** — remove `GenerateLocationChannels`, delegate to existing private helpers:

```go
func (d *LocationDomain) Routes(a *Article) []ChannelRoute {
	if a.LocationCountry == LocationCountryUnknown || a.LocationCountry == "" {
		return nil
	}
	prefixes := activeTopicPrefixes(a)
	if len(prefixes) == 0 {
		return nil
	}
	if a.LocationCountry != LocationCountryCanada {
		channels := make([]string, 0, len(prefixes))
		for _, prefix := range prefixes {
			channels = append(channels, prefix+":international")
		}
		return channelRoutesFromSlice(channels)
	}
	return channelRoutesFromSlice(generateCanadianChannels(a, prefixes))
}
```

**`anishinaabe.go`** — remove `GenerateAnishinaabeChannels`, inline in `Routes()`:

```go
func (d *AnishinaabeeDomain) Routes(a *Article) []ChannelRoute {
	if a.Anishinaabe == nil {
		return nil
	}
	rel := a.Anishinaabe.Relevance
	if rel == AnishinaabeRelevanceNot || rel == "" {
		return nil
	}
	channels := []string{"articles:anishinaabe"}
	for _, cat := range a.Anishinaabe.Categories {
		slug := strings.ToLower(strings.ReplaceAll(cat, " ", "-"))
		if slug != "" {
			channels = append(channels, "anishinaabe:category:"+slug)
		}
	}
	return channelRoutesFromSlice(channels)
}
```

**Step 6: Run all router tests**

```bash
cd publisher && GOWORK=off go test ./internal/router/... -v
```

Expected: all pass. If `integration_test.go` still has direct calls to
`GenerateLayer1Channels`, update them now (should already be done in Task 8).

**Step 7: Lint**

```bash
cd publisher && GOWORK=off golangci-lint run ./internal/router/...
```

Fix any issues before committing.

**Step 8: Run full publisher test suite**

```bash
cd publisher && GOWORK=off go test ./... -v
```

**Step 9: Commit**

```bash
git add publisher/internal/router/service.go \
        publisher/internal/router/crime.go \
        publisher/internal/router/mining.go \
        publisher/internal/router/entertainment.go \
        publisher/internal/router/location.go \
        publisher/internal/router/anishinaabe.go \
        publisher/internal/router/integration_test.go
git commit -m "refactor(publisher/router): wire domain slice in routeArticle, remove exported free functions"
```

---

## Task 12: Add `MIGRATION.md` Reviewer Notes

**Files:**
- Create: `publisher/internal/router/MIGRATION.md`

**Step 1: Create the file**

```markdown
# Router Refactor Migration Notes

> For reviewers: this document explains what changed, what stayed the same,
> and where to look to verify functional parity.

## What Changed

### New abstraction: `RoutingDomain`

Each routing layer is now a struct implementing `RoutingDomain`:

```go
type RoutingDomain interface {
    Name() string
    Routes(a *Article) []ChannelRoute
}
```

`routeArticle` iterates a fixed-order `[]RoutingDomain` slice instead of calling
7 hard-coded free functions. Adding a new domain means implementing the interface
and appending to the slice — no changes to `service.go` logic required.

### New: `ChannelRoute`

Replaces the plain `string` channel name in the routing path:

```go
type ChannelRoute struct {
    Channel   string
    ChannelID *uuid.UUID  // nil for auto-generated; set only by DBChannelDomain
}
```

`ChannelID` was previously threaded through as a parameter. It is now carried by
the route itself, making the `publishRoutes` helper simpler.

### New: Layer 8 — `CoforgeDomain`

Coforge is a product-specific namespace. It produces **no** catch-all channel.
Entry points:

| Channel | Condition |
|---------|-----------|
| `coforge:core` | `relevance == core_coforge` |
| `coforge:peripheral` | `relevance == peripheral` |
| `coforge:audience:{audience}` | audience field non-empty |
| `coforge:topic:{slug}` | each Coforge topic |
| `coforge:industry:{slug}` | each Coforge industry |

### New: `coforge` skip in Layer 1

`layer1SkipTopics` now includes `"coforge"` so that articles with the `coforge`
topic tag are not routed to `articles:coforge` by Layer 1.

### New: `article.go`

All article-related types (`Article`, `CrimeData`, `MiningData`, etc.) extracted
from `service.go` into `article.go`. `CoforgeData` added here. No field changes
to existing types.

### New: routing decision log

Each domain emits a `DEBUG` log line per article showing domain name and channel count.
Filter with `level=debug domain=crime` to trace routing decisions for a specific domain.

### New: channel count guardrail

A `WARN` log is emitted when an article produces more than 30 channels across all
domains. This is a guardrail only — routing is not blocked.

## What Stayed the Same

- **All Redis channel names** — no existing channel was renamed or removed.
- **Deduplication semantics** — per-channel, via `publish_history` table.
- **Redis message payload format** — same fields, same structure. `coforge` field added.
- **Polling and discovery logic** — unchanged.
- **Layer ordering** — Layers 1–7 execute in the same order as before.
- **DBChannelDomain refresh** — channels are still loaded from DB at each poll cycle
  and passed into `NewDBChannelDomain(channels)`.

## How to Verify Functional Parity

1. Run tests: `cd publisher && GOWORK=off go test ./...`
2. Compare `routeArticle` before and after — domain slice matches the old explicit calls.
3. `CrimeDomain.Routes()` contains identical logic to the deleted `GenerateCrimeChannels`.
   Same for Mining, Entertainment, Location, Anishinaabe.
4. The `publishToChannel` function is unchanged — only its caller changed from
   `publishToChannels(ctx, a, []string)` to `publishRoutes(ctx, a, []ChannelRoute)`.

## File Map Summary

| File | Before | After |
|------|--------|-------|
| `domain.go` | — | NEW: interface + ChannelRoute |
| `article.go` | — | NEW: types extracted from service.go |
| `domain_topic.go` | — | NEW: Layer 1 |
| `domain_dbchannel.go` | — | NEW: Layer 2 |
| `domain_coforge.go` | — | NEW: Layer 8 |
| `crime.go` | free function | domain struct |
| `mining.go` | free function | domain struct |
| `entertainment.go` | free function | domain struct |
| `location.go` | free function | domain struct |
| `anishinaabe.go` | free function | domain struct |
| `service.go` | 7 explicit calls | domain slice loop |
```

**Step 2: Commit**

```bash
git add publisher/internal/router/MIGRATION.md
git commit -m "docs(publisher/router): add MIGRATION.md reviewer notes"
```

---

## Final Verification

**Run full publisher suite with lint:**

```bash
cd publisher && GOWORK=off go test ./... -v
cd publisher && GOWORK=off golangci-lint run
```

Both must pass clean before considering the work complete.
