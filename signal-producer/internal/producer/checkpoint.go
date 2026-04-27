// Package producer implements the signal-producer's run-time pieces:
// checkpoint persistence (this file), and the orchestration that follows.
package producer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"time"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// DefaultColdStartLookback is the lookback window applied when no checkpoint
// exists or the on-disk checkpoint cannot be parsed. FR-004.
const DefaultColdStartLookback = 24 * time.Hour

// checkpointFileMode is the permission mask the producer enforces on the
// checkpoint file regardless of umask. C-010.
const checkpointFileMode os.FileMode = 0o640

// Checkpoint records the last successful signal-producer run.
//
// Persisted as JSON at the path configured by the operator (default
// /var/lib/signal-producer/checkpoint.json). The file is written atomically:
// see SaveCheckpoint.
type Checkpoint struct {
	LastSuccessfulRun time.Time `json:"last_successful_run"`
	LastBatchSize     int       `json:"last_batch_size"`
	// ConsecutiveEmpty counts consecutive runs with zero ES hits. When it
	// reaches sourceDownThreshold the producer emits a single WARN with a
	// stable code. Old checkpoint files without this field deserialize to
	// zero, which is safe — the threshold simply restarts.
	ConsecutiveEmpty int `json:"consecutive_empty"`
}

// coldStart returns a Checkpoint pinned to DefaultColdStartLookback before now.
// Used both for missing-file and corrupt-file recovery.
func coldStart() Checkpoint {
	return Checkpoint{
		LastSuccessfulRun: time.Now().Add(-DefaultColdStartLookback),
	}
}

// LoadCheckpoint reads the checkpoint at path.
//
// Behavior:
//   - File missing: returns a 24h-ago cold-start Checkpoint and a nil error.
//   - File present but unreadable (e.g. permission denied): returns a wrapped
//     error so the caller can decide to fail the run.
//   - File present but invalid JSON / fails validation: logs WARN, returns a
//     24h-ago cold-start Checkpoint and a nil error so the producer can
//     continue (FR-004 corrupt-file recovery).
//   - File present and valid: returns the parsed Checkpoint.
//
// The logger parameter lets callers (and tests) inject a fake.
func LoadCheckpoint(path string, log infralogger.Logger) (Checkpoint, error) {
	data, err := os.ReadFile(path) //nolint:gosec // operator-controlled path
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return coldStart(), nil
		}
		return Checkpoint{}, fmt.Errorf("checkpoint: open %s: %w", path, err)
	}

	var cp Checkpoint
	if unmarshalErr := json.Unmarshal(data, &cp); unmarshalErr != nil {
		log.Warn(
			"checkpoint file is corrupt; falling back to cold-start lookback",
			infralogger.String("path", path),
			infralogger.Error(unmarshalErr),
		)
		return coldStart(), nil
	}

	if !isValidCheckpoint(cp) {
		log.Warn(
			"checkpoint file failed validation; falling back to cold-start lookback",
			infralogger.String("path", path),
		)
		return coldStart(), nil
	}

	return cp, nil
}

// isValidCheckpoint enforces the data-model rules: timestamp must be set,
// batch size must be non-negative.
func isValidCheckpoint(cp Checkpoint) bool {
	if cp.LastSuccessfulRun.IsZero() {
		return false
	}
	if cp.LastBatchSize < 0 {
		return false
	}
	return true
}

// SaveCheckpoint persists cp to path atomically.
//
// The write sequence is: marshal -> open temp file at "<path>.tmp.<pid>" with
// mode 0640 -> write bytes -> fsync -> close -> rename. On any failure after
// the temp file is opened, the temp file is removed before returning the
// wrapped underlying error so the directory is not littered with stale tmp
// files. This satisfies NFR-005 (crash-safe writes).
func SaveCheckpoint(path string, cp Checkpoint) error {
	payload, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return fmt.Errorf("checkpoint: marshal: %w", err)
	}

	tmpPath := path + ".tmp." + strconv.Itoa(os.Getpid())
	return writeAtomic(path, tmpPath, payload)
}

// writeAtomic performs the open/write/fsync/close/rename dance with cleanup on
// any error. Split out from SaveCheckpoint to keep cognitive complexity low.
func writeAtomic(canonicalPath, tmpPath string, payload []byte) error {
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, checkpointFileMode)
	if err != nil {
		return fmt.Errorf("checkpoint: create tmp %s: %w", tmpPath, err)
	}

	if _, writeErr := f.Write(payload); writeErr != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("checkpoint: write tmp %s: %w", tmpPath, writeErr)
	}

	if syncErr := f.Sync(); syncErr != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("checkpoint: fsync tmp %s: %w", tmpPath, syncErr)
	}

	if closeErr := f.Close(); closeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("checkpoint: close tmp %s: %w", tmpPath, closeErr)
	}

	if renameErr := os.Rename(tmpPath, canonicalPath); renameErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("checkpoint: rename %s -> %s: %w", tmpPath, canonicalPath, renameErr)
	}

	return nil
}
