package models

import "slices"

// Rules defines the filtering rules for a custom channel
type Rules struct {
	IncludeTopics   []string `json:"include_topics"`
	ExcludeTopics   []string `json:"exclude_topics"`
	MinQualityScore int      `json:"min_quality_score"`
	ContentTypes    []string `json:"content_types"`
}

// IsEmpty returns true if no rules are defined (matches everything)
func (r *Rules) IsEmpty() bool {
	return len(r.IncludeTopics) == 0 &&
		len(r.ExcludeTopics) == 0 &&
		r.MinQualityScore == 0 &&
		len(r.ContentTypes) == 0
}

// Matches checks if an article matches the rules
func (r *Rules) Matches(qualityScore int, contentType string, topics []string) bool {
	// Fast path: empty rules match everything
	if r.IsEmpty() {
		return true
	}

	// Quality check
	if r.MinQualityScore > 0 && qualityScore < r.MinQualityScore {
		return false
	}

	// Content type check
	if len(r.ContentTypes) > 0 && !slices.Contains(r.ContentTypes, contentType) {
		return false
	}

	// Exclude topics check
	if hasAny(topics, r.ExcludeTopics) {
		return false
	}

	// Include topics check (empty = match all)
	if len(r.IncludeTopics) > 0 && !hasAny(topics, r.IncludeTopics) {
		return false
	}

	return true
}

// hasAny checks if any value from needles exists in haystack
func hasAny(haystack, needles []string) bool {
	for _, needle := range needles {
		if slices.Contains(haystack, needle) {
			return true
		}
	}
	return false
}
