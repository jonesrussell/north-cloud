package domain_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
)

func TestNewLifecycleEvent_StampsEventAt(t *testing.T) {
	t.Parallel()

	before := time.Now().UTC()
	a := fixtureAlert(t)
	ev := domain.NewLifecycleEvent(domain.EventCreated, a)
	after := time.Now().UTC()

	if ev.EventAt.Before(before) || ev.EventAt.After(after) {
		t.Errorf("EventAt %v not in expected range [%v, %v]", ev.EventAt, before, after)
	}
}

func TestNewLifecycleEvent_CopiesConvenienceFields(t *testing.T) {
	t.Parallel()

	a := fixtureAlert(t)
	ev := domain.NewLifecycleEvent(domain.EventCreated, a)

	if ev.AlertID != a.ID {
		t.Errorf("AlertID mismatch: got %q, want %q", ev.AlertID, a.ID)
	}

	if ev.Category != a.Category {
		t.Errorf("Category mismatch: got %q, want %q", ev.Category, a.Category)
	}

	if ev.Severity != a.Severity {
		t.Errorf("Severity mismatch: got %q, want %q", ev.Severity, a.Severity)
	}

	if len(ev.Scope) != len(a.Scope) {
		t.Errorf("Scope length mismatch: got %d, want %d", len(ev.Scope), len(a.Scope))
	}

	for i, s := range ev.Scope {
		if s != a.Scope[i] {
			t.Errorf("Scope[%d] mismatch: got %q, want %q", i, s, a.Scope[i])
		}
	}
}

func TestNewLifecycleEvent_PayloadMatchesAlert(t *testing.T) {
	t.Parallel()

	a := fixtureAlert(t)
	ev := domain.NewLifecycleEvent(domain.EventUpdated, a)

	if ev.Payload.ID != a.ID {
		t.Errorf("Payload.ID mismatch: got %q, want %q", ev.Payload.ID, a.ID)
	}

	if ev.Payload.Severity != a.Severity {
		t.Errorf("Payload.Severity mismatch: got %q, want %q", ev.Payload.Severity, a.Severity)
	}
}

func TestNewLifecycleEvent_EventTypeSet(t *testing.T) {
	t.Parallel()

	a := fixtureAlert(t)

	for _, et := range []domain.EventType{domain.EventCreated, domain.EventUpdated, domain.EventRescinded} {
		ev := domain.NewLifecycleEvent(et, a)
		if ev.EventType != et {
			t.Errorf("EventType mismatch: got %q, want %q", ev.EventType, et)
		}
	}
}

func TestLifecycleEvent_RoundTrip(t *testing.T) {
	t.Parallel()

	a := fixtureAlert(t)
	ev := domain.NewLifecycleEvent(domain.EventCreated, a)

	data, marshalErr := json.Marshal(ev)
	if marshalErr != nil {
		t.Fatalf("marshal failed: %v", marshalErr)
	}

	var decoded domain.LifecycleEvent
	if unmarshalErr := json.Unmarshal(data, &decoded); unmarshalErr != nil {
		t.Fatalf("unmarshal failed: %v", unmarshalErr)
	}

	if decoded.EventType != ev.EventType {
		t.Errorf("EventType mismatch: got %q, want %q", decoded.EventType, ev.EventType)
	}

	if decoded.AlertID != ev.AlertID {
		t.Errorf("AlertID mismatch: got %q, want %q", decoded.AlertID, ev.AlertID)
	}

	if decoded.Category != ev.Category {
		t.Errorf("Category mismatch: got %q, want %q", decoded.Category, ev.Category)
	}

	if decoded.Severity != ev.Severity {
		t.Errorf("Severity mismatch: got %q, want %q", decoded.Severity, ev.Severity)
	}

	if decoded.Payload.ID != ev.Payload.ID {
		t.Errorf("Payload.ID mismatch: got %q, want %q", decoded.Payload.ID, ev.Payload.ID)
	}
}

func TestEventTypeEnumCoverage(t *testing.T) {
	t.Parallel()

	types := []domain.EventType{
		domain.EventCreated,
		domain.EventUpdated,
		domain.EventRescinded,
	}

	for _, et := range types {
		if et == "" {
			t.Errorf("event type constant is empty string")
		}
	}
}

func TestAcquisitionStrategyEnumCoverage(t *testing.T) {
	t.Parallel()

	strategies := []domain.AcquisitionStrategy{
		domain.AcquisitionRSS,
		domain.AcquisitionAtom,
		domain.AcquisitionJSON,
		domain.AcquisitionHTML,
	}

	for _, s := range strategies {
		if s == "" {
			t.Errorf("acquisition strategy constant is empty string")
		}
	}
}

// fixtureAlertSource builds a valid AlertSource for tests.
func fixtureAlertSource(t *testing.T) domain.AlertSource {
	t.Helper()

	return domain.AlertSource{
		ID:                  "safersites",
		Name:                "Safer Sites Winnipeg",
		FeedURL:             "https://safersiteswinnipeg.ca/alerts.rss",
		AcquisitionStrategy: domain.AcquisitionRSS,
		PollInterval:        30 * time.Minute,
		DefaultCategory:     domain.CategoryHarmReduction,
		DefaultScope:        []string{"canada:manitoba:winnipeg"},
		DefaultExpiry:       30 * 24 * time.Hour,
		Enabled:             true,
	}
}

func TestAlertSource_Validate_Valid(t *testing.T) {
	t.Parallel()

	s := fixtureAlertSource(t)

	if validateErr := s.Validate(); validateErr != nil {
		t.Errorf("expected valid source to pass Validate(), got: %v", validateErr)
	}
}

func TestAlertSource_Validate_MaxPollInterval(t *testing.T) {
	t.Parallel()

	s := fixtureAlertSource(t)
	s.PollInterval = 60 * time.Minute

	if validateErr := s.Validate(); validateErr != nil {
		t.Errorf("expected 60m poll interval to pass Validate(), got: %v", validateErr)
	}
}

func TestAlertSource_Validate_PollIntervalTooShort(t *testing.T) {
	t.Parallel()

	s := fixtureAlertSource(t)
	s.PollInterval = 10 * time.Minute

	if validateErr := s.Validate(); validateErr == nil {
		t.Error("expected error for poll_interval < 30m, got nil")
	}
}

func TestAlertSource_Validate_PollIntervalTooLong(t *testing.T) {
	t.Parallel()

	s := fixtureAlertSource(t)
	s.PollInterval = 90 * time.Minute

	if validateErr := s.Validate(); validateErr == nil {
		t.Error("expected error for poll_interval > 60m, got nil")
	}
}

func TestAlertSource_Validate_EmptyID(t *testing.T) {
	t.Parallel()

	s := fixtureAlertSource(t)
	s.ID = ""

	if validateErr := s.Validate(); validateErr == nil {
		t.Error("expected error for empty source ID, got nil")
	}
}

func TestAlertSource_Validate_EmptyFeedURL(t *testing.T) {
	t.Parallel()

	s := fixtureAlertSource(t)
	s.FeedURL = ""

	if validateErr := s.Validate(); validateErr == nil {
		t.Error("expected error for empty feed_url, got nil")
	}
}
