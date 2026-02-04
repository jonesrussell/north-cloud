# Publishing Pipeline Refinement - Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement content-based location detection and refined crime sub-labels to fix mislabeled articles in the North Cloud → Streetcode publishing pipeline.

**Architecture:**
- Add a new `LocationClassifier` to the classifier service that extracts locations from content (never inheriting from publisher)
- Extend `CrimeResult` with `sub_label` field to distinguish `criminal_justice` from `crime_context`
- Update publisher to route based on new location and crime sub-label fields

**Tech Stack:** Go 1.25, Elasticsearch, Redis Pub/Sub, PostgreSQL

---

## Phase 1: Canadian Cities Data (Foundation)

### Task 1.1: Create Canadian Cities Data File

**Files:**
- Create: `classifier/internal/data/canadian_cities.go`
- Test: `classifier/internal/data/canadian_cities_test.go`

**Step 1: Write the failing test**

```go
// classifier/internal/data/canadian_cities_test.go
package data

import (
	"testing"
)

func TestIsValidCanadianCity(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		city     string
		expected bool
	}{
		{"valid city lowercase", "sudbury", true},
		{"valid city mixed case", "Sudbury", true},
		{"valid city uppercase", "SUDBURY", true},
		{"valid major city", "toronto", true},
		{"invalid city", "new york", false},
		{"empty string", "", false},
		{"us city", "chicago", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidCanadianCity(tt.city)
			if result != tt.expected {
				t.Errorf("IsValidCanadianCity(%q) = %v, want %v", tt.city, result, tt.expected)
			}
		})
	}
}

func TestNormalizeCityName(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"greater prefix", "Greater Sudbury", "sudbury"},
		{"city prefix", "City of Toronto", "toronto"},
		{"sault ste marie variants", "Sault Ste. Marie", "sault-ste-marie"},
		{"st johns vs saint john", "St. John's", "st-johns"},
		{"simple lowercase", "Vancouver", "vancouver"},
		{"hyphenated", "Trois-Rivières", "trois-rivieres"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeCityName(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeCityName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetProvinceForCity(t *testing.T) {
	t.Helper()

	tests := []struct {
		name         string
		city         string
		wantProvince string
		wantOK       bool
	}{
		{"sudbury", "sudbury", "ON", true},
		{"toronto", "toronto", "ON", true},
		{"vancouver", "vancouver", "BC", true},
		{"montreal", "montreal", "QC", true},
		{"invalid city", "new york", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			province, ok := GetProvinceForCity(tt.city)
			if ok != tt.wantOK {
				t.Errorf("GetProvinceForCity(%q) ok = %v, want %v", tt.city, ok, tt.wantOK)
			}
			if province != tt.wantProvince {
				t.Errorf("GetProvinceForCity(%q) province = %q, want %q", tt.city, province, tt.wantProvince)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd classifier && go test ./internal/data/... -v
```

Expected: FAIL with "package classifier/internal/data is not in std"

**Step 3: Write minimal implementation**

```go
// classifier/internal/data/canadian_cities.go
package data

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// CityInfo contains metadata about a Canadian city.
type CityInfo struct {
	Canonical string // Normalized slug form
	Province  string // Two-letter province code
}

// canadianCities maps normalized city names to their info.
// This is a curated list of major Canadian cities and municipalities.
// Source: Statistics Canada Census Metropolitan Areas + major municipalities.
var canadianCities = map[string]CityInfo{
	// Ontario
	"toronto":           {Canonical: "toronto", Province: "ON"},
	"ottawa":            {Canonical: "ottawa", Province: "ON"},
	"mississauga":       {Canonical: "mississauga", Province: "ON"},
	"brampton":          {Canonical: "brampton", Province: "ON"},
	"hamilton":          {Canonical: "hamilton", Province: "ON"},
	"london":            {Canonical: "london", Province: "ON"},
	"markham":           {Canonical: "markham", Province: "ON"},
	"vaughan":           {Canonical: "vaughan", Province: "ON"},
	"kitchener":         {Canonical: "kitchener", Province: "ON"},
	"windsor":           {Canonical: "windsor", Province: "ON"},
	"sudbury":           {Canonical: "sudbury", Province: "ON"},
	"greater sudbury":   {Canonical: "sudbury", Province: "ON"},
	"thunder bay":       {Canonical: "thunder-bay", Province: "ON"},
	"north bay":         {Canonical: "north-bay", Province: "ON"},
	"sault ste marie":   {Canonical: "sault-ste-marie", Province: "ON"},
	"sault ste. marie":  {Canonical: "sault-ste-marie", Province: "ON"},
	"timmins":           {Canonical: "timmins", Province: "ON"},
	"peterborough":      {Canonical: "peterborough", Province: "ON"},
	"kingston":          {Canonical: "kingston", Province: "ON"},
	"guelph":            {Canonical: "guelph", Province: "ON"},
	"cambridge":         {Canonical: "cambridge", Province: "ON"},
	"waterloo":          {Canonical: "waterloo", Province: "ON"},
	"barrie":            {Canonical: "barrie", Province: "ON"},
	"oshawa":            {Canonical: "oshawa", Province: "ON"},
	"st catharines":     {Canonical: "st-catharines", Province: "ON"},
	"st. catharines":    {Canonical: "st-catharines", Province: "ON"},
	"niagara falls":     {Canonical: "niagara-falls", Province: "ON"},
	"welland":           {Canonical: "welland", Province: "ON"},
	"brantford":         {Canonical: "brantford", Province: "ON"},
	"sarnia":            {Canonical: "sarnia", Province: "ON"},
	"belleville":        {Canonical: "belleville", Province: "ON"},
	"cornwall":          {Canonical: "cornwall", Province: "ON"},
	"chatham":           {Canonical: "chatham", Province: "ON"},
	"orillia":           {Canonical: "orillia", Province: "ON"},
	"owen sound":        {Canonical: "owen-sound", Province: "ON"},
	"espanola":          {Canonical: "espanola", Province: "ON"},
	"elliot lake":       {Canonical: "elliot-lake", Province: "ON"},
	"kirkland lake":     {Canonical: "kirkland-lake", Province: "ON"},
	"kapuskasing":       {Canonical: "kapuskasing", Province: "ON"},
	"kenora":            {Canonical: "kenora", Province: "ON"},

	// Quebec
	"montreal":          {Canonical: "montreal", Province: "QC"},
	"quebec city":       {Canonical: "quebec-city", Province: "QC"},
	"quebec":            {Canonical: "quebec-city", Province: "QC"},
	"laval":             {Canonical: "laval", Province: "QC"},
	"gatineau":          {Canonical: "gatineau", Province: "QC"},
	"longueuil":         {Canonical: "longueuil", Province: "QC"},
	"sherbrooke":        {Canonical: "sherbrooke", Province: "QC"},
	"trois-rivieres":    {Canonical: "trois-rivieres", Province: "QC"},
	"trois rivieres":    {Canonical: "trois-rivieres", Province: "QC"},
	"chicoutimi":        {Canonical: "chicoutimi", Province: "QC"},
	"saguenay":          {Canonical: "saguenay", Province: "QC"},

	// British Columbia
	"vancouver":         {Canonical: "vancouver", Province: "BC"},
	"surrey":            {Canonical: "surrey", Province: "BC"},
	"burnaby":           {Canonical: "burnaby", Province: "BC"},
	"richmond":          {Canonical: "richmond", Province: "BC"},
	"victoria":          {Canonical: "victoria", Province: "BC"},
	"kelowna":           {Canonical: "kelowna", Province: "BC"},
	"abbotsford":        {Canonical: "abbotsford", Province: "BC"},
	"nanaimo":           {Canonical: "nanaimo", Province: "BC"},
	"kamloops":          {Canonical: "kamloops", Province: "BC"},
	"prince george":     {Canonical: "prince-george", Province: "BC"},
	"chilliwack":        {Canonical: "chilliwack", Province: "BC"},
	"vernon":            {Canonical: "vernon", Province: "BC"},
	"courtenay":         {Canonical: "courtenay", Province: "BC"},

	// Alberta
	"calgary":           {Canonical: "calgary", Province: "AB"},
	"edmonton":          {Canonical: "edmonton", Province: "AB"},
	"red deer":          {Canonical: "red-deer", Province: "AB"},
	"lethbridge":        {Canonical: "lethbridge", Province: "AB"},
	"medicine hat":      {Canonical: "medicine-hat", Province: "AB"},
	"grande prairie":    {Canonical: "grande-prairie", Province: "AB"},
	"fort mcmurray":     {Canonical: "fort-mcmurray", Province: "AB"},

	// Manitoba
	"winnipeg":          {Canonical: "winnipeg", Province: "MB"},
	"brandon":           {Canonical: "brandon", Province: "MB"},
	"steinbach":         {Canonical: "steinbach", Province: "MB"},
	"thompson":          {Canonical: "thompson", Province: "MB"},

	// Saskatchewan
	"saskatoon":         {Canonical: "saskatoon", Province: "SK"},
	"regina":            {Canonical: "regina", Province: "SK"},
	"prince albert":     {Canonical: "prince-albert", Province: "SK"},
	"moose jaw":         {Canonical: "moose-jaw", Province: "SK"},

	// Nova Scotia
	"halifax":           {Canonical: "halifax", Province: "NS"},
	"dartmouth":         {Canonical: "dartmouth", Province: "NS"},
	"sydney":            {Canonical: "sydney", Province: "NS"},
	"truro":             {Canonical: "truro", Province: "NS"},

	// New Brunswick
	"saint john":        {Canonical: "saint-john", Province: "NB"},
	"moncton":           {Canonical: "moncton", Province: "NB"},
	"fredericton":       {Canonical: "fredericton", Province: "NB"},

	// Newfoundland and Labrador
	"st johns":          {Canonical: "st-johns", Province: "NL"},
	"st. johns":         {Canonical: "st-johns", Province: "NL"},
	"st. john's":        {Canonical: "st-johns", Province: "NL"},
	"corner brook":      {Canonical: "corner-brook", Province: "NL"},
	"mount pearl":       {Canonical: "mount-pearl", Province: "NL"},

	// Prince Edward Island
	"charlottetown":     {Canonical: "charlottetown", Province: "PE"},
	"summerside":        {Canonical: "summerside", Province: "PE"},

	// Territories
	"whitehorse":        {Canonical: "whitehorse", Province: "YT"},
	"yellowknife":       {Canonical: "yellowknife", Province: "NT"},
	"iqaluit":           {Canonical: "iqaluit", Province: "NU"},
}

// prefixesToRemove are common prefixes that should be stripped for normalization.
var prefixesToRemove = []string{
	"greater ",
	"city of ",
	"town of ",
	"municipality of ",
	"regional municipality of ",
}

// IsValidCanadianCity checks if the given city name is in the validated Canadian cities list.
func IsValidCanadianCity(city string) bool {
	if city == "" {
		return false
	}
	normalized := normalizeForLookup(city)
	_, ok := canadianCities[normalized]
	return ok
}

// NormalizeCityName returns the canonical slug form of a city name.
// Returns empty string if the city is not found.
func NormalizeCityName(city string) string {
	if city == "" {
		return ""
	}
	normalized := normalizeForLookup(city)
	if info, ok := canadianCities[normalized]; ok {
		return info.Canonical
	}
	// If not found, return a best-effort slug
	return toSlug(city)
}

// GetProvinceForCity returns the province code for a Canadian city.
func GetProvinceForCity(city string) (string, bool) {
	if city == "" {
		return "", false
	}
	normalized := normalizeForLookup(city)
	if info, ok := canadianCities[normalized]; ok {
		return info.Province, true
	}
	return "", false
}

// normalizeForLookup prepares a city name for map lookup.
func normalizeForLookup(city string) string {
	s := strings.ToLower(strings.TrimSpace(city))

	// Remove common prefixes
	for _, prefix := range prefixesToRemove {
		if strings.HasPrefix(s, prefix) {
			s = strings.TrimPrefix(s, prefix)
			break
		}
	}

	// Remove accents for lookup
	s = removeAccents(s)

	return s
}

// toSlug converts a city name to a URL-safe slug.
func toSlug(city string) string {
	s := strings.ToLower(strings.TrimSpace(city))

	// Remove common prefixes
	for _, prefix := range prefixesToRemove {
		if strings.HasPrefix(s, prefix) {
			s = strings.TrimPrefix(s, prefix)
			break
		}
	}

	// Remove accents
	s = removeAccents(s)

	// Replace spaces and special chars with hyphens
	s = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(s, "-")

	// Clean up multiple hyphens and trim
	s = regexp.MustCompile(`-+`).ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")

	return s
}

// removeAccents strips diacritical marks from a string.
func removeAccents(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, _ := transform.String(t, s)
	return result
}
```

**Step 4: Run test to verify it passes**

```bash
cd classifier && go test ./internal/data/... -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add classifier/internal/data/
git commit -m "feat(classifier): add Canadian cities validation data

Add curated list of Canadian cities with province mappings for
location detection. Includes normalization functions to handle
variants like 'Greater Sudbury' -> 'sudbury', 'St. John's' -> 'st-johns'."
```

---

## Phase 2: Location Detection (Classifier)

### Task 2.1: Define Location Domain Types

**Files:**
- Modify: `classifier/internal/domain/classification.go`
- Test: `classifier/internal/domain/classification_test.go`

**Step 1: Write the failing test**

```go
// classifier/internal/domain/classification_test.go
package domain

import (
	"testing"
)

func TestLocationResult_Specificity(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		location LocationResult
		want     string
	}{
		{
			name:     "city specificity",
			location: LocationResult{City: "sudbury", Province: "ON", Country: "canada"},
			want:     "city",
		},
		{
			name:     "province specificity",
			location: LocationResult{Province: "ON", Country: "canada"},
			want:     "province",
		},
		{
			name:     "country specificity",
			location: LocationResult{Country: "canada"},
			want:     "country",
		},
		{
			name:     "unknown specificity",
			location: LocationResult{Country: "unknown"},
			want:     "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.location.GetSpecificity(); got != tt.want {
				t.Errorf("LocationResult.GetSpecificity() = %v, want %v", got, tt.want)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd classifier && go test ./internal/domain/... -v -run TestLocationResult
```

Expected: FAIL with "LocationResult undefined"

**Step 3: Write minimal implementation**

Add to `classifier/internal/domain/classification.go`:

```go
// LocationResult holds the detected location for an article.
type LocationResult struct {
	City        string  `json:"city,omitempty"`
	Province    string  `json:"province,omitempty"`
	Country     string  `json:"country"`
	Specificity string  `json:"specificity"`
	Confidence  float64 `json:"confidence"`
}

// Location specificity constants.
const (
	SpecificityCity     = "city"
	SpecificityProvince = "province"
	SpecificityCountry  = "country"
	SpecificityUnknown  = "unknown"
)

// GetSpecificity returns the specificity level based on populated fields.
func (l *LocationResult) GetSpecificity() string {
	if l.City != "" {
		return SpecificityCity
	}
	if l.Province != "" {
		return SpecificityProvince
	}
	if l.Country != "" && l.Country != "unknown" {
		return SpecificityCountry
	}
	return SpecificityUnknown
}
```

**Step 4: Run test to verify it passes**

```bash
cd classifier && go test ./internal/domain/... -v -run TestLocationResult
```

Expected: PASS

**Step 5: Commit**

```bash
git add classifier/internal/domain/classification.go classifier/internal/domain/classification_test.go
git commit -m "feat(classifier): add LocationResult domain type

Add location result struct with city/province/country fields and
specificity detection method."
```

---

### Task 2.2: Create Location Classifier - Entity Extraction

**Files:**
- Create: `classifier/internal/classifier/location.go`
- Test: `classifier/internal/classifier/location_test.go`

**Step 1: Write the failing test**

```go
// classifier/internal/classifier/location_test.go
package classifier

import (
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

func TestLocationClassifier_ExtractEntities(t *testing.T) {
	t.Helper()

	lc := NewLocationClassifier(&mockLogger{})

	tests := []struct {
		name       string
		text       string
		wantCities []string
	}{
		{
			name:       "single Canadian city",
			text:       "A man was arrested in Sudbury today.",
			wantCities: []string{"sudbury"},
		},
		{
			name:       "multiple Canadian cities",
			text:       "The suspect fled from Toronto to Montreal.",
			wantCities: []string{"toronto", "montreal"},
		},
		{
			name:       "US city not detected as Canadian",
			text:       "The US Justice Department in Washington announced.",
			wantCities: []string{},
		},
		{
			name:       "city with province",
			text:       "Sudbury Police in Northern Ontario responded.",
			wantCities: []string{"sudbury"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entities := lc.extractEntities(tt.text)
			cities := extractCityNames(entities)
			if !stringSlicesEqual(cities, tt.wantCities) {
				t.Errorf("extractEntities() cities = %v, want %v", cities, tt.wantCities)
			}
		})
	}
}

func extractCityNames(entities []locationEntity) []string {
	var cities []string
	for _, e := range entities {
		if e.entityType == entityTypeCity {
			cities = append(cities, e.normalized)
		}
	}
	return cities
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
```

**Step 2: Run test to verify it fails**

```bash
cd classifier && go test ./internal/classifier/... -v -run TestLocationClassifier_ExtractEntities
```

Expected: FAIL with "NewLocationClassifier undefined"

**Step 3: Write minimal implementation**

```go
// classifier/internal/classifier/location.go
package classifier

import (
	"regexp"
	"strings"

	"github.com/jonesrussell/north-cloud/classifier/internal/data"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// Zone weights for location scoring.
const (
	HeadlineWeight = 3.0
	LedeWeight     = 2.5
	BodyWeight     = 1.0
)

// Specificity bonuses.
const (
	CityBonus     = 3
	ProvinceBonus = 2
	CountryBonus  = 1
)

// DominanceThreshold is the minimum margin winner must have over second place.
const DominanceThreshold = 0.30

// Entity types.
const (
	entityTypeCity     = "city"
	entityTypeProvince = "province"
	entityTypeCountry  = "country"
)

// locationEntity represents a detected location mention.
type locationEntity struct {
	raw        string
	normalized string
	entityType string
	province   string // For cities, the province they're in
}

// LocationClassifier detects article locations from content.
type LocationClassifier struct {
	log logger.Logger
}

// NewLocationClassifier creates a new location classifier.
func NewLocationClassifier(log logger.Logger) *LocationClassifier {
	return &LocationClassifier{log: log}
}

// canadianProvinces maps province names/abbreviations to codes.
var canadianProvinces = map[string]string{
	"ontario":                    "ON",
	"on":                         "ON",
	"quebec":                     "QC",
	"qc":                         "QC",
	"british columbia":           "BC",
	"bc":                         "BC",
	"alberta":                    "AB",
	"ab":                         "AB",
	"manitoba":                   "MB",
	"mb":                         "MB",
	"saskatchewan":               "SK",
	"sk":                         "SK",
	"nova scotia":                "NS",
	"ns":                         "NS",
	"new brunswick":              "NB",
	"nb":                         "NB",
	"newfoundland":               "NL",
	"newfoundland and labrador":  "NL",
	"nl":                         "NL",
	"prince edward island":       "PE",
	"pei":                        "PE",
	"pe":                         "PE",
	"northwest territories":      "NT",
	"nt":                         "NT",
	"yukon":                      "YT",
	"yt":                         "YT",
	"nunavut":                    "NU",
	"nu":                         "NU",
}

// countryPatterns maps patterns to country names.
var countryPatterns = map[string]string{
	"canada":        "canada",
	"canadian":      "canada",
	"united states": "united_states",
	"u.s.":          "united_states",
	"us":            "united_states",
	"u.s.a.":        "united_states",
	"usa":           "united_states",
	"american":      "united_states",
	"america":       "united_states",
}

// extractEntities finds location mentions in text.
func (lc *LocationClassifier) extractEntities(text string) []locationEntity {
	var entities []locationEntity
	textLower := strings.ToLower(text)

	// Extract Canadian cities (validated against our list)
	// Use word boundary matching to avoid partial matches
	wordPattern := regexp.MustCompile(`\b([a-zA-Z][a-zA-Z\s\.\-']+[a-zA-Z])\b`)
	matches := wordPattern.FindAllString(text, -1)

	seen := make(map[string]bool)
	for _, match := range matches {
		normalized := strings.ToLower(strings.TrimSpace(match))
		if seen[normalized] {
			continue
		}

		if data.IsValidCanadianCity(normalized) {
			seen[normalized] = true
			canonicalSlug := data.NormalizeCityName(normalized)
			province, _ := data.GetProvinceForCity(normalized)
			entities = append(entities, locationEntity{
				raw:        match,
				normalized: canonicalSlug,
				entityType: entityTypeCity,
				province:   province,
			})
		}
	}

	// Extract provinces
	for provinceName, code := range canadianProvinces {
		if strings.Contains(textLower, provinceName) {
			if !seen["province:"+code] {
				seen["province:"+code] = true
				entities = append(entities, locationEntity{
					raw:        provinceName,
					normalized: code,
					entityType: entityTypeProvince,
				})
			}
		}
	}

	// Extract countries
	for pattern, country := range countryPatterns {
		if strings.Contains(textLower, pattern) {
			if !seen["country:"+country] {
				seen["country:"+country] = true
				entities = append(entities, locationEntity{
					raw:        pattern,
					normalized: country,
					entityType: entityTypeCountry,
				})
			}
		}
	}

	return entities
}
```

**Step 4: Run test to verify it passes**

```bash
cd classifier && go test ./internal/classifier/... -v -run TestLocationClassifier_ExtractEntities
```

Expected: PASS

**Step 5: Commit**

```bash
git add classifier/internal/classifier/location.go classifier/internal/classifier/location_test.go
git commit -m "feat(classifier): add location entity extraction

Extract Canadian cities, provinces, and countries from text content.
Cities are validated against curated Canadian cities list."
```

---

### Task 2.3: Location Classifier - Zone Scoring

**Files:**
- Modify: `classifier/internal/classifier/location.go`
- Modify: `classifier/internal/classifier/location_test.go`

**Step 1: Write the failing test**

```go
// Add to classifier/internal/classifier/location_test.go

func TestLocationClassifier_ScoreLocations(t *testing.T) {
	t.Helper()

	lc := NewLocationClassifier(&mockLogger{})

	tests := []struct {
		name         string
		headline     string
		lede         string
		body         string
		wantCity     string
		wantCountry  string
		wantSpecific string
	}{
		{
			name:         "headline city wins",
			headline:     "Sudbury man charged with assault",
			lede:         "Police responded to the scene.",
			body:         "The incident occurred downtown.",
			wantCity:     "sudbury",
			wantCountry:  "canada",
			wantSpecific: "city",
		},
		{
			name:         "US in headline overrides Canadian body",
			headline:     "US Justice Department announces probe",
			lede:         "The federal agency said today.",
			body:         "A Canadian official commented on the news.",
			wantCity:     "",
			wantCountry:  "united_states",
			wantSpecific: "country",
		},
		{
			name:         "no location detected",
			headline:     "Breaking news today",
			lede:         "Something happened somewhere.",
			body:         "Details are emerging.",
			wantCity:     "",
			wantCountry:  "unknown",
			wantSpecific: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := lc.scoreLocations(tt.headline, tt.lede, tt.body)
			if result.City != tt.wantCity {
				t.Errorf("scoreLocations() city = %v, want %v", result.City, tt.wantCity)
			}
			if result.Country != tt.wantCountry {
				t.Errorf("scoreLocations() country = %v, want %v", result.Country, tt.wantCountry)
			}
			if result.Specificity != tt.wantSpecific {
				t.Errorf("scoreLocations() specificity = %v, want %v", result.Specificity, tt.wantSpecific)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd classifier && go test ./internal/classifier/... -v -run TestLocationClassifier_ScoreLocations
```

Expected: FAIL with "scoreLocations undefined"

**Step 3: Write minimal implementation**

Add to `classifier/internal/classifier/location.go`:

```go
// locationScore holds accumulated scores for a location.
type locationScore struct {
	entity locationEntity
	score  float64
}

// scoreLocations determines the dominant location from headline, lede, and body.
func (lc *LocationClassifier) scoreLocations(headline, lede, body string) *domain.LocationResult {
	scores := make(map[string]*locationScore)

	// Score headline entities (3.0x weight)
	lc.scoreZone(headline, HeadlineWeight, scores)

	// Score lede entities (2.5x weight)
	lc.scoreZone(lede, LedeWeight, scores)

	// Score body entities (1.0x weight)
	lc.scoreZone(body, BodyWeight, scores)

	// Find dominant location
	return lc.determineDominant(scores)
}

// scoreZone extracts entities from a text zone and adds weighted scores.
func (lc *LocationClassifier) scoreZone(text string, weight float64, scores map[string]*locationScore) {
	entities := lc.extractEntities(text)

	for _, e := range entities {
		key := e.entityType + ":" + e.normalized
		bonus := lc.getSpecificityBonus(e.entityType)

		if existing, ok := scores[key]; ok {
			existing.score += weight * float64(bonus)
		} else {
			scores[key] = &locationScore{
				entity: e,
				score:  weight * float64(bonus),
			}
		}
	}
}

// getSpecificityBonus returns the bonus multiplier for an entity type.
func (lc *LocationClassifier) getSpecificityBonus(entityType string) int {
	switch entityType {
	case entityTypeCity:
		return CityBonus
	case entityTypeProvince:
		return ProvinceBonus
	case entityTypeCountry:
		return CountryBonus
	default:
		return 0
	}
}

// determineDominant finds the winning location using dominance rule.
func (lc *LocationClassifier) determineDominant(scores map[string]*locationScore) *domain.LocationResult {
	if len(scores) == 0 {
		return &domain.LocationResult{
			Country:     "unknown",
			Specificity: domain.SpecificityUnknown,
			Confidence:  0,
		}
	}

	// Find top two scores
	var first, second *locationScore
	for _, s := range scores {
		if first == nil || s.score > first.score {
			second = first
			first = s
		} else if second == nil || s.score > second.score {
			second = s
		}
	}

	// Apply dominance rule: winner must beat second by 30%
	if second != nil {
		margin := (first.score - second.score) / first.score
		if margin < DominanceThreshold {
			return &domain.LocationResult{
				Country:     "unknown",
				Specificity: domain.SpecificityUnknown,
				Confidence:  0.5, // Ambiguous
			}
		}
	}

	// Build result from winner
	result := &domain.LocationResult{
		Confidence: lc.calculateConfidence(first.score, second),
	}

	switch first.entity.entityType {
	case entityTypeCity:
		result.City = first.entity.normalized
		result.Province = first.entity.province
		result.Country = "canada" // All validated cities are Canadian
		result.Specificity = domain.SpecificityCity
	case entityTypeProvince:
		result.Province = first.entity.normalized
		result.Country = "canada"
		result.Specificity = domain.SpecificityProvince
	case entityTypeCountry:
		result.Country = first.entity.normalized
		result.Specificity = domain.SpecificityCountry
	}

	return result
}

// calculateConfidence computes confidence based on score margin.
func (lc *LocationClassifier) calculateConfidence(winnerScore float64, second *locationScore) float64 {
	if second == nil {
		return 0.95 // No competition = high confidence
	}
	margin := (winnerScore - second.score) / winnerScore
	// Scale margin (0.3 to 1.0) to confidence (0.6 to 0.95)
	return 0.6 + (margin-DominanceThreshold)/(1-DominanceThreshold)*0.35
}
```

**Step 4: Run test to verify it passes**

```bash
cd classifier && go test ./internal/classifier/... -v -run TestLocationClassifier_ScoreLocations
```

Expected: PASS

**Step 5: Commit**

```bash
git add classifier/internal/classifier/location.go classifier/internal/classifier/location_test.go
git commit -m "feat(classifier): add location zone scoring

Implement weighted scoring for headline (3.0x), lede (2.5x), and body (1.0x).
Apply dominance rule requiring 30% margin for winner."
```

---

### Task 2.4: Location Classifier - Classify Method

**Files:**
- Modify: `classifier/internal/classifier/location.go`
- Modify: `classifier/internal/classifier/location_test.go`

**Step 1: Write the failing test**

```go
// Add to classifier/internal/classifier/location_test.go

import (
	"context"
)

func TestLocationClassifier_Classify(t *testing.T) {
	t.Helper()

	lc := NewLocationClassifier(&mockLogger{})
	ctx := context.Background()

	tests := []struct {
		name        string
		raw         *domain.RawContent
		wantCity    string
		wantCountry string
	}{
		{
			name: "Canadian city in title",
			raw: &domain.RawContent{
				Title:   "Sudbury Police arrest suspect in downtown stabbing",
				RawText: "A man was taken into custody after the incident.",
			},
			wantCity:    "sudbury",
			wantCountry: "canada",
		},
		{
			name: "US story from Canadian publisher",
			raw: &domain.RawContent{
				Title:   "US Justice Department opens probe into police shooting",
				RawText: "The federal investigation was announced today in Washington.",
			},
			wantCity:    "",
			wantCountry: "united_states",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := lc.Classify(ctx, tt.raw)
			if err != nil {
				t.Fatalf("Classify() error = %v", err)
			}
			if result.City != tt.wantCity {
				t.Errorf("Classify() city = %v, want %v", result.City, tt.wantCity)
			}
			if result.Country != tt.wantCountry {
				t.Errorf("Classify() country = %v, want %v", result.Country, tt.wantCountry)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd classifier && go test ./internal/classifier/... -v -run TestLocationClassifier_Classify
```

Expected: FAIL with "Classify method undefined"

**Step 3: Write minimal implementation**

Add to `classifier/internal/classifier/location.go`:

```go
import (
	"context"
	"strings"
)

// Classify determines the location of an article from its content.
// It never considers the publisher's location - only content-derived entities.
func (lc *LocationClassifier) Classify(ctx context.Context, raw *domain.RawContent) (*domain.LocationResult, error) {
	headline := raw.Title
	lede := lc.extractLede(raw.RawText)
	body := raw.RawText

	result := lc.scoreLocations(headline, lede, body)
	result.Specificity = result.GetSpecificity()

	return result, nil
}

// extractLede gets the first paragraph from the raw text.
func (lc *LocationClassifier) extractLede(text string) string {
	// Split by double newlines (paragraph breaks)
	paragraphs := strings.Split(text, "\n\n")
	if len(paragraphs) > 0 {
		return strings.TrimSpace(paragraphs[0])
	}

	// Fallback: first 500 characters
	if len(text) > 500 {
		return text[:500]
	}
	return text
}
```

**Step 4: Run test to verify it passes**

```bash
cd classifier && go test ./internal/classifier/... -v -run TestLocationClassifier_Classify
```

Expected: PASS

**Step 5: Commit**

```bash
git add classifier/internal/classifier/location.go classifier/internal/classifier/location_test.go
git commit -m "feat(classifier): add location Classify method

Main entry point that extracts headline, lede, body and delegates
to scoring logic. Extracts first paragraph as lede."
```

---

## Phase 3: Crime Sub-Labels

### Task 3.1: Add SubLabel to CrimeResult

**Files:**
- Modify: `classifier/internal/classifier/crime.go`
- Modify: `classifier/internal/domain/classification.go`
- Modify: `classifier/internal/classifier/crime_test.go`

**Step 1: Write the failing test**

```go
// Add to classifier/internal/classifier/crime_test.go

func TestCrimeClassifier_SubLabel_CriminalJustice(t *testing.T) {
	t.Helper()

	sc := NewCrimeClassifier(nil, &mockLogger{}, true)

	// Criminal justice: court proceedings, sentencing
	raw := &domain.RawContent{
		ID:      "test-cj-1",
		Title:   "Man sentenced to 10 years for Sudbury robbery",
		RawText: "The Crown prosecutor argued for a lengthy sentence. The judge agreed.",
	}

	result, err := sc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Relevance != "peripheral_crime" {
		t.Errorf("expected peripheral_crime, got %s", result.Relevance)
	}
	if result.SubLabel != "criminal_justice" {
		t.Errorf("expected criminal_justice sub_label, got %s", result.SubLabel)
	}
}

func TestCrimeClassifier_SubLabel_CrimeContext(t *testing.T) {
	t.Helper()

	sc := NewCrimeClassifier(nil, &mockLogger{}, true)

	// Crime context: document releases, historical
	raw := &domain.RawContent{
		ID:      "test-cc-1",
		Title:   "DOJ releases 3 million pages from Jeffrey Epstein files",
		RawText: "The declassified documents reveal details from the historical case.",
	}

	result, err := sc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Relevance != "peripheral_crime" {
		t.Errorf("expected peripheral_crime, got %s", result.Relevance)
	}
	if result.SubLabel != "crime_context" {
		t.Errorf("expected crime_context sub_label, got %s", result.SubLabel)
	}
}

func TestCrimeClassifier_SubLabel_CoreStreetCrime_NoSubLabel(t *testing.T) {
	t.Helper()

	sc := NewCrimeClassifier(nil, &mockLogger{}, true)

	// Core street crime should have no sub_label
	raw := &domain.RawContent{
		ID:      "test-core-1",
		Title:   "Man charged with murder after shooting",
		RawText: "Police arrested a suspect at the scene.",
	}

	result, err := sc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Relevance != "core_street_crime" {
		t.Errorf("expected core_street_crime, got %s", result.Relevance)
	}
	if result.SubLabel != "" {
		t.Errorf("expected empty sub_label for core_street_crime, got %s", result.SubLabel)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd classifier && go test ./internal/classifier/... -v -run TestCrimeClassifier_SubLabel
```

Expected: FAIL with "SubLabel field undefined"

**Step 3: Write minimal implementation**

Add `SubLabel` field to `classifier/internal/classifier/crime.go`:

```go
// Update CrimeResult struct (around line 37)
type CrimeResult struct {
	Relevance           string   `json:"street_crime_relevance"`
	SubLabel            string   `json:"sub_label,omitempty"` // NEW: "criminal_justice" or "crime_context"
	CrimeTypes          []string `json:"crime_types"`
	LocationSpecificity string   `json:"location_specificity"`
	FinalConfidence     float64  `json:"final_confidence"`
	HomepageEligible    bool     `json:"homepage_eligible"`
	CategoryPages       []string `json:"category_pages"`
	ReviewRequired      bool     `json:"review_required"`
	// Internal fields...
	RuleRelevance  string  `json:"rule_relevance"`
	RuleConfidence float64 `json:"rule_confidence"`
	MLRelevance    string  `json:"ml_relevance,omitempty"`
	MLConfidence   float64 `json:"ml_confidence,omitempty"`
}

// Sub-label constants
const (
	SubLabelCriminalJustice = "criminal_justice"
	SubLabelCrimeContext    = "crime_context"
)

// Criminal justice verbs (active legal proceedings)
var criminalJusticeVerbs = []string{
	"charged", "arrested", "arraigned", "pleads", "pleaded",
	"sentenced", "convicted", "acquitted", "appeals", "appealed",
	"investigation launched", "warrant issued", "indicted",
}

// Crime context verbs (archival, document-driven)
var crimeContextVerbs = []string{
	"releases", "released", "reveals", "revealed", "declassified",
	"publishes", "published", "unsealed", "revisits", "reviews",
	"commemorates", "reports on",
}

// Jurisdiction indicators
var jurisdictionIndicators = []string{
	"court", "judge", "prosecutor", "crown", "district attorney",
	"police", "rcmp", "opp", "fbi", "doj", "justice department",
}
```

Add sub-label detection to the classification flow. In the `mergeResults` or `applyDecisionLogic` function, add:

```go
// determineSubLabel sets the sub_label for peripheral_crime articles.
func (s *CrimeClassifier) determineSubLabel(result *CrimeResult, title, body string) {
	// Only peripheral_crime gets sub-labels
	if result.Relevance != "peripheral_crime" {
		result.SubLabel = ""
		return
	}

	text := strings.ToLower(title + " " + body)

	// Count signals for criminal_justice
	cjScore := 0

	// Check jurisdiction indicators
	for _, indicator := range jurisdictionIndicators {
		if strings.Contains(text, indicator) {
			cjScore++
			break // Only count once
		}
	}

	// Check criminal justice verbs
	for _, verb := range criminalJusticeVerbs {
		if strings.Contains(text, verb) {
			cjScore++
			break
		}
	}

	// Check for crime context verbs
	ccScore := 0
	for _, verb := range crimeContextVerbs {
		if strings.Contains(text, verb) {
			ccScore++
			break
		}
	}

	// Decision: criminal_justice needs 2+ signals, otherwise crime_context
	if cjScore >= 2 {
		result.SubLabel = SubLabelCriminalJustice
	} else {
		result.SubLabel = SubLabelCrimeContext
	}
}
```

Call `determineSubLabel` at the end of `Classify` method:

```go
// In Classify method, before returning result:
s.determineSubLabel(result, raw.Title, raw.RawText)
```

Also update domain type in `classifier/internal/domain/classification.go`:

```go
type CrimeResult struct {
	Relevance           string   `json:"street_crime_relevance"`
	SubLabel            string   `json:"sub_label,omitempty"` // NEW
	CrimeTypes          []string `json:"crime_types"`
	LocationSpecificity string   `json:"location_specificity"`
	FinalConfidence     float64  `json:"final_confidence"`
	HomepageEligible    bool     `json:"homepage_eligible"`
	CategoryPages       []string `json:"category_pages"`
	ReviewRequired      bool     `json:"review_required"`
}
```

**Step 4: Run test to verify it passes**

```bash
cd classifier && go test ./internal/classifier/... -v -run TestCrimeClassifier_SubLabel
```

Expected: PASS

**Step 5: Commit**

```bash
git add classifier/internal/classifier/crime.go classifier/internal/domain/classification.go classifier/internal/classifier/crime_test.go
git commit -m "feat(classifier): add sub_label to CrimeResult

Split peripheral_crime into criminal_justice and crime_context sub-labels.
Criminal justice: court proceedings, sentencing (2+ signals required).
Crime context: document releases, historical cases (default for peripheral)."
```

---

## Phase 4: Integrate into Classification Pipeline

### Task 4.1: Add Location to ClassificationResult

**Files:**
- Modify: `classifier/internal/domain/classification.go`
- Modify: `classifier/internal/classifier/classifier.go`

**Step 1: Write the failing test**

```go
// Add to classifier/internal/classifier/classifier_test.go (create if needed)

func TestClassifier_IncludesLocation(t *testing.T) {
	t.Helper()

	// Create classifier with location enabled
	// ... setup code ...

	raw := &domain.RawContent{
		ID:      "test-loc-1",
		Title:   "Sudbury Police arrest suspect",
		RawText: "The arrest occurred downtown.",
	}

	result, err := classifier.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("Classify() error = %v", err)
	}

	if result.Location == nil {
		t.Fatal("expected Location to be populated")
	}
	if result.Location.City != "sudbury" {
		t.Errorf("expected city=sudbury, got %s", result.Location.City)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd classifier && go test ./internal/classifier/... -v -run TestClassifier_IncludesLocation
```

Expected: FAIL

**Step 3: Write minimal implementation**

Update `classifier/internal/domain/classification.go`:

```go
// ClassificationResult holds all classification outputs.
type ClassificationResult struct {
	ContentType      string        `json:"content_type"`
	QualityScore     int           `json:"quality_score"`
	Topics           []string      `json:"topics"`
	SourceReputation int           `json:"source_reputation"`
	Confidence       float64       `json:"confidence"`
	Crime            *CrimeResult  `json:"crime,omitempty"`
	Location         *LocationResult `json:"location,omitempty"` // NEW
}
```

Update `classifier/internal/classifier/classifier.go` to call location classifier:

```go
// In Classifier struct, add:
locationClassifier *LocationClassifier

// In NewClassifier, add:
locationClassifier: NewLocationClassifier(log),

// In Classify method, add location classification:
locationResult, err := c.locationClassifier.Classify(ctx, raw)
if err != nil {
    c.log.Warn("location classification failed", logger.Error(err))
} else {
    result.Location = locationResult
}
```

**Step 4: Run test to verify it passes**

```bash
cd classifier && go test ./internal/classifier/... -v -run TestClassifier_IncludesLocation
```

Expected: PASS

**Step 5: Commit**

```bash
git add classifier/internal/domain/classification.go classifier/internal/classifier/classifier.go
git commit -m "feat(classifier): integrate location into classification pipeline

Add LocationResult to ClassificationResult and call LocationClassifier
in the main Classify method."
```

---

## Phase 5: Update Elasticsearch Mappings

### Task 5.1: Add Location Fields to ES Mapping

**Files:**
- Modify: `classifier/internal/elasticsearch/mappings/classified_content.go`

**Step 1: Review current mapping**

```bash
cd classifier && cat internal/elasticsearch/mappings/classified_content.go
```

**Step 2: Update mapping**

Add location fields to the mapping:

```go
// Add to the mapping struct
Location: LocationProperties{
	Type: "object",
	Properties: LocationFieldProperties{
		City:        KeywordProperty{Type: "keyword"},
		Province:    KeywordProperty{Type: "keyword"},
		Country:     KeywordProperty{Type: "keyword"},
		Specificity: KeywordProperty{Type: "keyword"},
		Confidence:  FloatProperty{Type: "float"},
	},
},
```

**Step 3: Run linter**

```bash
cd classifier && golangci-lint run ./internal/elasticsearch/...
```

Expected: PASS

**Step 4: Commit**

```bash
git add classifier/internal/elasticsearch/mappings/classified_content.go
git commit -m "feat(classifier): add location fields to ES mapping

Add location object with city, province, country, specificity, and
confidence fields to classified_content index mapping."
```

---

## Phase 6: Update Publisher Channel Routing

### Task 6.1: Add Location-Based Channel Generation

**Files:**
- Modify: `publisher/internal/router/service.go`
- Modify: `publisher/internal/router/crime.go`
- Test: `publisher/internal/router/service_test.go`

**Step 1: Write the failing test**

```go
// Add to publisher/internal/router/service_test.go

func TestGenerateLocationChannels(t *testing.T) {
	t.Helper()

	tests := []struct {
		name         string
		article      *Article
		wantChannels []string
	}{
		{
			name: "Canadian city generates local + province + national",
			article: &Article{
				LocationCity:        "sudbury",
				LocationProvince:    "ON",
				LocationCountry:     "canada",
				LocationSpecificity: "city",
			},
			wantChannels: []string{
				"crime:local:sudbury",
				"crime:province:on",
				"crime:canada",
			},
		},
		{
			name: "International generates international only",
			article: &Article{
				LocationCountry:     "united_states",
				LocationSpecificity: "country",
			},
			wantChannels: []string{"crime:international"},
		},
		{
			name: "Unknown location generates no geo channels",
			article: &Article{
				LocationCountry:     "unknown",
				LocationSpecificity: "unknown",
			},
			wantChannels: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channels := GenerateLocationChannels(tt.article)
			if !stringSlicesEqual(channels, tt.wantChannels) {
				t.Errorf("GenerateLocationChannels() = %v, want %v", channels, tt.wantChannels)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd publisher && go test ./internal/router/... -v -run TestGenerateLocationChannels
```

Expected: FAIL

**Step 3: Write minimal implementation**

Add to `publisher/internal/router/service.go`:

```go
// Add location fields to Article struct
type Article struct {
	// ... existing fields ...

	// Location fields (from classifier)
	LocationCity        string `json:"location_city,omitempty"`
	LocationProvince    string `json:"location_province,omitempty"`
	LocationCountry     string `json:"location_country"`
	LocationSpecificity string `json:"location_specificity"`
	LocationConfidence  float64 `json:"location_confidence"`
}

// GenerateLocationChannels creates geographic channels based on article location.
func GenerateLocationChannels(article *Article) []string {
	var channels []string

	// Skip unknown locations
	if article.LocationCountry == "unknown" || article.LocationCountry == "" {
		return channels
	}

	// International (non-Canadian)
	if article.LocationCountry != "canada" {
		return []string{"crime:international"}
	}

	// Canadian locations
	if article.LocationSpecificity == "city" && article.LocationCity != "" {
		channels = append(channels, "crime:local:"+article.LocationCity)
	}

	if article.LocationProvince != "" {
		channels = append(channels, "crime:province:"+strings.ToLower(article.LocationProvince))
	}

	channels = append(channels, "crime:canada")

	return channels
}
```

**Step 4: Run test to verify it passes**

```bash
cd publisher && go test ./internal/router/... -v -run TestGenerateLocationChannels
```

Expected: PASS

**Step 5: Commit**

```bash
git add publisher/internal/router/service.go publisher/internal/router/service_test.go
git commit -m "feat(publisher): add location-based channel generation

Generate crime:local:{city}, crime:province:{code}, crime:canada,
and crime:international channels based on article location metadata."
```

---

### Task 6.2: Add Crime Sub-Label Channel Generation

**Files:**
- Modify: `publisher/internal/router/crime.go`
- Test: `publisher/internal/router/crime_test.go`

**Step 1: Write the failing test**

```go
// Add to publisher/internal/router/crime_test.go

func TestGenerateCrimeChannels_WithSubLabel(t *testing.T) {
	t.Helper()

	tests := []struct {
		name         string
		article      *Article
		wantChannels []string
	}{
		{
			name: "criminal_justice sub-label",
			article: &Article{
				CrimeRelevance: "peripheral_crime",
				CrimeSubLabel:  "criminal_justice",
			},
			wantChannels: []string{"crime:courts"},
		},
		{
			name: "crime_context sub-label",
			article: &Article{
				CrimeRelevance: "peripheral_crime",
				CrimeSubLabel:  "crime_context",
			},
			wantChannels: []string{"crime:context"},
		},
		{
			name: "core_street_crime no sub-label channels",
			article: &Article{
				CrimeRelevance:   "core_street_crime",
				HomepageEligible: true,
				CategoryPages:    []string{"violent-crime"},
			},
			wantChannels: []string{"crime:homepage", "crime:category:violent-crime"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channels := GenerateCrimeChannels(tt.article)
			if !stringSlicesEqual(channels, tt.wantChannels) {
				t.Errorf("GenerateCrimeChannels() = %v, want %v", channels, tt.wantChannels)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd publisher && go test ./internal/router/... -v -run TestGenerateCrimeChannels_WithSubLabel
```

Expected: FAIL

**Step 3: Write minimal implementation**

Update `publisher/internal/router/crime.go`:

```go
// Add CrimeSubLabel to Article struct (in service.go)
CrimeSubLabel string `json:"crime_sub_label,omitempty"`

// Update GenerateCrimeChannels
func GenerateCrimeChannels(article *Article) []string {
	// Skip not_crime
	if article.CrimeRelevance == CrimeRelevanceNotCrime || article.CrimeRelevance == "" {
		return []string{}
	}

	channels := make([]string, 0)

	// Handle peripheral_crime with sub-labels
	if article.CrimeRelevance == "peripheral_crime" {
		switch article.CrimeSubLabel {
		case "criminal_justice":
			channels = append(channels, "crime:courts")
		case "crime_context":
			channels = append(channels, "crime:context")
		default:
			// Default to context if no sub-label
			channels = append(channels, "crime:context")
		}
		return channels
	}

	// Handle core_street_crime (existing logic)
	if article.HomepageEligible {
		channels = append(channels, "crime:homepage")
	}

	for _, category := range article.CategoryPages {
		channels = append(channels, "crime:category:"+category)
	}

	return channels
}
```

**Step 4: Run test to verify it passes**

```bash
cd publisher && go test ./internal/router/... -v -run TestGenerateCrimeChannels_WithSubLabel
```

Expected: PASS

**Step 5: Commit**

```bash
git add publisher/internal/router/crime.go publisher/internal/router/crime_test.go publisher/internal/router/service.go
git commit -m "feat(publisher): add crime sub-label channel routing

Route criminal_justice to crime:courts and crime_context to crime:context.
Core street crime continues using homepage and category channels."
```

---

## Phase 7: Integration and E2E Testing

### Task 7.1: Run Full Test Suite

**Step 1: Run classifier tests**

```bash
cd classifier && go test ./... -v
```

Expected: All PASS

**Step 2: Run publisher tests**

```bash
cd publisher && go test ./... -v
```

Expected: All PASS

**Step 3: Run linters**

```bash
cd classifier && golangci-lint run
cd publisher && golangci-lint run
```

Expected: No errors

**Step 4: Commit any fixes**

```bash
git add -A
git commit -m "test: fix any test issues from integration"
```

---

### Task 7.2: Final Commit and Summary

**Step 1: Review all changes**

```bash
git log --oneline -10
git diff main --stat
```

**Step 2: Create summary commit if needed**

```bash
git add -A
git commit -m "feat: publishing pipeline refinement - complete

Implements the publishing pipeline refinement design:
- Location detection: Content-based extraction with weighted scoring
- Crime sub-labels: criminal_justice and crime_context for peripheral_crime
- Channel routing: Location-based + sub-label channels
- Metadata schema: Updated with location and sub_label fields

Closes: design doc 2026-02-04-publishing-pipeline-refinement-design.md"
```

---

## Execution Notes

**Dependencies between tasks:**
- Phase 1 (Canadian Cities) must complete before Phase 2 (Location Detection)
- Phase 3 (Crime Sub-Labels) can run in parallel with Phase 2
- Phase 4 (Integration) requires Phase 2 and 3
- Phase 5 (ES Mappings) requires Phase 4
- Phase 6 (Publisher) can start after Phase 4
- Phase 7 (Testing) requires all phases

**Estimated task count:** 12 tasks across 7 phases

**Key files created:**
- `classifier/internal/data/canadian_cities.go`
- `classifier/internal/classifier/location.go`

**Key files modified:**
- `classifier/internal/classifier/crime.go`
- `classifier/internal/domain/classification.go`
- `classifier/internal/elasticsearch/mappings/classified_content.go`
- `publisher/internal/router/service.go`
- `publisher/internal/router/crime.go`
