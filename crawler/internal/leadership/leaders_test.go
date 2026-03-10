package leadership_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/leadership"
)

func TestExtractLeaders(t *testing.T) {
	t.Helper()

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
	t.Helper()

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
	t.Helper()

	leaders := leadership.ExtractLeaders("No leadership info here, just some text about programs.")

	if len(leaders) != 0 {
		t.Errorf("expected 0 leaders, got %d: %+v", len(leaders), leaders)
	}
}
