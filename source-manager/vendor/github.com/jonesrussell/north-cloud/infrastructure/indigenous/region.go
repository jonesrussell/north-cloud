// Package indigenous provides shared taxonomy constants and normalization
// for the indigenous content pipeline.
package indigenous

import (
	"fmt"
	"strings"
)

// Region slugs for the canonical indigenous region taxonomy.
const (
	RegionCanada       = "canada"
	RegionUS           = "us"
	RegionLatinAmerica = "latin_america"
	RegionOceania      = "oceania"
	RegionEurope       = "europe"
	RegionAsia         = "asia"
	RegionAfrica       = "africa"
)

// allowedRegionCount is the number of canonical region slugs.
const allowedRegionCount = 7

// AllowedRegions is the canonical set of valid region slugs.
var AllowedRegions = map[string]bool{
	RegionCanada:       true,
	RegionUS:           true,
	RegionLatinAmerica: true,
	RegionOceania:      true,
	RegionEurope:       true,
	RegionAsia:         true,
	RegionAfrica:       true,
}

// IsValidRegion reports whether slug is a canonical region.
func IsValidRegion(slug string) bool {
	return AllowedRegions[slug]
}

// NormalizeRegionSlug normalizes a region string to a canonical slug.
// It trims whitespace, lowercases, and replaces spaces/hyphens with underscores.
// Returns an error if the normalized value is not in the allowed set.
// Empty input returns ("", nil) — empty region is valid (means "not set").
func NormalizeRegionSlug(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}

	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")

	if !IsValidRegion(s) {
		return "", fmt.Errorf("invalid indigenous region %q: must be one of %s", raw, allowedRegionList())
	}

	return s, nil
}

// allowedRegionList returns a comma-separated list of valid regions for error messages.
func allowedRegionList() string {
	regions := make([]string, 0, allowedRegionCount)
	for r := range AllowedRegions {
		regions = append(regions, r)
	}
	return strings.Join(regions, ", ")
}
