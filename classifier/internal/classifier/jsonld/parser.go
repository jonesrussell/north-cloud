// Package jsonld provides utilities for parsing Schema.org JSON-LD structured
// data from HTML documents. It extracts <script type="application/ld+json">
// blocks and provides type-safe accessors for common Schema.org fields.
package jsonld

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

// jsonLDPattern matches <script type="application/ld+json">...</script> blocks.
// The (?s) flag enables dotall mode so . matches newlines.
var jsonLDPattern = regexp.MustCompile(
	`(?si)<script[^>]+type=["']application/ld\+json["'][^>]*>(.*?)</script>`,
)

// minutesPerHour is the number of minutes in one hour.
const minutesPerHour = 60

// durationPattern matches ISO 8601 duration strings like PT1H30M, PT30M, PT2H.
var durationPattern = regexp.MustCompile(`^PT(?:(\d+)H)?(?:(\d+)M)?$`)

// Extract finds all <script type="application/ld+json"> blocks in HTML,
// parses them as JSON, and returns them as a slice of maps. It handles both
// single JSON objects and JSON arrays (common pattern: [{@type: BreadcrumbList},
// {@type: Recipe}]). Malformed JSON blocks are silently skipped.
func Extract(html string) []map[string]any {
	matches := jsonLDPattern.FindAllStringSubmatch(html, -1)
	if len(matches) == 0 {
		return nil
	}

	var blocks []map[string]any

	for _, match := range matches {
		content := strings.TrimSpace(match[1])
		if content == "" {
			continue
		}

		parsed := parseJSONLDContent(content)
		blocks = append(blocks, parsed...)
	}

	return blocks
}

// parseJSONLDContent parses a JSON string that may be either a single object
// or an array of objects. Returns all successfully parsed objects.
func parseJSONLDContent(content string) []map[string]any {
	// Try parsing as a single object first.
	var single map[string]any
	if err := json.Unmarshal([]byte(content), &single); err == nil {
		return []map[string]any{single}
	}

	// Try parsing as an array of objects.
	var arr []map[string]any
	if err := json.Unmarshal([]byte(content), &arr); err == nil {
		return arr
	}

	return nil
}

// FindByType searches blocks for a specific @type value and returns the first
// match. Returns nil if no block matches.
func FindByType(blocks []map[string]any, typeName string) map[string]any {
	for _, block := range blocks {
		typeVal, ok := block["@type"].(string)
		if ok && typeVal == typeName {
			return block
		}
	}

	return nil
}

// ParseISO8601Duration converts an ISO 8601 duration string (e.g., "PT1H30M")
// to total minutes. Returns nil for unrecognized formats.
//
// Supported formats:
//   - PT30M   -> 30
//   - PT1H    -> 60
//   - PT1H30M -> 90
//   - PT2H15M -> 135
func ParseISO8601Duration(duration string) *int {
	matches := durationPattern.FindStringSubmatch(duration)
	if matches == nil {
		return nil
	}

	hours, minutes := 0, 0

	if matches[1] != "" {
		h, err := strconv.Atoi(matches[1])
		if err != nil {
			return nil
		}

		hours = h
	}

	if matches[2] != "" {
		m, err := strconv.Atoi(matches[2])
		if err != nil {
			return nil
		}

		minutes = m
	}

	// If neither hours nor minutes were captured, the duration is empty (just "PT").
	if matches[1] == "" && matches[2] == "" {
		return nil
	}

	total := hours*minutesPerHour + minutes

	return &total
}

// StringVal safely extracts a string value from a JSON-LD map.
// Returns empty string if the key is missing or the value is not a string.
func StringVal(m map[string]any, key string) string {
	if m == nil {
		return ""
	}

	val, ok := m[key].(string)
	if !ok {
		return ""
	}

	return val
}

// StringSliceVal safely extracts a string slice from a JSON-LD map.
// Handles both []any (from JSON unmarshal) and single string values.
// Non-string elements within an array are silently skipped.
func StringSliceVal(m map[string]any, key string) []string {
	if m == nil {
		return nil
	}

	raw, exists := m[key]
	if !exists {
		return nil
	}

	// Handle single string value.
	if s, ok := raw.(string); ok {
		return []string{s}
	}

	// Handle []any (standard JSON unmarshal output).
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}

	result := make([]string, 0, len(arr))

	for _, elem := range arr {
		if s, isStr := elem.(string); isStr {
			result = append(result, s)
		}
	}

	return result
}

// NestedStringVal safely extracts a string value from a nested map.
// For example, {"hiringOrganization": {"name": "Acme"}} can be accessed via
// NestedStringVal(m, "hiringOrganization", "name").
func NestedStringVal(m map[string]any, outerKey, innerKey string) string {
	if m == nil {
		return ""
	}

	outer, ok := m[outerKey].(map[string]any)
	if !ok {
		return ""
	}

	return StringVal(outer, innerKey)
}

// FloatVal safely extracts a float64 value from a JSON-LD map.
// Handles both float64 (from JSON unmarshal) and string representations.
// Returns nil if the key is missing or the value cannot be converted.
func FloatVal(m map[string]any, key string) *float64 {
	if m == nil {
		return nil
	}

	raw, exists := m[key]
	if !exists {
		return nil
	}

	// Handle native float64 (standard JSON unmarshal for numbers).
	if f, ok := raw.(float64); ok {
		return &f
	}

	// Handle string representation.
	if s, ok := raw.(string); ok {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil
		}

		return &f
	}

	return nil
}

// IntVal safely extracts an int value from a JSON-LD map.
// Handles both float64 (from JSON unmarshal) and string representations.
// Returns nil if the key is missing or the value cannot be converted.
func IntVal(m map[string]any, key string) *int {
	if m == nil {
		return nil
	}

	raw, exists := m[key]
	if !exists {
		return nil
	}

	// Handle native float64 (standard JSON unmarshal for numbers).
	if f, ok := raw.(float64); ok {
		val := int(f)
		return &val
	}

	// Handle string representation.
	if s, ok := raw.(string); ok {
		val, err := strconv.Atoi(s)
		if err != nil {
			return nil
		}

		return &val
	}

	return nil
}
