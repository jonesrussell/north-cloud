package logs

import (
	"sort"
	"sync"
	"sync/atomic"
)

// percentageMultiplier is used to convert a ratio to a percentage.
const percentageMultiplier = 100

// LogMetrics collects metrics during job execution.
type LogMetrics struct {
	pagesDiscovered atomic.Int64
	pagesCrawled    atomic.Int64
	itemsExtracted  atomic.Int64
	errorsCount     atomic.Int64
	bytesReceived   atomic.Int64
	requestsTotal   atomic.Int64
	requestsFailed  atomic.Int64
	logsEmitted     atomic.Int64
	logsThrottled   atomic.Int64
	queueDepth      atomic.Int64
	queueMaxDepth   atomic.Int64
	queueEnqueued   atomic.Int64
	queueDequeued   atomic.Int64

	statusCodes sync.Map // map[int]*atomic.Int64
	errorCounts sync.Map // map[string]*errorTracker
}

type errorTracker struct {
	count   atomic.Int64
	lastURL atomic.Value // string
}

// NewLogMetrics creates a new metrics collector.
func NewLogMetrics() *LogMetrics {
	return &LogMetrics{}
}

func (m *LogMetrics) IncrementPagesDiscovered() { m.pagesDiscovered.Add(1) }
func (m *LogMetrics) IncrementPagesCrawled()    { m.pagesCrawled.Add(1) }
func (m *LogMetrics) IncrementItemsExtracted()  { m.itemsExtracted.Add(1) }
func (m *LogMetrics) IncrementErrors()          { m.errorsCount.Add(1) }
func (m *LogMetrics) IncrementLogsEmitted()     { m.logsEmitted.Add(1) }
func (m *LogMetrics) IncrementThrottled()       { m.logsThrottled.Add(1) }

func (m *LogMetrics) RecordStatusCode(code int) {
	counter, _ := m.statusCodes.LoadOrStore(code, &atomic.Int64{})
	if c, ok := counter.(*atomic.Int64); ok {
		c.Add(1)
	}
}

func (m *LogMetrics) RecordError(msg, url string) {
	tracker, _ := m.errorCounts.LoadOrStore(msg, &errorTracker{})
	if t, ok := tracker.(*errorTracker); ok {
		t.count.Add(1)
		t.lastURL.Store(url)
	}
}

// BuildSummary returns the current metrics as a JobSummary.
func (m *LogMetrics) BuildSummary() *JobSummary {
	summary := &JobSummary{
		PagesDiscovered: m.pagesDiscovered.Load(),
		PagesCrawled:    m.pagesCrawled.Load(),
		ItemsExtracted:  m.itemsExtracted.Load(),
		ErrorsCount:     m.errorsCount.Load(),
		BytesFetched:    m.bytesReceived.Load(),
		RequestsTotal:   m.requestsTotal.Load(),
		RequestsFailed:  m.requestsFailed.Load(),
		LogsEmitted:     m.logsEmitted.Load(),
		LogsThrottled:   m.logsThrottled.Load(),
		QueueMaxDepth:   m.queueMaxDepth.Load(),
		QueueEnqueued:   m.queueEnqueued.Load(),
		QueueDequeued:   m.queueDequeued.Load(),
		StatusCodes:     make(map[int]int64),
	}

	// Collect status codes
	m.statusCodes.Range(func(key, value any) bool {
		code, codeOK := key.(int)
		counter, counterOK := value.(*atomic.Int64)
		if codeOK && counterOK {
			summary.StatusCodes[code] = counter.Load()
		}
		return true
	})

	// Collect top 5 errors
	const maxTopErrors = 5
	summary.TopErrors = m.getTopErrors(maxTopErrors)

	// Calculate throttle percent
	total := summary.LogsEmitted + summary.LogsThrottled
	if total > 0 {
		summary.ThrottlePercent = float64(summary.LogsThrottled) / float64(total) * percentageMultiplier
	}

	return summary
}

func (m *LogMetrics) getTopErrors(n int) []ErrorSummary {
	var errors []ErrorSummary

	m.errorCounts.Range(func(key, value any) bool {
		msg, msgOK := key.(string)
		tracker, trackerOK := value.(*errorTracker)
		if msgOK && trackerOK {
			lastURL, _ := tracker.lastURL.Load().(string)
			errors = append(errors, ErrorSummary{
				Message: msg,
				Count:   int(tracker.count.Load()),
				LastURL: lastURL,
			})
		}
		return true
	})

	sort.Slice(errors, func(i, j int) bool {
		return errors[i].Count > errors[j].Count
	})

	if len(errors) > n {
		errors = errors[:n]
	}

	return errors
}

// LogsEmitted returns the current count of emitted logs.
func (m *LogMetrics) LogsEmitted() int64 {
	return m.logsEmitted.Load()
}

// QueueDepth returns the current queue depth.
func (m *LogMetrics) QueueDepth() int64 {
	return m.queueDepth.Load()
}
