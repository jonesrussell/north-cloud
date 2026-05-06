package rss_test

import (
	"strings"
	"testing"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/adapter/rss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cleanDesc is a typical full-featured description matching the safersites.ca
// NationBuilder template, used across multiple extractor tests.
const cleanDesc = `
<p><strong>Location:</strong> Vancouver, BC</p>
<p><strong>Substances:</strong> Fentanyl, Benzodiazepines</p>
<p><strong>Composition:</strong> Fentanyl (72.4%), Etizolam (18.1%), Caffeine (9.5%)</p>
<p><strong>Lab Source:</strong> BCCDC Drug Checking Service</p>
<ul>
  <li>Do not use alone</li>
  <li>Have naloxone nearby</li>
  <li>Start with a small amount</li>
</ul>
`

// drugAlertDesc mirrors the safersites.ca "Drug Alert: <Location>" header style.
const drugAlertDesc = `
<p>Drug Alert: Winnipeg - Tue. May 5, 2026</p>
<p>Yellow chunk sold as fentanyl</p>
<ul>
  <li>Carfentanil (0.08%)</li>
  <li>Medetomidine (2.31%)</li>
  <li>Caffeine (10.8%)</li>
  <li>Fentanyl (33.91%)</li>
</ul>
<p>Tested by: Health Canada Drug Analysis Service</p>
<ul>
  <li>Use with a friend</li>
  <li>Have naloxone nearby</li>
</ul>
`

// ---------------------------------------------------------------------------
// ExtractTitle
// ---------------------------------------------------------------------------

func TestExtractTitle_FromItemTitle(t *testing.T) {
	t.Helper()
	item := rss.Item{Title: "Drug Alert: Winnipeg", Description: cleanDesc}
	assert.Equal(t, "Drug Alert: Winnipeg", rss.ExtractTitle(item))
}

func TestExtractTitle_FallbackToDescription(t *testing.T) {
	t.Helper()
	item := rss.Item{Description: "<p>First line of description</p>"}
	title := rss.ExtractTitle(item)
	assert.NotEmpty(t, title)
	assert.Contains(t, strings.ToLower(title), "first line")
}

func TestExtractTitle_EmptyEverything(t *testing.T) {
	t.Helper()
	item := rss.Item{}
	// Should return empty string, not panic.
	assert.Empty(t, rss.ExtractTitle(item))
}

// ---------------------------------------------------------------------------
// ExtractLocation
// ---------------------------------------------------------------------------

func TestExtractLocation_StrongTag(t *testing.T) {
	t.Helper()
	loc := rss.ExtractLocation(cleanDesc)
	assert.Equal(t, "Vancouver, BC", loc)
}

func TestExtractLocation_DrugAlertHeader(t *testing.T) {
	t.Helper()
	loc := rss.ExtractLocation(drugAlertDesc)
	assert.Equal(t, "Winnipeg", loc)
}

func TestExtractLocation_Missing(t *testing.T) {
	t.Helper()
	loc := rss.ExtractLocation("<p>No location here.</p>")
	assert.Empty(t, loc)
}

// ---------------------------------------------------------------------------
// ExtractSubstances
// ---------------------------------------------------------------------------

func TestExtractSubstances_HappyPath(t *testing.T) {
	t.Helper()
	subs := rss.ExtractSubstances(cleanDesc)
	require.NotEmpty(t, subs)
	assert.Contains(t, subs, "fentanyl")
	assert.Contains(t, subs, "etizolam")
	assert.Contains(t, subs, "caffeine")
}

func TestExtractSubstances_MultipleKnown(t *testing.T) {
	t.Helper()
	desc := "Contains carfentanil, nitazenes, and xylazine."
	subs := rss.ExtractSubstances(desc)
	assert.Contains(t, subs, "carfentanil")
	assert.Contains(t, subs, "xylazine")
}

func TestExtractSubstances_NoDuplicates(t *testing.T) {
	t.Helper()
	desc := "fentanyl fentanyl fentanyl"
	subs := rss.ExtractSubstances(desc)
	count := 0
	for _, s := range subs {
		if s == "fentanyl" {
			count++
		}
	}
	assert.Equal(t, 1, count, "fentanyl must appear only once")
}

func TestExtractSubstances_Missing(t *testing.T) {
	t.Helper()
	subs := rss.ExtractSubstances("<p>No substances mentioned here.</p>")
	assert.Empty(t, subs)
}

// ---------------------------------------------------------------------------
// ExtractComposition
// ---------------------------------------------------------------------------

func TestExtractComposition_HappyPath(t *testing.T) {
	t.Helper()
	comp := rss.ExtractComposition(cleanDesc)
	require.Len(t, comp, 3, "should extract Fentanyl, Etizolam, Caffeine")

	names := make([]string, 0, len(comp))
	for _, c := range comp {
		names = append(names, c.Name)
	}
	assert.Contains(t, names, "Fentanyl")
	assert.Contains(t, names, "Etizolam")
	assert.Contains(t, names, "Caffeine")

	// Verify percentage values are parsed correctly.
	for _, c := range comp {
		assert.Greater(t, c.Percentage, 0.0)
	}
}

func TestExtractComposition_DrugAlertFormat(t *testing.T) {
	t.Helper()
	comp := rss.ExtractComposition(drugAlertDesc)
	require.NotEmpty(t, comp)
	// Carfentanil should appear even at very low percentage.
	found := false
	for _, c := range comp {
		if c.Name == "Carfentanil" {
			assert.InDelta(t, 0.08, c.Percentage, 0.001)
			found = true
		}
	}
	assert.True(t, found, "Carfentanil entry expected")
}

func TestExtractComposition_Missing(t *testing.T) {
	t.Helper()
	comp := rss.ExtractComposition("<p>No composition data available.</p>")
	assert.Empty(t, comp)
}

// ---------------------------------------------------------------------------
// ExtractLabSource
// ---------------------------------------------------------------------------

func TestExtractLabSource_BCCDC(t *testing.T) {
	t.Helper()
	lab := rss.ExtractLabSource(cleanDesc)
	assert.Equal(t, "BCCDC Drug Checking Service", lab)
}

func TestExtractLabSource_HealthCanada(t *testing.T) {
	t.Helper()
	lab := rss.ExtractLabSource(drugAlertDesc)
	assert.Equal(t, "Health Canada Drug Analysis Service", lab)
}

func TestExtractLabSource_Missing(t *testing.T) {
	t.Helper()
	lab := rss.ExtractLabSource("<p>Testing pending. No lab source identified.</p>")
	assert.Empty(t, lab)
}

// ---------------------------------------------------------------------------
// ExtractGuidance
// ---------------------------------------------------------------------------

func TestExtractGuidance_HappyPath(t *testing.T) {
	t.Helper()
	guidance := rss.ExtractGuidance(cleanDesc)
	require.NotEmpty(t, guidance)
	assert.Contains(t, guidance, "Do not use alone")
	assert.Contains(t, guidance, "Have naloxone nearby")
	assert.Contains(t, guidance, "Start with a small amount")
}

func TestExtractGuidance_DrugAlertFormat(t *testing.T) {
	t.Helper()
	guidance := rss.ExtractGuidance(drugAlertDesc)
	require.NotEmpty(t, guidance)
	assert.Contains(t, guidance, "Use with a friend")
	assert.Contains(t, guidance, "Have naloxone nearby")
}

func TestExtractGuidance_Missing(t *testing.T) {
	t.Helper()
	guidance := rss.ExtractGuidance("<p>No bullets here.</p>")
	assert.Nil(t, guidance)
}

func TestExtractGuidance_NoBlanks(t *testing.T) {
	t.Helper()
	desc := "<ul><li>  </li><li>Use with a friend</li><li></li></ul>"
	guidance := rss.ExtractGuidance(desc)
	for _, g := range guidance {
		assert.NotEmpty(t, g, "blank lines must be filtered out")
	}
	assert.Contains(t, guidance, "Use with a friend")
}
