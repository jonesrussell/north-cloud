package rss_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/adapter/rss"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadFixture reads the named file from the testdata directory.
func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	require.NoError(t, err, "testdata file must be readable")
	return data
}

// makeParserSource builds a minimal AlertSource for parser tests.
// It is distinct from makeSource in client_test.go which requires a URL.
func makeParserSource(t *testing.T) domain.AlertSource {
	t.Helper()
	return domain.AlertSource{
		ID:            "safersites",
		Name:          "Safer Sites",
		DefaultScope:  []string{"indigenous:region:prairies"},
		DefaultExpiry: 72 * time.Hour,
	}
}

// ---------------------------------------------------------------------------
// TestParseFeed_Golden
// ---------------------------------------------------------------------------

func TestParseFeed_Golden(t *testing.T) {
	t.Helper()
	body := loadFixture(t, "safersites_sample.rss")

	feed, err := rss.ParseFeed(body)
	require.NoError(t, err)
	require.NotNil(t, feed)

	assert.Equal(t, "Safer Sites Drug Checking Alerts", feed.Channel.Title)
	assert.GreaterOrEqual(t, len(feed.Channel.Items), 10, "fixture must have at least 10 items")

	// Spot-check first item title.
	assert.Contains(t, feed.Channel.Items[0].Title, "Fentanyl Alert")
	// Spot-check link is non-empty.
	assert.NotEmpty(t, feed.Channel.Items[0].Link)
}

func TestParseFeed_InvalidXML(t *testing.T) {
	t.Helper()
	_, err := rss.ParseFeed([]byte("not xml"))
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// TestDeriveID_Stability
// ---------------------------------------------------------------------------

func TestDeriveID_Stability(t *testing.T) {
	t.Helper()
	link := "https://safersites.example.ca/alerts/2025/05/fentanyl-vdse-001"

	// Same input always yields same output.
	id1 := rss.DeriveID("safersites", link)
	id2 := rss.DeriveID("safersites", link)
	assert.Equal(t, id1, id2, "DeriveID must be deterministic")

	// Different link yields different ID.
	otherID := rss.DeriveID("safersites", "https://safersites.example.ca/alerts/2025/05/meth-surrey-002")
	assert.NotEqual(t, id1, otherID)

	// Result is entirely lowercase.
	assert.Equal(t, strings.ToLower(id1), id1, "ID must be lowercase")

	// Contains the source prefix.
	assert.True(t, strings.HasPrefix(id1, "safersites:"), "ID must be prefixed with source ID")
}

// ---------------------------------------------------------------------------
// TestParsePubDate_Variants
// ---------------------------------------------------------------------------

func TestParsePubDate_Variants(t *testing.T) {
	t.Helper()
	cases := []struct {
		name  string
		input string
	}{
		{
			name:  "RFC1123Z",
			input: "Tue, 06 May 2025 09:15:00 +0000",
		},
		{
			name:  "RFC822Z",
			input: "06 May 25 09:15 +0000",
		},
		{
			name:  "RFC1123 (GMT)",
			input: "Tue, 06 May 2025 09:15:00 GMT",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Helper()
			got, err := rss.ParsePubDate(tc.input)
			require.NoError(t, err, "must parse %q", tc.input)
			assert.Equal(t, time.UTC, got.Location(), "result must be UTC")
			assert.False(t, got.IsZero())
		})
	}
}

func TestParsePubDate_Unknown(t *testing.T) {
	t.Helper()
	_, err := rss.ParsePubDate("not a date")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// TestParseItem_Clean
// ---------------------------------------------------------------------------

func TestParseItem_Clean(t *testing.T) {
	t.Helper()
	src := makeParserSource(t)
	item := rss.Item{
		Title:   "Drug Alert: Winnipeg",
		Link:    "https://safersites.example.ca/alerts/2025/05/fentanyl-001",
		PubDate: "Tue, 06 May 2025 09:15:00 +0000",
		Description: `<p>Drug Alert: Winnipeg</p>
<ul>
  <li>Fentanyl (72.4%)</li>
  <li>Caffeine (27.6%)</li>
</ul>
<p>Tested by: Health Canada Drug Analysis Service</p>
<ul>
  <li>Use with a friend</li>
  <li>Have naloxone nearby</li>
</ul>`,
	}

	alert, err := rss.ParseItem(item, src)
	require.NoError(t, err)
	assert.Equal(t, domain.ParseClean, alert.ParseQuality)
	assert.Equal(t, domain.LifecycleActive, alert.LifecycleState)
	assert.Equal(t, "Drug Alert: Winnipeg", alert.Title)
	assert.False(t, alert.IssuedAt.IsZero())
	assert.NotNil(t, alert.ExpiresAt, "ExpiresAt must be set when DefaultExpiry > 0")
	assert.Equal(t, "safersites:fentanyl-001", alert.ID)
	assert.Len(t, alert.Sources, 1)
	assert.Equal(t, "safersites", alert.Sources[0].SourceID)
	require.NotNil(t, alert.Hazard.HarmReduction)
	assert.NotEmpty(t, alert.Hazard.HarmReduction.Composition)
	assert.Equal(t, "Health Canada Drug Analysis Service", alert.Hazard.HarmReduction.LabSource)
}

// ---------------------------------------------------------------------------
// TestParseItem_Degraded
// ---------------------------------------------------------------------------

func TestParseItem_Degraded(t *testing.T) {
	t.Helper()
	src := makeParserSource(t)
	// No lab source and no recognizable substances → degraded.
	item := rss.Item{
		Title:       "Unknown Supply Alert",
		Link:        "https://safersites.example.ca/alerts/2025/05/unknown-001",
		PubDate:     "Tue, 06 May 2025 09:15:00 +0000",
		Description: `<p>Some unknown supply. No composition available. No lab.</p>`,
	}

	alert, err := rss.ParseItem(item, src)
	require.NoError(t, err)
	assert.Equal(t, domain.ParseDegraded, alert.ParseQuality)
	// Raw description must be preserved in Summary when degraded.
	assert.Contains(t, alert.Summary, "unknown supply")
}

// ---------------------------------------------------------------------------
// TestParseItem_FailedRequired — missing required fields
// ---------------------------------------------------------------------------

func TestParseItem_FailedRequired_NoLink(t *testing.T) {
	t.Helper()
	src := makeParserSource(t)
	item := rss.Item{
		Title:   "Alert Without Link",
		PubDate: "Tue, 06 May 2025 09:15:00 +0000",
		// Link is empty.
	}

	alert, err := rss.ParseItem(item, src)
	require.Error(t, err)
	assert.Equal(t, domain.ParseFailed, alert.ParseQuality)
}

func TestParseItem_FailedRequired_NoPubDate(t *testing.T) {
	t.Helper()
	src := makeParserSource(t)
	item := rss.Item{
		Title: "Alert Without PubDate",
		Link:  "https://safersites.example.ca/alerts/2025/05/no-date",
		// PubDate is empty.
	}

	alert, err := rss.ParseItem(item, src)
	require.Error(t, err)
	assert.Equal(t, domain.ParseFailed, alert.ParseQuality)
}

func TestParseItem_FailedRequired_NoTitle(t *testing.T) {
	t.Helper()
	src := makeParserSource(t)
	item := rss.Item{
		Link:    "https://safersites.example.ca/alerts/2025/05/no-title",
		PubDate: "Tue, 06 May 2025 09:15:00 +0000",
		// Title is empty.
	}

	alert, err := rss.ParseItem(item, src)
	require.Error(t, err)
	assert.Equal(t, domain.ParseFailed, alert.ParseQuality)
}

func TestParseItem_ExpiresAt_ZeroExpiry(t *testing.T) {
	t.Helper()
	src := makeParserSource(t)
	src.DefaultExpiry = 0

	item := rss.Item{
		Title:   "Alert",
		Link:    "https://safersites.example.ca/alerts/2025/05/no-expiry",
		PubDate: "Tue, 06 May 2025 09:15:00 +0000",
		Description: `<p>Drug Alert: Test</p>
<p>Tested by: Health Canada Drug Analysis Service</p>
<ul><li>Fentanyl (90%)</li></ul>`,
	}

	alert, err := rss.ParseItem(item, src)
	require.NoError(t, err)
	assert.Nil(t, alert.ExpiresAt)
}
