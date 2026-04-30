package monitoring

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"
)

const (
	bytesPerKilobyte          = 1024
	bytesPerMegabyte          = bytesPerKilobyte * bytesPerKilobyte
	gcPauseRingBufferSize     = 256
	nanosecondsPerMicrosecond = 1000
	nanosecondsPerMillisecond = nanosecondsPerMicrosecond * 1000
)

// MemoryHealth represents memory health metrics
type MemoryHealth struct {
	Timestamp          time.Time `json:"timestamp"`
	HeapAllocMB        float64   `json:"heap_alloc_mb"`
	HeapInuseMB        float64   `json:"heap_inuse_mb"`
	HeapIdleMB         float64   `json:"heap_idle_mb"`
	StackInuseMB       float64   `json:"stack_inuse_mb"`
	NumGC              uint32    `json:"num_gc"`
	NumGoroutine       int       `json:"num_goroutine"`
	GOMaxProcs         int       `json:"gomaxprocs"`
	LastGCPauseMs      float64   `json:"last_gc_pause_ms,omitempty"`
	BaselineHeapMB     float64   `json:"baseline_heap_mb,omitempty"`
	BaselineGoroutines int       `json:"baseline_goroutines,omitempty"`
}

// MemoryHealthHandler returns current memory statistics as JSON
// Can be registered with any HTTP router (Gin, standard http, etc.)
func MemoryHealthHandler(w http.ResponseWriter, r *http.Request) {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	health := newMemoryHealth(stats)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(health); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// MemoryHealthHandlerWithMonitor returns a handler that includes baseline metrics from a monitor
func MemoryHealthHandlerWithMonitor(monitor *MemoryMonitor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)

		health := newMemoryHealth(stats)

		// Add baseline metrics if monitor is provided
		if monitor != nil {
			baselineHeap, baselineGoroutines := monitor.GetBaseline()
			health.BaselineHeapMB = baselineHeap
			health.BaselineGoroutines = baselineGoroutines
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(health); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}
	}
}

func newMemoryHealth(stats runtime.MemStats) MemoryHealth {
	health := MemoryHealth{
		Timestamp:    time.Now().UTC(),
		HeapAllocMB:  bytesToMegabytes(stats.Alloc),
		HeapInuseMB:  bytesToMegabytes(stats.HeapInuse),
		HeapIdleMB:   bytesToMegabytes(stats.HeapIdle),
		StackInuseMB: bytesToMegabytes(stats.StackInuse),
		NumGC:        stats.NumGC,
		NumGoroutine: runtime.NumGoroutine(),
		GOMaxProcs:   runtime.GOMAXPROCS(0),
	}

	if stats.NumGC > 0 {
		health.LastGCPauseMs = nanosecondsToMilliseconds(
			stats.PauseNs[(stats.NumGC+gcPauseRingBufferSize-1)%gcPauseRingBufferSize],
		)
	}

	return health
}

func bytesToMegabytes(bytes uint64) float64 {
	return float64(bytes) / bytesPerMegabyte
}

func nanosecondsToMilliseconds(nanoseconds uint64) float64 {
	return float64(nanoseconds) / nanosecondsPerMillisecond
}
