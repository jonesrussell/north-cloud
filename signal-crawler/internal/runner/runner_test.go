package runner_test

import (
	"context"
	"errors"
	"testing"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/signal-crawler/internal/adapter"
	"github.com/jonesrussell/north-cloud/signal-crawler/internal/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSource struct {
	name    string
	signals []adapter.Signal
	err     error
}

func (f *fakeSource) Name() string { return f.name }
func (f *fakeSource) Scan(_ context.Context) ([]adapter.Signal, error) {
	return f.signals, f.err
}

type fakeDedup struct {
	seen    map[string]bool
	seenErr error
	markErr error
}

func newFakeDedup(preseeded ...string) *fakeDedup {
	d := &fakeDedup{seen: make(map[string]bool)}
	for _, k := range preseeded {
		d.seen[k] = true
	}
	return d
}

func (f *fakeDedup) Seen(_ context.Context, source, externalID string) (bool, error) {
	if f.seenErr != nil {
		return false, f.seenErr
	}
	return f.seen[source+":"+externalID], nil
}

func (f *fakeDedup) Mark(_ context.Context, source, externalID string) error {
	if f.markErr != nil {
		return f.markErr
	}
	f.seen[source+":"+externalID] = true
	return nil
}

func (f *fakeDedup) WasSeen(source, externalID string) bool {
	return f.seen[source+":"+externalID]
}

type fakeIngest struct {
	posted []adapter.Signal
	err    error
}

func (f *fakeIngest) Post(_ context.Context, sig adapter.Signal) error {
	if f.err != nil {
		return f.err
	}
	f.posted = append(f.posted, sig)
	return nil
}

func TestRunner_ProcessesSignals(t *testing.T) {
	signals := []adapter.Signal{
		{Label: "Signal A", ExternalID: "ext-1"},
		{Label: "Signal B", ExternalID: "ext-2"},
	}
	src := &fakeSource{name: "test-source", signals: signals}
	dd := newFakeDedup()
	ing := &fakeIngest{}

	r := runner.New([]adapter.Source{src}, dd, ing, false, infralogger.NewNop())
	stats := r.Run(context.Background())

	require.Len(t, stats, 1)
	s := stats[0]
	assert.Equal(t, 2, s.Scanned)
	assert.Equal(t, 2, s.Ingested)
	assert.Equal(t, 0, s.Skipped)
	assert.Equal(t, 0, s.Errors)
	assert.Len(t, ing.posted, 2)
	assert.True(t, dd.WasSeen("test-source", "ext-1"))
	assert.True(t, dd.WasSeen("test-source", "ext-2"))
}

func TestRunner_SkipsSeen(t *testing.T) {
	signals := []adapter.Signal{
		{Label: "Old Signal", ExternalID: "ext-old"},
		{Label: "New Signal", ExternalID: "ext-new"},
	}
	src := &fakeSource{name: "test-source", signals: signals}
	dd := newFakeDedup("test-source:ext-old")
	ing := &fakeIngest{}

	r := runner.New([]adapter.Source{src}, dd, ing, false, infralogger.NewNop())
	stats := r.Run(context.Background())

	require.Len(t, stats, 1)
	s := stats[0]
	assert.Equal(t, 2, s.Scanned)
	assert.Equal(t, 1, s.Ingested)
	assert.Equal(t, 1, s.Skipped)
	require.Len(t, ing.posted, 1)
	assert.Equal(t, "ext-new", ing.posted[0].ExternalID)
}

func TestRunner_DryRun(t *testing.T) {
	signals := []adapter.Signal{
		{Label: "Signal A", ExternalID: "ext-1"},
		{Label: "Signal B", ExternalID: "ext-2"},
	}
	src := &fakeSource{name: "test-source", signals: signals}
	dd := newFakeDedup()
	ing := &fakeIngest{}

	r := runner.New([]adapter.Source{src}, dd, ing, true, infralogger.NewNop())
	stats := r.Run(context.Background())

	require.Len(t, stats, 1)
	s := stats[0]
	assert.Equal(t, 2, s.Ingested, "dry-run should count as ingested")
	assert.Empty(t, ing.posted, "dry-run should not POST")
	assert.False(t, dd.WasSeen("test-source", "ext-1"), "dry-run should not mark")
	assert.False(t, dd.WasSeen("test-source", "ext-2"), "dry-run should not mark")
}

func TestRunner_ScanError(t *testing.T) {
	src := &fakeSource{name: "bad-source", err: errors.New("network error")}
	dd := newFakeDedup()
	ing := &fakeIngest{}

	r := runner.New([]adapter.Source{src}, dd, ing, false, infralogger.NewNop())
	stats := r.Run(context.Background())

	require.Len(t, stats, 1)
	s := stats[0]
	assert.Equal(t, 1, s.Errors)
	assert.Equal(t, 0, s.Scanned)
}

func TestRunner_IngestError(t *testing.T) {
	signals := []adapter.Signal{
		{Label: "Signal A", ExternalID: "ext-1"},
		{Label: "Signal B", ExternalID: "ext-2"},
	}
	src := &fakeSource{name: "test-source", signals: signals}
	dd := newFakeDedup()
	ing := &fakeIngest{err: errors.New("server down")}

	r := runner.New([]adapter.Source{src}, dd, ing, false, infralogger.NewNop())
	stats := r.Run(context.Background())

	require.Len(t, stats, 1)
	s := stats[0]
	assert.Equal(t, 2, s.Scanned)
	assert.Equal(t, 0, s.Ingested, "failed ingests should not count as ingested")
	assert.Equal(t, 2, s.Errors)
	assert.False(t, dd.WasSeen("test-source", "ext-1"), "failed ingest should not mark")
}

func TestRunner_MarkError(t *testing.T) {
	signals := []adapter.Signal{
		{Label: "Signal A", ExternalID: "ext-1"},
	}
	src := &fakeSource{name: "test-source", signals: signals}
	dd := newFakeDedup()
	dd.markErr = errors.New("disk full")
	ing := &fakeIngest{}

	r := runner.New([]adapter.Source{src}, dd, ing, false, infralogger.NewNop())
	stats := r.Run(context.Background())

	require.Len(t, stats, 1)
	s := stats[0]
	assert.Equal(t, 1, s.Errors)
	assert.Equal(t, 0, s.Ingested, "mark failure should not count as ingested")
	assert.Len(t, ing.posted, 1, "signal was posted before mark failed")
}

func TestRunner_MultipleSources(t *testing.T) {
	src1 := &fakeSource{name: "hn", signals: []adapter.Signal{
		{Label: "HN Signal", ExternalID: "hn-1"},
	}}
	src2 := &fakeSource{name: "funding", signals: []adapter.Signal{
		{Label: "Grant Signal", ExternalID: "fund-1"},
		{Label: "Grant Signal 2", ExternalID: "fund-2"},
	}}
	dd := newFakeDedup()
	ing := &fakeIngest{}

	r := runner.New([]adapter.Source{src1, src2}, dd, ing, false, infralogger.NewNop())
	stats := r.Run(context.Background())

	require.Len(t, stats, 2)
	assert.Equal(t, "hn", stats[0].Source)
	assert.Equal(t, 1, stats[0].Ingested)
	assert.Equal(t, "funding", stats[1].Source)
	assert.Equal(t, 2, stats[1].Ingested)
	assert.Len(t, ing.posted, 3)
}
