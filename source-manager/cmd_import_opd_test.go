package main

import (
	"testing"

	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
)

func TestApplyConsentPublicDisplayEnabled(t *testing.T) {
	entries := []models.DictionaryEntry{
		{Lemma: "makwa", ConsentPublicDisplay: false},
		{Lemma: "waawaashkeshi", ConsentPublicDisplay: false},
	}

	applyConsentPublicDisplay(entries, true)

	for i := range entries {
		if !entries[i].ConsentPublicDisplay {
			t.Fatalf("expected entry %d to have consent_public_display=true", i)
		}
	}
}

func TestApplyConsentPublicDisplayDisabled(t *testing.T) {
	entries := []models.DictionaryEntry{
		{Lemma: "makwa", ConsentPublicDisplay: false},
		{Lemma: "waawaashkeshi", ConsentPublicDisplay: true},
	}

	applyConsentPublicDisplay(entries, false)

	if entries[0].ConsentPublicDisplay {
		t.Fatal("expected disabled helper to leave false entry unchanged")
	}
	if !entries[1].ConsentPublicDisplay {
		t.Fatal("expected disabled helper to leave true entry unchanged")
	}
}
