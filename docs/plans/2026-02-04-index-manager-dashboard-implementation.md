# Index-Manager Enhancement + Dashboard Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Expose classifier intelligence (crime/location) in index-manager API and redesign dashboard for operator workflows.

**Architecture:** Extend index-manager with nested crime/location ES mappings, structured API types, new filters, and aggregation endpoints. Restructure dashboard navigation from service-boundaries to operator-first workflows, adding new intelligence views.

**Tech Stack:** Go 1.25+, Gin, Elasticsearch 8.x, Vue 3, TypeScript, Tailwind CSS, Lucide icons

---

## Phase 1: Index-Manager Domain Types

### Task 1: Add CrimeInfo and LocationInfo types

**Files:**
- Modify: `index-manager/internal/domain/document.go:1-71`
- Create: `index-manager/internal/domain/document_test.go`

**Step 1: Write the failing test**

```go
// index-manager/internal/domain/document_test.go
package domain

import (
	"testing"
)

func TestCrimeInfo_IsCrimeRelated(t *testing.T) {
	t.Helper()
	tests := []struct {
		name     string
		crime    *CrimeInfo
		expected bool
	}{
		{"nil crime", nil, false},
		{"not_crime relevance", &CrimeInfo{Relevance: "not_crime"}, false},
		{"core_street_crime relevance", &CrimeInfo{Relevance: "core_street_crime"}, true},
		{"peripheral_crime relevance", &CrimeInfo{Relevance: "peripheral_crime"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.crime.IsCrimeRelated()
			if got != tt.expected {
				t.Errorf("IsCrimeRelated() = %v, want %v", got, tt.expected)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd index-manager && go test ./internal/domain/... -v -run TestCrimeInfo_IsCrimeRelated`
Expected: FAIL with "undefined: CrimeInfo"

**Step 3: Add the domain types**

Add to `index-manager/internal/domain/document.go` after line 4 (after imports):

```go
// CrimeInfo contains structured crime classification data
type CrimeInfo struct {
	SubLabel         string   `json:"sub_label,omitempty"`
	PrimaryCrimeType string   `json:"primary_crime_type,omitempty"`
	Relevance        string   `json:"relevance,omitempty"`
	CrimeTypes       []string `json:"crime_types,omitempty"`
	Confidence       float64  `json:"confidence,omitempty"`
	HomepageEligible bool     `json:"homepage_eligible,omitempty"`
	ReviewRequired   bool     `json:"review_required,omitempty"`
	ModelVersion     string   `json:"model_version,omitempty"`
}

// IsCrimeRelated returns true if this represents crime-related content
func (c *CrimeInfo) IsCrimeRelated() bool {
	if c == nil {
		return false
	}
	return c.Relevance != "not_crime" && c.Relevance != ""
}

// LocationInfo contains structured location data
type LocationInfo struct {
	City        string  `json:"city,omitempty"`
	Province    string  `json:"province,omitempty"`
	Country     string  `json:"country,omitempty"`
	Specificity string  `json:"specificity,omitempty"`
	Confidence  float64 `json:"confidence,omitempty"`
}
```

**Step 4: Run test to verify it passes**

Run: `cd index-manager && go test ./internal/domain/... -v -run TestCrimeInfo_IsCrimeRelated`
Expected: PASS

**Step 5: Commit**

```bash
git add index-manager/internal/domain/document.go index-manager/internal/domain/document_test.go
git commit -m "$(cat <<'EOF'
feat(index-manager): add CrimeInfo and LocationInfo domain types

Add structured types for crime and location classification data
with IsCrimeRelated() helper method.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: Update Document struct with Crime and Location fields

**Files:**
- Modify: `index-manager/internal/domain/document.go:6-21`
- Modify: `index-manager/internal/domain/document_test.go`

**Step 1: Write the failing test**

Add to `index-manager/internal/domain/document_test.go`:

```go
func TestDocument_ComputedIsCrimeRelated(t *testing.T) {
	t.Helper()
	tests := []struct {
		name     string
		doc      Document
		expected bool
	}{
		{
			name:     "nil crime",
			doc:      Document{},
			expected: false,
		},
		{
			name:     "crime related",
			doc:      Document{Crime: &CrimeInfo{Relevance: "core_street_crime"}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.doc.ComputedIsCrimeRelated()
			if got != tt.expected {
				t.Errorf("ComputedIsCrimeRelated() = %v, want %v", got, tt.expected)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd index-manager && go test ./internal/domain/... -v -run TestDocument_ComputedIsCrimeRelated`
Expected: FAIL with "undefined: Document.Crime" or "undefined: ComputedIsCrimeRelated"

**Step 3: Update Document struct**

Replace the Document struct in `index-manager/internal/domain/document.go`:

```go
// Document represents a document in Elasticsearch
type Document struct {
	ID            string         `json:"id"`
	Title         string         `json:"title,omitempty"`
	URL           string         `json:"url,omitempty"`
	SourceName    string         `json:"source_name,omitempty"`
	PublishedDate *time.Time     `json:"published_date,omitempty"`
	CrawledAt     *time.Time     `json:"crawled_at,omitempty"`
	QualityScore  int            `json:"quality_score,omitempty"`
	ContentType   string         `json:"content_type,omitempty"`
	Topics        []string       `json:"topics,omitempty"`
	Body          string         `json:"body,omitempty"`
	RawText       string         `json:"raw_text,omitempty"`
	RawHTML       string         `json:"raw_html,omitempty"`

	// Structured classification fields
	Crime    *CrimeInfo    `json:"crime,omitempty"`
	Location *LocationInfo `json:"location,omitempty"`

	// Backward compatibility (computed from Crime.Relevance)
	IsCrimeRelated bool `json:"is_crime_related,omitempty"`

	// Unstructured spillover
	Meta map[string]any `json:"meta,omitempty"`
}

// ComputedIsCrimeRelated returns whether this document is crime-related
func (d *Document) ComputedIsCrimeRelated() bool {
	if d.Crime != nil {
		return d.Crime.IsCrimeRelated()
	}
	return d.IsCrimeRelated
}
```

**Step 4: Run test to verify it passes**

Run: `cd index-manager && go test ./internal/domain/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add index-manager/internal/domain/document.go index-manager/internal/domain/document_test.go
git commit -m "$(cat <<'EOF'
feat(index-manager): add Crime and Location fields to Document

Update Document struct with structured crime/location fields
and ComputedIsCrimeRelated() for backward compatibility.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: Extend DocumentFilters with crime and location filters

**Files:**
- Modify: `index-manager/internal/domain/document.go:32-44`

**Step 1: Update DocumentFilters**

Replace DocumentFilters in `index-manager/internal/domain/document.go`:

```go
// DocumentFilters holds filter criteria for document queries
type DocumentFilters struct {
	// Existing filters
	Title           string     `json:"title,omitempty"`
	URL             string     `json:"url,omitempty"`
	ContentType     string     `json:"content_type,omitempty"`
	MinQualityScore int        `json:"min_quality_score,omitempty"`
	MaxQualityScore int        `json:"max_quality_score,omitempty"`
	Topics          []string   `json:"topics,omitempty"`
	FromDate        *time.Time `json:"from_date,omitempty"`
	ToDate          *time.Time `json:"to_date,omitempty"`
	FromCrawledAt   *time.Time `json:"from_crawled_at,omitempty"`
	ToCrawledAt     *time.Time `json:"to_crawled_at,omitempty"`

	// Crime filters (new)
	CrimeRelevance   []string `json:"crime_relevance,omitempty"`
	CrimeSubLabels   []string `json:"crime_sub_labels,omitempty"`
	CrimeTypes       []string `json:"crime_types,omitempty"`
	HomepageEligible *bool    `json:"homepage_eligible,omitempty"`
	ReviewRequired   *bool    `json:"review_required,omitempty"`

	// Location filters (new)
	Cities      []string `json:"cities,omitempty"`
	Provinces   []string `json:"provinces,omitempty"`
	Countries   []string `json:"countries,omitempty"`
	Specificity []string `json:"specificity,omitempty"`

	// Source filter (new)
	Sources []string `json:"sources,omitempty"`

	// Legacy filter (deprecated, use CrimeRelevance instead)
	IsCrimeRelated *bool `json:"is_crime_related,omitempty"`
}
```

**Step 2: Run linter to verify no issues**

Run: `cd index-manager && golangci-lint run ./internal/domain/...`
Expected: No errors

**Step 3: Commit**

```bash
git add index-manager/internal/domain/document.go
git commit -m "$(cat <<'EOF'
feat(index-manager): extend DocumentFilters with crime/location filters

Add filters for crime relevance, sub-labels, types, homepage eligibility,
review required, cities, provinces, countries, specificity, and sources.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Phase 2: Elasticsearch Mappings

### Task 4: Add crime and location nested object mappings

**Files:**
- Modify: `index-manager/internal/elasticsearch/mappings/classified_content.go:76-119`

**Step 1: Update getClassificationFields**

Replace the function in `index-manager/internal/elasticsearch/mappings/classified_content.go`:

```go
// getClassificationFields returns the classification result field definitions
func getClassificationFields() map[string]any {
	return map[string]any{
		"content_type": map[string]any{
			"type": "keyword",
		},
		"content_subtype": map[string]any{
			"type": "keyword",
		},
		"quality_score": map[string]any{
			"type": "integer",
		},
		"quality_factors": map[string]any{
			"type": "object",
		},
		"topics": map[string]any{
			"type": "keyword",
		},
		"topic_scores": map[string]any{
			"type": "object",
		},
		// Nested crime object (replaces is_crime_related boolean)
		"crime": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"sub_label": map[string]any{
					"type": "keyword",
				},
				"primary_crime_type": map[string]any{
					"type": "keyword",
				},
				"relevance": map[string]any{
					"type": "keyword",
				},
				"crime_types": map[string]any{
					"type": "keyword",
				},
				"final_confidence": map[string]any{
					"type": "float",
				},
				"homepage_eligible": map[string]any{
					"type": "boolean",
				},
				"review_required": map[string]any{
					"type": "boolean",
				},
				"model_version": map[string]any{
					"type": "keyword",
				},
			},
		},
		// Nested location object
		"location": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"city": map[string]any{
					"type": "keyword",
				},
				"province": map[string]any{
					"type": "keyword",
				},
				"country": map[string]any{
					"type": "keyword",
				},
				"specificity": map[string]any{
					"type": "keyword",
				},
				"confidence": map[string]any{
					"type": "float",
				},
			},
		},
		// Keep is_crime_related for backward compatibility (computed field)
		"is_crime_related": map[string]any{
			"type": "boolean",
		},
		"source_reputation": map[string]any{
			"type": "integer",
		},
		"source_category": map[string]any{
			"type": "keyword",
		},
		"classifier_version": map[string]any{
			"type": "keyword",
		},
		"classification_method": map[string]any{
			"type": "keyword",
		},
		"model_version": map[string]any{
			"type": "keyword",
		},
		"confidence": map[string]any{
			"type": "float",
		},
	}
}
```

**Step 2: Run linter**

Run: `cd index-manager && golangci-lint run ./internal/elasticsearch/mappings/...`
Expected: No errors

**Step 3: Commit**

```bash
git add index-manager/internal/elasticsearch/mappings/classified_content.go
git commit -m "$(cat <<'EOF'
feat(index-manager): add crime/location nested object ES mappings

Replace is_crime_related boolean with structured crime object containing
sub_label, primary_crime_type, relevance, crime_types, confidence,
homepage_eligible, review_required, and model_version.

Add location object with city, province, country, specificity, confidence.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Phase 3: Document Service Updates

### Task 5: Update mapToDocument to extract crime and location

**Files:**
- Modify: `index-manager/internal/service/document_service.go:209-282`

**Step 1: Update mapToDocument function**

Replace the function in `index-manager/internal/service/document_service.go`:

```go
// mapToDocument converts Elasticsearch source map to domain Document
//
//nolint:gocognit // Complex mapping with many field extractions
func (s *DocumentService) mapToDocument(id string, source map[string]any) *domain.Document {
	doc := &domain.Document{
		ID:   id,
		Meta: make(map[string]any),
	}

	// Extract common fields
	if title, ok := source["title"].(string); ok {
		doc.Title = title
	}
	if url, ok := source["url"].(string); ok {
		doc.URL = url
	}
	if sourceName, ok := source["source_name"].(string); ok {
		doc.SourceName = sourceName
	}
	if contentType, ok := source["content_type"].(string); ok {
		doc.ContentType = contentType
	}
	if qualityScore, ok := source["quality_score"].(float64); ok {
		doc.QualityScore = int(qualityScore)
	}
	if body, ok := source["body"].(string); ok {
		doc.Body = body
	}
	if rawText, ok := source["raw_text"].(string); ok {
		doc.RawText = rawText
	}
	if rawHTML, ok := source["raw_html"].(string); ok {
		doc.RawHTML = rawHTML
	}

	// Extract topics array
	if topics, ok := source["topics"].([]any); ok {
		doc.Topics = make([]string, 0, len(topics))
		for _, topic := range topics {
			if topicStr, okTopic := topic.(string); okTopic {
				doc.Topics = append(doc.Topics, topicStr)
			}
		}
	}

	// Extract dates
	if publishedDateStr, ok := source["published_date"].(string); ok {
		if publishedDate, err := time.Parse(time.RFC3339, publishedDateStr); err == nil {
			doc.PublishedDate = &publishedDate
		}
	}
	if crawledAtStr, ok := source["crawled_at"].(string); ok {
		if crawledAt, err := time.Parse(time.RFC3339, crawledAtStr); err == nil {
			doc.CrawledAt = &crawledAt
		}
	}

	// Extract crime object
	doc.Crime = s.extractCrimeInfo(source)

	// Extract location object
	doc.Location = s.extractLocationInfo(source)

	// Compute is_crime_related for backward compatibility
	doc.IsCrimeRelated = doc.ComputedIsCrimeRelated()

	// Store remaining fields in Meta
	excludedKeys := map[string]bool{
		"title": true, "url": true, "source_name": true, "content_type": true,
		"quality_score": true, "body": true, "raw_text": true, "raw_html": true,
		"topics": true, "published_date": true, "crawled_at": true,
		"crime": true, "location": true, "is_crime_related": true,
	}
	for key, value := range source {
		if !excludedKeys[key] {
			doc.Meta[key] = value
		}
	}

	return doc
}

// extractCrimeInfo extracts crime classification from ES source
func (s *DocumentService) extractCrimeInfo(source map[string]any) *domain.CrimeInfo {
	crimeData, ok := source["crime"].(map[string]any)
	if !ok {
		// Fallback to legacy is_crime_related boolean
		if isCrime, okBool := source["is_crime_related"].(bool); okBool && isCrime {
			return &domain.CrimeInfo{Relevance: "core_street_crime"}
		}
		return nil
	}

	crime := &domain.CrimeInfo{}
	if v, ok := crimeData["sub_label"].(string); ok {
		crime.SubLabel = v
	}
	if v, ok := crimeData["primary_crime_type"].(string); ok {
		crime.PrimaryCrimeType = v
	}
	if v, ok := crimeData["relevance"].(string); ok {
		crime.Relevance = v
	}
	if v, ok := crimeData["final_confidence"].(float64); ok {
		crime.Confidence = v
	}
	if v, ok := crimeData["homepage_eligible"].(bool); ok {
		crime.HomepageEligible = v
	}
	if v, ok := crimeData["review_required"].(bool); ok {
		crime.ReviewRequired = v
	}
	if v, ok := crimeData["model_version"].(string); ok {
		crime.ModelVersion = v
	}

	// Extract crime_types array
	if types, ok := crimeData["crime_types"].([]any); ok {
		crime.CrimeTypes = make([]string, 0, len(types))
		for _, t := range types {
			if ts, okStr := t.(string); okStr {
				crime.CrimeTypes = append(crime.CrimeTypes, ts)
			}
		}
	}

	return crime
}

// extractLocationInfo extracts location data from ES source
func (s *DocumentService) extractLocationInfo(source map[string]any) *domain.LocationInfo {
	locData, ok := source["location"].(map[string]any)
	if !ok {
		return nil
	}

	loc := &domain.LocationInfo{}
	if v, ok := locData["city"].(string); ok {
		loc.City = v
	}
	if v, ok := locData["province"].(string); ok {
		loc.Province = v
	}
	if v, ok := locData["country"].(string); ok {
		loc.Country = v
	}
	if v, ok := locData["specificity"].(string); ok {
		loc.Specificity = v
	}
	if v, ok := locData["confidence"].(float64); ok {
		loc.Confidence = v
	}

	return loc
}
```

**Step 2: Run linter**

Run: `cd index-manager && golangci-lint run ./internal/service/...`
Expected: No errors

**Step 3: Commit**

```bash
git add index-manager/internal/service/document_service.go
git commit -m "$(cat <<'EOF'
feat(index-manager): extract crime/location from ES documents

Update mapToDocument to deserialize nested crime and location objects
from Elasticsearch responses into structured domain types.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: Update documentToMap to include crime and location

**Files:**
- Modify: `index-manager/internal/service/document_service.go:284-329`

**Step 1: Update documentToMap function**

Replace the function in `index-manager/internal/service/document_service.go`:

```go
// documentToMap converts domain Document to map for Elasticsearch update
func (s *DocumentService) documentToMap(doc *domain.Document) map[string]any {
	result := make(map[string]any)

	if doc.Title != "" {
		result["title"] = doc.Title
	}
	if doc.URL != "" {
		result["url"] = doc.URL
	}
	if doc.SourceName != "" {
		result["source_name"] = doc.SourceName
	}
	if doc.ContentType != "" {
		result["content_type"] = doc.ContentType
	}
	if doc.QualityScore > 0 {
		result["quality_score"] = doc.QualityScore
	}
	if doc.Body != "" {
		result["body"] = doc.Body
	}
	if doc.RawText != "" {
		result["raw_text"] = doc.RawText
	}
	if doc.RawHTML != "" {
		result["raw_html"] = doc.RawHTML
	}
	if len(doc.Topics) > 0 {
		result["topics"] = doc.Topics
	}
	if doc.PublishedDate != nil {
		result["published_date"] = doc.PublishedDate.Format(time.RFC3339)
	}
	if doc.CrawledAt != nil {
		result["crawled_at"] = doc.CrawledAt.Format(time.RFC3339)
	}

	// Add crime object
	if doc.Crime != nil {
		result["crime"] = s.crimeInfoToMap(doc.Crime)
		result["is_crime_related"] = doc.Crime.IsCrimeRelated()
	} else {
		result["is_crime_related"] = doc.IsCrimeRelated
	}

	// Add location object
	if doc.Location != nil {
		result["location"] = s.locationInfoToMap(doc.Location)
	}

	// Merge meta fields
	for key, value := range doc.Meta {
		result[key] = value
	}

	return result
}

// crimeInfoToMap converts CrimeInfo to map for ES
func (s *DocumentService) crimeInfoToMap(crime *domain.CrimeInfo) map[string]any {
	result := make(map[string]any)
	if crime.SubLabel != "" {
		result["sub_label"] = crime.SubLabel
	}
	if crime.PrimaryCrimeType != "" {
		result["primary_crime_type"] = crime.PrimaryCrimeType
	}
	if crime.Relevance != "" {
		result["relevance"] = crime.Relevance
	}
	if len(crime.CrimeTypes) > 0 {
		result["crime_types"] = crime.CrimeTypes
	}
	if crime.Confidence > 0 {
		result["final_confidence"] = crime.Confidence
	}
	result["homepage_eligible"] = crime.HomepageEligible
	result["review_required"] = crime.ReviewRequired
	if crime.ModelVersion != "" {
		result["model_version"] = crime.ModelVersion
	}
	return result
}

// locationInfoToMap converts LocationInfo to map for ES
func (s *DocumentService) locationInfoToMap(loc *domain.LocationInfo) map[string]any {
	result := make(map[string]any)
	if loc.City != "" {
		result["city"] = loc.City
	}
	if loc.Province != "" {
		result["province"] = loc.Province
	}
	if loc.Country != "" {
		result["country"] = loc.Country
	}
	if loc.Specificity != "" {
		result["specificity"] = loc.Specificity
	}
	if loc.Confidence > 0 {
		result["confidence"] = loc.Confidence
	}
	return result
}
```

**Step 2: Run linter**

Run: `cd index-manager && golangci-lint run ./internal/service/...`
Expected: No errors

**Step 3: Commit**

```bash
git add index-manager/internal/service/document_service.go
git commit -m "$(cat <<'EOF'
feat(index-manager): serialize crime/location to ES format

Update documentToMap to convert structured crime/location domain
types back to Elasticsearch format for document updates.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Phase 4: Query Builder Filter Support

### Task 7: Add crime and location filter support to query builder

**Files:**
- Modify: `index-manager/internal/elasticsearch/query_builder.go:164-272`

**Step 1: Update buildFilters function**

Replace the function in `index-manager/internal/elasticsearch/query_builder.go`:

```go
// buildFilters constructs filter clauses
//
//nolint:gocognit // Complex filter building with multiple conditionals
func (qb *DocumentQueryBuilder) buildFilters(filters *domain.DocumentFilters) []any {
	var result []any

	// Title filter (contains)
	if filters.Title != "" {
		result = append(result, map[string]any{
			"wildcard": map[string]any{
				"title.keyword": map[string]any{
					"value":            "*" + strings.ToLower(filters.Title) + "*",
					"case_insensitive": true,
				},
			},
		})
	}

	// URL filter (contains)
	if filters.URL != "" {
		result = append(result, map[string]any{
			"wildcard": map[string]any{
				"url.keyword": map[string]any{
					"value":            "*" + strings.ToLower(filters.URL) + "*",
					"case_insensitive": true,
				},
			},
		})
	}

	// Content type filter
	if filters.ContentType != "" {
		result = append(result, map[string]any{
			"term": map[string]any{
				"content_type.keyword": filters.ContentType,
			},
		})
	}

	// Quality score range filter
	if filters.MinQualityScore > 0 || filters.MaxQualityScore < maxQualityScore {
		qualityRange := make(map[string]any)
		if filters.MinQualityScore > 0 {
			qualityRange["gte"] = filters.MinQualityScore
		}
		if filters.MaxQualityScore < maxQualityScore {
			qualityRange["lte"] = filters.MaxQualityScore
		}
		if len(qualityRange) > 0 {
			result = append(result, map[string]any{
				"range": map[string]any{
					"quality_score": qualityRange,
				},
			})
		}
	}

	// Topics filter
	if len(filters.Topics) > 0 {
		result = append(result, map[string]any{
			"terms": map[string]any{
				"topics.keyword": filters.Topics,
			},
		})
	}

	// Published date range filter
	if filters.FromDate != nil || filters.ToDate != nil {
		dateRange := make(map[string]any)
		if filters.FromDate != nil {
			dateRange["gte"] = filters.FromDate.Format(time.RFC3339)
		}
		if filters.ToDate != nil {
			dateRange["lte"] = filters.ToDate.Format(time.RFC3339)
		}
		result = append(result, map[string]any{
			"range": map[string]any{
				"published_date": dateRange,
			},
		})
	}

	// Crawled at date range filter
	if filters.FromCrawledAt != nil || filters.ToCrawledAt != nil {
		dateRange := make(map[string]any)
		if filters.FromCrawledAt != nil {
			dateRange["gte"] = filters.FromCrawledAt.Format(time.RFC3339)
		}
		if filters.ToCrawledAt != nil {
			dateRange["lte"] = filters.ToCrawledAt.Format(time.RFC3339)
		}
		result = append(result, map[string]any{
			"range": map[string]any{
				"crawled_at": dateRange,
			},
		})
	}

	// Crime filters
	result = qb.appendCrimeFilters(result, filters)

	// Location filters
	result = qb.appendLocationFilters(result, filters)

	// Sources filter
	if len(filters.Sources) > 0 {
		result = append(result, map[string]any{
			"terms": map[string]any{
				"source_name": filters.Sources,
			},
		})
	}

	// Legacy crime-related filter (backward compatibility)
	if filters.IsCrimeRelated != nil && len(filters.CrimeRelevance) == 0 {
		if *filters.IsCrimeRelated {
			// true = core or peripheral crime
			result = append(result, map[string]any{
				"terms": map[string]any{
					"crime.relevance": []string{"core_street_crime", "peripheral_crime"},
				},
			})
		} else {
			// false = not crime
			result = append(result, map[string]any{
				"term": map[string]any{
					"crime.relevance": "not_crime",
				},
			})
		}
	}

	return result
}

// appendCrimeFilters adds crime-related filters to the result slice
func (qb *DocumentQueryBuilder) appendCrimeFilters(result []any, filters *domain.DocumentFilters) []any {
	// Crime relevance filter
	if len(filters.CrimeRelevance) > 0 {
		result = append(result, map[string]any{
			"terms": map[string]any{
				"crime.relevance": filters.CrimeRelevance,
			},
		})
	}

	// Crime sub-labels filter
	if len(filters.CrimeSubLabels) > 0 {
		result = append(result, map[string]any{
			"terms": map[string]any{
				"crime.sub_label": filters.CrimeSubLabels,
			},
		})
	}

	// Crime types filter
	if len(filters.CrimeTypes) > 0 {
		result = append(result, map[string]any{
			"terms": map[string]any{
				"crime.crime_types": filters.CrimeTypes,
			},
		})
	}

	// Homepage eligible filter
	if filters.HomepageEligible != nil {
		result = append(result, map[string]any{
			"term": map[string]any{
				"crime.homepage_eligible": *filters.HomepageEligible,
			},
		})
	}

	// Review required filter
	if filters.ReviewRequired != nil {
		result = append(result, map[string]any{
			"term": map[string]any{
				"crime.review_required": *filters.ReviewRequired,
			},
		})
	}

	return result
}

// appendLocationFilters adds location-related filters to the result slice
func (qb *DocumentQueryBuilder) appendLocationFilters(result []any, filters *domain.DocumentFilters) []any {
	// Cities filter
	if len(filters.Cities) > 0 {
		result = append(result, map[string]any{
			"terms": map[string]any{
				"location.city": filters.Cities,
			},
		})
	}

	// Provinces filter
	if len(filters.Provinces) > 0 {
		result = append(result, map[string]any{
			"terms": map[string]any{
				"location.province": filters.Provinces,
			},
		})
	}

	// Countries filter
	if len(filters.Countries) > 0 {
		result = append(result, map[string]any{
			"terms": map[string]any{
				"location.country": filters.Countries,
			},
		})
	}

	// Specificity filter
	if len(filters.Specificity) > 0 {
		result = append(result, map[string]any{
			"terms": map[string]any{
				"location.specificity": filters.Specificity,
			},
		})
	}

	return result
}
```

**Step 2: Run linter**

Run: `cd index-manager && golangci-lint run ./internal/elasticsearch/...`
Expected: No errors

**Step 3: Commit**

```bash
git add index-manager/internal/elasticsearch/query_builder.go
git commit -m "$(cat <<'EOF'
feat(index-manager): add crime/location filter support to query builder

Support filtering by crime relevance, sub-labels, types, homepage
eligibility, review required, cities, provinces, countries,
specificity, and sources.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Phase 5: Aggregation Endpoints

### Task 8: Add aggregation domain types

**Files:**
- Create: `index-manager/internal/domain/aggregation.go`

**Step 1: Create aggregation types**

Create `index-manager/internal/domain/aggregation.go`:

```go
package domain

// CrimeAggregation represents crime distribution statistics
type CrimeAggregation struct {
	BySubLabel        map[string]int64 `json:"by_sub_label"`
	ByRelevance       map[string]int64 `json:"by_relevance"`
	ByCrimeType       map[string]int64 `json:"by_crime_type"`
	TotalCrimeRelated int64            `json:"total_crime_related"`
	TotalDocuments    int64            `json:"total_documents"`
}

// LocationAggregation represents geographic distribution statistics
type LocationAggregation struct {
	ByCountry     map[string]int64 `json:"by_country"`
	ByProvince    map[string]int64 `json:"by_province"`
	ByCity        map[string]int64 `json:"by_city"`
	BySpecificity map[string]int64 `json:"by_specificity"`
}

// OverviewAggregation represents high-level pipeline statistics
type OverviewAggregation struct {
	TotalDocuments      int64            `json:"total_documents"`
	TotalCrimeRelated   int64            `json:"total_crime_related"`
	TopCities           []string         `json:"top_cities"`
	TopCrimeTypes       []string         `json:"top_crime_types"`
	QualityDistribution QualityBuckets   `json:"quality_distribution"`
}

// QualityBuckets represents quality score distribution
type QualityBuckets struct {
	High   int64 `json:"high"`   // 70-100
	Medium int64 `json:"medium"` // 40-69
	Low    int64 `json:"low"`    // 0-39
}

// AggregationRequest represents a request for aggregated statistics
type AggregationRequest struct {
	Filters *DocumentFilters `json:"filters,omitempty"`
}
```

**Step 2: Run linter**

Run: `cd index-manager && golangci-lint run ./internal/domain/...`
Expected: No errors

**Step 3: Commit**

```bash
git add index-manager/internal/domain/aggregation.go
git commit -m "$(cat <<'EOF'
feat(index-manager): add aggregation domain types

Add CrimeAggregation, LocationAggregation, OverviewAggregation,
and QualityBuckets types for aggregation endpoints.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 9: Add aggregation service

**Files:**
- Create: `index-manager/internal/service/aggregation_service.go`

**Step 1: Create aggregation service**

Create `index-manager/internal/service/aggregation_service.go`:

```go
package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jonesrussell/north-cloud/index-manager/internal/domain"
	"github.com/jonesrussell/north-cloud/index-manager/internal/elasticsearch"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
	topCitiesLimit      = 10
	topCrimeTypesLimit  = 10
	qualityHighMin      = 70
	qualityMediumMin    = 40
)

// AggregationService provides aggregation operations on classified content
type AggregationService struct {
	esClient     *elasticsearch.Client
	queryBuilder *elasticsearch.DocumentQueryBuilder
	logger       infralogger.Logger
}

// NewAggregationService creates a new aggregation service
func NewAggregationService(esClient *elasticsearch.Client, logger infralogger.Logger) *AggregationService {
	return &AggregationService{
		esClient:     esClient,
		queryBuilder: elasticsearch.NewDocumentQueryBuilder(),
		logger:       logger,
	}
}

// GetCrimeAggregation returns crime distribution statistics
func (s *AggregationService) GetCrimeAggregation(
	ctx context.Context,
	req *domain.AggregationRequest,
) (*domain.CrimeAggregation, error) {
	query := s.buildAggregationQuery(req, map[string]any{
		"by_sub_label": map[string]any{
			"terms": map[string]any{
				"field": "crime.sub_label",
				"size":  topCitiesLimit,
			},
		},
		"by_relevance": map[string]any{
			"terms": map[string]any{
				"field": "crime.relevance",
				"size":  topCitiesLimit,
			},
		},
		"by_crime_type": map[string]any{
			"terms": map[string]any{
				"field": "crime.crime_types",
				"size":  topCrimeTypesLimit,
			},
		},
		"crime_related": map[string]any{
			"filter": map[string]any{
				"terms": map[string]any{
					"crime.relevance": []string{"core_street_crime", "peripheral_crime"},
				},
			},
		},
	})

	res, err := s.esClient.SearchAllClassifiedContent(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute aggregation: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	var esResp aggregationResponse
	if decodeErr := json.NewDecoder(res.Body).Decode(&esResp); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", decodeErr)
	}

	return &domain.CrimeAggregation{
		BySubLabel:        extractBuckets(esResp.Aggregations["by_sub_label"]),
		ByRelevance:       extractBuckets(esResp.Aggregations["by_relevance"]),
		ByCrimeType:       extractBuckets(esResp.Aggregations["by_crime_type"]),
		TotalCrimeRelated: extractFilterCount(esResp.Aggregations["crime_related"]),
		TotalDocuments:    esResp.Hits.Total.Value,
	}, nil
}

// GetLocationAggregation returns geographic distribution statistics
func (s *AggregationService) GetLocationAggregation(
	ctx context.Context,
	req *domain.AggregationRequest,
) (*domain.LocationAggregation, error) {
	query := s.buildAggregationQuery(req, map[string]any{
		"by_country": map[string]any{
			"terms": map[string]any{
				"field": "location.country",
				"size":  topCitiesLimit,
			},
		},
		"by_province": map[string]any{
			"terms": map[string]any{
				"field": "location.province",
				"size":  topCitiesLimit,
			},
		},
		"by_city": map[string]any{
			"terms": map[string]any{
				"field": "location.city",
				"size":  topCitiesLimit,
			},
		},
		"by_specificity": map[string]any{
			"terms": map[string]any{
				"field": "location.specificity",
				"size":  topCitiesLimit,
			},
		},
	})

	res, err := s.esClient.SearchAllClassifiedContent(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute aggregation: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	var esResp aggregationResponse
	if decodeErr := json.NewDecoder(res.Body).Decode(&esResp); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", decodeErr)
	}

	return &domain.LocationAggregation{
		ByCountry:     extractBuckets(esResp.Aggregations["by_country"]),
		ByProvince:    extractBuckets(esResp.Aggregations["by_province"]),
		ByCity:        extractBuckets(esResp.Aggregations["by_city"]),
		BySpecificity: extractBuckets(esResp.Aggregations["by_specificity"]),
	}, nil
}

// GetOverviewAggregation returns high-level pipeline statistics
func (s *AggregationService) GetOverviewAggregation(
	ctx context.Context,
	req *domain.AggregationRequest,
) (*domain.OverviewAggregation, error) {
	query := s.buildAggregationQuery(req, map[string]any{
		"top_cities": map[string]any{
			"terms": map[string]any{
				"field": "location.city",
				"size":  topCitiesLimit,
			},
		},
		"top_crime_types": map[string]any{
			"terms": map[string]any{
				"field": "crime.crime_types",
				"size":  topCrimeTypesLimit,
			},
		},
		"crime_related": map[string]any{
			"filter": map[string]any{
				"terms": map[string]any{
					"crime.relevance": []string{"core_street_crime", "peripheral_crime"},
				},
			},
		},
		"quality_high": map[string]any{
			"filter": map[string]any{
				"range": map[string]any{
					"quality_score": map[string]any{"gte": qualityHighMin},
				},
			},
		},
		"quality_medium": map[string]any{
			"filter": map[string]any{
				"range": map[string]any{
					"quality_score": map[string]any{"gte": qualityMediumMin, "lt": qualityHighMin},
				},
			},
		},
		"quality_low": map[string]any{
			"filter": map[string]any{
				"range": map[string]any{
					"quality_score": map[string]any{"lt": qualityMediumMin},
				},
			},
		},
	})

	res, err := s.esClient.SearchAllClassifiedContent(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute aggregation: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	var esResp aggregationResponse
	if decodeErr := json.NewDecoder(res.Body).Decode(&esResp); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", decodeErr)
	}

	return &domain.OverviewAggregation{
		TotalDocuments:    esResp.Hits.Total.Value,
		TotalCrimeRelated: extractFilterCount(esResp.Aggregations["crime_related"]),
		TopCities:         extractBucketKeys(esResp.Aggregations["top_cities"]),
		TopCrimeTypes:     extractBucketKeys(esResp.Aggregations["top_crime_types"]),
		QualityDistribution: domain.QualityBuckets{
			High:   extractFilterCount(esResp.Aggregations["quality_high"]),
			Medium: extractFilterCount(esResp.Aggregations["quality_medium"]),
			Low:    extractFilterCount(esResp.Aggregations["quality_low"]),
		},
	}, nil
}

// buildAggregationQuery constructs an ES aggregation query with optional filters
func (s *AggregationService) buildAggregationQuery(
	req *domain.AggregationRequest,
	aggs map[string]any,
) map[string]any {
	query := map[string]any{
		"size":            0,
		"track_total_hits": true,
		"aggs":            aggs,
	}

	// Add filters if provided
	if req != nil && req.Filters != nil {
		// Use query builder to construct filter query
		boolQuery := map[string]any{
			"filter": s.queryBuilder.BuildFiltersOnly(req.Filters),
		}
		query["query"] = map[string]any{"bool": boolQuery}
	}

	return query
}

// aggregationResponse represents the ES aggregation response structure
type aggregationResponse struct {
	Hits struct {
		Total struct {
			Value int64 `json:"value"`
		} `json:"total"`
	} `json:"hits"`
	Aggregations map[string]json.RawMessage `json:"aggregations"`
}

// bucketAggResult represents a terms aggregation result
type bucketAggResult struct {
	Buckets []struct {
		Key      string `json:"key"`
		DocCount int64  `json:"doc_count"`
	} `json:"buckets"`
}

// filterAggResult represents a filter aggregation result
type filterAggResult struct {
	DocCount int64 `json:"doc_count"`
}

// extractBuckets extracts key-count pairs from a terms aggregation
func extractBuckets(raw json.RawMessage) map[string]int64 {
	result := make(map[string]int64)
	var agg bucketAggResult
	if err := json.Unmarshal(raw, &agg); err != nil {
		return result
	}
	for _, bucket := range agg.Buckets {
		result[bucket.Key] = bucket.DocCount
	}
	return result
}

// extractBucketKeys extracts just the keys from a terms aggregation
func extractBucketKeys(raw json.RawMessage) []string {
	var agg bucketAggResult
	if err := json.Unmarshal(raw, &agg); err != nil {
		return nil
	}
	keys := make([]string, 0, len(agg.Buckets))
	for _, bucket := range agg.Buckets {
		keys = append(keys, bucket.Key)
	}
	return keys
}

// extractFilterCount extracts doc_count from a filter aggregation
func extractFilterCount(raw json.RawMessage) int64 {
	var agg filterAggResult
	if err := json.Unmarshal(raw, &agg); err != nil {
		return 0
	}
	return agg.DocCount
}
```

**Step 2: Add BuildFiltersOnly method to query builder**

Add to `index-manager/internal/elasticsearch/query_builder.go`:

```go
// BuildFiltersOnly returns just the filter array without wrapping in bool query
func (qb *DocumentQueryBuilder) BuildFiltersOnly(filters *domain.DocumentFilters) []any {
	if filters == nil {
		return []any{}
	}
	return qb.buildFilters(filters)
}
```

**Step 3: Add SearchAllClassifiedContent to ES client**

This method will need to be added to the ES client to search across all `*_classified_content` indexes. Add to `index-manager/internal/elasticsearch/client.go`:

```go
// SearchAllClassifiedContent executes a search across all classified content indexes
func (c *Client) SearchAllClassifiedContent(ctx context.Context, query map[string]any) (*esapi.Response, error) {
	body, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	return c.es.Search(
		c.es.Search.WithContext(ctx),
		c.es.Search.WithIndex("*_classified_content"),
		c.es.Search.WithBody(bytes.NewReader(body)),
	)
}
```

**Step 4: Run linter**

Run: `cd index-manager && golangci-lint run ./internal/...`
Expected: No errors

**Step 5: Commit**

```bash
git add index-manager/internal/service/aggregation_service.go \
        index-manager/internal/elasticsearch/query_builder.go \
        index-manager/internal/elasticsearch/client.go
git commit -m "$(cat <<'EOF'
feat(index-manager): add aggregation service for crime/location stats

Implement GetCrimeAggregation, GetLocationAggregation, and
GetOverviewAggregation methods with filter support.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 10: Add aggregation API handlers

**Files:**
- Modify: `index-manager/internal/api/handlers.go`
- Modify: `index-manager/internal/api/routes.go`

**Step 1: Add handler methods**

Add to `index-manager/internal/api/handlers.go`:

```go
// GetCrimeAggregation handles GET /api/v1/aggregations/crime
func (h *Handler) GetCrimeAggregation(c *gin.Context) {
	req := h.parseAggregationRequest(c)

	result, err := h.aggregationService.GetCrimeAggregation(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("Failed to get crime aggregation", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetLocationAggregation handles GET /api/v1/aggregations/location
func (h *Handler) GetLocationAggregation(c *gin.Context) {
	req := h.parseAggregationRequest(c)

	result, err := h.aggregationService.GetLocationAggregation(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("Failed to get location aggregation", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetOverviewAggregation handles GET /api/v1/aggregations/overview
func (h *Handler) GetOverviewAggregation(c *gin.Context) {
	req := h.parseAggregationRequest(c)

	result, err := h.aggregationService.GetOverviewAggregation(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("Failed to get overview aggregation", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// parseAggregationRequest extracts filters from query parameters
func (h *Handler) parseAggregationRequest(c *gin.Context) *domain.AggregationRequest {
	req := &domain.AggregationRequest{
		Filters: &domain.DocumentFilters{},
	}

	// Parse crime filters
	if v := c.QueryArray("crime_relevance"); len(v) > 0 {
		req.Filters.CrimeRelevance = v
	}
	if v := c.QueryArray("crime_sub_labels"); len(v) > 0 {
		req.Filters.CrimeSubLabels = v
	}
	if v := c.QueryArray("crime_types"); len(v) > 0 {
		req.Filters.CrimeTypes = v
	}

	// Parse location filters
	if v := c.QueryArray("cities"); len(v) > 0 {
		req.Filters.Cities = v
	}
	if v := c.QueryArray("provinces"); len(v) > 0 {
		req.Filters.Provinces = v
	}
	if v := c.QueryArray("countries"); len(v) > 0 {
		req.Filters.Countries = v
	}

	// Parse source filter
	if v := c.QueryArray("sources"); len(v) > 0 {
		req.Filters.Sources = v
	}

	// Parse quality filters
	if minQ := c.Query("min_quality"); minQ != "" {
		if val, err := strconv.Atoi(minQ); err == nil {
			req.Filters.MinQualityScore = val
		}
	}

	return req
}
```

**Step 2: Update Handler struct**

Update the Handler struct in `index-manager/internal/api/handlers.go`:

```go
// Handler handles HTTP requests for the index manager API
type Handler struct {
	indexService       *service.IndexService
	documentService    *service.DocumentService
	aggregationService *service.AggregationService
	logger             infralogger.Logger
}

// NewHandler creates a new API handler
func NewHandler(
	indexService *service.IndexService,
	documentService *service.DocumentService,
	aggregationService *service.AggregationService,
	logger infralogger.Logger,
) *Handler {
	return &Handler{
		indexService:       indexService,
		documentService:    documentService,
		aggregationService: aggregationService,
		logger:             logger,
	}
}
```

**Step 3: Add routes**

Add to the route registration in `index-manager/internal/api/routes.go`:

```go
// Aggregation routes
aggregations := v1.Group("/aggregations")
{
	aggregations.GET("/crime", h.GetCrimeAggregation)
	aggregations.GET("/location", h.GetLocationAggregation)
	aggregations.GET("/overview", h.GetOverviewAggregation)
}
```

**Step 4: Update bootstrap to wire aggregation service**

Update the service initialization in bootstrap to create and inject the aggregation service.

**Step 5: Run linter**

Run: `cd index-manager && golangci-lint run ./internal/api/...`
Expected: No errors

**Step 6: Commit**

```bash
git add index-manager/internal/api/handlers.go \
        index-manager/internal/api/routes.go \
        index-manager/internal/bootstrap/
git commit -m "$(cat <<'EOF'
feat(index-manager): add aggregation API endpoints

Add GET /api/v1/aggregations/crime, /location, /overview
endpoints with filter support for crime/location queries.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Phase 6: Dashboard Navigation Restructure

### Task 11: Update navigation config

**Files:**
- Modify: `dashboard/src/config/navigation.ts`

**Step 1: Replace navigation config**

Replace the contents of `dashboard/src/config/navigation.ts`:

```typescript
import {
  Activity,
  FileText,
  AlertTriangle,
  Brain,
  MapPin,
  Database,
  Download,
  ListTodo,
  Link,
  Filter,
  Globe,
  Building2,
  Star,
  Radio,
  GitBranch,
  ScrollText,
  Settings,
  HeartPulse,
  Shield,
  HardDrive,
  type LucideIcon,
} from 'lucide-vue-next'

export interface NavItem {
  title: string
  path: string
  icon: LucideIcon
}

export interface NavSection {
  title: string
  icon: LucideIcon
  path?: string
  quickAction?: {
    label: string
    path: string
  }
  children?: NavItem[]
}

export const navigation: NavSection[] = [
  // Operations - daily cockpit
  {
    title: 'Operations',
    icon: Activity,
    children: [
      { title: 'Pipeline Monitor', path: '/', icon: Activity },
      { title: 'Recent Articles', path: '/operations/articles', icon: FileText },
      { title: 'Review Queue', path: '/operations/review', icon: AlertTriangle },
    ],
  },
  // Intelligence - new value from Option B
  {
    title: 'Intelligence',
    icon: Brain,
    quickAction: { label: 'View Stats', path: '/intelligence/crime' },
    children: [
      { title: 'Crime Breakdown', path: '/intelligence/crime', icon: AlertTriangle },
      { title: 'Location Breakdown', path: '/intelligence/location', icon: MapPin },
      { title: 'Index Explorer', path: '/intelligence/indexes', icon: Database },
    ],
  },
  // Content Intake - fix upstream issues
  {
    title: 'Content Intake',
    icon: Download,
    quickAction: { label: 'New Job', path: '/intake/jobs?create=true' },
    children: [
      { title: 'Crawler Jobs', path: '/intake/jobs', icon: ListTodo },
      { title: 'Discovered Links', path: '/intake/discovered-links', icon: Link },
      { title: 'Rules', path: '/intake/rules', icon: Filter },
    ],
  },
  // Sources - manage the ecosystem
  {
    title: 'Sources',
    icon: Globe,
    quickAction: { label: 'Add Source', path: '/sources/new' },
    children: [
      { title: 'All Sources', path: '/sources', icon: Globe },
      { title: 'Cities', path: '/sources/cities', icon: Building2 },
      { title: 'Reputation', path: '/sources/reputation', icon: Star },
    ],
  },
  // Distribution - where content goes
  {
    title: 'Distribution',
    icon: Radio,
    quickAction: { label: 'New Route', path: '/distribution/routes/new' },
    children: [
      { title: 'Channels', path: '/distribution/channels', icon: Radio },
      { title: 'Routes', path: '/distribution/routes', icon: GitBranch },
      { title: 'Delivery Logs', path: '/distribution/logs', icon: ScrollText },
    ],
  },
  // System - rarely used but essential
  {
    title: 'System',
    icon: Settings,
    children: [
      { title: 'Health', path: '/system/health', icon: HeartPulse },
      { title: 'Auth', path: '/system/auth', icon: Shield },
      { title: 'Cache', path: '/system/cache', icon: HardDrive },
    ],
  },
]

// Helper to find the current section based on route path
export function getCurrentSection(path: string): NavSection | undefined {
  for (const section of navigation) {
    if (section.path === path) return section
    if (section.children) {
      const childMatch = section.children.find(
        (child) => path === child.path || path.startsWith(child.path + '/')
      )
      if (childMatch) return section
    }
  }
  return undefined
}

// Helper to get breadcrumb items for a path
export function getBreadcrumbs(path: string): { label: string; path: string }[] {
  const breadcrumbs: { label: string; path: string }[] = []

  for (const section of navigation) {
    if (section.children) {
      for (const child of section.children) {
        if (path === child.path || path.startsWith(child.path + '/')) {
          breadcrumbs.push({ label: section.title, path: section.children[0].path })
          breadcrumbs.push({ label: child.title, path: child.path })
          return breadcrumbs
        }
      }
    }
  }

  return breadcrumbs
}
```

**Step 2: Run lint**

Run: `cd dashboard && npm run lint`
Expected: No errors

**Step 3: Commit**

```bash
git add dashboard/src/config/navigation.ts
git commit -m "$(cat <<'EOF'
feat(dashboard): restructure navigation for operator workflows

Replace service-boundary menu with operator-first structure:
- Operations (Pipeline Monitor, Recent Articles, Review Queue)
- Intelligence (Crime Breakdown, Location Breakdown, Index Explorer)
- Content Intake (Crawler Jobs, Discovered Links, Rules)
- Sources (All Sources, Cities, Reputation)
- Distribution (Channels, Routes, Delivery Logs)
- System (Health, Auth, Cache)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 12: Update router with new routes

**Files:**
- Modify: `dashboard/src/router/index.ts`

**Step 1: Update routes**

The router needs to be updated with new routes for:
- `/operations/articles` (move from `/distribution/articles`)
- `/operations/review` (new)
- `/intelligence/crime` (new)
- `/intelligence/location` (new)
- `/distribution/routes` (real view, not redirect)
- `/distribution/logs` (move from `/feeds/logs`)
- `/sources/*` (consolidate from `/scheduling/*`)

Add legacy redirects for backward compatibility.

**Step 2: Run lint**

Run: `cd dashboard && npm run lint`
Expected: No errors

**Step 3: Commit**

```bash
git add dashboard/src/router/index.ts
git commit -m "$(cat <<'EOF'
feat(dashboard): update router for new navigation structure

Add routes for new views:
- /operations/articles, /operations/review
- /intelligence/crime, /intelligence/location
- /distribution/routes (real view)
- /sources/* (consolidated)

Add legacy redirects for backward compatibility.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Phase 7: New Dashboard Views

### Task 13: Create Review Queue view

**Files:**
- Create: `dashboard/src/views/operations/ReviewQueueView.vue`

**Step 1: Create the view**

This is a filtered document list showing articles with `review_required=true`.

**Step 2: Run lint**

Run: `cd dashboard && npm run lint`
Expected: No errors

**Step 3: Commit**

```bash
git add dashboard/src/views/operations/ReviewQueueView.vue
git commit -m "$(cat <<'EOF'
feat(dashboard): add Review Queue view

Show articles flagged for review (review_required=true)
with filtering and sorting capabilities.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 14: Create Crime Breakdown view

**Files:**
- Create: `dashboard/src/views/intelligence/CrimeBreakdownView.vue`
- Create: `dashboard/src/api/aggregations.ts`

**Step 1: Create API client**

Create `dashboard/src/api/aggregations.ts`:

```typescript
import axios from './axios'
import type { CrimeAggregation, LocationAggregation, OverviewAggregation } from '@/types/aggregation'

export interface AggregationFilters {
  crime_relevance?: string[]
  crime_sub_labels?: string[]
  crime_types?: string[]
  cities?: string[]
  provinces?: string[]
  countries?: string[]
  sources?: string[]
  min_quality?: number
}

function buildQueryString(filters?: AggregationFilters): string {
  if (!filters) return ''
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(filters)) {
    if (Array.isArray(value)) {
      value.forEach((v) => params.append(key, v))
    } else if (value !== undefined) {
      params.append(key, String(value))
    }
  }
  return params.toString() ? `?${params.toString()}` : ''
}

export const aggregationsApi = {
  getCrime: (filters?: AggregationFilters): Promise<CrimeAggregation> =>
    axios.get(`/api/index-manager/api/v1/aggregations/crime${buildQueryString(filters)}`).then((r) => r.data),

  getLocation: (filters?: AggregationFilters): Promise<LocationAggregation> =>
    axios.get(`/api/index-manager/api/v1/aggregations/location${buildQueryString(filters)}`).then((r) => r.data),

  getOverview: (filters?: AggregationFilters): Promise<OverviewAggregation> =>
    axios.get(`/api/index-manager/api/v1/aggregations/overview${buildQueryString(filters)}`).then((r) => r.data),
}
```

**Step 2: Create types**

Create `dashboard/src/types/aggregation.ts`:

```typescript
export interface CrimeAggregation {
  by_sub_label: Record<string, number>
  by_relevance: Record<string, number>
  by_crime_type: Record<string, number>
  total_crime_related: number
  total_documents: number
}

export interface LocationAggregation {
  by_country: Record<string, number>
  by_province: Record<string, number>
  by_city: Record<string, number>
  by_specificity: Record<string, number>
}

export interface QualityBuckets {
  high: number
  medium: number
  low: number
}

export interface OverviewAggregation {
  total_documents: number
  total_crime_related: number
  top_cities: string[]
  top_crime_types: string[]
  quality_distribution: QualityBuckets
}
```

**Step 3: Create the view**

Create the Crime Breakdown view with charts showing crime distribution.

**Step 4: Run lint**

Run: `cd dashboard && npm run lint`
Expected: No errors

**Step 5: Commit**

```bash
git add dashboard/src/views/intelligence/CrimeBreakdownView.vue \
        dashboard/src/api/aggregations.ts \
        dashboard/src/types/aggregation.ts
git commit -m "$(cat <<'EOF'
feat(dashboard): add Crime Breakdown view with aggregation API

Display crime distribution by sub-label, relevance, and type
with interactive charts and filter controls.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 15: Create Location Breakdown view

**Files:**
- Create: `dashboard/src/views/intelligence/LocationBreakdownView.vue`

**Step 1: Create the view**

Create the Location Breakdown view with charts showing geographic distribution.

**Step 2: Run lint**

Run: `cd dashboard && npm run lint`
Expected: No errors

**Step 3: Commit**

```bash
git add dashboard/src/views/intelligence/LocationBreakdownView.vue
git commit -m "$(cat <<'EOF'
feat(dashboard): add Location Breakdown view

Display geographic distribution by country, province, and city
with interactive charts and filter controls.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 16: Create Routes view

**Files:**
- Create: `dashboard/src/views/distribution/RoutesView.vue`

**Step 1: Create the view**

Create a proper Routes view (instead of redirect to Channels) for route management.

**Step 2: Run lint**

Run: `cd dashboard && npm run lint`
Expected: No errors

**Step 3: Commit**

```bash
git add dashboard/src/views/distribution/RoutesView.vue
git commit -m "$(cat <<'EOF'
feat(dashboard): add Routes view for distribution management

Replace redirect hack with actual route management view
supporting CRUD operations on publisher routes.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Phase 8: Enhanced Views

### Task 17: Enhance Pipeline Monitor with overview aggregations

**Files:**
- Modify: `dashboard/src/views/PipelineMonitorView.vue`

**Step 1: Add aggregation data**

Update Pipeline Monitor to fetch and display overview aggregations:
- Total documents
- Crime percentage
- Top cities
- Top crime types
- Quality distribution

**Step 2: Run lint**

Run: `cd dashboard && npm run lint`
Expected: No errors

**Step 3: Commit**

```bash
git add dashboard/src/views/PipelineMonitorView.vue
git commit -m "$(cat <<'EOF'
feat(dashboard): enhance Pipeline Monitor with aggregations

Add overview statistics: total documents, crime percentage,
top cities, top crime types, quality distribution.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 18: Rename Indexes to Index Explorer and add filters

**Files:**
- Rename: `dashboard/src/views/intelligence/IndexesView.vue` → `dashboard/src/views/intelligence/IndexExplorerView.vue`
- Modify: `dashboard/src/views/intelligence/IndexExplorerView.vue`

**Step 1: Rename and update**

Rename the view and add crime/location filter controls.

**Step 2: Run lint**

Run: `cd dashboard && npm run lint`
Expected: No errors

**Step 3: Commit**

```bash
git add dashboard/src/views/intelligence/
git commit -m "$(cat <<'EOF'
feat(dashboard): rename Indexes to Index Explorer with filters

Add crime/location filter controls to document browsing.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Phase 9: Final Integration

### Task 19: Run full test suite

**Step 1: Run index-manager tests**

Run: `cd index-manager && go test ./... -v`
Expected: All tests pass

**Step 2: Run index-manager linter**

Run: `cd index-manager && golangci-lint run`
Expected: No errors

**Step 3: Run dashboard lint**

Run: `cd dashboard && npm run lint`
Expected: No errors

**Step 4: Build dashboard**

Run: `cd dashboard && npm run build`
Expected: Build succeeds

---

### Task 20: Create final commit and PR

**Step 1: Verify all changes**

Run: `git status`

**Step 2: Create summary commit if needed**

If there are any uncommitted changes, create a final commit.

**Step 3: Push and create PR**

```bash
git push -u origin claude/index-manager-dashboard-redesign
gh pr create --title "feat: index-manager crime/location intelligence + dashboard redesign" --body "$(cat <<'EOF'
## Summary

- Expose classifier intelligence (crime/location) in index-manager API
- Add aggregation endpoints for crime, location, and overview statistics
- Restructure dashboard navigation for operator workflows
- Add new intelligence views (Crime Breakdown, Location Breakdown)

## Changes

### Index-Manager
- Add CrimeInfo and LocationInfo domain types
- Update ES mappings with nested crime/location objects
- Extend DocumentFilters with crime/location/source filters
- Add aggregation service and API endpoints

### Dashboard
- Restructure navigation (Operations, Intelligence, Content Intake, Sources, Distribution, System)
- Add Crime Breakdown view
- Add Location Breakdown view
- Add Review Queue view
- Add Routes view (real view, not redirect)
- Enhance Pipeline Monitor with aggregations
- Rename Indexes to Index Explorer with filters

## Test Plan
- [ ] Index-manager unit tests pass
- [ ] Index-manager linter passes
- [ ] Dashboard builds successfully
- [ ] Manual testing of new aggregation endpoints
- [ ] Manual testing of new dashboard views

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

---

## Success Criteria

- [ ] CrimeInfo and LocationInfo domain types created with tests
- [ ] Document struct updated with Crime and Location fields
- [ ] DocumentFilters extended with crime/location filters
- [ ] ES mappings include nested crime/location objects
- [ ] mapToDocument extracts crime/location from ES
- [ ] Query builder supports crime/location filters
- [ ] Aggregation endpoints return correct data
- [ ] Dashboard navigation restructured
- [ ] Crime Breakdown view displays aggregations
- [ ] Location Breakdown view displays aggregations
- [ ] Review Queue view filters by review_required
- [ ] Routes view provides route management
- [ ] Pipeline Monitor shows overview stats
- [ ] Index Explorer has crime/location filters
- [ ] All tests pass
- [ ] Linters pass
- [ ] Dashboard builds successfully
