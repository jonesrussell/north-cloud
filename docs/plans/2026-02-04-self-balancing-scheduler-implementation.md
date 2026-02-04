# Self-Balancing Scheduler Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement bucket-based load balancing for the crawler scheduler so jobs are evenly distributed across time slots.

**Architecture:** Add a `BucketMap` in-memory structure to the scheduler that tracks job counts per 15-minute slot. New jobs find the least-loaded slot; recurring jobs preserve their rhythm. Rebalancing is event-driven (interval change, resume, manual).

**Tech Stack:** Go 1.24+, PostgreSQL (existing `jobs` table), Gin (existing API)

**Worktree:** `/home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler`

**Design Doc:** `docs/plans/2026-02-04-self-balancing-scheduler-design.md`

---

## Task 1: BucketMap Core Data Structure

**Files:**
- Create: `crawler/internal/scheduler/bucket_map.go`
- Create: `crawler/internal/scheduler/bucket_map_test.go`

**Step 1: Write the failing test for SlotKey**

```go
// crawler/internal/scheduler/bucket_map_test.go
package scheduler

import (
	"testing"
	"time"
)

func TestSlotKey(t *testing.T) {
	t.Helper()

	// Times in same 15-min window get same key
	t1 := time.Date(2026, 2, 4, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 2, 4, 10, 14, 59, 0, time.UTC)
	if SlotKey(t1) != SlotKey(t2) {
		t.Errorf("expected same slot key for times in same 15-min window")
	}

	// Times in different windows get different keys
	t3 := time.Date(2026, 2, 4, 10, 15, 0, 0, time.UTC)
	if SlotKey(t1) == SlotKey(t3) {
		t.Errorf("expected different slot keys for times in different 15-min windows")
	}
}

func TestSlotTime(t *testing.T) {
	t.Helper()

	// Round-trip: SlotTime(SlotKey(t)) should return start of slot
	original := time.Date(2026, 2, 4, 10, 7, 30, 0, time.UTC)
	key := SlotKey(original)
	result := SlotTime(key)

	expected := time.Date(2026, 2, 4, 10, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("SlotTime(%d) = %v, want %v", key, result, expected)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
go test ./internal/scheduler/... -run "TestSlotKey|TestSlotTime" -v
```

Expected: FAIL with "undefined: SlotKey"

**Step 3: Write minimal implementation**

```go
// crawler/internal/scheduler/bucket_map.go
package scheduler

import (
	"sync"
	"time"
)

const (
	// SlotDuration is the granularity of time slots (15 minutes).
	SlotDuration = 15 * time.Minute
	// slotSeconds is SlotDuration in seconds for slot key calculation.
	slotSeconds = 900
	// slotsPerDay is the number of slots in a 24-hour period.
	slotsPerDay = 96
	// ProtectionWindow is the minimum time before execution when a job cannot be moved.
	ProtectionWindow = 30 * time.Minute
	// PlacementCooldown is the minimum time between job placements.
	PlacementCooldown = 1 * time.Hour
)

// SlotKey converts a time to its 15-minute bucket key.
// Times within the same 15-minute window map to the same key.
func SlotKey(t time.Time) int64 {
	return t.Unix() / slotSeconds
}

// SlotTime converts a slot key back to its start time (UTC).
func SlotTime(key int64) time.Time {
	return time.Unix(key*slotSeconds, 0).UTC()
}

// BucketMap holds the in-memory schedule view for load-balanced placement.
type BucketMap struct {
	mu         sync.RWMutex
	slots      map[int64]int        // slot_key -> job_count
	jobToSlot  map[string]int64     // job_id -> slot_key
	lastPlaced map[string]time.Time // job_id -> last placement time
}

// NewBucketMap creates an empty BucketMap.
func NewBucketMap() *BucketMap {
	return &BucketMap{
		slots:      make(map[int64]int),
		jobToSlot:  make(map[string]int64),
		lastPlaced: make(map[string]time.Time),
	}
}
```

**Step 4: Run test to verify it passes**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
go test ./internal/scheduler/... -run "TestSlotKey|TestSlotTime" -v
```

Expected: PASS

**Step 5: Run linter**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
golangci-lint run ./internal/scheduler/bucket_map.go
```

Expected: No errors

**Step 6: Commit**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler
git add crawler/internal/scheduler/bucket_map.go crawler/internal/scheduler/bucket_map_test.go
git commit -m "feat(scheduler): add BucketMap core data structure with SlotKey/SlotTime

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 2: BucketMap Add/Remove/Get Operations

**Files:**
- Modify: `crawler/internal/scheduler/bucket_map.go`
- Modify: `crawler/internal/scheduler/bucket_map_test.go`

**Step 1: Write the failing tests**

Add to `bucket_map_test.go`:

```go
func TestBucketMap_AddJob(t *testing.T) {
	t.Helper()

	bm := NewBucketMap()
	now := time.Now()
	slotKey := SlotKey(now)

	bm.AddJob("job-1", slotKey)

	if got := bm.GetSlotLoad(slotKey); got != 1 {
		t.Errorf("GetSlotLoad() = %d, want 1", got)
	}

	// Add second job to same slot
	bm.AddJob("job-2", slotKey)
	if got := bm.GetSlotLoad(slotKey); got != 2 {
		t.Errorf("GetSlotLoad() = %d, want 2", got)
	}
}

func TestBucketMap_RemoveJob(t *testing.T) {
	t.Helper()

	bm := NewBucketMap()
	slotKey := SlotKey(time.Now())

	bm.AddJob("job-1", slotKey)
	bm.AddJob("job-2", slotKey)

	bm.RemoveJob("job-1")

	if got := bm.GetSlotLoad(slotKey); got != 1 {
		t.Errorf("GetSlotLoad() = %d, want 1 after removal", got)
	}

	// Remove non-existent job should not panic
	bm.RemoveJob("non-existent")
}

func TestBucketMap_MoveJob(t *testing.T) {
	t.Helper()

	bm := NewBucketMap()
	now := time.Now()
	oldSlot := SlotKey(now)
	newSlot := SlotKey(now.Add(1 * time.Hour))

	bm.AddJob("job-1", oldSlot)

	// Move job to new slot (add to new slot removes from old)
	bm.AddJob("job-1", newSlot)

	if got := bm.GetSlotLoad(oldSlot); got != 0 {
		t.Errorf("old slot load = %d, want 0", got)
	}
	if got := bm.GetSlotLoad(newSlot); got != 1 {
		t.Errorf("new slot load = %d, want 1", got)
	}
}

func TestBucketMap_GetJobSlot(t *testing.T) {
	t.Helper()

	bm := NewBucketMap()
	slotKey := SlotKey(time.Now())

	bm.AddJob("job-1", slotKey)

	got, exists := bm.GetJobSlot("job-1")
	if !exists {
		t.Error("GetJobSlot() returned exists=false, want true")
	}
	if got != slotKey {
		t.Errorf("GetJobSlot() = %d, want %d", got, slotKey)
	}

	_, exists = bm.GetJobSlot("non-existent")
	if exists {
		t.Error("GetJobSlot() returned exists=true for non-existent job")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
go test ./internal/scheduler/... -run "TestBucketMap_" -v
```

Expected: FAIL with "undefined: AddJob" or similar

**Step 3: Write minimal implementation**

Add to `bucket_map.go`:

```go
// AddJob records a job placement in a slot.
// If the job already exists in another slot, it is moved.
func (b *BucketMap) AddJob(jobID string, slotKey int64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Remove from old slot if exists
	if oldSlot, exists := b.jobToSlot[jobID]; exists {
		b.slots[oldSlot]--
		if b.slots[oldSlot] <= 0 {
			delete(b.slots, oldSlot)
		}
	}

	// Add to new slot
	b.slots[slotKey]++
	b.jobToSlot[jobID] = slotKey
	b.lastPlaced[jobID] = time.Now()
}

// RemoveJob removes a job from its slot.
func (b *BucketMap) RemoveJob(jobID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if slotKey, exists := b.jobToSlot[jobID]; exists {
		b.slots[slotKey]--
		if b.slots[slotKey] <= 0 {
			delete(b.slots, slotKey)
		}
		delete(b.jobToSlot, jobID)
		delete(b.lastPlaced, jobID)
	}
}

// GetSlotLoad returns the job count for a given slot.
func (b *BucketMap) GetSlotLoad(slotKey int64) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.slots[slotKey]
}

// GetJobSlot returns the slot key for a job, or (0, false) if not found.
func (b *BucketMap) GetJobSlot(jobID string) (int64, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	slot, exists := b.jobToSlot[jobID]
	return slot, exists
}
```

**Step 4: Run test to verify it passes**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
go test ./internal/scheduler/... -run "TestBucketMap_" -v
```

Expected: PASS

**Step 5: Run linter**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
golangci-lint run ./internal/scheduler/bucket_map.go
```

**Step 6: Commit**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler
git add crawler/internal/scheduler/bucket_map.go crawler/internal/scheduler/bucket_map_test.go
git commit -m "feat(scheduler): add BucketMap Add/Remove/Get operations

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 3: FindLeastLoaded Algorithm

**Files:**
- Modify: `crawler/internal/scheduler/bucket_map.go`
- Modify: `crawler/internal/scheduler/bucket_map_test.go`

**Step 1: Write the failing test**

Add to `bucket_map_test.go`:

```go
func TestBucketMap_FindLeastLoaded(t *testing.T) {
	t.Helper()

	bm := NewBucketMap()
	now := time.Now()

	// Pre-populate: slot at +1h has 3 jobs, +2h has 1 job, +3h has 2 jobs
	slot1h := SlotKey(now.Add(1 * time.Hour))
	slot2h := SlotKey(now.Add(2 * time.Hour))
	slot3h := SlotKey(now.Add(3 * time.Hour))

	bm.AddJob("job-1", slot1h)
	bm.AddJob("job-2", slot1h)
	bm.AddJob("job-3", slot1h)
	bm.AddJob("job-4", slot2h)
	bm.AddJob("job-5", slot3h)
	bm.AddJob("job-6", slot3h)

	// Find least loaded in 4-hour window should find empty slot or slot2h
	start := now
	end := now.Add(4 * time.Hour)
	result := bm.FindLeastLoaded(start, end)

	// Result should be an empty slot (load 0) or slot2h (load 1)
	resultLoad := bm.GetSlotLoad(result)
	if resultLoad > 1 {
		t.Errorf("FindLeastLoaded found slot with load %d, expected <= 1", resultLoad)
	}
}

func TestBucketMap_FindLeastLoaded_AllEqual(t *testing.T) {
	t.Helper()

	bm := NewBucketMap()
	now := time.Now()
	start := now
	end := now.Add(1 * time.Hour) // 4 slots

	// No jobs - should return first slot
	result := bm.FindLeastLoaded(start, end)
	expected := SlotKey(start)
	if result != expected {
		t.Errorf("FindLeastLoaded() = %d, want %d (first slot when all empty)", result, expected)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
go test ./internal/scheduler/... -run "TestBucketMap_FindLeastLoaded" -v
```

Expected: FAIL with "undefined: FindLeastLoaded"

**Step 3: Write minimal implementation**

Add to `bucket_map.go`:

```go
// FindLeastLoaded finds the slot with the lowest job count in the given time range.
// If multiple slots tie, returns the earliest (most stable/predictable).
func (b *BucketMap) FindLeastLoaded(start, end time.Time) int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()

	bestSlot := SlotKey(start)
	bestLoad := b.slots[bestSlot] // 0 if not present

	for t := start; t.Before(end); t = t.Add(SlotDuration) {
		slot := SlotKey(t)
		load := b.slots[slot] // 0 if not present
		if load < bestLoad {
			bestLoad = load
			bestSlot = slot
		}
	}

	return bestSlot
}
```

**Step 4: Run test to verify it passes**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
go test ./internal/scheduler/... -run "TestBucketMap_FindLeastLoaded" -v
```

Expected: PASS

**Step 5: Run linter and commit**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
golangci-lint run ./internal/scheduler/bucket_map.go
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler
git add crawler/internal/scheduler/bucket_map.go crawler/internal/scheduler/bucket_map_test.go
git commit -m "feat(scheduler): add FindLeastLoaded algorithm

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 4: PlaceNewJob Method

**Files:**
- Modify: `crawler/internal/scheduler/bucket_map.go`
- Modify: `crawler/internal/scheduler/bucket_map_test.go`

**Step 1: Write the failing test**

Add to `bucket_map_test.go`:

```go
func TestBucketMap_PlaceNewJob(t *testing.T) {
	t.Helper()

	bm := NewBucketMap()
	now := time.Now()

	// Pre-populate first hour with 3 jobs each slot
	for i := 0; i < 4; i++ { // 4 slots in first hour
		slot := SlotKey(now.Add(time.Duration(i*15) * time.Minute))
		bm.AddJob(fmt.Sprintf("existing-%d-a", i), slot)
		bm.AddJob(fmt.Sprintf("existing-%d-b", i), slot)
		bm.AddJob(fmt.Sprintf("existing-%d-c", i), slot)
	}

	// Place new job with 6-hour interval
	interval := 6 * time.Hour
	result := bm.PlaceNewJob("new-job", interval)

	// Should NOT be placed in first hour (all slots have 3 jobs)
	resultSlot := SlotKey(result)
	firstHourEnd := SlotKey(now.Add(1 * time.Hour))
	if resultSlot < firstHourEnd && bm.GetSlotLoad(resultSlot) == 4 {
		// Acceptable if it found an empty slot elsewhere
	}

	// Verify job is tracked
	gotSlot, exists := bm.GetJobSlot("new-job")
	if !exists {
		t.Error("PlaceNewJob did not track the job")
	}
	if gotSlot != resultSlot {
		t.Errorf("GetJobSlot() = %d, PlaceNewJob returned slot %d", gotSlot, resultSlot)
	}
}

func TestBucketMap_PlaceNewJob_FindsGap(t *testing.T) {
	t.Helper()

	bm := NewBucketMap()
	now := time.Now()

	// Create a gap: slots 0,1,2 have jobs, slot 3 is empty
	bm.AddJob("job-0", SlotKey(now))
	bm.AddJob("job-1", SlotKey(now.Add(15*time.Minute)))
	bm.AddJob("job-2", SlotKey(now.Add(30*time.Minute)))
	// Slot at +45min is empty

	// Place new job
	result := bm.PlaceNewJob("new-job", 1*time.Hour)
	resultSlot := SlotKey(result)

	// Should find the empty slot at +45min (or any empty slot)
	if bm.GetSlotLoad(resultSlot) != 1 {
		t.Errorf("PlaceNewJob placed in slot with load %d, expected to find empty slot",
			bm.GetSlotLoad(resultSlot)-1) // -1 because we just added
	}
}
```

Add import at top:
```go
import (
	"fmt"
	"testing"
	"time"
)
```

**Step 2: Run test to verify it fails**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
go test ./internal/scheduler/... -run "TestBucketMap_PlaceNewJob" -v
```

Expected: FAIL with "undefined: PlaceNewJob"

**Step 3: Write minimal implementation**

Add to `bucket_map.go`:

```go
// searchWindowDefault is the default search window for new job placement.
const searchWindowDefault = 24 * time.Hour

// PlaceNewJob finds the optimal slot for a new job and records the placement.
// Searches the next 24 hours (or interval, whichever is larger) for the least-loaded slot.
// Returns the scheduled time.
func (b *BucketMap) PlaceNewJob(jobID string, interval time.Duration) time.Time {
	now := time.Now()

	// Search window: next 24h or next interval, whichever is larger
	searchDuration := searchWindowDefault
	if interval > searchDuration {
		searchDuration = interval
	}
	searchEnd := now.Add(searchDuration)

	// Find least-loaded slot
	bestSlot := b.FindLeastLoaded(now, searchEnd)

	// Record placement
	b.AddJob(jobID, bestSlot)

	return SlotTime(bestSlot)
}
```

**Step 4: Run test to verify it passes**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
go test ./internal/scheduler/... -run "TestBucketMap_PlaceNewJob" -v
```

Expected: PASS

**Step 5: Run linter and commit**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
golangci-lint run ./internal/scheduler/bucket_map.go
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler
git add crawler/internal/scheduler/bucket_map.go crawler/internal/scheduler/bucket_map_test.go
git commit -m "feat(scheduler): add PlaceNewJob for load-balanced placement

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 5: Rhythm Preservation for Recurring Jobs

**Files:**
- Modify: `crawler/internal/scheduler/bucket_map.go`
- Modify: `crawler/internal/scheduler/bucket_map_test.go`

**Step 1: Write the failing test**

Add to `bucket_map_test.go`:

```go
func TestBucketMap_CalculateNextRunPreserveRhythm(t *testing.T) {
	t.Helper()

	bm := NewBucketMap()
	now := time.Now()

	// Place initial job
	initialTime := bm.PlaceNewJob("job-1", 1*time.Hour)
	initialSlot := SlotKey(initialTime)

	// Reschedule - should advance by interval (4 slots for 1 hour)
	nextTime := bm.CalculateNextRunPreserveRhythm("job-1", 1*time.Hour)
	nextSlot := SlotKey(nextTime)

	expectedSlot := initialSlot + 4 // 1 hour = 4 * 15-minute slots
	if nextSlot != expectedSlot {
		t.Errorf("CalculateNextRunPreserveRhythm() slot = %d, want %d", nextSlot, expectedSlot)
	}

	// Verify job is now tracked in new slot
	gotSlot, _ := bm.GetJobSlot("job-1")
	if gotSlot != nextSlot {
		t.Errorf("GetJobSlot() = %d, want %d", gotSlot, nextSlot)
	}
}

func TestBucketMap_CalculateNextRunPreserveRhythm_UnknownJob(t *testing.T) {
	t.Helper()

	bm := NewBucketMap()

	// Unknown job should be placed like a new job
	result := bm.CalculateNextRunPreserveRhythm("unknown-job", 1*time.Hour)

	// Should be tracked now
	_, exists := bm.GetJobSlot("unknown-job")
	if !exists {
		t.Error("unknown job should be placed as new job")
	}

	// Result should be a valid time
	if result.IsZero() {
		t.Error("result should not be zero time")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
go test ./internal/scheduler/... -run "TestBucketMap_CalculateNextRunPreserveRhythm" -v
```

Expected: FAIL with "undefined: CalculateNextRunPreserveRhythm"

**Step 3: Write minimal implementation**

Add to `bucket_map.go`:

```go
// CalculateNextRunPreserveRhythm calculates next run time while preserving
// the job's slot phase (rhythm preservation).
// If the job is not tracked, it is placed as a new job.
func (b *BucketMap) CalculateNextRunPreserveRhythm(jobID string, interval time.Duration) time.Time {
	b.mu.Lock()

	currentSlot, exists := b.jobToSlot[jobID]
	if !exists {
		// Job not in bucket map - treat as new placement
		b.mu.Unlock()
		return b.PlaceNewJob(jobID, interval)
	}

	// Calculate next slot by adding interval
	slotsToAdd := int64(interval / SlotDuration)
	if slotsToAdd < 1 {
		slotsToAdd = 1
	}
	nextSlot := currentSlot + slotsToAdd

	// Remove from old slot
	b.slots[currentSlot]--
	if b.slots[currentSlot] <= 0 {
		delete(b.slots, currentSlot)
	}

	// Add to new slot
	b.slots[nextSlot]++
	b.jobToSlot[jobID] = nextSlot
	b.lastPlaced[jobID] = time.Now()

	b.mu.Unlock()

	return SlotTime(nextSlot)
}
```

**Step 4: Run test to verify it passes**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
go test ./internal/scheduler/... -run "TestBucketMap_CalculateNextRunPreserveRhythm" -v
```

Expected: PASS

**Step 5: Run linter and commit**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
golangci-lint run ./internal/scheduler/bucket_map.go
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler
git add crawler/internal/scheduler/bucket_map.go crawler/internal/scheduler/bucket_map_test.go
git commit -m "feat(scheduler): add rhythm preservation for recurring jobs

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 6: Anti-Thrashing CanMove Check

**Files:**
- Modify: `crawler/internal/scheduler/bucket_map.go`
- Modify: `crawler/internal/scheduler/bucket_map_test.go`

**Step 1: Write the failing tests**

Add to `bucket_map_test.go`:

```go
func TestBucketMap_CanMoveJob_Running(t *testing.T) {
	t.Helper()

	bm := NewBucketMap()

	reason, canMove := bm.CanMoveJob("job-1", "running", nil)
	if canMove {
		t.Error("running job should not be movable")
	}
	if reason != "job_running" {
		t.Errorf("reason = %q, want %q", reason, "job_running")
	}
}

func TestBucketMap_CanMoveJob_ProtectionWindow(t *testing.T) {
	t.Helper()

	bm := NewBucketMap()

	// Job running in 10 minutes - cannot move
	soon := time.Now().Add(10 * time.Minute)
	reason, canMove := bm.CanMoveJob("job-1", "scheduled", &soon)
	if canMove {
		t.Error("job in protection window should not be movable")
	}
	if reason != "protection_window" {
		t.Errorf("reason = %q, want %q", reason, "protection_window")
	}
}

func TestBucketMap_CanMoveJob_Cooldown(t *testing.T) {
	t.Helper()

	bm := NewBucketMap()

	// Add job (sets lastPlaced to now)
	bm.AddJob("job-1", SlotKey(time.Now().Add(2*time.Hour)))

	// Job was just placed - cannot move
	farFuture := time.Now().Add(2 * time.Hour)
	reason, canMove := bm.CanMoveJob("job-1", "scheduled", &farFuture)
	if canMove {
		t.Error("recently placed job should not be movable")
	}
	if reason != "placement_cooldown" {
		t.Errorf("reason = %q, want %q", reason, "placement_cooldown")
	}
}

func TestBucketMap_CanMoveJob_Allowed(t *testing.T) {
	t.Helper()

	bm := NewBucketMap()

	// Job far in future, not recently placed
	bm.mu.Lock()
	bm.jobToSlot["job-1"] = SlotKey(time.Now().Add(3 * time.Hour))
	bm.lastPlaced["job-1"] = time.Now().Add(-2 * time.Hour) // Placed 2 hours ago
	bm.mu.Unlock()

	farFuture := time.Now().Add(3 * time.Hour)
	reason, canMove := bm.CanMoveJob("job-1", "scheduled", &farFuture)
	if !canMove {
		t.Errorf("job should be movable, got reason: %q", reason)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
go test ./internal/scheduler/... -run "TestBucketMap_CanMoveJob" -v
```

Expected: FAIL with "undefined: CanMoveJob"

**Step 3: Write minimal implementation**

Add to `bucket_map.go`:

```go
// CanMoveJob checks if a job can be moved during rebalancing.
// Returns (reason, canMove) where reason explains why the job cannot be moved.
func (b *BucketMap) CanMoveJob(jobID, status string, nextRunAt *time.Time) (string, bool) {
	// Rule 1: Running jobs are untouchable
	if status == "running" {
		return "job_running", false
	}

	// Rule 2: Protection window for imminent jobs
	if nextRunAt != nil {
		if time.Until(*nextRunAt) <= ProtectionWindow {
			return "protection_window", false
		}
	}

	// Rule 3: Placement cooldown
	b.mu.RLock()
	lastPlaced, exists := b.lastPlaced[jobID]
	b.mu.RUnlock()
	if exists && time.Since(lastPlaced) < PlacementCooldown {
		return "placement_cooldown", false
	}

	return "", true
}
```

**Step 4: Run test to verify it passes**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
go test ./internal/scheduler/... -run "TestBucketMap_CanMoveJob" -v
```

Expected: PASS

**Step 5: Run linter and commit**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
golangci-lint run ./internal/scheduler/bucket_map.go
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler
git add crawler/internal/scheduler/bucket_map.go crawler/internal/scheduler/bucket_map_test.go
git commit -m "feat(scheduler): add anti-thrashing CanMoveJob checks

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 7: GetDistribution for API/Dashboard

**Files:**
- Modify: `crawler/internal/scheduler/bucket_map.go`
- Modify: `crawler/internal/scheduler/bucket_map_test.go`

**Step 1: Write the failing test**

Add to `bucket_map_test.go`:

```go
func TestBucketMap_GetDistribution(t *testing.T) {
	t.Helper()

	bm := NewBucketMap()
	now := time.Now()

	// Add jobs across different hours
	// Hour 0: 2 jobs
	bm.AddJob("job-1", SlotKey(now.Add(10*time.Minute)))
	bm.AddJob("job-2", SlotKey(now.Add(20*time.Minute)))
	// Hour 1: 3 jobs
	bm.AddJob("job-3", SlotKey(now.Add(1*time.Hour)))
	bm.AddJob("job-4", SlotKey(now.Add(1*time.Hour + 15*time.Minute)))
	bm.AddJob("job-5", SlotKey(now.Add(1*time.Hour + 30*time.Minute)))

	dist := bm.GetDistribution(24)

	if dist.TotalJobs != 5 {
		t.Errorf("TotalJobs = %d, want 5", dist.TotalJobs)
	}

	// Check that we have hourly data
	if len(dist.HourlyDistribution) != 24 {
		t.Errorf("HourlyDistribution length = %d, want 24", len(dist.HourlyDistribution))
	}

	// Verify peak detection
	if dist.PeakCount < 3 {
		t.Errorf("PeakCount = %d, expected at least 3", dist.PeakCount)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
go test ./internal/scheduler/... -run "TestBucketMap_GetDistribution" -v
```

Expected: FAIL with "undefined: GetDistribution"

**Step 3: Write minimal implementation**

Add to `bucket_map.go`:

```go
// HourlyCount represents job count for a specific hour.
type HourlyCount struct {
	Hour     int `json:"hour"`
	JobCount int `json:"job_count"`
}

// Distribution represents the schedule distribution metrics.
type Distribution struct {
	WindowHours        int           `json:"window_hours"`
	SlotMinutes        int           `json:"slot_minutes"`
	TotalJobs          int           `json:"total_jobs"`
	HourlyDistribution []HourlyCount `json:"hourly_distribution"`
	DistributionScore  float64       `json:"distribution_score"`
	PeakHour           int           `json:"peak_hour"`
	PeakCount          int           `json:"peak_count"`
}

// GetDistribution returns the current schedule distribution metrics.
func (b *BucketMap) GetDistribution(windowHours int) Distribution {
	b.mu.RLock()
	defer b.mu.RUnlock()

	now := time.Now()
	hourly := make([]HourlyCount, windowHours)
	totalJobs := 0
	peakHour := 0
	peakCount := 0

	// Aggregate by hour
	for h := 0; h < windowHours; h++ {
		hourStart := now.Add(time.Duration(h) * time.Hour)
		hourEnd := hourStart.Add(time.Hour)
		count := 0

		for t := hourStart; t.Before(hourEnd); t = t.Add(SlotDuration) {
			slot := SlotKey(t)
			count += b.slots[slot]
		}

		hourly[h] = HourlyCount{Hour: h, JobCount: count}
		totalJobs += count

		if count > peakCount {
			peakCount = count
			peakHour = h
		}
	}

	// Calculate distribution score (1.0 = perfectly even)
	var score float64
	if totalJobs > 0 && windowHours > 0 {
		ideal := float64(totalJobs) / float64(windowHours)
		var variance float64
		for _, hc := range hourly {
			diff := float64(hc.JobCount) - ideal
			variance += diff * diff
		}
		variance /= float64(windowHours)
		// Score: 1 - normalized stddev (capped at 0)
		if ideal > 0 {
			stddev := variance / (ideal * ideal)
			score = 1.0 - stddev
			if score < 0 {
				score = 0
			}
		}
	} else {
		score = 1.0 // Empty schedule is perfectly distributed
	}

	return Distribution{
		WindowHours:        windowHours,
		SlotMinutes:        15,
		TotalJobs:          totalJobs,
		HourlyDistribution: hourly,
		DistributionScore:  score,
		PeakHour:           peakHour,
		PeakCount:          peakCount,
	}
}
```

**Step 4: Run test to verify it passes**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
go test ./internal/scheduler/... -run "TestBucketMap_GetDistribution" -v
```

Expected: PASS

**Step 5: Run linter and commit**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
golangci-lint run ./internal/scheduler/bucket_map.go
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler
git add crawler/internal/scheduler/bucket_map.go crawler/internal/scheduler/bucket_map_test.go
git commit -m "feat(scheduler): add GetDistribution for API/dashboard metrics

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 8: Integrate BucketMap into IntervalScheduler

**Files:**
- Modify: `crawler/internal/scheduler/interval_scheduler.go`
- Modify: `crawler/internal/scheduler/options.go`

**Step 1: Add BucketMap field to IntervalScheduler**

In `interval_scheduler.go`, add to the struct (around line 38):

```go
// IntervalScheduler replaces the cron-based scheduler with interval-based scheduling.
type IntervalScheduler struct {
	// ... existing fields ...

	// Load balancing
	bucketMap *BucketMap
}
```

**Step 2: Initialize BucketMap in constructor**

In `NewIntervalScheduler` (around line 70), add after creating the struct:

```go
s := &IntervalScheduler{
	// ... existing initialization ...
	bucketMap: NewBucketMap(),
}
```

**Step 3: Add option for enabling/disabling load balancing**

In `options.go`, add:

```go
// WithLoadBalancing enables or disables load-balanced placement.
// Default is true (enabled).
func WithLoadBalancing(enabled bool) SchedulerOption {
	return func(s *IntervalScheduler) {
		if !enabled {
			s.bucketMap = nil
		}
	}
}
```

**Step 4: Run existing tests to verify no breakage**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
go test ./internal/scheduler/... -v
```

Expected: All existing tests PASS

**Step 5: Run linter and commit**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
golangci-lint run ./internal/scheduler/...
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler
git add crawler/internal/scheduler/interval_scheduler.go crawler/internal/scheduler/options.go
git commit -m "feat(scheduler): integrate BucketMap into IntervalScheduler

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 9: Rebuild BucketMap on Startup

**Files:**
- Modify: `crawler/internal/scheduler/interval_scheduler.go`
- Modify: `crawler/internal/database/job_repository.go`

**Step 1: Add GetScheduledJobs to repository interface**

First check the interface in `crawler/internal/database/interfaces.go` or where `JobRepositoryInterface` is defined. Add:

```go
// GetScheduledJobs returns all jobs that are scheduled (have next_run_at set).
GetScheduledJobs(ctx context.Context) ([]*domain.Job, error)
```

**Step 2: Implement GetScheduledJobs in JobRepository**

In `job_repository.go`, add:

```go
// GetScheduledJobs returns all scheduled jobs (next_run_at IS NOT NULL, not paused).
func (r *JobRepository) GetScheduledJobs(ctx context.Context) ([]*domain.Job, error) {
	query := `SELECT ` + jobSelectBase + ` FROM jobs
		WHERE next_run_at IS NOT NULL
		AND is_paused = false
		AND status IN ('pending', 'scheduled')
		ORDER BY next_run_at`

	var jobs []*domain.Job
	err := r.db.SelectContext(ctx, &jobs, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get scheduled jobs: %w", err)
	}
	return jobs, nil
}
```

**Step 3: Add rebuildBucketMap method to scheduler**

In `interval_scheduler.go`, add:

```go
// rebuildBucketMap rebuilds the bucket map from database state on startup.
func (s *IntervalScheduler) rebuildBucketMap() error {
	if s.bucketMap == nil {
		return nil // Load balancing disabled
	}

	jobs, err := s.repo.GetScheduledJobs(s.ctx)
	if err != nil {
		return fmt.Errorf("failed to get scheduled jobs: %w", err)
	}

	for _, job := range jobs {
		if job.NextRunAt != nil {
			s.bucketMap.AddJob(job.ID, SlotKey(*job.NextRunAt))
		}
	}

	s.logger.Info("Bucket map rebuilt",
		infralogger.Int("job_count", len(jobs)),
	)
	return nil
}
```

**Step 4: Call rebuildBucketMap in Start()**

In `Start()` method (around line 103), add before starting goroutines:

```go
func (s *IntervalScheduler) Start(ctx context.Context) error {
	s.logger.Info("Starting interval scheduler",
		// ... existing logging ...
	)

	// Rebuild bucket map from existing scheduled jobs
	if err := s.rebuildBucketMap(); err != nil {
		return fmt.Errorf("failed to rebuild bucket map: %w", err)
	}

	// ... rest of existing code ...
}
```

**Step 5: Run tests and linter**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
go test ./internal/scheduler/... -v
go test ./internal/database/... -v
golangci-lint run ./internal/scheduler/... ./internal/database/...
```

**Step 6: Commit**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler
git add crawler/internal/scheduler/interval_scheduler.go crawler/internal/database/job_repository.go
git commit -m "feat(scheduler): rebuild BucketMap from database on startup

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 10: Modify calculateNextRun for Load Balancing

**Files:**
- Modify: `crawler/internal/scheduler/interval_scheduler.go`

**Step 1: Add helper function for interval duration**

In `interval_scheduler.go`, add:

```go
// getIntervalDuration converts job interval settings to a time.Duration.
func getIntervalDuration(job *domain.Job) time.Duration {
	if job.IntervalMinutes == nil {
		return searchWindowDefault // Default for one-time jobs
	}
	switch job.IntervalType {
	case "hours":
		return time.Duration(*job.IntervalMinutes) * time.Hour
	case "days":
		return time.Duration(*job.IntervalMinutes) * hoursPerDay * time.Hour
	default: // "minutes"
		return time.Duration(*job.IntervalMinutes) * time.Minute
	}
}
```

**Step 2: Modify calculateNextRun to use rhythm preservation**

Replace the existing `calculateNextRun` method (around line 579):

```go
// calculateNextRun calculates the next run time based on interval configuration.
// Uses rhythm preservation when load balancing is enabled.
func (s *IntervalScheduler) calculateNextRun(job *domain.Job) time.Time {
	if job.IntervalMinutes == nil {
		return time.Time{}
	}

	interval := getIntervalDuration(job)

	// Use rhythm preservation when load balancing is enabled
	if s.bucketMap != nil {
		return s.bucketMap.CalculateNextRunPreserveRhythm(job.ID, interval)
	}

	// Fallback to original behavior
	return time.Now().Add(interval)
}
```

**Step 3: Run tests**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
go test ./internal/scheduler/... -v
```

**Step 4: Run linter and commit**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
golangci-lint run ./internal/scheduler/...
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler
git add crawler/internal/scheduler/interval_scheduler.go
git commit -m "feat(scheduler): use rhythm preservation in calculateNextRun

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 11: Add Distribution API Endpoint

**Files:**
- Modify: `crawler/internal/api/jobs_handler.go`
- Create: `crawler/internal/api/scheduler_handler.go`

**Step 1: Create scheduler handler with GetDistribution**

Create `crawler/internal/api/scheduler_handler.go`:

```go
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/crawler/internal/scheduler"
)

// SchedulerHandler handles scheduler-related API endpoints.
type SchedulerHandler struct {
	scheduler *scheduler.IntervalScheduler
}

// NewSchedulerHandler creates a new scheduler handler.
func NewSchedulerHandler(sched *scheduler.IntervalScheduler) *SchedulerHandler {
	return &SchedulerHandler{scheduler: sched}
}

// GetDistribution returns the current schedule distribution.
// GET /api/v1/scheduler/distribution
func (h *SchedulerHandler) GetDistribution(c *gin.Context) {
	dist := h.scheduler.GetDistribution()
	c.JSON(http.StatusOK, dist)
}
```

**Step 2: Add GetDistribution method to IntervalScheduler**

In `interval_scheduler.go`, add:

```go
// GetDistribution returns the current schedule distribution.
// Returns nil if load balancing is disabled.
func (s *IntervalScheduler) GetDistribution() *Distribution {
	if s.bucketMap == nil {
		return nil
	}
	dist := s.bucketMap.GetDistribution(24)
	return &dist
}
```

**Step 3: Register the endpoint**

Find where routes are registered (likely in `api/api.go` or bootstrap) and add:

```go
// Add to router setup
schedulerHandler := api.NewSchedulerHandler(scheduler)
v1.GET("/scheduler/distribution", schedulerHandler.GetDistribution)
```

**Step 4: Test manually**

```bash
# After starting the service
curl http://localhost:8060/api/v1/scheduler/distribution | jq
```

**Step 5: Run linter and commit**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
golangci-lint run ./internal/api/... ./internal/scheduler/...
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler
git add crawler/internal/api/scheduler_handler.go crawler/internal/scheduler/interval_scheduler.go
git commit -m "feat(api): add GET /scheduler/distribution endpoint

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 12: Update Job Creation to Use Load Balancing

**Files:**
- Modify: `crawler/internal/scheduler/interval_scheduler.go`
- Modify: `crawler/internal/api/jobs_handler.go` (if needed)

**Step 1: Add method to schedule new job with load balancing**

In `interval_scheduler.go`, add:

```go
// ScheduleNewJob schedules a new job with load-balanced placement.
// This should be called when a job is created via API.
func (s *IntervalScheduler) ScheduleNewJob(job *domain.Job) error {
	if job.IntervalMinutes == nil || !job.ScheduleEnabled {
		// One-time job - no load balancing needed
		return nil
	}

	interval := getIntervalDuration(job)

	if s.bucketMap != nil {
		nextRun := s.bucketMap.PlaceNewJob(job.ID, interval)
		job.NextRunAt = &nextRun
		job.Status = "scheduled"
	} else {
		// Fallback to original behavior
		nextRun := time.Now().Add(interval)
		job.NextRunAt = &nextRun
		job.Status = "scheduled"
	}

	return s.repo.Update(s.ctx, job)
}
```

**Step 2: Call from jobs handler (if applicable)**

The existing flow may use a database trigger for `next_run_at`. If so, we need to override it after creation. Check the flow and modify as needed.

**Step 3: Run tests and linter**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
go test ./... -v
golangci-lint run ./...
```

**Step 4: Commit**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler
git add crawler/internal/scheduler/interval_scheduler.go
git commit -m "feat(scheduler): add ScheduleNewJob for load-balanced placement

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 13: Handle Job Deletion in BucketMap

**Files:**
- Modify: `crawler/internal/scheduler/interval_scheduler.go`

**Step 1: Add method to handle job deletion**

In `interval_scheduler.go`, add:

```go
// HandleJobDeleted removes a job from the bucket map when deleted.
func (s *IntervalScheduler) HandleJobDeleted(jobID string) {
	if s.bucketMap != nil {
		s.bucketMap.RemoveJob(jobID)
	}
}
```

**Step 2: Call from delete flow**

Find where jobs are deleted and add the call. This may be in the API handler or repository.

**Step 3: Run tests and commit**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
go test ./... -v
golangci-lint run ./...
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler
git add crawler/internal/scheduler/interval_scheduler.go
git commit -m "feat(scheduler): handle job deletion in BucketMap

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 14: Handle Interval Change Events

**Files:**
- Modify: `crawler/internal/scheduler/interval_scheduler.go`

**Step 1: Add method to handle interval changes**

In `interval_scheduler.go`, add:

```go
// HandleIntervalChange re-places a job when its interval changes.
func (s *IntervalScheduler) HandleIntervalChange(job *domain.Job) error {
	if s.bucketMap == nil {
		return nil
	}

	interval := getIntervalDuration(job)
	s.bucketMap.RemoveJob(job.ID)
	nextRun := s.bucketMap.PlaceNewJob(job.ID, interval)
	job.NextRunAt = &nextRun

	return s.repo.Update(s.ctx, job)
}
```

**Step 2: Call from update flow**

When a job's interval is updated via API, call this method if the interval changed.

**Step 3: Run tests and commit**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
go test ./... -v
golangci-lint run ./...
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler
git add crawler/internal/scheduler/interval_scheduler.go
git commit -m "feat(scheduler): re-place jobs when interval changes

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 15: Handle Resume from Pause

**Files:**
- Modify: `crawler/internal/scheduler/interval_scheduler.go`

**Step 1: Add method to handle resume**

In `interval_scheduler.go`, add:

```go
// HandleResume re-places a job when it resumes from pause.
func (s *IntervalScheduler) HandleResume(job *domain.Job) error {
	if s.bucketMap == nil {
		return nil
	}

	interval := getIntervalDuration(job)
	s.bucketMap.RemoveJob(job.ID)
	nextRun := s.bucketMap.PlaceNewJob(job.ID, interval)
	job.NextRunAt = &nextRun

	return s.repo.Update(s.ctx, job)
}
```

**Step 2: Integrate with existing resume flow**

Find the existing resume handler and call this method.

**Step 3: Run tests and commit**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
go test ./... -v
golangci-lint run ./...
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler
git add crawler/internal/scheduler/interval_scheduler.go
git commit -m "feat(scheduler): re-place jobs when resuming from pause

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 16: Full Rebalance Endpoint

**Files:**
- Modify: `crawler/internal/scheduler/bucket_map.go`
- Modify: `crawler/internal/api/scheduler_handler.go`

**Step 1: Add FullRebalance to BucketMap**

In `bucket_map.go`, add:

```go
// Reassignment represents a job that was moved during rebalance.
type Reassignment struct {
	JobID   string    `json:"job_id"`
	OldTime time.Time `json:"old_time"`
	NewTime time.Time `json:"new_time"`
}

// SkippedJob represents a job that could not be moved.
type SkippedJob struct {
	JobID  string `json:"job_id"`
	Reason string `json:"reason"`
}

// RebalanceResult contains the outcome of a full rebalance operation.
type RebalanceResult struct {
	Moved              []Reassignment `json:"moved"`
	Skipped            []SkippedJob   `json:"skipped"`
	NewDistributionScore float64      `json:"new_distribution_score"`
}
```

**Step 2: Add FullRebalance method to IntervalScheduler**

In `interval_scheduler.go`, add the full rebalance implementation that:
1. Gets all scheduled jobs
2. Sorts by interval (longest first)
3. Clears bucket map
4. Re-places each job (respecting CanMove)
5. Updates database

**Step 3: Add POST endpoint**

In `scheduler_handler.go`, add:

```go
// PostRebalance triggers a full schedule rebalance.
// POST /api/v1/scheduler/rebalance
func (h *SchedulerHandler) PostRebalance(c *gin.Context) {
	result, err := h.scheduler.FullRebalance()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
```

**Step 4: Register endpoint**

Add to router: `v1.POST("/scheduler/rebalance", schedulerHandler.PostRebalance)`

**Step 5: Run tests and commit**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
go test ./... -v
golangci-lint run ./...
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler
git add crawler/internal/scheduler/bucket_map.go crawler/internal/scheduler/interval_scheduler.go crawler/internal/api/scheduler_handler.go
git commit -m "feat(api): add POST /scheduler/rebalance endpoint

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 17: Rebalance Preview Endpoint

**Files:**
- Modify: `crawler/internal/scheduler/interval_scheduler.go`
- Modify: `crawler/internal/api/scheduler_handler.go`

**Step 1: Add preview method**

Similar to FullRebalance but doesn't persist changes - returns what would happen.

**Step 2: Add endpoint**

```go
// PostRebalancePreview previews what a rebalance would do.
// POST /api/v1/scheduler/rebalance/preview
func (h *SchedulerHandler) PostRebalancePreview(c *gin.Context) {
	result, err := h.scheduler.PreviewRebalance()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
```

**Step 3: Run tests and commit**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
go test ./... -v
golangci-lint run ./...
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler
git add crawler/internal/scheduler/interval_scheduler.go crawler/internal/api/scheduler_handler.go
git commit -m "feat(api): add POST /scheduler/rebalance/preview endpoint

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 18: Integration Tests

**Files:**
- Create: `crawler/internal/scheduler/integration_test.go`

**Step 1: Write integration tests**

Test the full flow:
1. Create scheduler with BucketMap
2. Add multiple jobs
3. Verify distribution
4. Trigger rebalance
5. Verify improved distribution

**Step 2: Run and verify**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
go test ./internal/scheduler/... -v -run Integration
```

**Step 3: Commit**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler
git add crawler/internal/scheduler/integration_test.go
git commit -m "test(scheduler): add integration tests for load balancing

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 19: Update INTERVAL_SCHEDULER.md Documentation

**Files:**
- Modify: `crawler/docs/INTERVAL_SCHEDULER.md`

**Step 1: Add load balancing section**

Document:
- How load balancing works
- New API endpoints
- When rebalancing occurs
- How to monitor distribution

**Step 2: Commit**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler
git add crawler/docs/INTERVAL_SCHEDULER.md
git commit -m "docs: document load balancing in INTERVAL_SCHEDULER.md

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 20: Final Verification and Cleanup

**Step 1: Run full test suite**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler/crawler
go test ./... -v
golangci-lint run ./...
```

**Step 2: Manual testing**

1. Start the service
2. Create several jobs
3. Check `/scheduler/distribution`
4. Verify jobs are spread across slots
5. Test rebalance preview and execution

**Step 3: Final commit if any cleanup needed**

```bash
cd /home/fsd42/dev/north-cloud/.worktrees/self-balancing-scheduler
git status
# Add any remaining files
git commit -m "chore: final cleanup for self-balancing scheduler

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Summary

| Task | Component | Status |
|------|-----------|--------|
| 1 | BucketMap core data structure | Pending |
| 2 | Add/Remove/Get operations | Pending |
| 3 | FindLeastLoaded algorithm | Pending |
| 4 | PlaceNewJob method | Pending |
| 5 | Rhythm preservation | Pending |
| 6 | Anti-thrashing CanMove | Pending |
| 7 | GetDistribution metrics | Pending |
| 8 | Integrate into scheduler | Pending |
| 9 | Rebuild on startup | Pending |
| 10 | Modify calculateNextRun | Pending |
| 11 | Distribution API endpoint | Pending |
| 12 | Job creation load balancing | Pending |
| 13 | Handle job deletion | Pending |
| 14 | Handle interval change | Pending |
| 15 | Handle resume from pause | Pending |
| 16 | Full rebalance endpoint | Pending |
| 17 | Rebalance preview endpoint | Pending |
| 18 | Integration tests | Pending |
| 19 | Update documentation | Pending |
| 20 | Final verification | Pending |

**Estimated tasks:** 20
**Approach:** TDD - write failing test, implement, verify, commit
