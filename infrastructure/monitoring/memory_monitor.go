package monitoring

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

// MemoryMonitor tracks memory usage and detects potential leaks
type MemoryMonitor struct {
	mu                  sync.RWMutex
	baselineHeap        uint64
	baselineGoroutines  int
	threshold           float64 // e.g., 2.0 for 200% growth
	checkInterval       time.Duration
	warningCallback     func(report string)
	ticker              *time.Ticker
	stopCh              chan struct{}
}

// MemorySnapshot represents a point-in-time memory state
type MemorySnapshot struct {
	Timestamp       time.Time
	HeapAlloc       uint64
	HeapIdle        uint64
	HeapInuse       uint64
	StackInuse      uint64
	NumGC           uint32
	PauseTotalNs    uint64
	NumGoroutine    int
}

// NewMemoryMonitor creates a new memory monitor
// threshold: multiplier for growth detection (e.g., 2.0 = 200% growth triggers warning)
// checkInterval: how often to check for leaks (e.g., 5 * time.Minute)
func NewMemoryMonitor(threshold float64, checkInterval time.Duration) *MemoryMonitor {
	return &MemoryMonitor{
		threshold:     threshold,
		checkInterval: checkInterval,
		stopCh:        make(chan struct{}),
	}
}

// EstablishBaseline sets the baseline after service warmup
// Should be called after the service has been running for a few minutes
// to establish a stable baseline
func (m *MemoryMonitor) EstablishBaseline() {
	// Force GC to get clean baseline
	runtime.GC()

	// Wait a moment for GC to complete
	time.Sleep(100 * time.Millisecond)

	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	m.mu.Lock()
	m.baselineHeap = stats.Alloc
	m.baselineGoroutines = runtime.NumGoroutine()
	m.mu.Unlock()
}

// TakeSnapshot captures current memory state
func (m *MemoryMonitor) TakeSnapshot() MemorySnapshot {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	return MemorySnapshot{
		Timestamp:    time.Now(),
		HeapAlloc:    stats.Alloc,
		HeapIdle:     stats.HeapIdle,
		HeapInuse:    stats.HeapInuse,
		StackInuse:   stats.StackInuse,
		NumGC:        stats.NumGC,
		PauseTotalNs: stats.PauseTotalNs,
		NumGoroutine: runtime.NumGoroutine(),
	}
}

// CheckForLeaks compares current memory to baseline
// Returns (leaked bool, report string)
func (m *MemoryMonitor) CheckForLeaks() (leaked bool, report string) {
	m.mu.RLock()
	baselineHeap := m.baselineHeap
	baselineGoroutines := m.baselineGoroutines
	threshold := m.threshold
	m.mu.RUnlock()

	// If baseline not established, can't check for leaks
	if baselineHeap == 0 {
		return false, ""
	}

	snapshot := m.TakeSnapshot()

	// Check heap growth
	heapGrowth := float64(snapshot.HeapAlloc) / float64(baselineHeap)
	if heapGrowth > threshold {
		return true, fmt.Sprintf(
			"Memory leak detected: heap grew %.2fx (%.2f MB → %.2f MB)",
			heapGrowth,
			float64(baselineHeap)/1024/1024,
			float64(snapshot.HeapAlloc)/1024/1024,
		)
	}

	// Check goroutine growth
	goroutineGrowth := float64(snapshot.NumGoroutine) / float64(baselineGoroutines)
	if goroutineGrowth > threshold {
		return true, fmt.Sprintf(
			"Goroutine leak detected: count grew %.2fx (%d → %d)",
			goroutineGrowth,
			baselineGoroutines,
			snapshot.NumGoroutine,
		)
	}

	return false, ""
}

// StartMonitoring begins periodic leak checks
func (m *MemoryMonitor) StartMonitoring() {
	m.ticker = time.NewTicker(m.checkInterval)

	go func() {
		for {
			select {
			case <-m.ticker.C:
				if leaked, report := m.CheckForLeaks(); leaked {
					m.mu.RLock()
					callback := m.warningCallback
					m.mu.RUnlock()

					if callback != nil {
						callback(report)
					}
				}
			case <-m.stopCh:
				return
			}
		}
	}()
}

// StopMonitoring stops the periodic leak checks
func (m *MemoryMonitor) StopMonitoring() {
	if m.ticker != nil {
		m.ticker.Stop()
	}
	close(m.stopCh)
}

// SetWarningCallback sets callback for leak warnings
func (m *MemoryMonitor) SetWarningCallback(callback func(string)) {
	m.mu.Lock()
	m.warningCallback = callback
	m.mu.Unlock()
}

// GetBaseline returns the current baseline metrics
func (m *MemoryMonitor) GetBaseline() (heapMB float64, goroutines int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return float64(m.baselineHeap) / 1024 / 1024, m.baselineGoroutines
}
