---
work_package_id: WP02
title: Checkpoint Persistence
dependencies:
- WP01
requirement_refs:
- FR-003
- FR-004
- FR-005
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T006
- T007
- T008
- T009
agent: "claude:opus-4.7:reviewer:reviewer"
shell_pid: "8528"
history:
- event: created
  at: '2026-04-27T05:55:00Z'
  by: /spec-kitty.tasks
authoritative_surface: signal-producer/internal/producer/
execution_mode: code_change
mission_id: 01KQ6QZSNBQF3VW515AKM0SQ46
mission_slug: signal-producer-pipeline-01KQ6QZS
owned_files:
- signal-producer/internal/producer/checkpoint.go
- signal-producer/internal/producer/checkpoint_test.go
tags: []
---

# WP02 — Checkpoint Persistence

## Objective

Implement persistent run-to-run idempotence for the signal producer. Define the `Checkpoint` struct, load it safely (handling missing and corrupt files), and save it atomically so a crash mid-write cannot corrupt the file.

## Context

Read first:
- [spec.md](../spec.md) FR-003, FR-004, FR-005, NFR-005, C-010 (file mode 0640).
- [data-model.md](../data-model.md) "Checkpoint (persistent)" section — exact JSON shape and validation rules.
- [research.md](../research.md) D3 (checkpoint stays inside `internal/producer`).

Charter constraints: C-002 (golangci-lint clean), C-003 (`infrastructure/logger` for any logging), C-004 (no `os.Getenv` here).

## Branch Strategy

Planning base: `main`. Merge target: `main`. Lane workspace from `lanes.json`. Independent leaf WP — runs in parallel with WP03 and WP04 after WP01 lands.

## Subtask Guidance

### T006 — Define `Checkpoint` struct

**Purpose**: Establish the persistent shape and the package boundary.

**Steps**:

1. Create `signal-producer/internal/producer/checkpoint.go`. Package: `producer`.
2. Define:
   ```go
   // Checkpoint records the last successful signal-producer run.
   type Checkpoint struct {
       LastSuccessfulRun time.Time `json:"last_successful_run"`
       LastBatchSize     int       `json:"last_batch_size"`
   }
   ```
3. Define one named constant for the cold-start lookback (`DefaultColdStartLookback = 24 * time.Hour`) and one for file mode (`checkpointFileMode os.FileMode = 0640`). No magic numbers per C-002.

**Files**: `signal-producer/internal/producer/checkpoint.go`.

**Validation**:

- [ ] Struct has the two fields with correct JSON tags.
- [ ] Constants exist for the lookback and file mode.

### T007 — `LoadCheckpoint(path string) (Checkpoint, error)`

**Purpose**: Load the checkpoint from disk, defaulting safely on missing or corrupt files.

**Steps**:

1. If `os.Open(path)` returns `errors.Is(err, fs.ErrNotExist)`, return `Checkpoint{LastSuccessfulRun: time.Now().Add(-DefaultColdStartLookback)}, nil`. Caller logs this as a first-run signal (FR-004).
2. Other open errors: return them wrapped: `fmt.Errorf("checkpoint: open %s: %w", path, err)`.
3. Read and `json.Unmarshal` into a `Checkpoint`. If unmarshal fails: log a WARN with the path and the unmarshal error, then return the same 24-hour-ago default and a nil error. (Corrupt file is recoverable; the producer continues.) Use `infrastructure/logger` (per C-003) — accept a logger parameter so callers can inject.
4. Validate: `LastSuccessfulRun` must parse as a valid time; `LastBatchSize` must be ≥ 0. If either fails, fall back to the 24-hour default with a WARN log.

**Files**: `signal-producer/internal/producer/checkpoint.go` (extended).

**Signature**: `func LoadCheckpoint(path string, log infralogger.Logger) (Checkpoint, error)` — note the logger parameter; this lets tests inject a no-op logger.

**Validation**:

- [ ] Missing file → 24h-ago Checkpoint, nil error.
- [ ] Corrupt file → 24h-ago Checkpoint, nil error, WARN logged.
- [ ] Permission-denied → wrapped error returned (no recovery).
- [ ] Valid file → parsed Checkpoint returned.

### T008 — `SaveCheckpoint(path string, cp Checkpoint) error`

**Purpose**: Write the checkpoint to disk atomically (write tmp + fsync + rename) with mode 0640.

**Steps**:

1. Marshal `cp` to indented JSON.
2. Construct a temp path: `path + ".tmp." + strconv.Itoa(os.Getpid())`. Use `os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, checkpointFileMode)`.
3. Write the JSON bytes; on error, close the file and `os.Remove(tmpPath)` before returning the wrapped error.
4. `f.Sync()` to fsync to disk.
5. `f.Close()`.
6. `os.Rename(tmpPath, path)` for atomic replacement.
7. On any step failure, attempt `os.Remove(tmpPath)` (ignoring its error) so we don't litter the directory. Return the underlying error wrapped.

**Files**: `signal-producer/internal/producer/checkpoint.go` (extended).

**Validation**:

- [ ] Successful save: file exists at canonical path with mode 0640 and parses back as the same Checkpoint.
- [ ] Crash between write and rename leaves the canonical file unchanged.
- [ ] Error path cleans up the temp file.

### T009 — Unit tests in `checkpoint_test.go`

**Purpose**: ≥ 80% coverage on the checkpoint package per NFR-003. Cover happy path and the two interesting failure modes.

**Steps**:

1. Create `signal-producer/internal/producer/checkpoint_test.go`. Use `t.TempDir()` for isolated workspace per test. Every helper starts with `t.Helper()` per C-002.
2. Tests:
   - `TestLoadCheckpoint_Missing_DefaultsToColdStart`: no file → 24h-ago, nil error.
   - `TestLoadCheckpoint_Corrupt_FallsBackWithWarn`: write garbage to file → 24h-ago, nil error, captured logger received a WARN.
   - `TestSaveLoadRoundtrip`: save a Checkpoint with a known timestamp and batch size, load it back, expect equality.
   - `TestSaveCheckpoint_AtomicOnFailure`: simulate a write failure (e.g., wrap the open with a directory whose permissions are 0500); assert the canonical file is unchanged AND no `.tmp.` artifact remains.
   - `TestSaveCheckpoint_FileMode`: after save, `os.Stat` reports `Mode().Perm() == 0640`.
3. Use a no-op or capturing logger for tests — make a small fake or use the existing test helper from `infrastructure/logger`.

**Files**: `signal-producer/internal/producer/checkpoint_test.go`.

**Validation**:

- [ ] `task test:signal-producer` passes.
- [ ] `task test:cover` shows ≥ 80% coverage on `internal/producer/checkpoint.go`.
- [ ] All test helpers call `t.Helper()`.

## Definition of Done

- [ ] All four subtasks complete with their validation checklists ticked.
- [ ] `task lint:signal-producer` and `task test:signal-producer` green.
- [ ] Coverage on `checkpoint.go` ≥ 80%.
- [ ] No files modified outside `owned_files`.
- [ ] Atomic-write fault-injection test exists and passes (NFR-005).

## Reviewer Guidance

1. **Atomicity is real, not just a comment**. Open `checkpoint.go` and trace the save flow: temp path, fsync, rename, cleanup. Reject if any of those is missing or if the temp path is predictable enough to collide with a parallel run (PID suffix is the bare minimum).
2. **Permission mode is enforced by the producer, not relying on umask**. The `os.OpenFile` call MUST pass `0640` explicitly (not `0644` or `0666`).
3. **Corrupt-file recovery is logged**, not silent. Confirm a WARN log line is emitted with the path and parse error.
4. **Tests use `t.TempDir()`**, not hard-coded paths. Reject anything that writes to `/tmp/` directly.
5. **Coverage**: run `go test -cover ./internal/producer/...` and confirm ≥ 80% for `checkpoint.go`.

## Risks and Mitigations

| Risk                                                                  | Mitigation                                                                                                              |
| --------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| `os.Rename` is not atomic on Windows for cross-device renames.        | Producer runs on Linux only; not a real concern. Comment in code if it confuses future readers.                         |
| Logger interface drift — `infrastructure/logger` evolving.           | Take a logger by interface, not struct. Test with a fake.                                                              |
| Temp-file PID suffix collides under heavy concurrent test parallelism. | Use `t.TempDir()` per test (separate dirs); collision impossible.                                                      |

## Implementation Command

```bash
spec-kitty agent action implement WP02 --agent <agent-name> --mission signal-producer-pipeline-01KQ6QZS
```

## Activity Log

- 2026-04-27T06:33:01Z – claude:opus-4.7:implementer:implementer – shell_pid=15216 – Started implementation via action command
- 2026-04-27T06:36:57Z – claude:opus-4.7:implementer:implementer – shell_pid=15216 – Checkpoint persistence ready for review
- 2026-04-27T06:37:43Z – claude:opus-4.7:reviewer:reviewer – shell_pid=8528 – Started review via action command
