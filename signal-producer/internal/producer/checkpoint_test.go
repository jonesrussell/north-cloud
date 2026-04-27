package producer_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/signal-producer/internal/producer"
)

// capturedLog is one Warn/Info/Error call recorded by the test logger.
type capturedLog struct {
	level   string
	message string
}

// captureLogger is a minimal infralogger.Logger fake that records Warn calls.
// It lets the corrupt-file test assert that a WARN was actually emitted.
type captureLogger struct {
	mu      sync.Mutex
	entries []capturedLog
}

func newCaptureLogger() *captureLogger {
	return &captureLogger{}
}

func (l *captureLogger) record(level, msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, capturedLog{level: level, message: msg})
}

func (l *captureLogger) Debug(msg string, _ ...infralogger.Field) { l.record("debug", msg) }
func (l *captureLogger) Info(msg string, _ ...infralogger.Field)  { l.record("info", msg) }
func (l *captureLogger) Warn(msg string, _ ...infralogger.Field)  { l.record("warn", msg) }
func (l *captureLogger) Error(msg string, _ ...infralogger.Field) { l.record("error", msg) }
func (l *captureLogger) Fatal(msg string, _ ...infralogger.Field) { l.record("fatal", msg) }
func (l *captureLogger) With(_ ...infralogger.Field) infralogger.Logger {
	return l
}
func (l *captureLogger) Sync() error { return nil }

func (l *captureLogger) hasLevel(level string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, e := range l.entries {
		if e.level == level {
			return true
		}
	}
	return false
}

// withinColdStartWindow asserts that ts falls inside the expected cold-start
// band: roughly DefaultColdStartLookback before "now", with a generous slack
// for slow CI machines.
func withinColdStartWindow(t *testing.T, ts time.Time) bool {
	t.Helper()
	now := time.Now()
	expected := now.Add(-producer.DefaultColdStartLookback)
	delta := ts.Sub(expected)
	if delta < 0 {
		delta = -delta
	}
	return delta < time.Minute
}

func TestLoadCheckpoint_Missing_DefaultsToColdStart(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.json")
	log := newCaptureLogger()

	cp, err := producer.LoadCheckpoint(path, log)
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if !withinColdStartWindow(t, cp.LastSuccessfulRun) {
		t.Fatalf("expected cold-start ~24h ago, got %v", cp.LastSuccessfulRun)
	}
	if cp.LastBatchSize != 0 {
		t.Fatalf("expected zero batch size on cold start, got %d", cp.LastBatchSize)
	}
	if log.hasLevel("warn") {
		t.Fatalf("missing file should not emit a WARN")
	}
}

func TestLoadCheckpoint_Corrupt_FallsBackWithWarn(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.json")
	if err := os.WriteFile(path, []byte("not-json"), 0o640); err != nil {
		t.Fatalf("seed corrupt file: %v", err)
	}
	log := newCaptureLogger()

	cp, err := producer.LoadCheckpoint(path, log)
	if err != nil {
		t.Fatalf("expected nil error on corrupt file, got %v", err)
	}
	if !withinColdStartWindow(t, cp.LastSuccessfulRun) {
		t.Fatalf("expected cold-start fallback, got %v", cp.LastSuccessfulRun)
	}
	if !log.hasLevel("warn") {
		t.Fatalf("expected WARN log on corrupt file")
	}
}

func TestLoadCheckpoint_InvalidValues_FallsBackWithWarn(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.json")
	// Negative batch size violates the data-model rule.
	body := `{"last_successful_run":"2026-04-27T05:30:00Z","last_batch_size":-5}`
	if err := os.WriteFile(path, []byte(body), 0o640); err != nil {
		t.Fatalf("seed invalid file: %v", err)
	}
	log := newCaptureLogger()

	cp, err := producer.LoadCheckpoint(path, log)
	if err != nil {
		t.Fatalf("expected nil error on invalid values, got %v", err)
	}
	if !withinColdStartWindow(t, cp.LastSuccessfulRun) {
		t.Fatalf("expected cold-start fallback, got %v", cp.LastSuccessfulRun)
	}
	if !log.hasLevel("warn") {
		t.Fatalf("expected WARN log on invalid values")
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.json")
	want := producer.Checkpoint{
		LastSuccessfulRun: time.Date(2026, 4, 27, 5, 30, 0, 0, time.UTC),
		LastBatchSize:     23,
	}
	if err := producer.SaveCheckpoint(path, want); err != nil {
		t.Fatalf("SaveCheckpoint: %v", err)
	}

	got, err := producer.LoadCheckpoint(path, infralogger.NewNop())
	if err != nil {
		t.Fatalf("LoadCheckpoint: %v", err)
	}
	if !got.LastSuccessfulRun.Equal(want.LastSuccessfulRun) {
		t.Errorf("LastSuccessfulRun mismatch: got %v, want %v",
			got.LastSuccessfulRun, want.LastSuccessfulRun)
	}
	if got.LastBatchSize != want.LastBatchSize {
		t.Errorf("LastBatchSize mismatch: got %d, want %d",
			got.LastBatchSize, want.LastBatchSize)
	}

	// Verify the on-disk JSON has the documented field names.
	raw, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read back: %v", readErr)
	}
	var asMap map[string]any
	if unmarshalErr := json.Unmarshal(raw, &asMap); unmarshalErr != nil {
		t.Fatalf("re-parse: %v", unmarshalErr)
	}
	if _, ok := asMap["last_successful_run"]; !ok {
		t.Error("missing last_successful_run field on disk")
	}
	if _, ok := asMap["last_batch_size"]; !ok {
		t.Error("missing last_batch_size field on disk")
	}
}

// TestSaveCheckpoint_AtomicOnFailure injects a rename failure by pre-creating
// the canonical path as a directory. The temp file is created and written
// successfully, then os.Rename(tmp, dir) fails. The test asserts:
//  1. SaveCheckpoint returns an error.
//  2. The canonical path is still a directory (i.e., not clobbered).
//  3. No `.tmp.` artifact remains in the parent directory (cleanup ran).
func TestSaveCheckpoint_AtomicOnFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	canonical := filepath.Join(dir, "checkpoint.json")
	// Pre-create canonical as a directory so the rename step fails.
	if err := os.Mkdir(canonical, 0o750); err != nil {
		t.Fatalf("seed canonical-as-dir: %v", err)
	}

	cp := producer.Checkpoint{
		LastSuccessfulRun: time.Now().UTC(),
		LastBatchSize:     1,
	}
	err := producer.SaveCheckpoint(canonical, cp)
	if err == nil {
		t.Fatal("expected SaveCheckpoint to fail when canonical path is a directory")
	}

	info, statErr := os.Stat(canonical)
	if statErr != nil {
		t.Fatalf("canonical path disappeared: %v", statErr)
	}
	if !info.IsDir() {
		t.Fatalf("canonical path should still be a directory after failed save")
	}

	entries, readDirErr := os.ReadDir(dir)
	if readDirErr != nil {
		t.Fatalf("readdir: %v", readDirErr)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp.") {
			t.Fatalf("tmp artifact left behind: %s", e.Name())
		}
	}
}

// TestSaveCheckpoint_FileMode verifies the on-disk file permission is exactly
// 0640 per C-010, regardless of umask. Skipped on Windows where Unix-style
// permission bits are not honored.
func TestSaveCheckpoint_FileMode(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not enforce Unix file mode bits")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.json")
	cp := producer.Checkpoint{
		LastSuccessfulRun: time.Now().UTC(),
		LastBatchSize:     7,
	}
	if err := producer.SaveCheckpoint(path, cp); err != nil {
		t.Fatalf("SaveCheckpoint: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != producer.CheckpointFileMode {
		t.Fatalf("file mode = %#o, want %#o", got, producer.CheckpointFileMode)
	}
}

func TestSaveCheckpoint_OpenError_ReturnsWrappedError(t *testing.T) {
	t.Parallel()
	// A path whose parent directory does not exist forces the OpenFile to fail
	// before any tmp file is created.
	bogus := filepath.Join(t.TempDir(), "no-such-dir", "checkpoint.json")
	err := producer.SaveCheckpoint(bogus, producer.Checkpoint{
		LastSuccessfulRun: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected error for non-existent parent directory")
	}
	if !strings.Contains(err.Error(), "checkpoint:") {
		t.Fatalf("error not wrapped with package prefix: %v", err)
	}
}

func TestLoadCheckpoint_PermissionDenied_ReturnsError(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not enforce Unix-style read permissions reliably")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.json")
	body := `{"last_successful_run":"2026-04-27T05:30:00Z","last_batch_size":1}`
	if err := os.WriteFile(path, []byte(body), 0o000); err != nil {
		t.Fatalf("seed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(path, 0o600)
	})

	if _, err := producer.LoadCheckpoint(path, infralogger.NewNop()); err == nil {
		t.Fatal("expected wrapped error for permission-denied open")
	}
}
