package feed_test

import (
	"context"
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/feed"
)

const rssFixture = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test RSS</title>
    <item>
      <title>First Article</title>
      <link>https://example.com/first</link>
      <pubDate>Mon, 01 Jan 2024 12:00:00 +0000</pubDate>
    </item>
    <item>
      <title>Second Article</title>
      <link>https://example.com/second</link>
      <pubDate>Tue, 02 Jan 2024 12:00:00 +0000</pubDate>
    </item>
  </channel>
</rss>`

const atomFixture = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Test Atom</title>
  <entry>
    <title>Alpha Entry</title>
    <link href="https://example.com/alpha"/>
    <updated>2024-01-01T12:00:00Z</updated>
  </entry>
  <entry>
    <title>Beta Entry</title>
    <link href="https://example.com/beta"/>
    <updated>2024-01-02T12:00:00Z</updated>
  </entry>
</feed>`

const emptyFeedFixture = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Empty Feed</title>
  </channel>
</rss>`

const noLinkFixture = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>No Link Feed</title>
    <item>
      <title>No Link Item</title>
      <guid isPermaLink="false">some-opaque-id</guid>
    </item>
  </channel>
</rss>`

const guidAsLinkFixture = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>GUID Feed</title>
    <item>
      <title>GUID Article</title>
      <guid>https://example.com/guid-article</guid>
    </item>
  </channel>
</rss>`

// expectedRSSItemCount is the number of items in rssFixture.
const expectedRSSItemCount = 2

// expectedAtomItemCount is the number of items in atomFixture.
const expectedAtomItemCount = 2

func TestParseFeed_RSS(t *testing.T) {
	t.Parallel()

	items, err := feed.ParseFeed(context.Background(), rssFixture)
	requireNoError(t, err)
	requireLen(t, items, expectedRSSItemCount)

	assertEqual(t, "https://example.com/first", items[0].URL)
	assertEqual(t, "First Article", items[0].Title)
	assertNotEmpty(t, items[0].PublishedAt)

	assertEqual(t, "https://example.com/second", items[1].URL)
	assertEqual(t, "Second Article", items[1].Title)
}

func TestParseFeed_Atom(t *testing.T) {
	t.Parallel()

	items, err := feed.ParseFeed(context.Background(), atomFixture)
	requireNoError(t, err)
	requireLen(t, items, expectedAtomItemCount)

	assertEqual(t, "https://example.com/alpha", items[0].URL)
	assertEqual(t, "Alpha Entry", items[0].Title)

	assertEqual(t, "https://example.com/beta", items[1].URL)
	assertEqual(t, "Beta Entry", items[1].Title)
}

func TestParseFeed_EmptyFeed(t *testing.T) {
	t.Parallel()

	items, err := feed.ParseFeed(context.Background(), emptyFeedFixture)
	requireNoError(t, err)

	if items == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}

	requireLen(t, items, 0)
}

func TestParseFeed_EntryWithNoLink(t *testing.T) {
	t.Parallel()

	items, err := feed.ParseFeed(context.Background(), noLinkFixture)
	requireNoError(t, err)
	requireLen(t, items, 0)
}

func TestParseFeed_GUIDAsFallbackLink(t *testing.T) {
	t.Parallel()

	items, err := feed.ParseFeed(context.Background(), guidAsLinkFixture)
	requireNoError(t, err)
	requireLen(t, items, 1)

	assertEqual(t, "https://example.com/guid-article", items[0].URL)
	assertEqual(t, "GUID Article", items[0].Title)
}

func TestParseFeed_InvalidXML(t *testing.T) {
	t.Parallel()

	_, err := feed.ParseFeed(context.Background(), "not xml at all")
	if err == nil {
		t.Fatal("expected error for invalid XML, got nil")
	}
}

// assertEqual fails the test if got does not equal want.
func assertEqual(t *testing.T, want, got string) {
	t.Helper()

	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// assertNotEmpty fails the test if the value is an empty string.
func assertNotEmpty(t *testing.T, value string) {
	t.Helper()

	if value == "" {
		t.Error("expected non-empty string, got empty")
	}
}
