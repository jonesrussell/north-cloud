package rss

import (
	"encoding/xml"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
)

// Feed is the top-level RSS 2.0 document.
type Feed struct {
	XMLName xml.Name `xml:"rss"`
	Channel Channel  `xml:"channel"`
}

// Channel is the RSS channel element.
type Channel struct {
	Title string `xml:"title"`
	Items []Item `xml:"item"`
}

// Item is a single RSS item element.
type Item struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Link        string `xml:"link"`
	PubDate     string `xml:"pubDate"`
	Author      string `xml:"author"`
}

// pubDateLayouts lists the RFC variants accepted by safersites.ca feeds, in
// priority order. RFC1123Z and RFC822Z are tried first because they carry an
// explicit numeric timezone offset, which is the most reliable form.
var pubDateLayouts = []string{
	time.RFC1123Z,
	time.RFC1123,
	time.RFC822Z,
	time.RFC822,
}

// ParseFeed decodes an RSS 2.0 XML body and returns the structured Feed.
func ParseFeed(body []byte) (*Feed, error) {
	var f Feed
	if err := xml.Unmarshal(body, &f); err != nil {
		return nil, fmt.Errorf("parse feed: %w", err)
	}
	return &f, nil
}

// ParsePubDate parses an RSS pubDate string accepting RFC1123Z, RFC1123,
// RFC822Z, and RFC822 formats. The returned time is always UTC.
func ParsePubDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range pubDateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("parse pubDate %q: unrecognized format", s)
}

// DeriveID constructs a stable, deterministic alert ID from a source ID and
// item link. The slug is the last non-empty path segment of the URL, lowercased.
// Format: "{sourceID}:{slug}".
func DeriveID(sourceID, link string) string {
	u, err := url.Parse(link)
	if err != nil || u.Path == "" {
		// Fall back to lowercased raw link when URL parsing fails.
		return sourceID + ":" + strings.ToLower(link)
	}
	slug := path.Base(u.Path)
	slug = strings.ToLower(slug)
	return sourceID + ":" + slug
}

// parseItemRequired validates the fields that must be present for any alert.
// Returns a non-nil error when any required field is absent.
func parseItemRequired(item Item) error {
	var missing []string
	if strings.TrimSpace(item.Link) == "" {
		missing = append(missing, "link")
	}
	if strings.TrimSpace(item.PubDate) == "" {
		missing = append(missing, "pubDate")
	}
	if strings.TrimSpace(item.Title) == "" {
		missing = append(missing, "title")
	}
	if len(missing) > 0 {
		return fmt.Errorf("parse item: required fields missing: %s", strings.Join(missing, ", "))
	}
	return nil
}

// buildHazard assembles a HarmReductionHazard from extractor outputs.
// degraded is true when at least one optional field could not be extracted.
func buildHazard(item Item) (*domain.HarmReductionHazard, bool) {
	desc := item.Description
	substances := ExtractSubstances(desc)
	composition := ExtractComposition(desc)
	labSource := ExtractLabSource(desc)

	degraded := len(substances) == 0 || labSource == ""

	return &domain.HarmReductionHazard{
		HazardType:        domain.HazardOpioidSupply,
		Substances:        substances,
		Composition:       composition,
		VisualDescription: "",
		LabSource:         labSource,
	}, degraded
}

// ParseItem converts a raw RSS Item into a domain.Alert.
//
// Quality rules (TC-010):
//   - Required fields absent (link, pubDate, title) → return error + ParseFailed.
//   - All optional fields extracted cleanly → ParseClean.
//   - Optional fields partial (substances empty or lab missing) → ParseDegraded;
//     raw description preserved in Alert.Summary.
//
// ParseFailed items must NEVER trigger auto-rescission in the catalogue diff;
// the runner (WP15) must skip them on error return.
func ParseItem(item Item, src domain.AlertSource) (domain.Alert, error) {
	if err := parseItemRequired(item); err != nil {
		return domain.Alert{ParseQuality: domain.ParseFailed}, err
	}

	issuedAt, err := ParsePubDate(item.PubDate)
	if err != nil {
		return domain.Alert{ParseQuality: domain.ParseFailed},
			fmt.Errorf("parse item pubDate: %w", err)
	}

	now := time.Now().UTC()

	hazard, degraded := buildHazard(item)

	quality := domain.ParseClean
	summary := ExtractTitle(item)

	if degraded {
		quality = domain.ParseDegraded
		// Preserve the raw description so operators can inspect it.
		summary = item.Description
	}

	alert := domain.Alert{
		ID:             DeriveID(src.ID, item.Link),
		Category:       domain.CategoryHarmReduction,
		Severity:       domain.SeverityHigh,
		Scope:          src.DefaultScope,
		IssuedAt:       issuedAt,
		LifecycleState: domain.LifecycleActive,
		Title:          ExtractTitle(item),
		Summary:        summary,
		Hazard: domain.Hazard{
			HarmReduction: hazard,
		},
		Guidance:      ExtractGuidance(item.Description),
		Sources:       buildSources(src, item.Link),
		ParseQuality:  quality,
		CrawledAt:     now,
		LastUpdatedAt: now,
	}

	if src.DefaultExpiry > 0 {
		exp := issuedAt.Add(src.DefaultExpiry)
		alert.ExpiresAt = &exp
	}

	return alert, nil
}

// buildSources constructs the SourceAttribution slice for the alert.
func buildSources(src domain.AlertSource, itemLink string) []domain.SourceAttribution {
	return []domain.SourceAttribution{
		{
			SourceID:   src.ID,
			SourceName: src.Name,
			URL:        itemLink,
		},
	}
}
