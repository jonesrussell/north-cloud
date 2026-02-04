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
	// ProtectionWindow is the minimum time before execution when a job cannot be moved.
	ProtectionWindow = 30 * time.Minute
	// PlacementCooldown is the minimum time between job placements.
	PlacementCooldown = 1 * time.Hour
	// searchWindowDefault is the default search window for new job placement.
	searchWindowDefault = 24 * time.Hour
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
