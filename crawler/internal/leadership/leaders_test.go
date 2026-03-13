package leadership_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/leadership"
)

func TestExtractLeaders(t *testing.T) {
	text := `Chief
John Smith

Councillors
Mary Johnson
Robert Williams
Sarah Brown`

	leaders := leadership.ExtractLeaders(text)

	if len(leaders) != 4 {
		t.Fatalf("expected 4 leaders, got %d: %+v", len(leaders), leaders)
	}

	if leaders[0].Name != "John Smith" || leaders[0].Role != "chief" {
		t.Errorf("leader[0] = %+v, want Chief John Smith", leaders[0])
	}

	if leaders[1].Role != "councillor" {
		t.Errorf("leader[1].Role = %q, want %q", leaders[1].Role, "councillor")
	}
}

func TestExtractLeaders_InlineRole(t *testing.T) {
	text := `Chief John Smith
Councillor Mary Johnson
Councillor Robert Williams`

	leaders := leadership.ExtractLeaders(text)

	if len(leaders) != 3 {
		t.Fatalf("expected 3 leaders, got %d: %+v", len(leaders), leaders)
	}

	if leaders[0].Name != "John Smith" || leaders[0].Role != "chief" {
		t.Errorf("leader[0] = %+v, want Chief John Smith", leaders[0])
	}
}

func TestExtractLeaders_Empty(t *testing.T) {
	leaders := leadership.ExtractLeaders("No leadership info here, just some text about programs.")

	if len(leaders) != 0 {
		t.Errorf("expected 0 leaders, got %d: %+v", len(leaders), leaders)
	}
}

func TestExtractLeaders_EmailOnNextLine(t *testing.T) {
	text := `Chief
John Smith
john.smith@firstnation.ca`

	leaders := leadership.ExtractLeaders(text)

	if len(leaders) != 1 {
		t.Fatalf("expected 1 leader, got %d: %+v", len(leaders), leaders)
	}

	if leaders[0].Email != "john.smith@firstnation.ca" {
		t.Errorf("leader[0].Email = %q, want %q", leaders[0].Email, "john.smith@firstnation.ca")
	}

	if leaders[0].Phone != "" {
		t.Errorf("leader[0].Phone = %q, want empty", leaders[0].Phone)
	}
}

func TestExtractLeaders_PhoneOnNearbyLine(t *testing.T) {
	text := `Chief
John Smith
Office: (807) 555-1234`

	leaders := leadership.ExtractLeaders(text)

	if len(leaders) != 1 {
		t.Fatalf("expected 1 leader, got %d: %+v", len(leaders), leaders)
	}

	if leaders[0].Phone != "(807) 555-1234" {
		t.Errorf("leader[0].Phone = %q, want %q", leaders[0].Phone, "(807) 555-1234")
	}

	if leaders[0].Email != "" {
		t.Errorf("leader[0].Email = %q, want empty", leaders[0].Email)
	}
}

func TestExtractLeaders_NoContactInfo(t *testing.T) {
	text := `Chief
John Smith

Councillors
Mary Johnson`

	leaders := leadership.ExtractLeaders(text)

	if len(leaders) != 2 {
		t.Fatalf("expected 2 leaders, got %d: %+v", len(leaders), leaders)
	}

	if leaders[0].Email != "" || leaders[0].Phone != "" {
		t.Errorf("leader[0] should have no contact info, got email=%q phone=%q",
			leaders[0].Email, leaders[0].Phone)
	}

	if leaders[1].Email != "" || leaders[1].Phone != "" {
		t.Errorf("leader[1] should have no contact info, got email=%q phone=%q",
			leaders[1].Email, leaders[1].Phone)
	}
}
