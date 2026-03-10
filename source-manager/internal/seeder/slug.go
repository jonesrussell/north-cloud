package seeder

import (
	"regexp"
	"strings"
)

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify converts a community name to a URL-friendly slug.
func Slugify(name string) string {
	slug := strings.ToLower(strings.TrimSpace(name))
	slug = nonAlphanumeric.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	return slug
}
