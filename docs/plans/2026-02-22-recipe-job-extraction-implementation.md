# Recipe & Job Structured Extraction Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add structured extraction for cooking recipes and career job postings to the classification pipeline, with Schema.org + keyword detection, ES storage, publisher routing, and search support.

**Architecture:** New extractors (not hybrid classifiers) run as step 5 in the classifier pipeline, after topic detection. Schema.org JSON-LD parsing provides high-fidelity extraction; keyword rules provide fallback detection. Two new publisher routing layers (9 & 10) route to dedicated Redis channels.

**Tech Stack:** Go 1.26+, Elasticsearch 9, Redis Pub/Sub, PostgreSQL migrations

**Design doc:** `docs/plans/2026-02-22-recipe-job-extraction-design.md`

---

## Phase 1: Foundation (Domain Models & Extractors)

### Task 1: Add RecipeResult and JobResult domain models

**Files:**
- Modify: `classifier/internal/domain/classification.go`

**Step 1: Add ContentTypeRecipe constant and result structs**

Add after line 191 (after `ContentTypeJob`):

```go
ContentTypeRecipe = "recipe"
```

Add RecipeResult after LocationResult (after line 251):

```go
// RecipeResult holds structured recipe extraction results.
type RecipeResult struct {
	ExtractionMethod string   `json:"extraction_method"`          // "schema_org" or "heuristic"
	Name             string   `json:"name,omitempty"`
	Ingredients      []string `json:"ingredients,omitempty"`
	Instructions     string   `json:"instructions,omitempty"`
	PrepTimeMinutes  *int     `json:"prep_time_minutes,omitempty"`
	CookTimeMinutes  *int     `json:"cook_time_minutes,omitempty"`
	TotalTimeMinutes *int     `json:"total_time_minutes,omitempty"`
	Servings         string   `json:"servings,omitempty"`
	Category         string   `json:"category,omitempty"`
	Cuisine          string   `json:"cuisine,omitempty"`
	Calories         string   `json:"calories,omitempty"`
	ImageURL         string   `json:"image_url,omitempty"`
	Rating           *float64 `json:"rating,omitempty"`
	RatingCount      *int     `json:"rating_count,omitempty"`
}

// JobResult holds structured job posting extraction results.
type JobResult struct {
	ExtractionMethod string   `json:"extraction_method"`             // "schema_org" or "heuristic"
	Title            string   `json:"title,omitempty"`
	Company          string   `json:"company,omitempty"`
	Location         string   `json:"location,omitempty"`
	SalaryMin        *float64 `json:"salary_min,omitempty"`
	SalaryMax        *float64 `json:"salary_max,omitempty"`
	SalaryCurrency   string   `json:"salary_currency,omitempty"`
	EmploymentType   string   `json:"employment_type,omitempty"`     // full_time, part_time, contract, temporary, internship
	PostedDate       string   `json:"posted_date,omitempty"`
	ExpiresDate      string   `json:"expires_date,omitempty"`
	Description      string   `json:"description,omitempty"`
	Industry         string   `json:"industry,omitempty"`
	Qualifications   string   `json:"qualifications,omitempty"`
	Benefits         string   `json:"benefits,omitempty"`
}
```

**Step 2: Add Recipe and Job fields to ClassificationResult**

Add after line 51 (after `Location`):

```go
// Recipe structured extraction (optional)
Recipe *RecipeResult `json:"recipe,omitempty"`

// Job structured extraction (optional)
Job *JobResult `json:"job,omitempty"`
```

**Step 3: Add Recipe and Job fields to ClassifiedContent**

Add after line 177 (after `Location`):

```go
// Recipe structured extraction (optional)
Recipe *RecipeResult `json:"recipe,omitempty"`

// Job structured extraction (optional)
Job *JobResult `json:"job,omitempty"`
```

**Step 4: Run tests**

Run: `cd classifier && GOWORK=off go test ./internal/domain/... -v`
Expected: PASS (no breaking changes, only additions)

**Step 5: Run linter**

Run: `cd classifier && GOWORK=off golangci-lint run ./internal/domain/...`
Expected: No errors

**Step 6: Commit**

```bash
git add classifier/internal/domain/classification.go
git commit -m "feat(classifier): add RecipeResult and JobResult domain models"
```

---

### Task 2: Create JSON-LD parser utility

**Files:**
- Create: `classifier/internal/classifier/jsonld/parser.go`
- Create: `classifier/internal/classifier/jsonld/parser_test.go`

**Step 1: Write the failing tests**

Create `classifier/internal/classifier/jsonld/parser_test.go`:

```go
package jsonld_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/classifier/jsonld"
)

func TestExtractJSONLD_RecipeType(t *testing.T) {
	t.Helper()

	html := `<html><head>
	<script type="application/ld+json">
	{"@type": "Recipe", "name": "Pasta Carbonara", "recipeIngredient": ["pasta", "eggs", "bacon"]}
	</script>
	</head><body></body></html>`

	blocks := jsonld.Extract(html)
	if len(blocks) == 0 {
		t.Fatal("expected at least one JSON-LD block")
	}
}

func TestExtractJSONLD_ArrayWithMultipleTypes(t *testing.T) {
	t.Helper()

	html := `<html><head>
	<script type="application/ld+json">
	[{"@type": "BreadcrumbList"}, {"@type": "Recipe", "name": "Tacos"}]
	</script>
	</head><body></body></html>`

	blocks := jsonld.Extract(html)
	if len(blocks) == 0 {
		t.Fatal("expected JSON-LD blocks")
	}

	recipe := jsonld.FindByType(blocks, "Recipe")
	if recipe == nil {
		t.Fatal("expected to find Recipe type")
	}
	if recipe["name"] != "Tacos" {
		t.Errorf("expected name 'Tacos', got %v", recipe["name"])
	}
}

func TestExtractJSONLD_JobPostingType(t *testing.T) {
	t.Helper()

	html := `<html><head>
	<script type="application/ld+json">
	{"@type": "JobPosting", "title": "Go Developer", "hiringOrganization": {"name": "Acme"}}
	</script>
	</head><body></body></html>`

	blocks := jsonld.Extract(html)
	job := jsonld.FindByType(blocks, "JobPosting")
	if job == nil {
		t.Fatal("expected to find JobPosting type")
	}
	if job["title"] != "Go Developer" {
		t.Errorf("expected title 'Go Developer', got %v", job["title"])
	}
}

func TestExtractJSONLD_NoScriptTags(t *testing.T) {
	t.Helper()

	html := `<html><body><p>No JSON-LD here</p></body></html>`
	blocks := jsonld.Extract(html)
	if len(blocks) != 0 {
		t.Errorf("expected no blocks, got %d", len(blocks))
	}
}

func TestExtractJSONLD_MalformedJSON(t *testing.T) {
	t.Helper()

	html := `<html><head>
	<script type="application/ld+json">{not valid json}</script>
	</head></html>`

	blocks := jsonld.Extract(html)
	if len(blocks) != 0 {
		t.Errorf("expected no blocks for malformed JSON, got %d", len(blocks))
	}
}

func TestParseDuration_ISO8601(t *testing.T) {
	t.Helper()

	tests := []struct {
		input    string
		expected int
	}{
		{"PT30M", 30},
		{"PT1H", 60},
		{"PT1H30M", 90},
		{"PT45M", 45},
		{"PT2H15M", 135},
	}

	for _, tc := range tests {
		result := jsonld.ParseISO8601Duration(tc.input)
		if result == nil || *result != tc.expected {
			t.Errorf("ParseISO8601Duration(%q) = %v, want %d", tc.input, result, tc.expected)
		}
	}
}

func TestParseDuration_Invalid(t *testing.T) {
	t.Helper()

	result := jsonld.ParseISO8601Duration("not a duration")
	if result != nil {
		t.Errorf("expected nil for invalid duration, got %d", *result)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd classifier && GOWORK=off go test ./internal/classifier/jsonld/... -v`
Expected: FAIL (package does not exist)

**Step 3: Write minimal implementation**

Create `classifier/internal/classifier/jsonld/parser.go`:

```go
package jsonld

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

// jsonLDPattern matches <script type="application/ld+json">...</script> blocks.
var jsonLDPattern = regexp.MustCompile(`(?is)<script[^>]*type=["']application/ld\+json["'][^>]*>(.*?)</script>`)

// Extract parses all JSON-LD blocks from raw HTML and returns them as a slice
// of maps. Array JSON-LD blocks are flattened into individual entries.
// Malformed JSON is silently skipped.
func Extract(html string) []map[string]any {
	matches := jsonLDPattern.FindAllStringSubmatch(html, -1)
	if len(matches) == 0 {
		return nil
	}

	var results []map[string]any
	for _, match := range matches {
		content := strings.TrimSpace(match[1])
		if content == "" {
			continue
		}
		results = append(results, parseJSONLDContent(content)...)
	}
	return results
}

// parseJSONLDContent parses a single JSON-LD content string (object or array).
func parseJSONLDContent(content string) []map[string]any {
	// Try as single object first
	var obj map[string]any
	if err := json.Unmarshal([]byte(content), &obj); err == nil {
		return []map[string]any{obj}
	}

	// Try as array
	var arr []map[string]any
	if err := json.Unmarshal([]byte(content), &arr); err == nil {
		return arr
	}

	return nil
}

// FindByType searches JSON-LD blocks for a specific @type value.
// Returns the first matching block, or nil if not found.
func FindByType(blocks []map[string]any, typeName string) map[string]any {
	for _, block := range blocks {
		if blockType, ok := block["@type"].(string); ok && blockType == typeName {
			return block
		}
	}
	return nil
}

// durationPattern matches ISO 8601 duration format (e.g., PT1H30M).
var durationPattern = regexp.MustCompile(`^PT(?:(\d+)H)?(?:(\d+)M)?$`)

// ParseISO8601Duration converts an ISO 8601 duration string (e.g., "PT1H30M")
// to total minutes. Returns nil if the format is unrecognized.
func ParseISO8601Duration(duration string) *int {
	matches := durationPattern.FindStringSubmatch(strings.TrimSpace(duration))
	if matches == nil {
		return nil
	}

	totalMinutes := 0
	if matches[1] != "" {
		hours, _ := strconv.Atoi(matches[1])
		const minutesPerHour = 60
		totalMinutes += hours * minutesPerHour
	}
	if matches[2] != "" {
		minutes, _ := strconv.Atoi(matches[2])
		totalMinutes += minutes
	}

	return &totalMinutes
}

// StringVal safely extracts a string value from a JSON-LD map.
func StringVal(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// StringSliceVal extracts a string slice from a JSON-LD map.
// Handles both []string and []any (from JSON unmarshaling).
func StringSliceVal(m map[string]any, key string) []string {
	val, ok := m[key]
	if !ok {
		return nil
	}
	switch v := val.(type) {
	case []any:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case []string:
		return v
	default:
		return nil
	}
}

// NestedStringVal extracts a string from a nested object, e.g. {"hiringOrganization": {"name": "Acme"}}.
func NestedStringVal(m map[string]any, outerKey, innerKey string) string {
	nested, ok := m[outerKey].(map[string]any)
	if !ok {
		return ""
	}
	return StringVal(nested, innerKey)
}

// FloatVal safely extracts a float64 from a JSON-LD map.
// Handles both float64 and json.Number types.
func FloatVal(m map[string]any, key string) *float64 {
	switch v := m[key].(type) {
	case float64:
		return &v
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil
		}
		return &f
	default:
		return nil
	}
}

// IntVal safely extracts an int from a JSON-LD map.
func IntVal(m map[string]any, key string) *int {
	switch v := m[key].(type) {
	case float64:
		i := int(v)
		return &i
	case string:
		i, err := strconv.Atoi(v)
		if err != nil {
			return nil
		}
		return &i
	default:
		return nil
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `cd classifier && GOWORK=off go test ./internal/classifier/jsonld/... -v`
Expected: PASS

**Step 5: Run linter**

Run: `cd classifier && GOWORK=off golangci-lint run ./internal/classifier/jsonld/...`
Expected: No errors

**Step 6: Commit**

```bash
git add classifier/internal/classifier/jsonld/
git commit -m "feat(classifier): add JSON-LD parser utility for Schema.org extraction"
```

---

### Task 3: Create recipe extractor

**Files:**
- Create: `classifier/internal/classifier/recipe_extractor.go`
- Create: `classifier/internal/classifier/recipe_extractor_test.go`

**Step 1: Write the failing tests**

Create `classifier/internal/classifier/recipe_extractor_test.go`:

```go
package classifier_test

import (
	"context"
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/classifier"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

func TestRecipeExtractor_SchemaOrg(t *testing.T) {
	t.Helper()

	logger := infralogger.NewNoop()
	extractor := classifier.NewRecipeExtractor(logger)

	raw := &domain.RawContent{
		ID: "test-1",
		RawHTML: `<html><head>
		<script type="application/ld+json">
		{
			"@type": "Recipe",
			"name": "Pasta Carbonara",
			"recipeIngredient": ["400g spaghetti", "200g pancetta", "4 eggs"],
			"recipeInstructions": "Cook pasta. Fry pancetta. Mix eggs. Combine.",
			"prepTime": "PT15M",
			"cookTime": "PT20M",
			"totalTime": "PT35M",
			"recipeYield": "4 servings",
			"recipeCategory": "Main Course",
			"recipeCuisine": "Italian"
		}
		</script>
		</head><body></body></html>`,
	}

	result, err := extractor.Extract(context.Background(), raw, domain.ContentTypeRecipe, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ExtractionMethod != "schema_org" {
		t.Errorf("expected extraction_method 'schema_org', got %q", result.ExtractionMethod)
	}
	if result.Name != "Pasta Carbonara" {
		t.Errorf("expected name 'Pasta Carbonara', got %q", result.Name)
	}
	if len(result.Ingredients) != 3 {
		t.Errorf("expected 3 ingredients, got %d", len(result.Ingredients))
	}
	if result.PrepTimeMinutes == nil || *result.PrepTimeMinutes != 15 {
		t.Errorf("expected prep time 15, got %v", result.PrepTimeMinutes)
	}
	if result.Cuisine != "Italian" {
		t.Errorf("expected cuisine 'Italian', got %q", result.Cuisine)
	}
}

func TestRecipeExtractor_NotApplicable(t *testing.T) {
	t.Helper()

	logger := infralogger.NewNoop()
	extractor := classifier.NewRecipeExtractor(logger)

	raw := &domain.RawContent{
		ID:      "test-2",
		RawHTML: `<html><body>Just a news article</body></html>`,
	}

	result, err := extractor.Extract(context.Background(), raw, domain.ContentTypeArticle, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result for non-recipe content")
	}
}

func TestRecipeExtractor_HeuristicFallback(t *testing.T) {
	t.Helper()

	logger := infralogger.NewNoop()
	extractor := classifier.NewRecipeExtractor(logger)

	raw := &domain.RawContent{
		ID:      "test-3",
		RawHTML: `<html><body>No schema</body></html>`,
		RawText: "Chocolate Chip Cookies\n\nIngredients:\n- 2 cups flour\n- 1 cup sugar\n- 1 cup butter\n\nInstructions:\n1. Mix flour and sugar\n2. Add butter\n3. Bake at 350F",
	}

	// Trigger via topic match
	result, err := extractor.Extract(context.Background(), raw, domain.ContentTypeArticle, []string{"recipe"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result from heuristic extraction")
	}
	if result.ExtractionMethod != "heuristic" {
		t.Errorf("expected extraction_method 'heuristic', got %q", result.ExtractionMethod)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd classifier && GOWORK=off go test ./internal/classifier/... -run TestRecipeExtractor -v`
Expected: FAIL (function not defined)

**Step 3: Write implementation**

Create `classifier/internal/classifier/recipe_extractor.go`:

```go
package classifier

import (
	"context"
	"regexp"
	"strings"

	"github.com/jonesrussell/north-cloud/classifier/internal/classifier/jsonld"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// RecipeExtractor extracts structured recipe data from raw content.
// Uses Schema.org JSON-LD (tier 1) or heuristic text parsing (tier 2).
type RecipeExtractor struct {
	logger infralogger.Logger
}

// NewRecipeExtractor creates a RecipeExtractor.
func NewRecipeExtractor(logger infralogger.Logger) *RecipeExtractor {
	return &RecipeExtractor{logger: logger}
}

// Extract attempts to extract structured recipe fields from raw content.
// Returns (nil, nil) when content is not a recipe.
func (e *RecipeExtractor) Extract(
	_ context.Context, raw *domain.RawContent, contentType string, topics []string,
) (*domain.RecipeResult, error) {
	// Gate: only run for recipe content type or recipe topic
	isRecipeType := contentType == domain.ContentTypeRecipe
	hasRecipeTopic := containsTopic(topics, "recipe")

	if !isRecipeType && !hasRecipeTopic {
		return nil, nil
	}

	// Tier 1: Schema.org JSON-LD extraction
	if result := e.extractFromSchemaOrg(raw); result != nil {
		e.logger.Debug("Recipe extracted via Schema.org",
			infralogger.String("content_id", raw.ID),
			infralogger.String("name", result.Name),
		)
		return result, nil
	}

	// Tier 2: Heuristic extraction (only if topic matched)
	if hasRecipeTopic {
		if result := e.extractFromHeuristics(raw); result != nil {
			e.logger.Debug("Recipe extracted via heuristics",
				infralogger.String("content_id", raw.ID),
			)
			return result, nil
		}
	}

	// Content type was recipe but extraction found nothing useful
	if isRecipeType {
		return &domain.RecipeResult{ExtractionMethod: "schema_org"}, nil
	}

	return nil, nil
}

func (e *RecipeExtractor) extractFromSchemaOrg(raw *domain.RawContent) *domain.RecipeResult {
	if raw.RawHTML == "" {
		return nil
	}

	blocks := jsonld.Extract(raw.RawHTML)
	recipe := jsonld.FindByType(blocks, "Recipe")
	if recipe == nil {
		return nil
	}

	return &domain.RecipeResult{
		ExtractionMethod: "schema_org",
		Name:             jsonld.StringVal(recipe, "name"),
		Ingredients:      jsonld.StringSliceVal(recipe, "recipeIngredient"),
		Instructions:     extractInstructions(recipe),
		PrepTimeMinutes:  jsonld.ParseISO8601Duration(jsonld.StringVal(recipe, "prepTime")),
		CookTimeMinutes:  jsonld.ParseISO8601Duration(jsonld.StringVal(recipe, "cookTime")),
		TotalTimeMinutes: jsonld.ParseISO8601Duration(jsonld.StringVal(recipe, "totalTime")),
		Servings:         jsonld.StringVal(recipe, "recipeYield"),
		Category:         jsonld.StringVal(recipe, "recipeCategory"),
		Cuisine:          jsonld.StringVal(recipe, "recipeCuisine"),
		Calories:         jsonld.NestedStringVal(recipe, "nutrition", "calories"),
		ImageURL:         extractImageURL(recipe),
		Rating:           jsonld.FloatVal(getNestedMap(recipe, "aggregateRating"), "ratingValue"),
		RatingCount:      jsonld.IntVal(getNestedMap(recipe, "aggregateRating"), "ratingCount"),
	}
}

// ingredientsSectionPattern matches "Ingredients:" section headers.
var ingredientsSectionPattern = regexp.MustCompile(`(?im)^(?:ingredients|ingredient list)\s*:?\s*$`)

func (e *RecipeExtractor) extractFromHeuristics(raw *domain.RawContent) *domain.RecipeResult {
	text := raw.RawText
	if text == "" {
		return nil
	}

	// Look for ingredients section
	hasIngredients := ingredientsSectionPattern.MatchString(text)
	hasInstructions := regexp.MustCompile(`(?im)^(?:instructions|directions|method|steps)\s*:?\s*$`).MatchString(text)

	if !hasIngredients && !hasInstructions {
		return nil
	}

	return &domain.RecipeResult{
		ExtractionMethod: "heuristic",
		Ingredients:      extractSectionItems(text, ingredientsSectionPattern),
		Instructions:     extractSectionText(text, regexp.MustCompile(`(?im)^(?:instructions|directions|method|steps)\s*:?\s*$`)),
	}
}

// extractInstructions handles both string and array instruction formats.
func extractInstructions(recipe map[string]any) string {
	// Try as string first
	if s := jsonld.StringVal(recipe, "recipeInstructions"); s != "" {
		return s
	}
	// Try as array of strings
	if steps := jsonld.StringSliceVal(recipe, "recipeInstructions"); len(steps) > 0 {
		return strings.Join(steps, "\n")
	}
	// Try as array of HowToStep objects
	if arr, ok := recipe["recipeInstructions"].([]any); ok {
		var steps []string
		for _, item := range arr {
			if m, ok := item.(map[string]any); ok {
				if text := jsonld.StringVal(m, "text"); text != "" {
					steps = append(steps, text)
				}
			}
		}
		if len(steps) > 0 {
			return strings.Join(steps, "\n")
		}
	}
	return ""
}

func extractImageURL(recipe map[string]any) string {
	// Try as string
	if s := jsonld.StringVal(recipe, "image"); s != "" {
		return s
	}
	// Try as object with url field
	return jsonld.NestedStringVal(recipe, "image", "url")
}

func getNestedMap(m map[string]any, key string) map[string]any {
	if nested, ok := m[key].(map[string]any); ok {
		return nested
	}
	return map[string]any{}
}

// extractSectionItems extracts bullet/list items after a section header.
func extractSectionItems(text string, headerPattern *regexp.Regexp) []string {
	loc := headerPattern.FindStringIndex(text)
	if loc == nil {
		return nil
	}

	section := text[loc[1]:]
	lines := strings.Split(section, "\n")
	var items []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Stop at next section header
		if regexp.MustCompile(`(?i)^(?:instructions|directions|method|steps|notes)\s*:`).MatchString(line) {
			break
		}
		// Strip bullet markers
		line = strings.TrimLeft(line, "-*•")
		line = regexp.MustCompile(`^\d+[.)]\s*`).ReplaceAllString(line, "")
		line = strings.TrimSpace(line)
		if line != "" {
			items = append(items, line)
		}
	}
	return items
}

// extractSectionText extracts all text after a section header until the next section.
func extractSectionText(text string, headerPattern *regexp.Regexp) string {
	loc := headerPattern.FindStringIndex(text)
	if loc == nil {
		return ""
	}

	section := text[loc[1]:]
	lines := strings.Split(section, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" && len(result) > 0 {
			// Allow one blank line, stop at two consecutive
			if len(result) > 0 && result[len(result)-1] == "" {
				break
			}
			result = append(result, "")
			continue
		}
		// Stop at next section header
		if regexp.MustCompile(`(?i)^(?:ingredients|notes|nutrition|tips)\s*:`).MatchString(trimmed) {
			break
		}
		result = append(result, trimmed)
	}
	return strings.TrimSpace(strings.Join(result, "\n"))
}

// containsTopic checks if a topic exists in the slice.
func containsTopic(topics []string, target string) bool {
	for _, t := range topics {
		if t == target {
			return true
		}
	}
	return false
}
```

**Step 4: Run tests to verify they pass**

Run: `cd classifier && GOWORK=off go test ./internal/classifier/... -run TestRecipeExtractor -v`
Expected: PASS

**Step 5: Run linter**

Run: `cd classifier && GOWORK=off golangci-lint run ./internal/classifier/...`
Expected: No errors

**Step 6: Commit**

```bash
git add classifier/internal/classifier/recipe_extractor.go classifier/internal/classifier/recipe_extractor_test.go
git commit -m "feat(classifier): add recipe extractor with Schema.org and heuristic support"
```

---

### Task 4: Create job extractor

**Files:**
- Create: `classifier/internal/classifier/job_extractor.go`
- Create: `classifier/internal/classifier/job_extractor_test.go`

**Step 1: Write the failing tests**

Create `classifier/internal/classifier/job_extractor_test.go`:

```go
package classifier_test

import (
	"context"
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/classifier"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

func TestJobExtractor_SchemaOrg(t *testing.T) {
	t.Helper()

	logger := infralogger.NewNoop()
	extractor := classifier.NewJobExtractor(logger)

	raw := &domain.RawContent{
		ID: "test-1",
		RawHTML: `<html><head>
		<script type="application/ld+json">
		{
			"@type": "JobPosting",
			"title": "Senior Go Developer",
			"hiringOrganization": {"@type": "Organization", "name": "Acme Corp"},
			"jobLocation": {"address": {"addressLocality": "Toronto", "addressRegion": "ON"}},
			"baseSalary": {"value": {"minValue": 120000, "maxValue": 160000}, "currency": "CAD"},
			"employmentType": "FULL_TIME",
			"datePosted": "2026-02-20",
			"validThrough": "2026-03-20",
			"description": "We are looking for an experienced Go developer.",
			"industry": "Technology"
		}
		</script>
		</head><body></body></html>`,
	}

	result, err := extractor.Extract(context.Background(), raw, domain.ContentTypeJob, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ExtractionMethod != "schema_org" {
		t.Errorf("expected extraction_method 'schema_org', got %q", result.ExtractionMethod)
	}
	if result.Title != "Senior Go Developer" {
		t.Errorf("expected title 'Senior Go Developer', got %q", result.Title)
	}
	if result.Company != "Acme Corp" {
		t.Errorf("expected company 'Acme Corp', got %q", result.Company)
	}
	if result.EmploymentType != "full_time" {
		t.Errorf("expected employment_type 'full_time', got %q", result.EmploymentType)
	}
}

func TestJobExtractor_NotApplicable(t *testing.T) {
	t.Helper()

	logger := infralogger.NewNoop()
	extractor := classifier.NewJobExtractor(logger)

	raw := &domain.RawContent{
		ID:      "test-2",
		RawHTML: `<html><body>Just an article</body></html>`,
	}

	result, err := extractor.Extract(context.Background(), raw, domain.ContentTypeArticle, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result for non-job content")
	}
}

func TestJobExtractor_HeuristicFallback(t *testing.T) {
	t.Helper()

	logger := infralogger.NewNoop()
	extractor := classifier.NewJobExtractor(logger)

	raw := &domain.RawContent{
		ID:      "test-3",
		RawHTML: `<html><body>No schema</body></html>`,
		RawText: "Senior Developer Position\n\nCompany: TechStart Inc\nLocation: Vancouver, BC\nSalary: $100,000 - $130,000\n\nRequirements:\n- 5 years experience\n- Go proficiency",
	}

	result, err := extractor.Extract(context.Background(), raw, domain.ContentTypeArticle, []string{"jobs"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result from heuristic extraction")
	}
	if result.ExtractionMethod != "heuristic" {
		t.Errorf("expected extraction_method 'heuristic', got %q", result.ExtractionMethod)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd classifier && GOWORK=off go test ./internal/classifier/... -run TestJobExtractor -v`
Expected: FAIL

**Step 3: Write implementation**

Create `classifier/internal/classifier/job_extractor.go`:

```go
package classifier

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/jonesrussell/north-cloud/classifier/internal/classifier/jsonld"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// JobExtractor extracts structured job posting data from raw content.
type JobExtractor struct {
	logger infralogger.Logger
}

// NewJobExtractor creates a JobExtractor.
func NewJobExtractor(logger infralogger.Logger) *JobExtractor {
	return &JobExtractor{logger: logger}
}

// Extract attempts to extract structured job fields from raw content.
// Returns (nil, nil) when content is not a job posting.
func (e *JobExtractor) Extract(
	_ context.Context, raw *domain.RawContent, contentType string, topics []string,
) (*domain.JobResult, error) {
	isJobType := contentType == domain.ContentTypeJob
	hasJobTopic := containsTopic(topics, "jobs")

	if !isJobType && !hasJobTopic {
		return nil, nil
	}

	// Tier 1: Schema.org JSON-LD extraction
	if result := e.extractFromSchemaOrg(raw); result != nil {
		e.logger.Debug("Job extracted via Schema.org",
			infralogger.String("content_id", raw.ID),
			infralogger.String("title", result.Title),
		)
		return result, nil
	}

	// Tier 2: Heuristic extraction
	if hasJobTopic {
		if result := e.extractFromHeuristics(raw); result != nil {
			e.logger.Debug("Job extracted via heuristics",
				infralogger.String("content_id", raw.ID),
			)
			return result, nil
		}
	}

	if isJobType {
		return &domain.JobResult{ExtractionMethod: "schema_org"}, nil
	}

	return nil, nil
}

func (e *JobExtractor) extractFromSchemaOrg(raw *domain.RawContent) *domain.JobResult {
	if raw.RawHTML == "" {
		return nil
	}

	blocks := jsonld.Extract(raw.RawHTML)
	job := jsonld.FindByType(blocks, "JobPosting")
	if job == nil {
		return nil
	}

	salaryMin, salaryMax, salaryCurrency := extractSalary(job)

	return &domain.JobResult{
		ExtractionMethod: "schema_org",
		Title:            jsonld.StringVal(job, "title"),
		Company:          jsonld.NestedStringVal(job, "hiringOrganization", "name"),
		Location:         extractJobLocation(job),
		SalaryMin:        salaryMin,
		SalaryMax:        salaryMax,
		SalaryCurrency:   salaryCurrency,
		EmploymentType:   normalizeEmploymentType(jsonld.StringVal(job, "employmentType")),
		PostedDate:       jsonld.StringVal(job, "datePosted"),
		ExpiresDate:      jsonld.StringVal(job, "validThrough"),
		Description:      jsonld.StringVal(job, "description"),
		Industry:         jsonld.StringVal(job, "industry"),
		Qualifications:   jsonld.StringVal(job, "qualifications"),
		Benefits:         jsonld.StringVal(job, "jobBenefits"),
	}
}

// companyPattern matches "Company: <value>" lines.
var companyPattern = regexp.MustCompile(`(?im)^company\s*:\s*(.+)$`)

// locationPattern matches "Location: <value>" lines.
var locationPattern = regexp.MustCompile(`(?im)^location\s*:\s*(.+)$`)

func (e *JobExtractor) extractFromHeuristics(raw *domain.RawContent) *domain.JobResult {
	text := raw.RawText
	if text == "" {
		return nil
	}

	result := &domain.JobResult{ExtractionMethod: "heuristic"}
	found := false

	if m := companyPattern.FindStringSubmatch(text); len(m) > 1 {
		result.Company = strings.TrimSpace(m[1])
		found = true
	}
	if m := locationPattern.FindStringSubmatch(text); len(m) > 1 {
		result.Location = strings.TrimSpace(m[1])
		found = true
	}

	// Extract requirements/qualifications section
	reqPattern := regexp.MustCompile(`(?im)^(?:requirements|qualifications|what we.re looking for)\s*:?\s*$`)
	if section := extractSectionText(text, reqPattern); section != "" {
		result.Qualifications = section
		found = true
	}

	if !found {
		return nil
	}

	return result
}

// extractSalary extracts salary range from Schema.org baseSalary object.
// Schema.org supports multiple formats: {value: {minValue, maxValue}, currency}
// or {value: number, currency}.
func extractSalary(job map[string]any) (*float64, *float64, string) {
	salary, ok := job["baseSalary"].(map[string]any)
	if !ok {
		return nil, nil, ""
	}

	currency := jsonld.StringVal(salary, "currency")

	// Try nested value object with min/max
	if valueObj, ok := salary["value"].(map[string]any); ok {
		minVal := jsonld.FloatVal(valueObj, "minValue")
		maxVal := jsonld.FloatVal(valueObj, "maxValue")
		return minVal, maxVal, currency
	}

	// Try direct numeric value
	if val := jsonld.FloatVal(salary, "value"); val != nil {
		return val, val, currency
	}

	return nil, nil, currency
}

// extractJobLocation normalizes location from Schema.org jobLocation.
func extractJobLocation(job map[string]any) string {
	loc, ok := job["jobLocation"].(map[string]any)
	if !ok {
		return ""
	}

	// Try address object
	addr, ok := loc["address"].(map[string]any)
	if !ok {
		return jsonld.StringVal(loc, "name")
	}

	city := jsonld.StringVal(addr, "addressLocality")
	region := jsonld.StringVal(addr, "addressRegion")

	parts := make([]string, 0, 2) //nolint:mnd // city + region
	if city != "" {
		parts = append(parts, city)
	}
	if region != "" {
		parts = append(parts, region)
	}

	if len(parts) > 0 {
		return strings.Join(parts, ", ")
	}

	return fmt.Sprintf("%s, %s", jsonld.StringVal(addr, "streetAddress"), jsonld.StringVal(addr, "addressCountry"))
}

// normalizeEmploymentType converts Schema.org employment types to snake_case.
func normalizeEmploymentType(raw string) string {
	normalized := map[string]string{
		"FULL_TIME":  "full_time",
		"PART_TIME":  "part_time",
		"CONTRACT":   "contract",
		"TEMPORARY":  "temporary",
		"INTERN":     "internship",
		"INTERNSHIP": "internship",
		"VOLUNTEER":  "volunteer",
		"PER_DIEM":   "per_diem",
		"OTHER":      "other",
	}
	if v, ok := normalized[strings.ToUpper(strings.TrimSpace(raw))]; ok {
		return v
	}
	return strings.ToLower(strings.TrimSpace(raw))
}
```

**Step 4: Run tests to verify they pass**

Run: `cd classifier && GOWORK=off go test ./internal/classifier/... -run TestJobExtractor -v`
Expected: PASS

**Step 5: Run linter**

Run: `cd classifier && GOWORK=off golangci-lint run ./internal/classifier/...`
Expected: No errors

**Step 6: Commit**

```bash
git add classifier/internal/classifier/job_extractor.go classifier/internal/classifier/job_extractor_test.go
git commit -m "feat(classifier): add job extractor with Schema.org and heuristic support"
```

---

## Phase 2: Detection & Pipeline Integration

### Task 5: Add Schema.org content type detection

**Files:**
- Modify: `classifier/internal/classifier/content_type.go`
- Modify: `classifier/internal/classifier/content_type_test.go` (add tests)

**Step 1: Write the failing tests**

Add to the existing content type test file:

```go
func TestContentTypeClassifier_SchemaOrgRecipe(t *testing.T) {
	t.Helper()

	logger := infralogger.NewNoop()
	c := classifier.NewContentTypeClassifier(logger)

	raw := &domain.RawContent{
		ID: "test-schema-recipe",
		RawHTML: `<html><head>
		<script type="application/ld+json">{"@type": "Recipe", "name": "Test"}</script>
		</head><body></body></html>`,
		OGType: "article", // Should be overridden by Schema.org
		Title:  "Test Recipe",
	}

	result, err := c.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Type != domain.ContentTypeRecipe {
		t.Errorf("expected content type %q, got %q", domain.ContentTypeRecipe, result.Type)
	}
	if result.Method != "schema_org" {
		t.Errorf("expected method 'schema_org', got %q", result.Method)
	}
	if result.Confidence != 1.0 {
		t.Errorf("expected confidence 1.0, got %f", result.Confidence)
	}
}

func TestContentTypeClassifier_SchemaOrgJobPosting(t *testing.T) {
	t.Helper()

	logger := infralogger.NewNoop()
	c := classifier.NewContentTypeClassifier(logger)

	raw := &domain.RawContent{
		ID: "test-schema-job",
		RawHTML: `<html><head>
		<script type="application/ld+json">{"@type": "JobPosting", "title": "Developer"}</script>
		</head><body></body></html>`,
		Title: "Job Opening",
	}

	result, err := c.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Type != domain.ContentTypeJob {
		t.Errorf("expected content type %q, got %q", domain.ContentTypeJob, result.Type)
	}
	if result.Method != "schema_org" {
		t.Errorf("expected method 'schema_org', got %q", result.Method)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd classifier && GOWORK=off go test ./internal/classifier/... -run TestContentTypeClassifier_SchemaOrg -v`
Expected: FAIL (no schema_org detection yet)

**Step 3: Implement Schema.org detection**

In `content_type.go`, add a new strategy after Strategy 0a (`classifyFromDetectedType`) and before Strategy 0b (`isNonArticleURL`). Insert between lines 87 and 89:

```go
// Strategy 0a.5: Check Schema.org JSON-LD structured data (high precision)
// This runs early and short-circuits all subsequent checks when found.
if result := c.classifyFromSchemaOrg(raw); result != nil {
	return result, nil
}
```

Add the implementation method:

```go
// classifyFromSchemaOrg checks for Schema.org JSON-LD structured data in raw HTML.
// Returns non-nil for Recipe and JobPosting types, nil otherwise.
func (c *ContentTypeClassifier) classifyFromSchemaOrg(raw *domain.RawContent) *ContentTypeResult {
	if raw.RawHTML == "" {
		return nil
	}

	blocks := jsonld.Extract(raw.RawHTML)
	if len(blocks) == 0 {
		return nil
	}

	if jsonld.FindByType(blocks, "Recipe") != nil {
		c.logger.Debug("Content type detected via Schema.org Recipe",
			infralogger.String("content_id", raw.ID),
		)
		return &ContentTypeResult{
			Type:       domain.ContentTypeRecipe,
			Confidence: schemaOrgConfidence,
			Method:     "schema_org",
			Reason:     "Schema.org JSON-LD Recipe type detected",
		}
	}

	if jsonld.FindByType(blocks, "JobPosting") != nil {
		c.logger.Debug("Content type detected via Schema.org JobPosting",
			infralogger.String("content_id", raw.ID),
		)
		return &ContentTypeResult{
			Type:       domain.ContentTypeJob,
			Confidence: schemaOrgConfidence,
			Method:     "schema_org",
			Reason:     "Schema.org JSON-LD JobPosting type detected",
		}
	}

	return nil
}
```

Add the constant:

```go
const schemaOrgConfidence = 1.0
```

Add the import for the jsonld package.

**Step 4: Run tests to verify they pass**

Run: `cd classifier && GOWORK=off go test ./internal/classifier/... -run TestContentTypeClassifier -v`
Expected: PASS

**Step 5: Run all classifier tests**

Run: `cd classifier && GOWORK=off go test ./... -v`
Expected: PASS (no regressions)

**Step 6: Run linter**

Run: `cd classifier && GOWORK=off golangci-lint run`
Expected: No errors

**Step 7: Commit**

```bash
git add classifier/internal/classifier/content_type.go classifier/internal/classifier/content_type_test.go
git commit -m "feat(classifier): add Schema.org JSON-LD content type detection for Recipe and JobPosting"
```

---

### Task 6: Add classification rules migration

**Files:**
- Create: `classifier/migrations/012_add_recipe_job_topics.up.sql`
- Create: `classifier/migrations/012_add_recipe_job_topics.down.sql`

**Step 1: Create up migration**

```sql
-- Add recipe and job topic keyword rules for fallback detection
-- when Schema.org structured data is not present.
INSERT INTO classification_rules (type, topic, keywords, priority, min_confidence, enabled)
VALUES
    ('topic', 'recipe', '["recipe","ingredients","prep time","cook time","servings","tablespoon","preheat","bake at"]', 80, 0.5, true),
    ('topic', 'jobs', '["job posting","apply now","salary","qualifications","employment type","full-time","part-time","hiring","career opportunity"]', 80, 0.5, true);
```

**Step 2: Create down migration**

```sql
DELETE FROM classification_rules WHERE topic IN ('recipe', 'jobs') AND type = 'topic';
```

**Step 3: Commit**

```bash
git add classifier/migrations/012_add_recipe_job_topics.up.sql classifier/migrations/012_add_recipe_job_topics.down.sql
git commit -m "feat(classifier): add recipe and job topic keyword rules migration"
```

---

### Task 7: Wire extractors into classifier pipeline

**Files:**
- Modify: `classifier/internal/classifier/classifier.go`

**Step 1: Add extractor fields to Classifier struct**

Find the Classifier struct and add:

```go
recipeExtractor *RecipeExtractor
jobExtractor    *JobExtractor
```

**Step 2: Update NewClassifier (or equivalent constructor) to accept extractors**

Add parameters for the extractors and assign them in the constructor.

**Step 3: Add extraction call in Classify method**

In `classifier.go:Classify()`, after the optional classifiers call (line 171) and before the reputation update (line 174), add:

```go
// 5b. Structured extraction — recipes and jobs (not ML-based, no sidecar needed)
recipeResult := c.runRecipeExtraction(ctx, raw, contentTypeResult.Type, topicResult.Topics)
jobResult := c.runJobExtraction(ctx, raw, contentTypeResult.Type, topicResult.Topics)
```

Add the helper methods:

```go
func (c *Classifier) runRecipeExtraction(
	ctx context.Context, raw *domain.RawContent, contentType string, topics []string,
) *domain.RecipeResult {
	if c.recipeExtractor == nil {
		return nil
	}
	result, err := c.recipeExtractor.Extract(ctx, raw, contentType, topics)
	if err != nil {
		c.logger.Warn("Recipe extraction failed",
			infralogger.String("content_id", raw.ID),
			infralogger.Error(err),
		)
		return nil
	}
	return result
}

func (c *Classifier) runJobExtraction(
	ctx context.Context, raw *domain.RawContent, contentType string, topics []string,
) *domain.JobResult {
	if c.jobExtractor == nil {
		return nil
	}
	result, err := c.jobExtractor.Extract(ctx, raw, contentType, topics)
	if err != nil {
		c.logger.Warn("Job extraction failed",
			infralogger.String("content_id", raw.ID),
			infralogger.Error(err),
		)
		return nil
	}
	return result
}
```

**Step 4: Add results to ClassificationResult**

In the result builder (around line 207-212), add:

```go
Recipe:  recipeResult,
Job:     jobResult,
```

**Step 5: Update BuildClassifiedContent**

In the `BuildClassifiedContent` function, add Recipe and Job fields to the output struct.

**Step 6: Run tests**

Run: `cd classifier && GOWORK=off go test ./... -v`
Expected: PASS

**Step 7: Run linter**

Run: `cd classifier && GOWORK=off golangci-lint run`
Expected: No errors

**Step 8: Commit**

```bash
git add classifier/internal/classifier/classifier.go
git commit -m "feat(classifier): wire recipe and job extractors into classification pipeline"
```

---

## Phase 3: Elasticsearch Mappings

### Task 8: Add recipe and job mappings to classifier

**Files:**
- Modify: `classifier/internal/elasticsearch/mappings/classified_content.go`

**Step 1: Add getRecipeMapping() helper**

Follow the pattern of existing domain mapping helpers (e.g., `getCrimeMapping()`):

```go
func getRecipeMapping() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"extraction_method":  map[string]any{"type": "keyword"},
			"name":              map[string]any{"type": "text", "analyzer": "standard"},
			"ingredients":       map[string]any{"type": "text", "analyzer": "standard"},
			"instructions":      map[string]any{"type": "text", "analyzer": "standard"},
			"prep_time_minutes": map[string]any{"type": "integer"},
			"cook_time_minutes": map[string]any{"type": "integer"},
			"total_time_minutes": map[string]any{"type": "integer"},
			"servings":          map[string]any{"type": "keyword"},
			"category":          map[string]any{"type": "keyword"},
			"cuisine":           map[string]any{"type": "keyword"},
			"calories":          map[string]any{"type": "keyword"},
			"image_url":         map[string]any{"type": "keyword", "index": false},
			"rating":            map[string]any{"type": "float"},
			"rating_count":      map[string]any{"type": "integer"},
		},
	}
}
```

**Step 2: Add getJobMapping() helper**

```go
func getJobMapping() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"extraction_method": map[string]any{"type": "keyword"},
			"title":            map[string]any{"type": "text", "analyzer": "standard"},
			"company":          map[string]any{"type": "keyword"},
			"location":         map[string]any{"type": "keyword"},
			"salary_min":       map[string]any{"type": "float"},
			"salary_max":       map[string]any{"type": "float"},
			"salary_currency":  map[string]any{"type": "keyword"},
			"employment_type":  map[string]any{"type": "keyword"},
			"posted_date":      map[string]any{"type": "date", "format": "strict_date_optional_time||epoch_millis"},
			"expires_date":     map[string]any{"type": "date", "format": "strict_date_optional_time||epoch_millis"},
			"description":      map[string]any{"type": "text", "analyzer": "standard"},
			"industry":         map[string]any{"type": "keyword"},
			"qualifications":   map[string]any{"type": "text", "analyzer": "standard"},
			"benefits":         map[string]any{"type": "text", "analyzer": "standard"},
		},
	}
}
```

**Step 3: Add to classified content properties**

Add `"recipe": getRecipeMapping()` and `"job": getJobMapping()` to the properties map in the mapping builder function.

**Step 4: Run tests and linter**

Run: `cd classifier && GOWORK=off go test ./... -v && golangci-lint run`
Expected: PASS, no lint errors

**Step 5: Commit**

```bash
git add classifier/internal/elasticsearch/mappings/classified_content.go
git commit -m "feat(classifier): add recipe and job Elasticsearch mappings"
```

---

### Task 9: Add recipe and job mappings to index-manager

**Files:**
- Modify: `index-manager/internal/elasticsearch/mappings/classified_content.go` (or equivalent)

**Step 1: Add matching recipe and job mappings**

Add the same mapping structure as Task 8 to the index-manager's classified content mapping. This prevents mapping drift between classifier and index-manager.

**Step 2: Run tests and linter**

Run: `cd index-manager && GOWORK=off go test ./... -v && golangci-lint run`
Expected: PASS

**Step 3: Commit**

```bash
git add index-manager/
git commit -m "feat(index-manager): add recipe and job Elasticsearch mappings"
```

---

## Phase 4: Publisher Routing

### Task 10: Add RecipeData and JobData to publisher Article struct

**Files:**
- Modify: `publisher/internal/router/article.go`

**Step 1: Add data structs**

Add after EntertainmentData (line 67):

```go
// RecipeData holds structured recipe extraction fields from Elasticsearch.
type RecipeData struct {
	ExtractionMethod string   `json:"extraction_method"`
	Name             string   `json:"name,omitempty"`
	Ingredients      []string `json:"ingredients,omitempty"`
	Category         string   `json:"category,omitempty"`
	Cuisine          string   `json:"cuisine,omitempty"`
}

// JobData holds structured job extraction fields from Elasticsearch.
type JobData struct {
	ExtractionMethod string `json:"extraction_method"`
	Title            string `json:"title,omitempty"`
	Company          string `json:"company,omitempty"`
	Location         string `json:"location,omitempty"`
	EmploymentType   string `json:"employment_type,omitempty"`
	Industry         string `json:"industry,omitempty"`
}
```

**Step 2: Add to Article struct**

Add after the Coforge field (line 109):

```go
// Recipe and Job structured extraction
Recipe *RecipeData `json:"recipe,omitempty"`
Job    *JobData    `json:"job,omitempty"`
```

**Step 3: No extractNestedFields changes needed**

Recipe and job routing domains will read directly from the nested objects (like Anishinaabe does).

**Step 4: Run tests and linter**

Run: `cd publisher && GOWORK=off go test ./... -v && golangci-lint run`
Expected: PASS

**Step 5: Commit**

```bash
git add publisher/internal/router/article.go
git commit -m "feat(publisher): add RecipeData and JobData to Article struct"
```

---

### Task 11: Create recipe routing domain (Layer 9)

**Files:**
- Create: `publisher/internal/router/domain_recipe.go`
- Create: `publisher/internal/router/domain_recipe_test.go`

**Step 1: Write the failing test**

```go
package router

import (
	"testing"
)

func TestRecipeDomain_NilRecipe(t *testing.T) {
	t.Helper()
	d := NewRecipeDomain()
	routes := d.Routes(&Article{})
	if routes != nil {
		t.Error("expected nil routes for article without recipe data")
	}
}

func TestRecipeDomain_WithCategoryAndCuisine(t *testing.T) {
	t.Helper()
	d := NewRecipeDomain()
	article := &Article{
		Recipe: &RecipeData{
			ExtractionMethod: "schema_org",
			Category:         "Dessert",
			Cuisine:          "Italian",
		},
	}
	routes := d.Routes(article)
	if len(routes) == 0 {
		t.Fatal("expected routes")
	}

	channels := make(map[string]bool)
	for _, r := range routes {
		channels[r.Channel] = true
	}

	if !channels["articles:recipes"] {
		t.Error("expected articles:recipes channel")
	}
	if !channels["recipes:category:dessert"] {
		t.Error("expected recipes:category:dessert channel")
	}
	if !channels["recipes:cuisine:italian"] {
		t.Error("expected recipes:cuisine:italian channel")
	}
}

func TestRecipeDomain_Name(t *testing.T) {
	t.Helper()
	d := NewRecipeDomain()
	if d.Name() != "recipe" {
		t.Errorf("expected name 'recipe', got %q", d.Name())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd publisher && GOWORK=off go test ./internal/router/... -run TestRecipeDomain -v`
Expected: FAIL

**Step 3: Write implementation**

Create `publisher/internal/router/domain_recipe.go`:

```go
package router

import "strings"

// RecipeDomain routes recipe-classified content to recipes:* channels.
// Channels produced:
//   - articles:recipes (catch-all)
//   - recipes:category:{slug} (per category, e.g. recipes:category:dessert)
//   - recipes:cuisine:{slug} (per cuisine, e.g. recipes:cuisine:italian)
type RecipeDomain struct{}

// NewRecipeDomain creates a RecipeDomain.
func NewRecipeDomain() *RecipeDomain { return &RecipeDomain{} }

// Name returns the domain identifier.
func (d *RecipeDomain) Name() string { return "recipe" }

// Routes returns recipe channels for the article.
func (d *RecipeDomain) Routes(a *Article) []ChannelRoute {
	if a.Recipe == nil {
		return nil
	}

	channels := []string{"articles:recipes"}

	if a.Recipe.Category != "" {
		slug := strings.ToLower(strings.ReplaceAll(a.Recipe.Category, " ", "-"))
		channels = append(channels, "recipes:category:"+slug)
	}

	if a.Recipe.Cuisine != "" {
		slug := strings.ToLower(strings.ReplaceAll(a.Recipe.Cuisine, " ", "-"))
		channels = append(channels, "recipes:cuisine:"+slug)
	}

	return channelRoutesFromSlice(channels)
}
```

**Step 4: Run tests to verify they pass**

Run: `cd publisher && GOWORK=off go test ./internal/router/... -run TestRecipeDomain -v`
Expected: PASS

**Step 5: Run linter**

Run: `cd publisher && GOWORK=off golangci-lint run`
Expected: No errors

**Step 6: Commit**

```bash
git add publisher/internal/router/domain_recipe.go publisher/internal/router/domain_recipe_test.go
git commit -m "feat(publisher): add recipe routing domain (Layer 9)"
```

---

### Task 12: Create job routing domain (Layer 10)

**Files:**
- Create: `publisher/internal/router/domain_job.go`
- Create: `publisher/internal/router/domain_job_test.go`

**Step 1: Write the failing test**

```go
package router

import (
	"testing"
)

func TestJobDomain_NilJob(t *testing.T) {
	t.Helper()
	d := NewJobDomain()
	routes := d.Routes(&Article{})
	if routes != nil {
		t.Error("expected nil routes for article without job data")
	}
}

func TestJobDomain_WithTypeAndIndustry(t *testing.T) {
	t.Helper()
	d := NewJobDomain()
	article := &Article{
		Job: &JobData{
			ExtractionMethod: "schema_org",
			EmploymentType:   "full_time",
			Industry:         "Technology",
		},
	}
	routes := d.Routes(article)
	if len(routes) == 0 {
		t.Fatal("expected routes")
	}

	channels := make(map[string]bool)
	for _, r := range routes {
		channels[r.Channel] = true
	}

	if !channels["articles:jobs"] {
		t.Error("expected articles:jobs channel")
	}
	if !channels["jobs:type:full-time"] {
		t.Error("expected jobs:type:full-time channel")
	}
	if !channels["jobs:industry:technology"] {
		t.Error("expected jobs:industry:technology channel")
	}
}

func TestJobDomain_Name(t *testing.T) {
	t.Helper()
	d := NewJobDomain()
	if d.Name() != "job" {
		t.Errorf("expected name 'job', got %q", d.Name())
	}
}
```

**Step 2: Run test to verify it fails**

**Step 3: Write implementation**

Create `publisher/internal/router/domain_job.go`:

```go
package router

import "strings"

// JobDomain routes job-classified content to jobs:* channels.
// Channels produced:
//   - articles:jobs (catch-all)
//   - jobs:type:{slug} (per employment type, e.g. jobs:type:full-time)
//   - jobs:industry:{slug} (per industry, e.g. jobs:industry:technology)
type JobDomain struct{}

// NewJobDomain creates a JobDomain.
func NewJobDomain() *JobDomain { return &JobDomain{} }

// Name returns the domain identifier.
func (d *JobDomain) Name() string { return "job" }

// Routes returns job channels for the article.
func (d *JobDomain) Routes(a *Article) []ChannelRoute {
	if a.Job == nil {
		return nil
	}

	channels := []string{"articles:jobs"}

	if a.Job.EmploymentType != "" {
		slug := strings.ToLower(strings.ReplaceAll(a.Job.EmploymentType, "_", "-"))
		channels = append(channels, "jobs:type:"+slug)
	}

	if a.Job.Industry != "" {
		slug := strings.ToLower(strings.ReplaceAll(a.Job.Industry, " ", "-"))
		channels = append(channels, "jobs:industry:"+slug)
	}

	return channelRoutesFromSlice(channels)
}
```

**Step 4: Run tests, linter, commit**

Run: `cd publisher && GOWORK=off go test ./internal/router/... -run TestJobDomain -v`
Expected: PASS

```bash
git add publisher/internal/router/domain_job.go publisher/internal/router/domain_job_test.go
git commit -m "feat(publisher): add job routing domain (Layer 10)"
```

---

### Task 13: Wire routing domains and expand fetch filter

**Files:**
- Modify: `publisher/internal/router/service.go` (lines 195-204 and 320-324)
- Modify: `publisher/internal/router/domain_topic.go` (lines 6-10)

**Step 1: Add recipe and job domains to routing**

In `service.go:routeArticle()` (line 203), add after `NewCoforgeDomain()`:

```go
NewRecipeDomain(),
NewJobDomain(),
```

**Step 2: Expand fetch filter to include recipe and job content types**

In `service.go:buildESQuery()` (lines 320-326), change from:

```go
"term": map[string]any{
	"content_type": "article",
}
```

To:

```go
"terms": map[string]any{
	"content_type": []string{"article", "recipe", "job"},
}
```

**Step 3: Add recipe and jobs to layer1SkipTopics**

In `domain_topic.go` (lines 6-10), add:

```go
"recipe": true,
"jobs":   true,
```

**Step 4: Run all publisher tests**

Run: `cd publisher && GOWORK=off go test ./... -v`
Expected: PASS

**Step 5: Run linter**

Run: `cd publisher && GOWORK=off golangci-lint run`
Expected: No errors

**Step 6: Commit**

```bash
git add publisher/internal/router/service.go publisher/internal/router/domain_topic.go
git commit -m "feat(publisher): wire recipe/job routing domains and expand content type filter"
```

---

## Phase 5: Search Service

### Task 14: Add recipe and job filters and facets to search

**Files:**
- Modify: `search/internal/domain/search.go`
- Modify: `search/internal/elasticsearch/query_builder.go`

**Step 1: Add new filter fields to Filters struct**

In `search.go`, add after `CrimeRelevance` (line 26):

```go
// Recipe filters
RecipeCuisine  []string `json:"recipe_cuisine,omitempty"`
RecipeCategory []string `json:"recipe_category,omitempty"`
MaxPrepTime    *int     `json:"max_prep_time,omitempty"`
MaxTotalTime   *int     `json:"max_total_time,omitempty"`

// Job filters
JobEmploymentType []string  `json:"job_employment_type,omitempty"`
JobIndustry       []string  `json:"job_industry,omitempty"`
JobLocation       []string  `json:"job_location,omitempty"`
SalaryMin         *float64  `json:"salary_min,omitempty"`
```

**Step 2: Add new facet fields to Facets struct**

In `search.go`, add after `QualityRanges` (line 86):

```go
RecipeCuisines   []FacetBucket `json:"recipe_cuisines,omitempty"`
RecipeCategories []FacetBucket `json:"recipe_categories,omitempty"`
JobTypes         []FacetBucket `json:"job_types,omitempty"`
JobIndustries    []FacetBucket `json:"job_industries,omitempty"`
JobLocations     []FacetBucket `json:"job_locations,omitempty"`
```

**Step 3: Add filter clauses in query builder**

In `query_builder.go`, in the filter-building function, add term/range filters for the new fields:

```go
// Recipe filters
if len(filters.RecipeCuisine) > 0 {
	filterClauses = append(filterClauses, map[string]any{
		"terms": map[string]any{"recipe.cuisine": filters.RecipeCuisine},
	})
}
if len(filters.RecipeCategory) > 0 {
	filterClauses = append(filterClauses, map[string]any{
		"terms": map[string]any{"recipe.category": filters.RecipeCategory},
	})
}
if filters.MaxPrepTime != nil {
	filterClauses = append(filterClauses, map[string]any{
		"range": map[string]any{"recipe.prep_time_minutes": map[string]any{"lte": *filters.MaxPrepTime}},
	})
}
if filters.MaxTotalTime != nil {
	filterClauses = append(filterClauses, map[string]any{
		"range": map[string]any{"recipe.total_time_minutes": map[string]any{"lte": *filters.MaxTotalTime}},
	})
}

// Job filters
if len(filters.JobEmploymentType) > 0 {
	filterClauses = append(filterClauses, map[string]any{
		"terms": map[string]any{"job.employment_type": filters.JobEmploymentType},
	})
}
if len(filters.JobIndustry) > 0 {
	filterClauses = append(filterClauses, map[string]any{
		"terms": map[string]any{"job.industry": filters.JobIndustry},
	})
}
if len(filters.JobLocation) > 0 {
	filterClauses = append(filterClauses, map[string]any{
		"terms": map[string]any{"job.location": filters.JobLocation},
	})
}
if filters.SalaryMin != nil {
	filterClauses = append(filterClauses, map[string]any{
		"range": map[string]any{"job.salary_min": map[string]any{"gte": *filters.SalaryMin}},
	})
}
```

**Step 4: Add aggregations in query builder**

```go
// Recipe facets
"recipe_cuisines":   map[string]any{"terms": map[string]any{"field": "recipe.cuisine", "size": facetSize}},
"recipe_categories": map[string]any{"terms": map[string]any{"field": "recipe.category", "size": facetSize}},

// Job facets
"job_types":      map[string]any{"terms": map[string]any{"field": "job.employment_type", "size": facetSize}},
"job_industries": map[string]any{"terms": map[string]any{"field": "job.industry", "size": facetSize}},
"job_locations":  map[string]any{"terms": map[string]any{"field": "job.location", "size": facetSize}},
```

**Step 5: Parse new facets in response handler**

Update the facet parsing code to extract the new aggregation buckets.

**Step 6: Run tests and linter**

Run: `cd search && GOWORK=off go test ./... -v && golangci-lint run`
Expected: PASS

**Step 7: Commit**

```bash
git add search/
git commit -m "feat(search): add recipe and job filters and facets"
```

---

## Phase 6: Bootstrap & Config

### Task 15: Add extractor config and bootstrap wiring

**Files:**
- Modify: `classifier/internal/config/config.go`
- Modify: `classifier/internal/bootstrap/classifier.go`

**Step 1: Add config structs**

Recipe and job extractors don't need ML service URLs (they're deterministic). Add simple enabled flags:

```go
// In ClassificationConfig struct
Recipe RecipeExtractionConfig `yaml:"recipe" env-prefix:"RECIPE_"`
Job    JobExtractionConfig    `yaml:"job"    env-prefix:"JOB_"`
```

```go
type RecipeExtractionConfig struct {
	Enabled bool `env:"ENABLED" yaml:"enabled"`
}

type JobExtractionConfig struct {
	Enabled bool `env:"ENABLED" yaml:"enabled"`
}
```

**Step 2: Wire extractors in bootstrap**

In `bootstrap/classifier.go`, create extractors when enabled and pass them to the Classifier constructor:

```go
var recipeExtractor *classifier.RecipeExtractor
if cfg.Classification.Recipe.Enabled {
	recipeExtractor = classifier.NewRecipeExtractor(logger)
	logger.Info("Recipe extractor enabled")
}

var jobExtractor *classifier.JobExtractor
if cfg.Classification.Job.Enabled {
	jobExtractor = classifier.NewJobExtractor(logger)
	logger.Info("Job extractor enabled")
}
```

Pass to the Classifier constructor.

**Step 3: Update docker-compose dev config**

Add environment variables to classifier service in `docker-compose.dev.yml`:

```yaml
RECIPE_ENABLED: "true"
JOB_ENABLED: "true"
```

**Step 4: Run tests and linter**

Run: `cd classifier && GOWORK=off go test ./... -v && golangci-lint run`
Expected: PASS

**Step 5: Commit**

```bash
git add classifier/internal/config/ classifier/internal/bootstrap/ docker-compose.dev.yml
git commit -m "feat(classifier): add recipe/job extractor config and bootstrap wiring"
```

---

## Verification

### Task 16: End-to-end verification

**Step 1: Run all service tests**

```bash
task test
```

Expected: All services pass.

**Step 2: Run all linters**

```bash
task lint:force
```

Expected: No lint errors.

**Step 3: Verify migration applies**

```bash
cd classifier && go run cmd/migrate/main.go up
```

Expected: Migration 012 applied successfully.

**Step 4: Manual smoke test**

Start dev environment and test with a recipe URL that has Schema.org markup. Verify:
- Content type detected as "recipe"
- Structured fields extracted (ingredients, instructions, etc.)
- Published to `articles:recipes` Redis channel
- Searchable via search API with `content_type: "recipe"` filter

---

## Summary

| Phase | Tasks | Services |
|-------|-------|----------|
| 1. Foundation | Domain models, JSON-LD parser, Recipe extractor, Job extractor | classifier |
| 2. Detection & Integration | Schema.org detection, Keyword rules migration, Pipeline wiring | classifier |
| 3. ES Mappings | Classifier mappings, Index-manager mappings | classifier, index-manager |
| 4. Publisher Routing | Article struct, Recipe domain, Job domain, Fetch filter | publisher |
| 5. Search | Filters, Facets, Query builder | search |
| 6. Bootstrap & Config | Config structs, Bootstrap wiring, Docker env | classifier |
