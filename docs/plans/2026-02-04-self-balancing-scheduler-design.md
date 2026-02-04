# Self-Balancing Crawler Scheduler

**Date:** 2026-02-04
**Status:** Design Complete
**Author:** Claude + Human Collaboration

## Problem Statement

The current crawler scheduler assigns `next_run_at` as `now + interval` without awareness of other scheduled jobs. This produces incidental distribution based on when sources were created, leading to:

1. **Unpredictable load patterns** - Jobs cluster randomly, making it hard to reason about system behavior
2. **Avoidable resource spikes** - CPU and network usage spikes when multiple crawls run simultaneously
3. **Operational opacity** - Operators cannot easily understand or predict the schedule

## Goals

1. **Predictable behavior** - Operators can reason about load, timing, and system behavior
2. **Even distribution** - Jobs spread across a rolling 24h window to minimize clustering
3. **Automatic adjustment** - Schedule adapts as sources are added, removed, or intervals change
4. **Stability** - Jobs maintain consistent timing once placed (rhythm preservation)
5. **Operator control** - Manual rebalancing available; no unexpected automatic reshuffling

## Non-Goals

- Per-domain throttling (not hitting external rate limits)
- Multi-instance bucket coordination (single scheduler instance)
- Weighted scheduling by job cost (all jobs treated equally for now)
- Sub-second scheduling precision

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                     IntervalScheduler                            │
│                                                                  │
│  ┌──────────────────┐     ┌─────────────────────────────────┐  │
│  │   Job Poller     │     │         BucketMap               │  │
│  │   (every 10s)    │────▶│  ┌─────────────────────────┐   │  │
│  └──────────────────┘     │  │ slots: map[int64]int    │   │  │
│                           │  │ jobToSlot: map[str]int64│   │  │
│  ┌──────────────────┐     │  │ lastPlaced: map[str]time│   │  │
│  │ Placement Logic  │────▶│  └─────────────────────────┘   │  │
│  │ - PlaceNewJob    │     │                                 │  │
│  │ - HandleInterval │     │  Methods:                       │  │
│  │ - HandleResume   │     │  - PlaceNewJob()                │  │
│  │ - FullRebalance  │     │  - RemoveJob()                  │  │
│  └──────────────────┘     │  - FindLeastLoaded()            │  │
│                           │  - CanMove()                    │  │
│                           └─────────────────────────────────┘  │
│                                        │                        │
│                                        ▼                        │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                    PostgreSQL                             │  │
│  │   jobs.next_run_at (source of truth)                     │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Bucket storage | Hybrid (DB + in-memory) | Fast placement, no schema changes |
| Bucket granularity | 15-minute slots | Balances precision with simplicity |
| Recurring jobs | Rhythm preservation | Predictable timing over micro-optimization |
| Rebalancing trigger | Event-driven only | Predictability, operator control |

---

## Data Structures

### BucketMap

The in-memory schedule view, rebuilt on startup from `jobs.next_run_at`:

```go
package scheduler

import (
    "sync"
    "time"
)

const (
    SlotDuration      = 15 * time.Minute
    SlotSeconds       = 900  // 15 minutes in seconds
    SlotsPerDay       = 96   // 24h / 15min
    ProtectionWindow  = 30 * time.Minute
    PlacementCooldown = 1 * time.Hour
)

// BucketMap holds the in-memory schedule view for load-balanced placement.
type BucketMap struct {
    mu         sync.RWMutex
    slots      map[int64]int       // slot_key -> job_count
    jobToSlot  map[string]int64    // job_id -> slot_key
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

// SlotKey converts a time to its 15-minute bucket key.
// Times within the same 15-minute window map to the same key.
func SlotKey(t time.Time) int64 {
    return t.Unix() / SlotSeconds
}

// SlotTime converts a slot key back to its start time.
func SlotTime(key int64) time.Time {
    return time.Unix(key*SlotSeconds, 0)
}
```

### Core Operations

```go
// AddJob records a job placement in a slot.
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

// GetJobSlot returns the slot key for a job, or 0 if not found.
func (b *BucketMap) GetJobSlot(jobID string) (int64, bool) {
    b.mu.RLock()
    defer b.mu.RUnlock()
    slot, exists := b.jobToSlot[jobID]
    return slot, exists
}
```

---

## Placement Algorithm

### New Job Placement

When a new source is added, find the least-loaded slot in the search window:

```go
// PlaceNewJob finds the optimal slot for a new job.
// Returns the scheduled time for the job.
func (b *BucketMap) PlaceNewJob(jobID string, interval time.Duration) time.Time {
    now := time.Now()

    // Search window: next 24h or next interval, whichever is larger
    searchDuration := 24 * time.Hour
    if interval > searchDuration {
        searchDuration = interval
    }
    searchEnd := now.Add(searchDuration)

    b.mu.Lock()
    defer b.mu.Unlock()

    // Find least-loaded slot
    bestSlot := SlotKey(now)
    bestLoad := b.getSlotLoadLocked(bestSlot)

    for t := now; t.Before(searchEnd); t = t.Add(SlotDuration) {
        slot := SlotKey(t)
        load := b.getSlotLoadLocked(slot)
        if load < bestLoad {
            bestLoad = load
            bestSlot = slot
        }
    }

    // Record placement
    b.addJobLocked(jobID, bestSlot)
    return SlotTime(bestSlot)
}

// getSlotLoadLocked returns slot load (must hold lock).
func (b *BucketMap) getSlotLoadLocked(slotKey int64) int {
    return b.slots[slotKey]
}

// addJobLocked adds a job to a slot (must hold lock).
func (b *BucketMap) addJobLocked(jobID string, slotKey int64) {
    if oldSlot, exists := b.jobToSlot[jobID]; exists {
        b.slots[oldSlot]--
        if b.slots[oldSlot] <= 0 {
            delete(b.slots, oldSlot)
        }
    }
    b.slots[slotKey]++
    b.jobToSlot[jobID] = slotKey
    b.lastPlaced[jobID] = time.Now()
}
```

### Recurring Job Reschedule (Rhythm Preservation)

After a job completes, preserve its slot phase:

```go
// CalculateNextRunPreserveRhythm calculates next run time while preserving
// the job's slot phase (rhythm preservation).
func (b *BucketMap) CalculateNextRunPreserveRhythm(
    jobID string,
    interval time.Duration,
) time.Time {
    b.mu.Lock()
    defer b.mu.Unlock()

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

    // Update placement
    b.addJobLocked(jobID, nextSlot)
    return SlotTime(nextSlot)
}
```

---

## Rebalancing Strategy

### Event Triggers

| Event | Handler | Behavior |
|-------|---------|----------|
| Source added | `PlaceNewJob` | Find least-loaded slot |
| Source deleted | `RemoveJob` | Decrement slot, leave gap |
| Interval changed | `HandleIntervalChange` | Fresh placement |
| Resume from pause | `HandleResume` | Fresh placement |
| Manual rebalance | `FullRebalance` | Redistribute all movable jobs |

### Interval Change Handler

```go
// HandleIntervalChange re-places a job when its interval changes.
func (b *BucketMap) HandleIntervalChange(
    jobID string,
    newInterval time.Duration,
) time.Time {
    b.RemoveJob(jobID)
    return b.PlaceNewJob(jobID, newInterval)
}
```

### Resume from Pause Handler

```go
// HandleResume re-places a job when it resumes from pause.
func (b *BucketMap) HandleResume(
    jobID string,
    interval time.Duration,
) time.Time {
    b.RemoveJob(jobID)
    return b.PlaceNewJob(jobID, interval)
}
```

### Full Rebalance (Operator-Initiated)

```go
// RebalanceResult contains the outcome of a full rebalance operation.
type RebalanceResult struct {
    Moved   []Reassignment
    Skipped []SkippedJob
}

// Reassignment represents a job that was moved during rebalance.
type Reassignment struct {
    JobID   string
    OldTime time.Time
    NewTime time.Time
}

// SkippedJob represents a job that could not be moved.
type SkippedJob struct {
    JobID  string
    Reason string
}

// FullRebalance redistributes all movable jobs for optimal distribution.
// Jobs are sorted by interval (longest first) to place constrained jobs first.
func (b *BucketMap) FullRebalance(jobs []*domain.Job) RebalanceResult {
    result := RebalanceResult{
        Moved:   make([]Reassignment, 0),
        Skipped: make([]SkippedJob, 0),
    }

    // Sort by interval descending (longest first)
    sortedJobs := make([]*domain.Job, len(jobs))
    copy(sortedJobs, jobs)
    sort.Slice(sortedJobs, func(i, j int) bool {
        return getIntervalDuration(sortedJobs[i]) > getIntervalDuration(sortedJobs[j])
    })

    // Clear all placements
    b.mu.Lock()
    b.slots = make(map[int64]int)
    b.jobToSlot = make(map[string]int64)
    // Note: lastPlaced is NOT cleared - cooldown still applies after rebalance
    b.mu.Unlock()

    // Place each job optimally
    for _, job := range sortedJobs {
        if reason, canMove := b.canMoveJob(job); !canMove {
            result.Skipped = append(result.Skipped, SkippedJob{
                JobID:  job.ID,
                Reason: reason,
            })
            // Re-add at original position
            if job.NextRunAt != nil {
                b.AddJob(job.ID, SlotKey(*job.NextRunAt))
            }
            continue
        }

        oldTime := job.NextRunAt
        interval := getIntervalDuration(job)
        newTime := b.PlaceNewJob(job.ID, interval)

        if oldTime == nil || !oldTime.Equal(newTime) {
            result.Moved = append(result.Moved, Reassignment{
                JobID:   job.ID,
                OldTime: safeDeref(oldTime),
                NewTime: newTime,
            })
        }
    }

    return result
}

func getIntervalDuration(job *domain.Job) time.Duration {
    if job.IntervalMinutes == nil {
        return 24 * time.Hour // Default for one-time jobs
    }
    switch job.IntervalType {
    case "hours":
        return time.Duration(*job.IntervalMinutes) * time.Hour
    case "days":
        return time.Duration(*job.IntervalMinutes) * 24 * time.Hour
    default: // "minutes"
        return time.Duration(*job.IntervalMinutes) * time.Minute
    }
}

func safeDeref(t *time.Time) time.Time {
    if t == nil {
        return time.Time{}
    }
    return *t
}
```

---

## Anti-Thrashing Rules

### CanMove Check

```go
// canMoveJob checks if a job can be moved during rebalancing.
// Returns (reason, canMove).
func (b *BucketMap) canMoveJob(job *domain.Job) (string, bool) {
    // Rule 1: Running jobs are untouchable
    if job.Status == "running" {
        return "job_running", false
    }

    // Rule 2: Protection window for imminent jobs
    if job.NextRunAt != nil {
        if time.Until(*job.NextRunAt) <= ProtectionWindow {
            return "protection_window", false
        }
    }

    // Rule 3: Placement cooldown
    b.mu.RLock()
    lastPlaced, exists := b.lastPlaced[job.ID]
    b.mu.RUnlock()
    if exists && time.Since(lastPlaced) < PlacementCooldown {
        return "placement_cooldown", false
    }

    return "", true
}
```

### Rules Summary

| Rule | Condition | Effect |
|------|-----------|--------|
| Running guard | `status == "running"` | Cannot move |
| Protection window | `next_run < now + 30min` | Cannot move |
| Placement cooldown | `last_placed < 1 hour ago` | Cannot move |

---

## Integration with IntervalScheduler

### Startup: Rebuild BucketMap

```go
func (s *IntervalScheduler) Start(ctx context.Context) error {
    // Rebuild bucket map from existing scheduled jobs
    if err := s.rebuildBucketMap(); err != nil {
        return fmt.Errorf("failed to rebuild bucket map: %w", err)
    }

    // ... existing startup code
}

func (s *IntervalScheduler) rebuildBucketMap() error {
    jobs, err := s.repo.GetScheduledJobs(s.ctx)
    if err != nil {
        return err
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

### Modified: calculateNextRun

Replace the existing method:

```go
// calculateNextRun calculates the next run time based on interval configuration.
// For recurring jobs, preserves rhythm. For new placements, finds optimal slot.
func (s *IntervalScheduler) calculateNextRun(job *domain.Job) time.Time {
    if job.IntervalMinutes == nil {
        return time.Time{}
    }

    interval := getIntervalDuration(job)

    // Use rhythm preservation for recurring reschedules
    return s.bucketMap.CalculateNextRunPreserveRhythm(job.ID, interval)
}
```

### Modified: Job Creation

When a new job is created:

```go
func (s *IntervalScheduler) scheduleNewJob(job *domain.Job) error {
    if job.IntervalMinutes == nil || !job.ScheduleEnabled {
        // One-time job - run immediately
        return nil
    }

    interval := getIntervalDuration(job)
    nextRun := s.bucketMap.PlaceNewJob(job.ID, interval)
    job.NextRunAt = &nextRun
    job.Status = "scheduled"

    return s.repo.Update(s.ctx, job)
}
```

### Modified: Job Deletion

```go
func (s *IntervalScheduler) deleteJob(jobID string) error {
    s.bucketMap.RemoveJob(jobID)
    return s.repo.Delete(s.ctx, jobID)
}
```

---

## API Changes

### New Endpoint: Schedule Distribution

```http
GET /api/v1/scheduler/distribution

Response: 200 OK
{
  "window_hours": 24,
  "slot_minutes": 15,
  "total_jobs": 42,
  "hourly_distribution": [
    {"hour": 0, "job_count": 4},
    {"hour": 1, "job_count": 3},
    ...
  ],
  "distribution_score": 0.85,  // 1.0 = perfectly even
  "peak_hour": 14,
  "peak_count": 8,
  "suggestion": "Distribution is acceptable"
}
```

### New Endpoint: Manual Rebalance

```http
POST /api/v1/scheduler/rebalance

Response: 200 OK
{
  "moved": [
    {"job_id": "abc123", "old_time": "2026-02-04T14:15:00Z", "new_time": "2026-02-04T10:30:00Z"},
    ...
  ],
  "skipped": [
    {"job_id": "def456", "reason": "protection_window"},
    ...
  ],
  "new_distribution_score": 0.92
}
```

### New Endpoint: Rebalance Preview

```http
POST /api/v1/scheduler/rebalance/preview

Response: 200 OK
{
  "would_move": 12,
  "would_skip": 3,
  "current_score": 0.65,
  "projected_score": 0.91,
  "preview": [
    {"job_id": "abc123", "current_time": "...", "proposed_time": "..."},
    ...
  ]
}
```

---

## Migration Path

### Phase 1: Add BucketMap (No Behavior Change)

1. Implement `BucketMap` struct and methods
2. Add to `IntervalScheduler` as optional component
3. Rebuild on startup, log distribution metrics
4. **No changes to scheduling behavior yet**

### Phase 2: Enable Balanced Placement for New Jobs

1. New jobs use `PlaceNewJob` instead of `now + interval`
2. Existing jobs continue with rhythm preservation
3. Monitor distribution improvement over time

### Phase 3: Add API Endpoints

1. Implement `/distribution` endpoint
2. Implement `/rebalance/preview` endpoint
3. Implement `/rebalance` endpoint
4. Add dashboard UI for distribution visualization

### Phase 4: Full Integration

1. Enable all rebalancing triggers (interval change, resume)
2. Document operator workflows
3. Remove feature flag / make default behavior

---

## Testing Strategy

### Unit Tests

```go
func TestSlotKey(t *testing.T) {
    // Times in same 15-min window get same key
    t1 := time.Date(2026, 2, 4, 10, 0, 0, 0, time.UTC)
    t2 := time.Date(2026, 2, 4, 10, 14, 59, 0, time.UTC)
    assert.Equal(t, SlotKey(t1), SlotKey(t2))

    // Times in different windows get different keys
    t3 := time.Date(2026, 2, 4, 10, 15, 0, 0, time.UTC)
    assert.NotEqual(t, SlotKey(t1), SlotKey(t3))
}

func TestPlaceNewJob_FindsLeastLoaded(t *testing.T) {
    b := NewBucketMap()

    // Pre-populate some slots
    b.AddJob("job1", SlotKey(time.Now().Add(1*time.Hour)))
    b.AddJob("job2", SlotKey(time.Now().Add(1*time.Hour)))
    b.AddJob("job3", SlotKey(time.Now().Add(2*time.Hour)))

    // New job should avoid the loaded slot
    newTime := b.PlaceNewJob("job4", 6*time.Hour)
    newSlot := SlotKey(newTime)

    // Should not be in the most loaded slot
    loadedSlot := SlotKey(time.Now().Add(1 * time.Hour))
    assert.NotEqual(t, newSlot, loadedSlot)
}

func TestRhythmPreservation(t *testing.T) {
    b := NewBucketMap()

    // Place initial job
    initialTime := b.PlaceNewJob("job1", 1*time.Hour)
    initialSlot := SlotKey(initialTime)

    // Reschedule - should advance by interval
    nextTime := b.CalculateNextRunPreserveRhythm("job1", 1*time.Hour)
    nextSlot := SlotKey(nextTime)

    expectedSlot := initialSlot + 4 // 1 hour = 4 slots
    assert.Equal(t, expectedSlot, nextSlot)
}

func TestCanMoveJob_ProtectionWindow(t *testing.T) {
    b := NewBucketMap()

    // Job running in 10 minutes - cannot move
    soon := time.Now().Add(10 * time.Minute)
    job := &domain.Job{ID: "job1", NextRunAt: &soon, Status: "scheduled"}

    reason, canMove := b.canMoveJob(job)
    assert.False(t, canMove)
    assert.Equal(t, "protection_window", reason)
}

func TestFullRebalance_SortsLongestFirst(t *testing.T) {
    b := NewBucketMap()

    hourlyJob := &domain.Job{ID: "hourly", IntervalMinutes: ptr(1), IntervalType: "hours"}
    dailyJob := &domain.Job{ID: "daily", IntervalMinutes: ptr(1), IntervalType: "days"}

    // Daily job (more constrained) should be placed first
    // Implementation detail: verify via placement order or resulting distribution
}
```

### Integration Tests

```go
func TestSchedulerIntegration_NewJobBalancing(t *testing.T) {
    // Start scheduler with empty DB
    // Add 10 jobs with same interval
    // Verify they're distributed across different slots
}

func TestSchedulerIntegration_RebalanceAPI(t *testing.T) {
    // Create clustered jobs
    // Call rebalance preview
    // Verify projected improvement
    // Execute rebalance
    // Verify actual distribution matches projection
}
```

### Load Tests

```go
func BenchmarkPlaceNewJob(b *testing.B) {
    bucketMap := NewBucketMap()
    // Pre-populate with 1000 jobs
    for i := 0; i < 1000; i++ {
        bucketMap.AddJob(fmt.Sprintf("job%d", i), SlotKey(time.Now().Add(time.Duration(i)*time.Minute)))
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        bucketMap.PlaceNewJob(fmt.Sprintf("new%d", i), 6*time.Hour)
    }
}
```

---

## Dashboard Integration

### Distribution Visualization

Add a "Schedule Distribution" panel showing:

1. **Bar chart**: Jobs per hour for next 24h
2. **Distribution score**: Single metric (0-1) indicating evenness
3. **Peak indicator**: Highlight hours with highest load
4. **Rebalance button**: Triggers preview modal

### Rebalance Flow

1. Operator clicks "Rebalance Schedule"
2. Modal shows preview: current vs projected distribution
3. List of jobs that would move, and jobs that would be skipped
4. "Confirm Rebalance" executes the operation
5. Success notification shows actual changes

---

## Observability

### Metrics (Prometheus)

```
# Distribution metrics
scheduler_distribution_score gauge
scheduler_slot_job_count{hour="0..23"} gauge
scheduler_peak_hour_job_count gauge

# Placement metrics
scheduler_placements_total{type="new|reschedule|rebalance"} counter
scheduler_placement_duration_seconds histogram

# Rebalance metrics
scheduler_rebalance_total counter
scheduler_rebalance_jobs_moved gauge
scheduler_rebalance_jobs_skipped gauge
```

### Logging

```
INFO  Bucket map rebuilt                    job_count=42 distribution_score=0.78
INFO  Placed new job                        job_id=abc123 slot=2026-02-04T10:15:00Z load=3
INFO  Rescheduled job (rhythm preserved)    job_id=abc123 old_slot=10:15 new_slot=11:15
INFO  Full rebalance completed              moved=12 skipped=3 old_score=0.65 new_score=0.91
WARN  Job skipped during rebalance          job_id=def456 reason=protection_window
```

---

## Open Questions (Resolved)

| Question | Resolution |
|----------|------------|
| Bucket granularity? | 15-minute slots (96 per 24h) |
| Recurring job behavior? | Rhythm preservation (Option A) |
| Auto-rebalancing? | No - event-driven + manual only |
| Multi-instance support? | Not needed (single scheduler) |
| Weighted scheduling? | Deferred (all jobs equal for now) |

---

## Future Enhancements

Potential features for future development:

1. **Weighted scheduling** - Assign cost to heavy crawlers, balance by cost not count
2. **Time-of-day preferences** - "Run during off-peak hours" option
3. **Distribution alerts** - Notify when distribution score drops below threshold
4. **Historical distribution** - Track distribution changes over time
5. **Automatic suggestion** - "Distribution degraded, consider rebalancing"

---

## Summary

This design provides a self-balancing scheduler that:

- **Spreads jobs evenly** via bucket-based placement
- **Preserves predictability** via rhythm preservation for recurring jobs
- **Adapts automatically** when sources/intervals change
- **Gives operators control** via manual rebalance with preview
- **Avoids thrashing** via protection window and cooldown rules

The implementation is incremental: BucketMap can be added without changing behavior, then enabled progressively for new jobs, API endpoints, and full integration.
