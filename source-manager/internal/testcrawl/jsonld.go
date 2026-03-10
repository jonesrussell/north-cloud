package testcrawl

import (
	"encoding/json"
	"strings"
)

// extractJSONLDHeadline extracts the headline or name from a JSON-LD script tag's text content.
// Returns "" if parsing fails or no headline is found.
func extractJSONLDHeadline(scriptText string) string {
	scriptText = strings.TrimSpace(scriptText)
	if scriptText == "" {
		return ""
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(scriptText), &data); err != nil {
		return ""
	}
	typeVal, _ := data["@type"].(string)
	switch typeVal {
	case "NewsArticle", "Article", "BlogPosting", "PressRelease":
		if h, ok := data["headline"].(string); ok && h != "" {
			return strings.TrimSpace(h)
		}
	case "Event", "SpecialAnnouncement", "Report":
		if n, ok := data["name"].(string); ok && n != "" {
			return strings.TrimSpace(n)
		}
	}
	return ""
}

// extractJSONLDTypeFromText extracts the @type value from a JSON-LD script tag's text content.
func extractJSONLDTypeFromText(scriptText string) string {
	scriptText = strings.TrimSpace(scriptText)
	if scriptText == "" {
		return ""
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(scriptText), &data); err != nil {
		return ""
	}
	t, _ := data["@type"].(string)
	return t
}
