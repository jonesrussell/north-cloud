package api

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// EnrichmentRequest is the HTTP contract Waaseyaa sends to the enrichment service.
type EnrichmentRequest struct {
	LeadID         string         `json:"lead_id"`
	CompanyName    string         `json:"company_name"`
	Domain         string         `json:"domain,omitempty"`
	Sector         string         `json:"sector,omitempty"`
	RequestedTypes []string       `json:"requested_types"`
	Signals        map[string]any `json:"signals,omitempty"`
	CallbackURL    string         `json:"callback_url"`
	CallbackAPIKey string         `json:"callback_api_key"`
}

// AcceptedResponse is returned after a request has passed validation.
type AcceptedResponse struct {
	Status string `json:"status"`
	LeadID string `json:"lead_id"`
}

// ErrorResponse is returned for invalid API requests.
type ErrorResponse struct {
	Error  string       `json:"error"`
	Fields []FieldError `json:"fields,omitempty"`
}

// FieldError describes one invalid request field.
type FieldError struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

// ValidationError aggregates request validation failures.
type ValidationError struct {
	Fields []FieldError
}

// Error summarizes validation failure without echoing request values.
func (e ValidationError) Error() string {
	if len(e.Fields) == 0 {
		return "request validation failed"
	}
	return fmt.Sprintf("request validation failed: %d invalid field(s)", len(e.Fields))
}

// Validate checks required fields and basic callback/request shape.
func (r EnrichmentRequest) Validate() error {
	var fields []FieldError

	if strings.TrimSpace(r.LeadID) == "" {
		fields = append(fields, FieldError{Field: "lead_id", Reason: "is required"})
	}
	if strings.TrimSpace(r.CompanyName) == "" {
		fields = append(fields, FieldError{Field: "company_name", Reason: "is required"})
	}
	if len(r.RequestedTypes) == 0 {
		fields = append(fields, FieldError{Field: "requested_types", Reason: "must contain at least one type"})
	}
	for index, requestedType := range r.RequestedTypes {
		if strings.TrimSpace(requestedType) == "" {
			fields = append(fields, FieldError{
				Field:  fmt.Sprintf("requested_types[%d]", index),
				Reason: "must not be empty",
			})
		}
	}
	if strings.TrimSpace(r.CallbackURL) == "" {
		fields = append(fields, FieldError{Field: "callback_url", Reason: "is required"})
	} else if !isHTTPURL(r.CallbackURL) {
		fields = append(fields, FieldError{Field: "callback_url", Reason: "must be an absolute http or https URL"})
	}
	if strings.TrimSpace(r.CallbackAPIKey) == "" {
		fields = append(fields, FieldError{Field: "callback_api_key", Reason: "is required"})
	}

	if len(fields) > 0 {
		return ValidationError{Fields: fields}
	}
	return nil
}

// ValidationFields extracts field failures from a validation error.
func ValidationFields(err error) []FieldError {
	var validationErr ValidationError
	if errors.As(err, &validationErr) {
		return validationErr.Fields
	}
	return nil
}

func isHTTPURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return parsed.IsAbs() && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}
