package runner_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jonesrussell/north-cloud/signal-crawler/internal/adapter"
	"github.com/jonesrussell/north-cloud/signal-crawler/internal/runner"
)

// fakeSource returns canned signals.
type fakeSource struct {
	name    string
	signals []adapter.Signal
	err     error
}

func (f *fakeSource) Name() string { return f.name }
func (f *fakeSource) Scan(_ context.Context) ([]adapter.Signal, error) {
	return f.signals, f.err
}

// fakeDedup is a map-backed Seen/Mark implementation.
type fakeDedup struct {
	seen map[string]bool
}

func newFakeDedup(preseeded ...string) *fakeDedup {
	d := &fakeDedup{seen: make(map[string]bool)}
	for _, k := range preseeded {
		d.seen[k] = true
	}
	return d
}

func (f *fakeDedup) Seen(source, externalID string) (bool, error) {
	return f.seen[source+":"+externalID], nil
}

func (f *fakeDedup) Mark(source, externalID string) error {
	f.seen[source+":"+externalID] = true
	return nil
}

func (f *fakeDedup) WasSeen(source, externalID string) bool {
	return f.seen[source+":"+externalID]
}

// fakeIngest records posted signals.
type fakeIngest struct {
	posted []adapter.Signal
}

func (f *fakeIngest) Post(sig adapter.Signal) error {
	f.posted = append(f.posted, sig)
	return nil
}

// TestRunner_ProcessesSignals: 2 signals from fakeSource, both ingested, both marked.
func TestRunner_ProcessesSignals(t *testing.T) {
	signals := []adapter.Signal{
		{Label: "Signal A", ExternalID: "ext-1"},
		{Label: "Signal B", ExternalID: "ext-2"},
	}
	src := &fakeSource{name: "test-source", signals: signals}
	dedup := newFakeDedup()
	ingest := &fakeIngest{}

	r := runner.New([]adapter.Source{src}, dedup, ingest, false)
	stats := r.Run(context.Background())

	if len(stats) != 1 {
		t.Fatalf("expected 1 stats entry, got %d", len(stats))
	}
	s := stats[0]
	if s.Scanned != 2 {
		t.Errorf("expected Scanned=2, got %d", s.Scanned)
	}
	if s.Ingested != 2 {
		t.Errorf("expected Ingested=2, got %d", s.Ingested)
	}
	if s.Skipped != 0 {
		t.Errorf("expected Skipped=0, got %d", s.Skipped)
	}
	if s.Errors != 0 {
		t.Errorf("expected Errors=0, got %d", s.Errors)
	}
	if len(ingest.posted) != 2 {
		t.Errorf("expected 2 posted signals, got %d", len(ingest.posted))
	}
	if !dedup.WasSeen("test-source", "ext-1") {
		t.Error("expected ext-1 to be marked in dedup")
	}
	if !dedup.WasSeen("test-source", "ext-2") {
		t.Error("expected ext-2 to be marked in dedup")
	}
}

// TestRunner_SkipsSeen: 1 already in fakeDedup, 1 new; only new one posted.
func TestRunner_SkipsSeen(t *testing.T) {
	signals := []adapter.Signal{
		{Label: "Old Signal", ExternalID: "ext-old"},
		{Label: "New Signal", ExternalID: "ext-new"},
	}
	src := &fakeSource{name: "test-source", signals: signals}
	dedup := newFakeDedup("test-source:ext-old")
	ingest := &fakeIngest{}

	r := runner.New([]adapter.Source{src}, dedup, ingest, false)
	stats := r.Run(context.Background())

	if len(stats) != 1 {
		t.Fatalf("expected 1 stats entry, got %d", len(stats))
	}
	s := stats[0]
	if s.Scanned != 2 {
		t.Errorf("expected Scanned=2, got %d", s.Scanned)
	}
	if s.Ingested != 1 {
		t.Errorf("expected Ingested=1, got %d", s.Ingested)
	}
	if s.Skipped != 1 {
		t.Errorf("expected Skipped=1, got %d", s.Skipped)
	}
	if len(ingest.posted) != 1 {
		t.Errorf("expected 1 posted signal, got %d", len(ingest.posted))
	}
	if ingest.posted[0].ExternalID != "ext-new" {
		t.Errorf("expected posted signal to be ext-new, got %s", ingest.posted[0].ExternalID)
	}
}

// TestRunner_DryRun: signals counted as ingested but NOT posted and NOT marked.
func TestRunner_DryRun(t *testing.T) {
	signals := []adapter.Signal{
		{Label: "Signal A", ExternalID: "ext-1"},
		{Label: "Signal B", ExternalID: "ext-2"},
	}
	src := &fakeSource{name: "test-source", signals: signals}
	dedup := newFakeDedup()
	ingest := &fakeIngest{}

	r := runner.New([]adapter.Source{src}, dedup, ingest, true)
	stats := r.Run(context.Background())

	if len(stats) != 1 {
		t.Fatalf("expected 1 stats entry, got %d", len(stats))
	}
	s := stats[0]
	if s.Ingested != 2 {
		t.Errorf("expected Ingested=2 (dry-run counts), got %d", s.Ingested)
	}
	if len(ingest.posted) != 0 {
		t.Errorf("expected 0 posted in dry-run, got %d", len(ingest.posted))
	}
	if dedup.WasSeen("test-source", "ext-1") {
		t.Error("expected ext-1 NOT marked in dry-run")
	}
	if dedup.WasSeen("test-source", "ext-2") {
		t.Error("expected ext-2 NOT marked in dry-run")
	}
}

// TestRunner_ScanError: scan error increments Errors, continues to next source.
func TestRunner_ScanError(t *testing.T) {
	src := &fakeSource{name: "bad-source", err: errors.New("network error")}
	dedup := newFakeDedup()
	ingest := &fakeIngest{}

	r := runner.New([]adapter.Source{src}, dedup, ingest, false)
	stats := r.Run(context.Background())

	if len(stats) != 1 {
		t.Fatalf("expected 1 stats entry, got %d", len(stats))
	}
	s := stats[0]
	if s.Errors != 1 {
		t.Errorf("expected Errors=1, got %d", s.Errors)
	}
	if s.Scanned != 0 {
		t.Errorf("expected Scanned=0 on scan error, got %d", s.Scanned)
	}
}
