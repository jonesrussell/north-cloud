package crawler

import (
	"sync"
)

// LifecycleManager manages the lifecycle state of a crawler operation.
// It handles completion signaling, synchronization, and waiting for goroutines.
type LifecycleManager struct {
	done     chan struct{}
	doneOnce sync.Once
	wg       sync.WaitGroup
	mu       sync.RWMutex
}

// NewLifecycleManager creates a new lifecycle manager.
func NewLifecycleManager() *LifecycleManager {
	return &LifecycleManager{
		done:     make(chan struct{}),
		doneOnce: sync.Once{},
	}
}

// Reset prepares the lifecycle manager for a new execution.
// This is called at the start of each crawl to support concurrent jobs.
func (lm *LifecycleManager) Reset() {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	lm.done = make(chan struct{})
	lm.doneOnce = sync.Once{}
}

// Done returns a channel that's closed when the operation is complete.
func (lm *LifecycleManager) Done() <-chan struct{} {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.done
}

// SignalDone closes the done channel to signal completion.
// Safe to call multiple times - only the first call has effect.
func (lm *LifecycleManager) SignalDone() {
	lm.doneOnce.Do(func() {
		close(lm.done)
	})
}

// Add increments the WaitGroup counter.
func (lm *LifecycleManager) Add(delta int) {
	lm.wg.Add(delta)
}

// Done decrements the WaitGroup counter.
func (lm *LifecycleManager) WorkDone() {
	lm.wg.Done()
}

// Wait blocks until the WaitGroup counter is zero.
func (lm *LifecycleManager) Wait() {
	lm.wg.Wait()
}

// WaitWithChannel returns a channel that's closed when WaitGroup reaches zero.
// Useful for select statements with timeouts.
func (lm *LifecycleManager) WaitWithChannel() <-chan struct{} {
	waitDone := make(chan struct{})
	go func() {
		lm.wg.Wait()
		close(waitDone)
	}()
	return waitDone
}
