package database_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

func TestDomainStateUpsertFields(t *testing.T) {
	// Verify domain state model has required fields
	state := domain.DomainState{
		Domain: "example.com",
		Status: domain.DomainStatusIgnored,
	}
	if state.Domain != "example.com" {
		t.Errorf("expected domain example.com, got %s", state.Domain)
	}
	if state.Status != "ignored" {
		t.Errorf("expected status ignored, got %s", state.Status)
	}
}
