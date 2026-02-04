// crawler/internal/scheduler/bucket_map_test.go
package scheduler_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/scheduler"
)

func TestSlotKey(t *testing.T) {
	t.Helper()

	// Times in same 15-min window get same key
	t1 := time.Date(2026, 2, 4, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 2, 4, 10, 14, 59, 0, time.UTC)
	if scheduler.SlotKey(t1) != scheduler.SlotKey(t2) {
		t.Errorf("expected same slot key for times in same 15-min window")
	}

	// Times in different windows get different keys
	t3 := time.Date(2026, 2, 4, 10, 15, 0, 0, time.UTC)
	if scheduler.SlotKey(t1) == scheduler.SlotKey(t3) {
		t.Errorf("expected different slot keys for times in different 15-min windows")
	}
}

func TestSlotTime(t *testing.T) {
	t.Helper()

	// Round-trip: SlotTime(SlotKey(t)) should return start of slot
	original := time.Date(2026, 2, 4, 10, 7, 30, 0, time.UTC)
	key := scheduler.SlotKey(original)
	result := scheduler.SlotTime(key)

	expected := time.Date(2026, 2, 4, 10, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("SlotTime(%d) = %v, want %v", key, result, expected)
	}
}

func TestBucketMap_AddJob(t *testing.T) {
	t.Helper()

	bm := scheduler.NewBucketMap()
	now := time.Now()
	slotKey := scheduler.SlotKey(now)

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

	bm := scheduler.NewBucketMap()
	slotKey := scheduler.SlotKey(time.Now())

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

	bm := scheduler.NewBucketMap()
	now := time.Now()
	oldSlot := scheduler.SlotKey(now)
	newSlot := scheduler.SlotKey(now.Add(1 * time.Hour))

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

	bm := scheduler.NewBucketMap()
	slotKey := scheduler.SlotKey(time.Now())

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

func TestBucketMap_FindLeastLoaded(t *testing.T) {
	t.Helper()

	bm := scheduler.NewBucketMap()
	now := time.Now()

	// Pre-populate: slot at +1h has 3 jobs, +2h has 1 job, +3h has 2 jobs
	slot1h := scheduler.SlotKey(now.Add(1 * time.Hour))
	slot2h := scheduler.SlotKey(now.Add(2 * time.Hour))
	slot3h := scheduler.SlotKey(now.Add(3 * time.Hour))

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

	bm := scheduler.NewBucketMap()
	now := time.Now()
	start := now
	end := now.Add(1 * time.Hour) // 4 slots

	// No jobs - should return first slot
	result := bm.FindLeastLoaded(start, end)
	expected := scheduler.SlotKey(start)
	if result != expected {
		t.Errorf("FindLeastLoaded() = %d, want %d (first slot when all empty)", result, expected)
	}
}

func TestBucketMap_PlaceNewJob(t *testing.T) {
	t.Helper()

	bm := scheduler.NewBucketMap()
	now := time.Now()

	// Pre-populate first hour with 3 jobs each slot
	const slotsInFirstHour = 4
	for i := range slotsInFirstHour {
		slot := scheduler.SlotKey(now.Add(time.Duration(i*15) * time.Minute))
		bm.AddJob(fmt.Sprintf("existing-%d-a", i), slot)
		bm.AddJob(fmt.Sprintf("existing-%d-b", i), slot)
		bm.AddJob(fmt.Sprintf("existing-%d-c", i), slot)
	}

	// Place new job with 6-hour interval
	interval := 6 * time.Hour
	result := bm.PlaceNewJob("new-job", interval)

	// Should find a slot - verify it's tracked
	resultSlot := scheduler.SlotKey(result)

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

	bm := scheduler.NewBucketMap()
	now := time.Now()

	// Create a gap: slots 0,1,2 have jobs, slot 3 is empty
	bm.AddJob("job-0", scheduler.SlotKey(now))
	bm.AddJob("job-1", scheduler.SlotKey(now.Add(15*time.Minute)))
	bm.AddJob("job-2", scheduler.SlotKey(now.Add(30*time.Minute)))
	// Slot at +45min is empty

	// Place new job
	result := bm.PlaceNewJob("new-job", 1*time.Hour)
	resultSlot := scheduler.SlotKey(result)

	// Should find the empty slot at +45min (or any empty slot)
	if bm.GetSlotLoad(resultSlot) != 1 {
		t.Errorf("PlaceNewJob placed in slot with load %d, expected to find empty slot",
			bm.GetSlotLoad(resultSlot)-1) // -1 because we just added
	}
}

func TestBucketMap_CalculateNextRunPreserveRhythm(t *testing.T) {
	t.Helper()

	bm := scheduler.NewBucketMap()

	// Place initial job
	initialTime := bm.PlaceNewJob("job-1", 1*time.Hour)
	initialSlot := scheduler.SlotKey(initialTime)

	// Reschedule - should advance by interval (4 slots for 1 hour)
	nextTime := bm.CalculateNextRunPreserveRhythm("job-1", 1*time.Hour)
	nextSlot := scheduler.SlotKey(nextTime)

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

	bm := scheduler.NewBucketMap()

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
