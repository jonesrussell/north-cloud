package jsonld_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/classifier/jsonld"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Extract tests ---

func TestExtract_FindsRecipeJSONLD(t *testing.T) {
	html := `<html><head>
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "Recipe",
  "name": "Chocolate Cake",
  "prepTime": "PT30M",
  "cookTime": "PT1H",
  "recipeYield": "12 servings"
}
</script>
</head><body></body></html>`

	blocks := jsonld.Extract(html)
	require.Len(t, blocks, 1)
	assert.Equal(t, "Recipe", blocks[0]["@type"])
	assert.Equal(t, "Chocolate Cake", blocks[0]["name"])
}

func TestExtract_HandlesArrayWithMultipleTypes(t *testing.T) {
	html := `<html><head>
<script type="application/ld+json">
[
  {
    "@context": "https://schema.org",
    "@type": "BreadcrumbList",
    "itemListElement": []
  },
  {
    "@context": "https://schema.org",
    "@type": "Recipe",
    "name": "Pasta Carbonara"
  }
]
</script>
</head><body></body></html>`

	blocks := jsonld.Extract(html)
	require.Len(t, blocks, 2)

	recipe := jsonld.FindByType(blocks, "Recipe")
	require.NotNil(t, recipe)
	assert.Equal(t, "Pasta Carbonara", recipe["name"])

	breadcrumb := jsonld.FindByType(blocks, "BreadcrumbList")
	require.NotNil(t, breadcrumb)
}

func TestExtract_FindsJobPostingJSONLD(t *testing.T) {
	html := `<html><head>
<script type="application/ld+json">
{
  "@context": "https://schema.org/",
  "@type": "JobPosting",
  "title": "Software Engineer",
  "hiringOrganization": {
    "@type": "Organization",
    "name": "Acme Corp"
  },
  "jobLocation": {
    "@type": "Place",
    "address": {
      "@type": "PostalAddress",
      "addressLocality": "Toronto",
      "addressRegion": "ON"
    }
  }
}
</script>
</head><body></body></html>`

	blocks := jsonld.Extract(html)
	require.Len(t, blocks, 1)
	assert.Equal(t, "JobPosting", blocks[0]["@type"])
	assert.Equal(t, "Software Engineer", blocks[0]["title"])
}

func TestExtract_ReturnsEmptyForNoScriptTags(t *testing.T) {
	html := `<html><head><title>No JSON-LD</title></head><body><p>Hello</p></body></html>`

	blocks := jsonld.Extract(html)
	assert.Empty(t, blocks)
}

func TestExtract_ReturnsEmptyForMalformedJSON(t *testing.T) {
	html := `<html><head>
<script type="application/ld+json">
{ this is not valid JSON at all
</script>
</head><body></body></html>`

	blocks := jsonld.Extract(html)
	assert.Empty(t, blocks)
}

func TestExtract_HandlesMultipleScriptBlocks(t *testing.T) {
	html := `<html><head>
<script type="application/ld+json">
{"@type": "Organization", "name": "Acme"}
</script>
<script type="application/ld+json">
{"@type": "WebPage", "name": "Home"}
</script>
</head><body></body></html>`

	blocks := jsonld.Extract(html)
	require.Len(t, blocks, 2)
}

func TestExtract_IgnoresNonJSONLDScripts(t *testing.T) {
	html := `<html><head>
<script type="text/javascript">var x = 1;</script>
<script type="application/ld+json">
{"@type": "Recipe", "name": "Soup"}
</script>
<script>console.log("hi")</script>
</head><body></body></html>`

	blocks := jsonld.Extract(html)
	require.Len(t, blocks, 1)
	assert.Equal(t, "Recipe", blocks[0]["@type"])
}

func TestExtract_HandlesEmptyHTML(t *testing.T) {
	blocks := jsonld.Extract("")
	assert.Empty(t, blocks)
}

// --- FindByType tests ---

func TestFindByType_ReturnsFirstMatch(t *testing.T) {
	blocks := []map[string]any{
		{"@type": "BreadcrumbList", "name": "nav"},
		{"@type": "Recipe", "name": "First Recipe"},
		{"@type": "Recipe", "name": "Second Recipe"},
	}

	result := jsonld.FindByType(blocks, "Recipe")
	require.NotNil(t, result)
	assert.Equal(t, "First Recipe", result["name"])
}

func TestFindByType_ReturnsNilWhenNotFound(t *testing.T) {
	blocks := []map[string]any{
		{"@type": "BreadcrumbList"},
		{"@type": "Organization"},
	}

	result := jsonld.FindByType(blocks, "JobPosting")
	assert.Nil(t, result)
}

func TestFindByType_HandlesEmptyBlocks(t *testing.T) {
	result := jsonld.FindByType(nil, "Recipe")
	assert.Nil(t, result)
}

func TestFindByType_HandlesMissingTypeField(t *testing.T) {
	blocks := []map[string]any{
		{"name": "No type here"},
		{"@type": "Recipe", "name": "Found"},
	}

	result := jsonld.FindByType(blocks, "Recipe")
	require.NotNil(t, result)
	assert.Equal(t, "Found", result["name"])
}

// --- ParseISO8601Duration tests ---

func TestParseISO8601Duration_PT30M(t *testing.T) {
	result := jsonld.ParseISO8601Duration("PT30M")
	require.NotNil(t, result)
	assert.Equal(t, 30, *result)
}

func TestParseISO8601Duration_PT1H(t *testing.T) {
	result := jsonld.ParseISO8601Duration("PT1H")
	require.NotNil(t, result)
	assert.Equal(t, 60, *result)
}

func TestParseISO8601Duration_PT1H30M(t *testing.T) {
	result := jsonld.ParseISO8601Duration("PT1H30M")
	require.NotNil(t, result)
	assert.Equal(t, 90, *result)
}

func TestParseISO8601Duration_PT45M(t *testing.T) {
	result := jsonld.ParseISO8601Duration("PT45M")
	require.NotNil(t, result)
	assert.Equal(t, 45, *result)
}

func TestParseISO8601Duration_PT2H15M(t *testing.T) {
	result := jsonld.ParseISO8601Duration("PT2H15M")
	require.NotNil(t, result)
	assert.Equal(t, 135, *result)
}

func TestParseISO8601Duration_ReturnsNilForInvalidInput(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{name: "empty string", input: ""},
		{name: "no PT prefix", input: "30M"},
		{name: "random text", input: "about 30 minutes"},
		{name: "just PT", input: "PT"},
		{name: "P without T", input: "P30M"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := jsonld.ParseISO8601Duration(tc.input)
			assert.Nil(t, result, "expected nil for input %q", tc.input)
		})
	}
}

// --- StringVal tests ---

func TestStringVal_ReturnsValue(t *testing.T) {
	m := map[string]any{"name": "Test"}
	assert.Equal(t, "Test", jsonld.StringVal(m, "name"))
}

func TestStringVal_ReturnEmptyForMissingKey(t *testing.T) {
	m := map[string]any{"name": "Test"}
	assert.Empty(t, jsonld.StringVal(m, "missing"))
}

func TestStringVal_ReturnsEmptyForNonStringValue(t *testing.T) {
	m := map[string]any{"count": 42}
	assert.Empty(t, jsonld.StringVal(m, "count"))
}

func TestStringVal_ReturnsEmptyForNilMap(t *testing.T) {
	assert.Empty(t, jsonld.StringVal(nil, "key"))
}

// --- StringSliceVal tests ---

func TestStringSliceVal_HandlesStringSlice(t *testing.T) {
	// JSON unmarshal produces []any, not []string
	m := map[string]any{"tags": []any{"cooking", "baking", "desserts"}}
	result := jsonld.StringSliceVal(m, "tags")
	assert.Equal(t, []string{"cooking", "baking", "desserts"}, result)
}

func TestStringSliceVal_HandlesSingleString(t *testing.T) {
	// Some JSON-LD uses a string instead of an array for single values
	m := map[string]any{"recipeCategory": "Dessert"}
	result := jsonld.StringSliceVal(m, "recipeCategory")
	assert.Equal(t, []string{"Dessert"}, result)
}

func TestStringSliceVal_ReturnsNilForMissingKey(t *testing.T) {
	m := map[string]any{"name": "Test"}
	result := jsonld.StringSliceVal(m, "missing")
	assert.Nil(t, result)
}

func TestStringSliceVal_SkipsNonStringElements(t *testing.T) {
	m := map[string]any{"mixed": []any{"valid", 42, "also valid"}}
	result := jsonld.StringSliceVal(m, "mixed")
	assert.Equal(t, []string{"valid", "also valid"}, result)
}

// --- NestedStringVal tests ---

func TestNestedStringVal_ReturnsNestedValue(t *testing.T) {
	m := map[string]any{
		"hiringOrganization": map[string]any{
			"name": "Acme Corp",
		},
	}
	assert.Equal(t, "Acme Corp", jsonld.NestedStringVal(m, "hiringOrganization", "name"))
}

func TestNestedStringVal_ReturnsEmptyForMissingOuterKey(t *testing.T) {
	m := map[string]any{"name": "Test"}
	assert.Empty(t, jsonld.NestedStringVal(m, "missing", "name"))
}

func TestNestedStringVal_ReturnsEmptyForMissingInnerKey(t *testing.T) {
	m := map[string]any{
		"hiringOrganization": map[string]any{
			"type": "Organization",
		},
	}
	assert.Empty(t, jsonld.NestedStringVal(m, "hiringOrganization", "name"))
}

func TestNestedStringVal_ReturnsEmptyForNonMapOuter(t *testing.T) {
	m := map[string]any{"hiringOrganization": "just a string"}
	assert.Empty(t, jsonld.NestedStringVal(m, "hiringOrganization", "name"))
}

// --- FloatVal tests ---

func TestFloatVal_HandlesFloat64(t *testing.T) {
	m := map[string]any{"rating": 4.5}
	result := jsonld.FloatVal(m, "rating")
	require.NotNil(t, result)
	assert.InDelta(t, 4.5, *result, 0.001)
}

func TestFloatVal_HandlesStringNumber(t *testing.T) {
	m := map[string]any{"rating": "4.5"}
	result := jsonld.FloatVal(m, "rating")
	require.NotNil(t, result)
	assert.InDelta(t, 4.5, *result, 0.001)
}

func TestFloatVal_ReturnsNilForMissingKey(t *testing.T) {
	m := map[string]any{"name": "Test"}
	result := jsonld.FloatVal(m, "rating")
	assert.Nil(t, result)
}

func TestFloatVal_ReturnsNilForInvalidString(t *testing.T) {
	m := map[string]any{"rating": "not a number"}
	result := jsonld.FloatVal(m, "rating")
	assert.Nil(t, result)
}

// --- IntVal tests ---

func TestIntVal_HandlesFloat64(t *testing.T) {
	// JSON unmarshal always produces float64 for numbers
	m := map[string]any{"count": float64(42)}
	result := jsonld.IntVal(m, "count")
	require.NotNil(t, result)
	assert.Equal(t, 42, *result)
}

func TestIntVal_HandlesStringNumber(t *testing.T) {
	m := map[string]any{"count": "42"}
	result := jsonld.IntVal(m, "count")
	require.NotNil(t, result)
	assert.Equal(t, 42, *result)
}

func TestIntVal_ReturnsNilForMissingKey(t *testing.T) {
	m := map[string]any{"name": "Test"}
	result := jsonld.IntVal(m, "count")
	assert.Nil(t, result)
}

func TestIntVal_ReturnsNilForInvalidString(t *testing.T) {
	m := map[string]any{"count": "not a number"}
	result := jsonld.IntVal(m, "count")
	assert.Nil(t, result)
}
