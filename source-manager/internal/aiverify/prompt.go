package aiverify

import (
	"encoding/json"
	"errors"
	"fmt"
)

// SystemPrompt is the fixed system prompt for verification.
const SystemPrompt = `You are a data quality verifier for First Nations community leadership ` +
	`and contact records scraped from official websites. Your job is to evaluate whether ` +
	`extracted data is plausible and internally consistent.

Evaluate:
1. Name plausibility — Is this a real human name, or scraper noise ` +
	`(navigation text, template fragments, "Click Here", "Vacant", "TBD")?
2. Role plausibility — Is the role a recognized leadership/staff title ` +
	`(Chief, Councillor, Band Manager, Director, Elder, etc.)?
3. Cross-field consistency — Does phone area code match province? ` +
	`Does email domain relate to the community? Does address match expected region?

Return JSON only. Format:
{"confidence": 0.0-1.0, "issues": [{"field": "...", "issue": "...", "severity": "error|warning|info"}]}`

// VerifyInput holds the record data to send to the LLM.
type VerifyInput struct {
	RecordType    string `json:"record_type"`
	Name          string `json:"name,omitempty"`
	Role          string `json:"role,omitempty"`
	Email         string `json:"email,omitempty"`
	Phone         string `json:"phone,omitempty"`
	CommunityName string `json:"community_name"`
	Province      string `json:"province,omitempty"`
	SourceURL     string `json:"source_url,omitempty"`
	AddressLine1  string `json:"address_line1,omitempty"`
	AddressLine2  string `json:"address_line2,omitempty"`
	City          string `json:"city,omitempty"`
	PostalCode    string `json:"postal_code,omitempty"`
	Fax           string `json:"fax,omitempty"`
	TollFree      string `json:"toll_free,omitempty"`
	OfficeHours   string `json:"office_hours,omitempty"`
}

// VerifyResult is the parsed LLM response.
type VerifyResult struct {
	Confidence float64       `json:"confidence"`
	Issues     []VerifyIssue `json:"issues"`
}

// VerifyIssue is a single issue found by the LLM.
type VerifyIssue struct {
	Field    string `json:"field"`
	Issue    string `json:"issue"`
	Severity string `json:"severity"` // "error", "warning", "info"
}

// BuildUserPrompt renders the user prompt JSON for a record.
func BuildUserPrompt(input VerifyInput) string {
	data, marshalErr := json.MarshalIndent(input, "", "  ")
	if marshalErr != nil {
		return fmt.Sprintf(
			`{"record_type": %q, "community_name": %q}`,
			input.RecordType, input.CommunityName,
		)
	}
	return string(data)
}

// ParseVerifyResponse parses the LLM's JSON response.
func ParseVerifyResponse(raw string) (*VerifyResult, error) {
	var m map[string]any
	if unmarshalErr := json.Unmarshal([]byte(raw), &m); unmarshalErr != nil {
		return nil, fmt.Errorf("parse verify response: %w", unmarshalErr)
	}
	if _, ok := m["confidence"]; !ok {
		return nil, errors.New("parse verify response: missing confidence field")
	}

	var result VerifyResult
	if parseErr := json.Unmarshal([]byte(raw), &result); parseErr != nil {
		return nil, fmt.Errorf("parse verify response: %w", parseErr)
	}
	return &result, nil
}
