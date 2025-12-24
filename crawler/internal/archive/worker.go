// Package archive provides HTML archiving functionality using MinIO object storage.
package archive

import (
	"math"
	"sync"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/logger"
)

// UploadWorker processes upload tasks asynchronously.
type UploadWorker struct {
	archiver *Archiver
	logger   logger.Interface
	wg       sync.WaitGroup
	stopCh   chan struct{}
}

// NewUploadWorker creates a new upload worker.
func NewUploadWorker(archiver *Archiver, log logger.Interface) *UploadWorker {
	return &UploadWorker{
		archiver: archiver,
		logger:   log,
		stopCh:   make(chan struct{}),
	}
}

// Start starts the upload worker.
func (w *UploadWorker) Start() {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.logger.Info("Upload worker started")

		for {
			select {
			case task := <-w.archiver.uploadChan:
				w.processTask(task)
			case <-w.stopCh:
				w.logger.Info("Upload worker stopping, draining queue")
				w.drainQueue()
				w.logger.Info("Upload worker stopped")
				return
			}
		}
	}()
}

// Stop stops the upload worker and waits for it to finish.
func (w *UploadWorker) Stop() {
	close(w.stopCh)
	w.wg.Wait()
}

// processTask processes a single upload task with retry logic.
func (w *UploadWorker) processTask(task *UploadTask) {
	var lastErr error

	for attempt := 0; attempt <= w.archiver.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Calculate backoff with overflow protection
			// Limit shift to prevent integer overflow
			shift := attempt - 1
			if shift > maxBackoffShift {
				shift = maxBackoffShift
			}
			// Use math.Pow to calculate exponential backoff safely
			backoffSeconds := int64(math.Pow(backoffBase, float64(shift)))
			backoff := time.Duration(backoffSeconds) * time.Second
			w.logger.Debug("Retrying upload",
				"attempt", attempt,
				"backoff", backoff,
				"url", task.URL)
			time.Sleep(backoff)
		}

		err := w.archiver.uploadHTML(task.Ctx, task)
		if err == nil {
			// Success
			return
		}

		lastErr = err
		w.logger.Warn("Upload failed",
			"attempt", attempt+1,
			"max_retries", w.archiver.config.MaxRetries+1,
			"error", err,
			"url", task.URL)
	}

	// All retries exhausted
	if w.archiver.config.FailSilently {
		w.logger.Error("Upload failed after all retries, continuing",
			"error", lastErr,
			"url", task.URL)
	} else {
		w.logger.Error("Upload failed after all retries",
			"error", lastErr,
			"url", task.URL)
	}
}

const (
	// queueDrainTimeout is the timeout for draining the upload queue.
	queueDrainTimeout = 10 * time.Second
	// maxBackoffShift is the maximum bit shift for exponential backoff to prevent overflow.
	maxBackoffShift = 30
	// backoffBase is the base for exponential backoff calculation.
	backoffBase = 2.0
)

// drainQueue drains the upload queue with a timeout.
func (w *UploadWorker) drainQueue() {
	deadline := time.Now().Add(queueDrainTimeout)
	drained := 0

	for {
		select {
		case task := <-w.archiver.uploadChan:
			if time.Now().After(deadline) {
				w.logger.Warn("Queue drain timeout reached, dropping task",
					"url", task.URL,
					"drained", drained)
				continue
			}
			w.processTask(task)
			drained++
		default:
			// Queue is empty
			if drained > 0 {
				w.logger.Info("Queue drained successfully", "tasks_processed", drained)
			}
			return
		}
	}
}
