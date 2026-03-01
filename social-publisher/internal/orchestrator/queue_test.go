package orchestrator_test

import (
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/social-publisher/internal/orchestrator"
	"github.com/stretchr/testify/assert"
)

func TestPriorityQueue_RealtimeFirst(t *testing.T) {
	q := orchestrator.NewPriorityQueue(10, 10)

	q.EnqueueRealtime(orchestrator.PublishJob{ContentID: "realtime-1"})
	q.EnqueueRetry(orchestrator.PublishJob{ContentID: "retry-1"})

	job, ok := q.Dequeue(100 * time.Millisecond)
	assert.True(t, ok)
	assert.Equal(t, "realtime-1", job.ContentID)
}

func TestPriorityQueue_RetryWhenRealtimeEmpty(t *testing.T) {
	q := orchestrator.NewPriorityQueue(10, 10)

	q.EnqueueRetry(orchestrator.PublishJob{ContentID: "retry-1"})

	job, ok := q.Dequeue(100 * time.Millisecond)
	assert.True(t, ok)
	assert.Equal(t, "retry-1", job.ContentID)
}

func TestPriorityQueue_TimeoutWhenEmpty(t *testing.T) {
	q := orchestrator.NewPriorityQueue(10, 10)

	_, ok := q.Dequeue(50 * time.Millisecond)
	assert.False(t, ok)
}
