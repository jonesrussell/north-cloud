package orchestrator

import (
	"time"

	"github.com/jonesrussell/north-cloud/social-publisher/internal/domain"
)

// PublishJob represents a unit of work for the publish pipeline.
type PublishJob struct {
	ContentID  string
	DeliveryID string
	Platform   string
	Account    string
	Message    *domain.PublishMessage
	IsRetry    bool
}

// PriorityQueue implements a two-channel priority model where real-time
// messages are always drained before retry traffic.
type PriorityQueue struct {
	realtime chan PublishJob
	retries  chan PublishJob
}

// NewPriorityQueue creates a queue with the given channel buffer sizes.
func NewPriorityQueue(realtimeSize, retrySize int) *PriorityQueue {
	return &PriorityQueue{
		realtime: make(chan PublishJob, realtimeSize),
		retries:  make(chan PublishJob, retrySize),
	}
}

// EnqueueRealtime adds a job to the high-priority real-time channel.
func (pq *PriorityQueue) EnqueueRealtime(job PublishJob) {
	pq.realtime <- job
}

// EnqueueRetry adds a job to the lower-priority retry channel.
func (pq *PriorityQueue) EnqueueRetry(job PublishJob) {
	pq.retries <- job
}

// Dequeue returns the next job, preferring real-time over retry.
// Returns false if no job is available within the timeout.
func (pq *PriorityQueue) Dequeue(timeout time.Duration) (PublishJob, bool) {
	// Try realtime first (non-blocking)
	select {
	case job := <-pq.realtime:
		return job, true
	default:
	}

	// Block on both with timeout
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case job := <-pq.realtime:
		return job, true
	case job := <-pq.retries:
		return job, true
	case <-timer.C:
		return PublishJob{}, false
	}
}
