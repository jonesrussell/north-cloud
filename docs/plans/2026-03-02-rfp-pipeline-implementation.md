# RFP Pipeline Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `rfp` as a first-class content type to the North Cloud pipeline — crawl, classify with structured extraction, and publish to Redis channels for MyMe dashboard consumption.

**Architecture:** Follows the existing Job/Recipe extractor pattern: add content type constant + domain struct to classifier, keyword heuristic for detection, two-tier extractor (Schema.org + heuristic), nested ES mapping, and a new publisher routing domain (Layer 11).

**Tech Stack:** Go 1.26, Elasticsearch nested mappings, Redis pub/sub, existing classifier/publisher services.

**Design doc:** `docs/plans/2026-03-02-rfp-pipeline-design.md`

---

## Task 1: Add RFP Domain Model and Content Type Constant

**Files:**
- Modify: `classifier/internal/domain/classification.go`

**Step 1: Write the failing test**

Create `classifier/internal/domain/classification_rfp_test.go`:

```go
package domain_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestContentTypeRFPConstant(t *testing.T) {
	t.Helper()
	assert.Equal(t, "rfp", domain.ContentTypeRFP)
}

func TestRFPResultFields(t *testing.T) {
	t.Helper()
	budgetMin := 50000.0
	budgetMax := 200000.0
	result := &domain.RFPResult{
		ExtractionMethod: "heuristic",
		Title:            "IT Services RFP",
		ReferenceNumber:  "EN578-170432",
		OrganizationName: "Public Services and Procurement Canada",
		ClosingDate:      "2026-04-15T16:00:00Z",
		BudgetMin:        &budgetMin,
		BudgetMax:        &budgetMax,
		BudgetCurrency:   "CAD",
		ProcurementType:  "services",
		NAICSCodes:       []string{"541512"},
		Categories:       []string{"IT", "consulting"},
		Province:         "ON",
		Country:          "CA",
	}
	assert.Equal(t, "IT Services RFP", result.Title)
	assert.Equal(t, "EN578-170432", result.ReferenceNumber)
	assert.Equal(t, 50000.0, *result.BudgetMin)
	assert.Equal(t, "services", result.ProcurementType)
	assert.Len(t, result.Categories, 2)
}

func TestClassificationResultRFPField(t *testing.T) {
	t.Helper()
	result := &domain.ClassificationResult{
		ContentType: domain.ContentTypeRFP,
		RFP: &domain.RFPResult{
			Title: "Test RFP",
		},
	}
	assert.NotNil(t, result.RFP)
	assert.Equal(t, "Test RFP", result.RFP.Title)
}

func TestClassifiedContentRFPField(t *testing.T) {
	t.Helper()
	content := &domain.ClassifiedContent{
		ContentType: domain.ContentTypeRFP,
		RFP: &domain.RFPResult{
			Title: "Test RFP",
		},
	}
	assert.NotNil(t, content.RFP)
}
```

**Step 2: Run test to verify it fails**

Run: `cd classifier && go test ./internal/domain/... -run TestContentTypeRFP -v`
Expected: FAIL — `ContentTypeRFP` undefined, `RFPResult` undefined

**Step 3: Write implementation**

Add to `classifier/internal/domain/classification.go`:

1. Add `ContentTypeRFP = "rfp"` to the ContentType constants block (after `ContentTypeObituary`, line 207)

2. Add `RFPResult` struct after `JobResult` (after line 304):

```go
// RFPResult holds structured RFP/procurement extraction results.
// Non-nil values always have ExtractionMethod set ("schema_org", "structured_page", or "heuristic").
type RFPResult struct {
	ExtractionMethod string   `json:"extraction_method"`
	Title            string   `json:"title,omitempty"`
	ReferenceNumber  string   `json:"reference_number,omitempty"`
	OrganizationName string   `json:"organization_name,omitempty"`
	Description      string   `json:"description,omitempty"`
	PublishedDate    string   `json:"published_date,omitempty"`
	ClosingDate      string   `json:"closing_date,omitempty"`
	AmendmentDate    string   `json:"amendment_date,omitempty"`
	BudgetMin        *float64 `json:"budget_min,omitempty"`
	BudgetMax        *float64 `json:"budget_max,omitempty"`
	BudgetCurrency   string   `json:"budget_currency,omitempty"`
	ProcurementType  string   `json:"procurement_type,omitempty"`
	NAICSCodes       []string `json:"naics_codes,omitempty"`
	Categories       []string `json:"categories,omitempty"`
	Province         string   `json:"province,omitempty"`
	City             string   `json:"city,omitempty"`
	Country          string   `json:"country,omitempty"`
	Eligibility      string   `json:"eligibility,omitempty"`
	SourceURL        string   `json:"source_url,omitempty"`
	ContactName      string   `json:"contact_name,omitempty"`
	ContactEmail     string   `json:"contact_email,omitempty"`
}
```

3. Add `RFP *RFPResult` field to `ClassificationResult` (after `Job` field, line 57):

```go
// RFP structured extraction (optional)
RFP *RFPResult `json:"rfp,omitempty"`
```

4. Add `RFP *RFPResult` field to `ClassifiedContent` (after `Job` field, line 189):

```go
// RFP structured extraction (optional)
RFP *RFPResult `json:"rfp,omitempty"`
```

**Step 4: Run test to verify it passes**

Run: `cd classifier && go test ./internal/domain/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add classifier/internal/domain/classification.go classifier/internal/domain/classification_rfp_test.go
git commit -m "feat(classifier): add RFP domain model and content type constant"
```

---

## Task 2: Add RFP Content Type Keyword Heuristic

**Files:**
- Create: `classifier/internal/classifier/content_type_rfp_heuristic.go`
- Create: `classifier/internal/classifier/content_type_rfp_heuristic_test.go`
- Modify: `classifier/internal/classifier/content_type.go` (add call at line ~141)

**Step 1: Write the failing test**

Create `classifier/internal/classifier/content_type_rfp_heuristic_test.go`:

```go
package classifier

import (
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyFromRFPKeywords_Match(t *testing.T) {
	t.Helper()
	c := newTestContentTypeClassifier(t)

	raw := &domain.RawContent{
		ID:    "test-rfp-1",
		Title: "Request for Proposal - IT Infrastructure Modernization",
		RawText: "This request for proposal is for IT infrastructure services. " +
			"The submission deadline is April 15, 2026. " +
			"Proposals must include a detailed scope of work.",
	}

	result := c.classifyFromRFPKeywords(raw)
	require.NotNil(t, result)
	assert.Equal(t, domain.ContentTypeRFP, result.Type)
	assert.InDelta(t, keywordHeuristicConfidence, result.Confidence, 0.001)
	assert.Equal(t, "keyword_heuristic", result.Method)
}

func TestClassifyFromRFPKeywords_NoMatch(t *testing.T) {
	t.Helper()
	c := newTestContentTypeClassifier(t)

	raw := &domain.RawContent{
		ID:      "test-article-1",
		Title:   "City Council Approves New Budget",
		RawText: "The city council met Tuesday to approve the annual operating budget.",
	}

	result := c.classifyFromRFPKeywords(raw)
	assert.Nil(t, result)
}

func TestClassifyFromRFPKeywords_FrenchTender(t *testing.T) {
	t.Helper()
	c := newTestContentTypeClassifier(t)

	raw := &domain.RawContent{
		ID:    "test-rfp-fr-1",
		Title: "Appel d'offres - Services informatiques",
		RawText: "This call for tenders is for professional services. " +
			"The procurement department requires proposals by March 30.",
	}

	result := c.classifyFromRFPKeywords(raw)
	require.NotNil(t, result)
	assert.Equal(t, domain.ContentTypeRFP, result.Type)
}
```

**Step 2: Run test to verify it fails**

Run: `cd classifier && go test ./internal/classifier/... -run TestClassifyFromRFPKeywords -v`
Expected: FAIL — `classifyFromRFPKeywords` undefined

**Step 3: Write implementation**

Create `classifier/internal/classifier/content_type_rfp_heuristic.go`:

```go
package classifier

import (
	"strings"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// rfpKeywords are phrases whose presence (case-insensitive) strongly
// indicates that the page is an RFP or procurement document. Requiring 2+
// matches avoids false positives from pages that incidentally mention one term.
var rfpKeywords = []string{
	"request for proposal",
	"request for tender",
	"request for quotation",
	"call for tenders",
	"call for proposals",
	"invitation to tender",
	"solicitation notice",
	"submission deadline",
	"proposal deadline",
	"closing date for submissions",
	"procurement",
	"bid submission",
	"scope of work",
}

// classifyFromRFPKeywords checks title + raw_text for RFP-related
// keywords. Returns ContentTypeRFP with confidence 0.80 when at least
// 2 keyword matches are found.
// Returns nil if no RFP signal is detected.
func (c *ContentTypeClassifier) classifyFromRFPKeywords(
	raw *domain.RawContent,
) *ContentTypeResult {
	combinedText := strings.ToLower(raw.Title + " " + raw.RawText)

	matches := 0

	for _, kw := range rfpKeywords {
		if strings.Contains(combinedText, kw) {
			matches++
		}
		if matches >= minKeywordMatches {
			c.logger.Debug("RFP detected via keyword heuristic",
				infralogger.String("content_id", raw.ID),
				infralogger.Int("keyword_matches", matches),
			)
			return &ContentTypeResult{
				Type:       domain.ContentTypeRFP,
				Confidence: keywordHeuristicConfidence,
				Method:     "keyword_heuristic",
				Reason:     "RFP keywords detected in content",
			}
		}
	}

	return nil
}
```

**Step 4: Wire into content_type.go**

In `classifier/internal/classifier/content_type.go`, add an RFP heuristic call after the obituary check (line 145) and before the OG metadata check (line 148):

```go
	if result := c.classifyFromRFPKeywords(raw); result != nil {
		return result, nil
	}
```

**Step 5: Run tests**

Run: `cd classifier && go test ./internal/classifier/... -run TestClassifyFromRFP -v`
Expected: PASS

**Step 6: Commit**

```bash
git add classifier/internal/classifier/content_type_rfp_heuristic.go \
       classifier/internal/classifier/content_type_rfp_heuristic_test.go \
       classifier/internal/classifier/content_type.go
git commit -m "feat(classifier): add RFP content type keyword heuristic"
```

---

## Task 3: Add RFP Extractor

**Files:**
- Create: `classifier/internal/classifier/rfp_extractor.go`
- Create: `classifier/internal/classifier/rfp_extractor_test.go`

**Step 1: Write the failing test**

Create `classifier/internal/classifier/rfp_extractor_test.go`:

```go
package classifier

import (
	"context"
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRFPExtractor_NotRFP(t *testing.T) {
	t.Helper()
	e := NewRFPExtractor(newTestLogger(t))
	raw := &domain.RawContent{ID: "test-1", RawText: "City council met today."}
	result, err := e.Extract(context.Background(), raw, "article", nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestRFPExtractor_Heuristic(t *testing.T) {
	t.Helper()
	e := NewRFPExtractor(newTestLogger(t))
	raw := &domain.RawContent{
		ID: "test-rfp-1",
		RawText: "Reference Number: EN578-170432\n" +
			"Organization: Public Services and Procurement Canada\n" +
			"Closing Date: 2026-04-15\n" +
			"Estimated Value: $500,000\n" +
			"Category: IT Services\n" +
			"Province: Ontario\n",
	}
	result, err := e.Extract(context.Background(), raw, domain.ContentTypeRFP, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "heuristic", result.ExtractionMethod)
	assert.Equal(t, "EN578-170432", result.ReferenceNumber)
	assert.Equal(t, "Public Services and Procurement Canada", result.OrganizationName)
	assert.Equal(t, "2026-04-15", result.ClosingDate)
}

func TestRFPExtractor_TopicGated(t *testing.T) {
	t.Helper()
	e := NewRFPExtractor(newTestLogger(t))
	raw := &domain.RawContent{
		ID:      "test-rfp-topic",
		RawText: "Reference Number: ABC-123\nOrganization: City of Toronto\nClosing Date: 2026-05-01\n",
	}
	// Not RFP type, but has rfp topic
	result, err := e.Extract(context.Background(), raw, "article", []string{"rfp"})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "ABC-123", result.ReferenceNumber)
}

func TestRFPExtractor_NeitherTypeNorTopic(t *testing.T) {
	t.Helper()
	e := NewRFPExtractor(newTestLogger(t))
	raw := &domain.RawContent{
		ID:      "test-not-rfp",
		RawText: "Reference Number: ABC-123\nOrganization: City of Toronto\n",
	}
	result, err := e.Extract(context.Background(), raw, "article", []string{"crime"})
	require.NoError(t, err)
	assert.Nil(t, result)
}
```

**Step 2: Run test to verify it fails**

Run: `cd classifier && go test ./internal/classifier/... -run TestRFPExtractor -v`
Expected: FAIL — `NewRFPExtractor` undefined

**Step 3: Write implementation**

Create `classifier/internal/classifier/rfp_extractor.go`:

```go
package classifier

import (
	"context"
	"strings"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// RFP extractor constants.
const (
	rfpTopicName = "rfp"
)

// Heuristic labeled-value keys for RFP extraction.
const (
	rfpLabelReferenceNumber = "reference number:"
	rfpLabelOrganization    = "organization:"
	rfpLabelClosingDate     = "closing date:"
	rfpLabelEstimatedValue  = "estimated value:"
	rfpLabelCategory        = "category:"
	rfpLabelProvince        = "province:"
	rfpLabelCity            = "city:"
	rfpLabelCountry         = "country:"
	rfpLabelEligibility     = "eligibility:"
	rfpLabelContactName     = "contact name:"
	rfpLabelContactEmail    = "contact email:"
	rfpLabelProcurementType = "procurement type:"
)

// RFPExtractor extracts structured RFP/procurement data from raw content using
// heuristic text parsing (labeled fields like "Closing Date: value").
type RFPExtractor struct {
	logger infralogger.Logger
}

// NewRFPExtractor creates a new RFPExtractor.
func NewRFPExtractor(logger infralogger.Logger) *RFPExtractor {
	return &RFPExtractor{logger: logger}
}

// Extract attempts to extract structured RFP fields from raw content.
// Returns (nil, nil) when content is not an RFP.
func (e *RFPExtractor) Extract(
	ctx context.Context, raw *domain.RawContent, contentType string, topics []string,
) (*domain.RFPResult, error) {
	_ = ctx // reserved for future async/tracing use

	isRFPType := contentType == domain.ContentTypeRFP
	hasRFPTopic := containsTopic(topics, rfpTopicName)

	if !isRFPType && !hasRFPTopic {
		return nil, nil //nolint:nilnil // Intentional: nil result signals content is not an RFP
	}

	// Tier 1: Heuristic extraction (labeled fields).
	if result := e.extractHeuristic(raw.RawText); result != nil {
		e.logger.Debug("RFP extracted via heuristic",
			infralogger.String("content_id", raw.ID),
			infralogger.String("reference_number", result.ReferenceNumber),
		)
		return result, nil
	}

	return nil, nil //nolint:nilnil // Intentional: nil result signals no RFP data found
}

// extractHeuristic performs text-based RFP extraction by looking for
// labeled fields like "Reference Number: <value>", "Closing Date: <value>", etc.
// Returns nil if no recognizable RFP patterns are found.
func (e *RFPExtractor) extractHeuristic(rawText string) *domain.RFPResult {
	lowerText := strings.ToLower(rawText)

	refNumber := extractLabeledValue(rawText, lowerText, rfpLabelReferenceNumber)
	org := extractLabeledValue(rawText, lowerText, rfpLabelOrganization)
	closingDate := extractLabeledValue(rawText, lowerText, rfpLabelClosingDate)
	estimatedValue := extractLabeledValue(rawText, lowerText, rfpLabelEstimatedValue)
	category := extractLabeledValue(rawText, lowerText, rfpLabelCategory)
	province := extractLabeledValue(rawText, lowerText, rfpLabelProvince)
	city := extractLabeledValue(rawText, lowerText, rfpLabelCity)
	country := extractLabeledValue(rawText, lowerText, rfpLabelCountry)
	eligibility := extractLabeledValue(rawText, lowerText, rfpLabelEligibility)
	contactName := extractLabeledValue(rawText, lowerText, rfpLabelContactName)
	contactEmail := extractLabeledValue(rawText, lowerText, rfpLabelContactEmail)
	procurementType := extractLabeledValue(rawText, lowerText, rfpLabelProcurementType)

	// Require at least one recognizable RFP field beyond just a category
	if refNumber == "" && org == "" && closingDate == "" {
		return nil
	}

	result := &domain.RFPResult{
		ExtractionMethod: extractionMethodHeuristic,
		ReferenceNumber:  refNumber,
		OrganizationName: org,
		ClosingDate:      closingDate,
		Province:         province,
		City:             city,
		Country:          country,
		Eligibility:      eligibility,
		ContactName:      contactName,
		ContactEmail:     contactEmail,
		ProcurementType:  procurementType,
	}

	if category != "" {
		result.Categories = []string{category}
	}

	result.BudgetMin, result.BudgetMax, result.BudgetCurrency = parseBudgetValue(estimatedValue)

	return result
}

// parseBudgetValue extracts numeric budget from strings like "$500,000" or "$100,000 - $500,000".
// Returns (nil, nil, "") if no recognizable budget is found.
func parseBudgetValue(value string) (*float64, *float64, string) {
	if value == "" {
		return nil, nil, ""
	}

	currency := "CAD" // Default for Canadian sources
	if strings.Contains(value, "USD") || strings.Contains(value, "US$") {
		currency = "USD"
	}

	// Strip currency symbols and commas for parsing
	cleaned := strings.NewReplacer("$", "", ",", "", "CAD", "", "USD", "", "US", "").Replace(value)
	cleaned = strings.TrimSpace(cleaned)

	// Try range format: "100000 - 500000"
	if parts := strings.SplitN(cleaned, "-", 2); len(parts) == 2 { //nolint:mnd // split into exactly 2 parts for range parsing
		minStr := strings.TrimSpace(parts[0])
		maxStr := strings.TrimSpace(parts[1])
		minVal := parseFloat(minStr)
		maxVal := parseFloat(maxStr)
		if minVal != nil || maxVal != nil {
			return minVal, maxVal, currency
		}
	}

	// Single value — treat as both min and max
	val := parseFloat(cleaned)
	if val != nil {
		return val, val, currency
	}

	return nil, nil, ""
}

// parseFloat parses a string to *float64, returning nil on failure.
func parseFloat(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var val float64
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			val = val*10 + float64(ch-'0') //nolint:mnd // decimal base 10 digit conversion
		} else if ch != '.' {
			// Non-numeric, non-dot character — bail
			if val > 0 {
				return &val
			}
			return nil
		}
	}
	if val > 0 {
		return &val
	}
	return nil
}
```

**Step 4: Run tests**

Run: `cd classifier && go test ./internal/classifier/... -run TestRFPExtractor -v`
Expected: PASS

**Step 5: Commit**

```bash
git add classifier/internal/classifier/rfp_extractor.go \
       classifier/internal/classifier/rfp_extractor_test.go
git commit -m "feat(classifier): add RFP extractor with heuristic extraction"
```

---

## Task 4: Wire RFP Extractor into Classifier

**Files:**
- Modify: `classifier/internal/config/config.go` (add `RFPExtractionConfig`)
- Modify: `classifier/internal/classifier/classifier.go` (add `rfpExtractor` field and wiring)
- Modify: `classifier/internal/bootstrap/classifier.go` (conditional creation)

**Step 1: Add config**

In `classifier/internal/config/config.go`:

1. Add `RFPExtractionConfig` struct after `JobExtractionConfig` (line 194):

```go
// RFPExtractionConfig holds RFP extraction settings.
type RFPExtractionConfig struct {
	Enabled bool `env:"RFP_ENABLED" yaml:"enabled"`
}
```

2. Add `RFP RFPExtractionConfig` to `ClassificationConfig` (after `Job` field, line 142):

```go
	RFP  RFPExtractionConfig `yaml:"rfp"`
```

**Step 2: Wire into classifier.go**

In `classifier/internal/classifier/classifier.go`:

1. Add `rfpExtractor *RFPExtractor` to `Classifier` struct (after `jobExtractor`, line 33):

```go
	rfpExtractor *RFPExtractor
```

2. Add `RFPExtractor *RFPExtractor` to `Config` struct (after `JobExtractor`, line 52):

```go
	RFPExtractor *RFPExtractor // Optional: structured RFP extractor
```

3. Wire in `NewClassifier` (after `jobExtractor` assignment, line 105):

```go
	rfpExtractor: config.RFPExtractor,
```

4. Add extraction call in `Classify()` — after `jobResult` (line 181):

```go
	rfpResult := c.runRFPExtraction(ctx, raw, contentTypeResult.Type, topicResult.Topics)
```

5. Add `RFP: rfpResult` to the ClassificationResult (after `Job: jobResult`, line 224):

```go
		RFP:                  rfpResult,
```

6. Add `runRFPExtraction` method (after `runJobExtraction`, line 516):

```go
// runRFPExtraction runs RFP extraction when enabled. Extraction is best-effort:
// failure returns nil RFP and does not fail the overall classification.
func (c *Classifier) runRFPExtraction(
	ctx context.Context, raw *domain.RawContent, contentType string, topics []string,
) *domain.RFPResult {
	if c.rfpExtractor == nil {
		return nil
	}
	result, err := c.rfpExtractor.Extract(ctx, raw, contentType, topics)
	if err != nil {
		wrapped := fmt.Errorf("rfp extraction content_id=%s: %w", raw.ID, err)
		c.logger.Warn("RFP extraction failed",
			infralogger.String("content_id", raw.ID),
			infralogger.Error(wrapped),
		)
		return nil
	}
	return result
}
```

7. Add `RFP: result.RFP` to `BuildClassifiedContent` (after `Job: result.Job`, line 560):

```go
		RFP:                  result.RFP,
```

**Step 3: Wire into bootstrap**

In `classifier/internal/bootstrap/classifier.go`, add after job extractor creation (line 187):

```go
	var rfpExtractor *classifier.RFPExtractor
	if cfg.Classification.RFP.Enabled {
		rfpExtractor = classifier.NewRFPExtractor(logger)
		logger.Info("RFP extractor enabled")
	}
```

Add `RFPExtractor: rfpExtractor` to the `classifier.Config` return (after `JobExtractor`, line 214):

```go
		RFPExtractor:            rfpExtractor,
```

**Step 4: Run tests**

Run: `cd classifier && go test ./... -v`
Expected: PASS

**Step 5: Lint**

Run: `cd classifier && golangci-lint run`
Expected: No errors

**Step 6: Commit**

```bash
git add classifier/internal/config/config.go \
       classifier/internal/classifier/classifier.go \
       classifier/internal/bootstrap/classifier.go
git commit -m "feat(classifier): wire RFP extractor into classification pipeline"
```

---

## Task 5: Add RFP Elasticsearch Mapping

**Files:**
- Modify: `classifier/internal/elasticsearch/mappings/classified_content.go`

**Step 1: Add RFP mapping structs and properties**

Add after `LocationFieldProperties` (line 156):

```go
// RFPProperties defines the nested properties for RFP extraction.
type RFPProperties struct {
	Type       string             `json:"type,omitempty"`
	Properties RFPFieldProperties `json:"properties,omitempty"`
}

// RFPFieldProperties defines individual fields within RFP extraction.
type RFPFieldProperties struct {
	ExtractionMethod Field `json:"extraction_method"`
	Title            Field `json:"title"`
	ReferenceNumber  Field `json:"reference_number"`
	OrganizationName Field `json:"organization_name"`
	Description      Field `json:"description"`
	PublishedDate    Field `json:"published_date"`
	ClosingDate      Field `json:"closing_date"`
	AmendmentDate    Field `json:"amendment_date"`
	BudgetMin        Field `json:"budget_min"`
	BudgetMax        Field `json:"budget_max"`
	BudgetCurrency   Field `json:"budget_currency"`
	ProcurementType  Field `json:"procurement_type"`
	NAICSCodes       Field `json:"naics_codes"`
	Categories       Field `json:"categories"`
	Province         Field `json:"province"`
	City             Field `json:"city"`
	Country          Field `json:"country"`
	Eligibility      Field `json:"eligibility"`
	SourceURL        Field `json:"source_url"`
	ContactName      Field `json:"contact_name"`
	ContactEmail     Field `json:"contact_email"`
}
```

Add `RFP RFPProperties` to `ClassifiedContentProperties` (after `Location`, line 89):

```go
	// RFP structured extraction
	RFP RFPProperties `json:"rfp,omitempty"`
```

Add `createRFPProperties` function (after `createMiningProperties`, line 270):

```go
// createRFPProperties creates nested properties for RFP extraction.
func createRFPProperties() RFPProperties {
	dateFormat := "strict_date_optional_time||epoch_millis"
	return RFPProperties{
		Type: "object",
		Properties: RFPFieldProperties{
			ExtractionMethod: Field{Type: "keyword"},
			Title:            Field{Type: "text", Analyzer: "standard"},
			ReferenceNumber:  Field{Type: "keyword"},
			OrganizationName: Field{Type: "keyword"},
			Description:      Field{Type: "text", Analyzer: "standard"},
			PublishedDate:    Field{Type: "date", Format: dateFormat},
			ClosingDate:      Field{Type: "date", Format: dateFormat},
			AmendmentDate:    Field{Type: "date", Format: dateFormat},
			BudgetMin:        Field{Type: "float"},
			BudgetMax:        Field{Type: "float"},
			BudgetCurrency:   Field{Type: "keyword"},
			ProcurementType:  Field{Type: "keyword"},
			NAICSCodes:       Field{Type: "keyword"},
			Categories:       Field{Type: "keyword"},
			Province:         Field{Type: "keyword"},
			City:             Field{Type: "keyword"},
			Country:          Field{Type: "keyword"},
			Eligibility:      Field{Type: "keyword"},
			SourceURL:        Field{Type: "keyword"},
			ContactName:      Field{Type: "keyword"},
			ContactEmail:     Field{Type: "keyword"},
		},
	}
}
```

Add `RFP: createRFPProperties()` to `createClassificationProperties()` (after `Location`, line 206):

```go
		RFP:           createRFPProperties(),
```

Add `RFP: classified.RFP` to `mergeProperties()` (after `Location`, line 296):

```go
		RFP:           classified.RFP,
```

**Step 2: Run tests**

Run: `cd classifier && go test ./internal/elasticsearch/... -v`
Expected: PASS

**Step 3: Commit**

```bash
git add classifier/internal/elasticsearch/mappings/classified_content.go
git commit -m "feat(classifier): add RFP nested object to ES mapping"
```

---

## Task 6: Add RFP URL Patterns to Crawler Content Detector

**Files:**
- Modify: `crawler/internal/crawler/content_detector.go`

**Step 1: Add RFP patterns**

1. Add `DetectedContentRFP = "rfp"` to the constants block (after `DetectedContentJob`, line 27):

```go
	DetectedContentRFP = "rfp"
```

2. Add URL patterns to `urlContentTypePatterns` (after the `/careers/` pattern, line 117):

```go
	{"/rfp/", DetectedContentRFP},
	{"/rfps/", DetectedContentRFP},
	{"/tenders/", DetectedContentRFP},
	{"/tender/", DetectedContentRFP},
	{"/procurement/", DetectedContentRFP},
	{"/solicitations/", DetectedContentRFP},
	{"/solicitation/", DetectedContentRFP},
	{"/bids/", DetectedContentRFP},
	{"/opportunities/", DetectedContentRFP},
```

3. Add corresponding entries to `contentPathSegments` (after `careers`, line 158):

```go
	"rfp":           true,
	"rfps":          true,
	"tenders":       true,
	"tender":        true,
	"procurement":   true,
	"solicitations": true,
	"solicitation":  true,
	"bids":          true,
	"opportunities": true,
```

4. Add RFP section index paths to `sectionIndexPaths` in `content_type.go` (after `/opportunities`, line 59):

```go
	// RFP/procurement sections (index pages excluded, individual listings pass through)
	"/rfp", "/rfps", "/tenders", "/procurement", "/solicitations", "/bids",
```

**Step 2: Run tests**

Run: `cd crawler && go test ./internal/crawler/... -v`
Expected: PASS

**Step 3: Commit**

```bash
git add crawler/internal/crawler/content_detector.go \
       classifier/internal/classifier/content_type.go
git commit -m "feat(crawler): add RFP URL patterns to content detector"
```

---

## Task 7: Add RFP Publisher Routing Domain (Layer 11)

**Files:**
- Create: `publisher/internal/router/domain_rfp.go`
- Create: `publisher/internal/router/domain_rfp_test.go`
- Modify: `publisher/internal/router/content_item.go` (add `RFPData` struct and field)
- Modify: `publisher/internal/router/service.go` (register domain, add to ES query, add to payload)
- Modify: `publisher/internal/router/domain_topic.go` (add `rfp` to skip list)

**Step 1: Add RFPData to content_item.go**

Add after `JobData` (line 90):

```go
// RFPData holds the publisher view of structured RFP extraction from Elasticsearch.
type RFPData struct {
	ExtractionMethod string   `json:"extraction_method"`
	Title            string   `json:"title,omitempty"`
	ReferenceNumber  string   `json:"reference_number,omitempty"`
	OrganizationName string   `json:"organization_name,omitempty"`
	ClosingDate      string   `json:"closing_date,omitempty"`
	BudgetMin        *float64 `json:"budget_min,omitempty"`
	BudgetMax        *float64 `json:"budget_max,omitempty"`
	BudgetCurrency   string   `json:"budget_currency,omitempty"`
	ProcurementType  string   `json:"procurement_type,omitempty"`
	Categories       []string `json:"categories,omitempty"`
	Province         string   `json:"province,omitempty"`
	City             string   `json:"city,omitempty"`
	Country          string   `json:"country,omitempty"`
}
```

Add `RFP *RFPData` to `ContentItem` (after `Job`, line 136):

```go
	RFP *RFPData `json:"rfp,omitempty"`
```

**Step 2: Write the routing domain test**

Create `publisher/internal/router/domain_rfp_test.go`:

```go
package router

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRFPDomain_NilRFP(t *testing.T) {
	t.Helper()
	d := NewRFPDomain()
	routes := d.Routes(&ContentItem{})
	assert.Nil(t, routes)
}

func TestRFPDomain_BasicRouting(t *testing.T) {
	t.Helper()
	d := NewRFPDomain()
	item := &ContentItem{
		RFP: &RFPData{
			Province:        "ON",
			Country:         "CA",
			Categories:      []string{"IT", "consulting"},
			ProcurementType: "services",
		},
	}
	routes := d.Routes(item)
	channels := routeNames(routes)

	assert.Contains(t, channels, "content:rfps")
	assert.Contains(t, channels, "rfp:country:ca")
	assert.Contains(t, channels, "rfp:province:on")
	assert.Contains(t, channels, "rfp:sector:it")
	assert.Contains(t, channels, "rfp:sector:consulting")
	assert.Contains(t, channels, "rfp:type:services")
}

func TestRFPDomain_Name(t *testing.T) {
	t.Helper()
	d := NewRFPDomain()
	assert.Equal(t, "rfp", d.Name())
}

// routeNames extracts channel names from routes for test assertions.
func routeNames(routes []ChannelRoute) []string {
	names := make([]string, len(routes))
	for i, r := range routes {
		names[i] = r.Channel
	}
	return names
}
```

**Step 3: Run test to verify it fails**

Run: `cd publisher && go test ./internal/router/... -run TestRFPDomain -v`
Expected: FAIL — `NewRFPDomain` undefined

**Step 4: Write the routing domain**

Create `publisher/internal/router/domain_rfp.go`:

```go
package router

import "strings"

// RFPDomain routes RFP-classified content to rfp:* channels.
// Channels produced:
//   - content:rfps (catch-all)
//   - rfp:country:{code} (per country)
//   - rfp:province:{code} (per province)
//   - rfp:sector:{slug} (per category)
//   - rfp:type:{slug} (per procurement type)
type RFPDomain struct{}

// NewRFPDomain creates an RFPDomain.
func NewRFPDomain() *RFPDomain { return &RFPDomain{} }

// Name returns the domain identifier.
func (d *RFPDomain) Name() string { return "rfp" }

// Routes returns RFP channels for the content item.
func (d *RFPDomain) Routes(item *ContentItem) []ChannelRoute {
	if item.RFP == nil {
		return nil
	}

	channels := []string{"content:rfps"}

	if item.RFP.Country != "" {
		channels = append(channels, "rfp:country:"+strings.ToLower(item.RFP.Country))
	}

	if item.RFP.Province != "" {
		channels = append(channels, "rfp:province:"+strings.ToLower(item.RFP.Province))
	}

	for _, category := range item.RFP.Categories {
		slug := strings.ToLower(strings.ReplaceAll(category, " ", "-"))
		channels = append(channels, "rfp:sector:"+slug)
	}

	if item.RFP.ProcurementType != "" {
		slug := strings.ToLower(strings.ReplaceAll(item.RFP.ProcurementType, " ", "-"))
		channels = append(channels, "rfp:type:"+slug)
	}

	return channelRoutesFromSlice(channels)
}

// compile-time interface check
var _ RoutingDomain = (*RFPDomain)(nil)
```

**Step 5: Update service.go**

1. Add `NewRFPDomain()` to the `domains` slice in `routeContentItem()` (after `NewJobDomain()`, line 205):

```go
		NewRFPDomain(),
```

2. Add `"rfp"` to the content_type terms filter in `buildESQuery()` (line 325):

```go
				"content_type": []string{"article", "recipe", "job", "rfp"},
```

3. Add `"rfp": item.RFP` to the payload in `publishToChannel()` (after `"job"`, line 418):

```go
		// RFP extraction
		"rfp": item.RFP,
```

**Step 6: Update domain_topic.go**

Add `"rfp": true` to `layer1SkipTopics` (line 11):

```go
	"rfp": true,
```

**Step 7: Run tests**

Run: `cd publisher && go test ./internal/router/... -v`
Expected: PASS

**Step 8: Lint both services**

Run: `cd classifier && golangci-lint run && cd ../publisher && golangci-lint run && cd ../crawler && golangci-lint run`
Expected: No errors

**Step 9: Commit**

```bash
git add publisher/internal/router/domain_rfp.go \
       publisher/internal/router/domain_rfp_test.go \
       publisher/internal/router/content_item.go \
       publisher/internal/router/service.go \
       publisher/internal/router/domain_topic.go
git commit -m "feat(publisher): add RFP routing domain (Layer 11)"
```

---

## Task 8: Add RFP_ENABLED to Docker Compose

**Files:**
- Modify: `docker-compose.base.yml`

**Step 1: Add env var**

Add `RFP_ENABLED=${RFP_ENABLED:-false}` to the classifier service environment (alongside the other `*_ENABLED` variables).

**Step 2: Commit**

```bash
git add docker-compose.base.yml
git commit -m "feat(docker): add RFP_ENABLED env var to classifier service"
```

---

## Task 9: Final Lint and Test Sweep

**Step 1: Run full linter across all three services**

```bash
task lint:force
```

Expected: No errors across classifier, publisher, crawler

**Step 2: Run full test suite across all three services**

```bash
task test
```

Expected: All tests pass

**Step 3: Commit any fixups, then verify clean state**

```bash
git status
```
