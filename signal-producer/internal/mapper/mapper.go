// Package mapper converts an Elasticsearch classified-content hit into the
// Waaseyaa Signal wire format. It is a pure data transformation: no I/O, no
// networking, no logging dependencies.
package mapper

import (
	"fmt"
)

// Named constants — collision-prevention prefixes (FR-007) and the canonical
// source identifier (FR-006).
const (
	externalIDPrefixRFP        = "nc-rfp-"
	externalIDPrefixNeedSignal = "nc-sig-"
	sourceNorthCloud           = "north-cloud"

	contentTypeRFP        = "rfp"
	contentTypeNeedSignal = "need_signal"

	fieldID           = "_id"
	fieldTitle        = "title"
	fieldQualityScore = "quality_score"
	fieldURL          = "url"
	fieldCrawledAt    = "crawled_at"
	fieldContentType  = "content_type"
)

// Signal is the Waaseyaa wire format for a single signal record. Mirrors
// contracts/signals-post.yaml.
type Signal struct {
	SignalType       string         `json:"signal_type"`
	ExternalID       string         `json:"external_id"`
	Source           string         `json:"source"`
	SourceURL        string         `json:"source_url"`
	Label            string         `json:"label"`
	Strength         int            `json:"strength"`
	OrganizationName string         `json:"organization_name"`
	Sector           string         `json:"sector"`
	Province         string         `json:"province"`
	ExpiresAt        *string        `json:"expires_at,omitempty"`
	Payload          map[string]any `json:"payload"`
}

// MapHit converts a raw ES hit (a generic map) into a Signal. Returns a
// wrapped error if any required field is missing or if the content_type is
// unsupported. Optional fields tolerate missing or wrong-typed steps and
// default to the zero value of their target type (FR-008).
func MapHit(hit map[string]any) (Signal, error) {
	id, err := requiredString(hit, fieldID)
	if err != nil {
		return Signal{}, err
	}
	title, err := requiredString(hit, fieldTitle)
	if err != nil {
		return Signal{}, err
	}
	qualityScore, err := requiredInt(hit, fieldQualityScore)
	if err != nil {
		return Signal{}, err
	}
	url, err := requiredString(hit, fieldURL)
	if err != nil {
		return Signal{}, err
	}
	if _, err := requiredString(hit, fieldCrawledAt); err != nil {
		return Signal{}, err
	}
	contentType, err := requiredString(hit, fieldContentType)
	if err != nil {
		return Signal{}, err
	}

	signal := Signal{
		Source:    sourceNorthCloud,
		SourceURL: url,
		Label:     title,
		Strength:  qualityScore,
		Payload:   hit,
	}

	switch contentType {
	case contentTypeRFP:
		applyRFPFields(&signal, hit, id)
	case contentTypeNeedSignal:
		if err := applyNeedSignalFields(&signal, hit, id); err != nil {
			return Signal{}, err
		}
	default:
		return Signal{}, fmt.Errorf("mapper: unsupported content_type %q", contentType)
	}

	return signal, nil
}

// applyRFPFields fills RFP-specific fields. All RFP subfields are optional;
// missing values degrade to empty strings. ExpiresAt is set only when
// rfp.closing_date is present.
func applyRFPFields(signal *Signal, hit map[string]any, id string) {
	signal.SignalType = contentTypeRFP
	signal.ExternalID = externalIDPrefixRFP + id
	signal.OrganizationName = stringFromPath(hit, "rfp", "organization_name")
	signal.Province = stringFromPath(hit, "rfp", "province")
	signal.Sector = firstStringInSlice(hit, "rfp", "categories")
	if closing, ok := optionalStringFromPath(hit, "rfp", "closing_date"); ok {
		signal.ExpiresAt = &closing
	}
}

// applyNeedSignalFields fills need_signal-specific fields. Returns an error if
// the required need_signal.signal_type is missing.
func applyNeedSignalFields(signal *Signal, hit map[string]any, id string) error {
	signalType, ok := optionalStringFromPath(hit, contentTypeNeedSignal, "signal_type")
	if !ok || signalType == "" {
		return fmt.Errorf("mapper: missing required field %q", "need_signal.signal_type")
	}
	signal.SignalType = signalType
	signal.ExternalID = externalIDPrefixNeedSignal + id
	signal.OrganizationName = stringFromPath(hit, contentTypeNeedSignal, "organization_name")
	signal.Province = stringFromPath(hit, contentTypeNeedSignal, "province")
	signal.Sector = stringFromPath(hit, contentTypeNeedSignal, "sector")
	return nil
}

// requiredString fetches a top-level string field. Missing or wrong-typed
// values yield a wrapped error.
func requiredString(hit map[string]any, field string) (string, error) {
	raw, exists := hit[field]
	if !exists {
		return "", fmt.Errorf("mapper: missing required field %q", field)
	}
	value, ok := raw.(string)
	if !ok || value == "" {
		return "", fmt.Errorf("mapper: missing required field %q", field)
	}
	return value, nil
}

// requiredInt fetches a top-level numeric field. JSON unmarshal yields
// float64 for numbers; we accept both float64 and int.
func requiredInt(hit map[string]any, field string) (int, error) {
	raw, exists := hit[field]
	if !exists {
		return 0, fmt.Errorf("mapper: missing required field %q", field)
	}
	switch typed := raw.(type) {
	case float64:
		return int(typed), nil
	case int:
		return typed, nil
	default:
		return 0, fmt.Errorf("mapper: missing required field %q", field)
	}
}

// stringFromPath walks a nested path (typically "rfp" → "field_name") and
// returns the string value. Returns "" if any step is missing or wrong-typed.
func stringFromPath(hit map[string]any, path ...string) string {
	value, _ := optionalStringFromPath(hit, path...)
	return value
}

// optionalStringFromPath walks a nested path and reports both the value and
// whether it was actually found as a non-empty string.
func optionalStringFromPath(hit map[string]any, path ...string) (string, bool) {
	current := hit
	for i, step := range path {
		raw, exists := current[step]
		if !exists {
			return "", false
		}
		if i == len(path)-1 {
			value, ok := raw.(string)
			if !ok || value == "" {
				return "", false
			}
			return value, true
		}
		next, ok := raw.(map[string]any)
		if !ok {
			return "", false
		}
		current = next
	}
	return "", false
}

// firstStringInSlice walks the path to a []any and returns its first element
// as a string. Returns "" if the path is missing, the value isn't a slice,
// or the first element isn't a string.
func firstStringInSlice(hit map[string]any, path ...string) string {
	current := hit
	for i, step := range path {
		raw, exists := current[step]
		if !exists {
			return ""
		}
		if i == len(path)-1 {
			slice, ok := raw.([]any)
			if !ok || len(slice) == 0 {
				return ""
			}
			first, ok := slice[0].(string)
			if !ok {
				return ""
			}
			return first
		}
		next, ok := raw.(map[string]any)
		if !ok {
			return ""
		}
		current = next
	}
	return ""
}

// intFromPath walks a nested path to a numeric leaf, accepting both float64
// (the JSON unmarshal default) and int. Returns 0 on any failure.
func intFromPath(hit map[string]any, path ...string) int {
	current := hit
	for i, step := range path {
		raw, exists := current[step]
		if !exists {
			return 0
		}
		if i == len(path)-1 {
			switch typed := raw.(type) {
			case float64:
				return int(typed)
			case int:
				return typed
			default:
				return 0
			}
		}
		next, ok := raw.(map[string]any)
		if !ok {
			return 0
		}
		current = next
	}
	return 0
}
