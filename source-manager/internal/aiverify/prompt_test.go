package aiverify_test

import (
	"strings"
	"testing"

	"github.com/jonesrussell/north-cloud/source-manager/internal/aiverify"
)

func TestBuildPersonPrompt(t *testing.T) {
	input := aiverify.VerifyInput{
		RecordType:    "person",
		Name:          "John Smith",
		Role:          "Chief",
		Email:         "jsmith@fwfn.com",
		Phone:         "807-555-1234",
		CommunityName: "Fort William First Nation",
		Province:      "Ontario",
		SourceURL:     "https://fwfn.com/council",
	}

	prompt := aiverify.BuildUserPrompt(input)
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	if !strings.Contains(prompt, "Fort William First Nation") {
		t.Error("prompt missing community name")
	}
	if !strings.Contains(prompt, "person") {
		t.Error("prompt missing record_type")
	}
}

func TestBuildBandOfficePrompt(t *testing.T) {
	input := aiverify.VerifyInput{
		RecordType:    "band_office",
		CommunityName: "Fort William First Nation",
		Province:      "Ontario",
		Phone:         "807-623-9543",
		Email:         "reception@fwfn.com",
		AddressLine1:  "90 Anemki Drive",
		City:          "Thunder Bay",
		PostalCode:    "P7J 1L3",
		SourceURL:     "https://fwfn.com/contact",
	}

	prompt := aiverify.BuildUserPrompt(input)
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	if !strings.Contains(prompt, "band_office") {
		t.Error("prompt missing record_type")
	}
	if !strings.Contains(prompt, "90 Anemki Drive") {
		t.Error("prompt missing address")
	}
}

func TestParseVerifyResponse_Valid(t *testing.T) {
	raw := `{"confidence": 0.92, "issues": [` +
		`{"field": "email", "issue": "Generic domain", "severity": "info"}]}`
	result, err := aiverify.ParseVerifyResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Confidence != 0.92 {
		t.Errorf("expected confidence 0.92, got %f", result.Confidence)
	}
	if len(result.Issues) != 1 {
		t.Errorf("expected 1 issue, got %d", len(result.Issues))
	}
}

func TestParseVerifyResponse_InvalidJSON(t *testing.T) {
	_, err := aiverify.ParseVerifyResponse("not json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseVerifyResponse_MissingConfidence(t *testing.T) {
	raw := `{"issues": []}`
	_, err := aiverify.ParseVerifyResponse(raw)
	if err == nil {
		t.Error("expected error for missing confidence")
	}
}
