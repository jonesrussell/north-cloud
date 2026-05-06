package rss

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
)

// knownSubstances is the set of substance tokens the extractor recognises in
// free-text descriptions. Comparisons are always lowercased.
var knownSubstances = []string{
	"fentanyl",
	"carfentanil",
	"nitazene",
	"metonitazene",
	"xylazine",
	"etizolam",
	"clonazolam",
	"methamphetamine",
	"heroin",
	"cocaine",
	"mdma",
	"methylone",
	"levamisole",
	"medetomidine",
	"caffeine",
	"mannitol",
	"lactose",
	"cellulose",
	"msm",
}

// reLocation matches "Drug Alert: <Location>" or "<strong>Location:</strong> <text>"
// anchored on the location-bearing line as produced by NationBuilder/safersites.ca.
// The capture group stops at an HTML tag, newline, or an em-dash / en-dash separator
// but allows commas so "City, Province" is captured whole.
var reLocation = regexp.MustCompile(`(?i)(?:Drug Alert:\s+|<strong>Location:</strong>\s*)([^<\n\r–—]+)`)

// reComposition matches entries of the form "Name (XX.XX%)" or "Name (XX%)".
var reComposition = regexp.MustCompile(`([A-Za-z][A-Za-z0-9 \-]+?)\s*\(\s*(\d+(?:\.\d+)?)\s*%\s*\)`)

// reGuidanceBullet captures text inside <li>...</li> tags (guidance bullets).
var reGuidanceBullet = regexp.MustCompile(`(?i)<li[^>]*>\s*(.*?)\s*</li>`)

// reHTMLTag strips any HTML tag.
var reHTMLTag = regexp.MustCompile(`<[^>]+>`)

// minLocationMatchLen is the minimum number of sub-matches required from the
// location regex (full match + capture group).
const minLocationMatchLen = 2

// minCompositionMatchLen is the minimum number of sub-matches required from
// the composition regex (full match + name + percentage).
const minCompositionMatchLen = 3

// minGuidanceMatchLen is the minimum number of sub-matches required from
// the guidance bullet regex (full match + capture group).
const minGuidanceMatchLen = 2

// ExtractTitle returns the display title for an alert item.
// It prefers the RSS <title> element; if empty, it falls back to the first
// non-empty line of the description after stripping HTML tags.
func ExtractTitle(item Item) string {
	if t := strings.TrimSpace(item.Title); t != "" {
		return t
	}
	return firstDescriptionLine(item.Description)
}

// firstDescriptionLine strips HTML from description and returns the first
// non-empty line of text.
func firstDescriptionLine(description string) string {
	plain := reHTMLTag.ReplaceAllString(description, " ")
	for _, line := range strings.Split(plain, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

// ExtractLocation extracts the geographic location from a description string.
// It looks for "Drug Alert: <Location>" or "<strong>Location:</strong> <text>".
// Returns an empty string when no location can be found.
func ExtractLocation(description string) string {
	matches := reLocation.FindStringSubmatch(description)
	if len(matches) < minLocationMatchLen {
		return ""
	}
	loc := strings.TrimSpace(matches[1])
	// Strip any trailing HTML tags that slipped through.
	loc = reHTMLTag.ReplaceAllString(loc, "")
	loc = strings.TrimSpace(loc)
	// Trim trailing date/time suffix separated by " -" (e.g. "Winnipeg - Tue. May 5, 2026").
	if idx := strings.Index(loc, " -"); idx > 0 {
		loc = strings.TrimSpace(loc[:idx])
	}
	return loc
}

// ExtractSubstances returns the substance names mentioned in the description.
// It matches against the known-substances vocabulary (case-insensitive) and
// returns unique lowercase names in the order first encountered.
func ExtractSubstances(description string) []string {
	lower := strings.ToLower(description)
	seen := make(map[string]struct{}, len(knownSubstances))
	result := make([]string, 0, len(knownSubstances))
	for _, sub := range knownSubstances {
		if strings.Contains(lower, sub) {
			if _, ok := seen[sub]; !ok {
				seen[sub] = struct{}{}
				result = append(result, sub)
			}
		}
	}
	return result
}

// ExtractComposition parses "Name (XX.X%)" entries from the description and
// returns them as []domain.Substance. Entries whose percentage is at or below
// minCompositionPct are included as-is; the caller decides their relevance.
func ExtractComposition(description string) []domain.Substance {
	matches := reComposition.FindAllStringSubmatch(description, -1)
	result := make([]domain.Substance, 0, len(matches))
	for _, m := range matches {
		if len(m) < minCompositionMatchLen {
			continue
		}
		name := strings.TrimSpace(m[1])
		pct, err := strconv.ParseFloat(strings.TrimSpace(m[2]), 64)
		if err != nil {
			continue
		}
		result = append(result, domain.Substance{
			Name:       name,
			Percentage: pct,
		})
	}
	return result
}

// ExtractLabSource returns the testing laboratory identified in the description.
// It looks for "Health Canada Drug Analysis Service", "BCCDC", "Forensic", or
// any line containing "Drug Checking" after stripping HTML.
func ExtractLabSource(description string) string {
	plain := reHTMLTag.ReplaceAllString(description, " ")
	candidates := []string{
		"Health Canada Drug Analysis Service",
		"BCCDC Drug Checking Service",
		"Forensic Drug Checking Laboratory",
		"Island Drug Checking Network",
		"Interior Health Drug Checking",
		"Vancouver Island Drug Checking",
		"Fraser Health Drug Checking",
		"Metro Vancouver Drug Checking",
		"Provincial Drug Checking Network",
	}
	lower := strings.ToLower(plain)
	for _, c := range candidates {
		if strings.Contains(lower, strings.ToLower(c)) {
			return c
		}
	}
	// Fallback: any line that contains "drug checking" or "drug analysis".
	for _, line := range strings.Split(plain, "\n") {
		l := strings.TrimSpace(line)
		ll := strings.ToLower(l)
		if strings.Contains(ll, "drug checking") || strings.Contains(ll, "drug analysis") {
			return l
		}
	}
	return ""
}

// ExtractGuidance returns the bulleted recommendations from a description.
// It parses <li> elements and strips any inner HTML, returning clean text lines.
// When no <li> elements are found it returns nil.
func ExtractGuidance(description string) []string {
	matches := reGuidanceBullet.FindAllStringSubmatch(description, -1)
	if len(matches) == 0 {
		return nil
	}
	result := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) < minGuidanceMatchLen {
			continue
		}
		text := reHTMLTag.ReplaceAllString(m[1], "")
		text = strings.TrimSpace(text)
		if text != "" {
			result = append(result, text)
		}
	}
	return result
}
