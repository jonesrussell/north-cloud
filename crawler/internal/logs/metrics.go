package logs

import (
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// percentageMultiplier is used to convert a ratio to a percentage.
const percentageMultiplier = 100

// responseTimeUnset indicates that no response time has been recorded yet.
const responseTimeUnset = -1

// nanosPerMillisecond converts nanoseconds to milliseconds.
const nanosPerMillisecond = float64(time.Millisecond)

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

	// Visibility counters
	cloudflareBlocks  atomic.Int64
	rateLimits        atomic.Int64
	responseTimeTotal atomic.Int64 // nanoseconds
	responseTimeMin   atomic.Int64 // nanoseconds (-1 = unset)
	responseTimeMax   atomic.Int64 // nanoseconds
	skippedNonHTML    atomic.Int64
	skippedMaxDepth   atomic.Int64
	skippedRobotsTxt  atomic.Int64

	// Extraction quality (indexed items with empty title/body)
	itemsExtractedEmptyTitle atomic.Int64
	itemsExtractedEmptyBody  atomic.Int64

	statusCodes     sync.Map // map[int]*atomic.Int64
	errorCounts     sync.Map // map[string]*errorTracker
	errorCategories sync.Map // map[string]*atomic.Int64
}

type errorTracker struct {
	count   atomic.Int64
	lastURL atomic.Value // string
}

// NewLogMetrics creates a new metrics collector.
func NewLogMetrics() *LogMetrics {
	m := &LogMetrics{}
	m.responseTimeMin.Store(responseTimeUnset)
	return m
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

func (m *LogMetrics) IncrementRequestsTotal()    { m.requestsTotal.Add(1) }
func (m *LogMetrics) IncrementRequestsFailed()   { m.requestsFailed.Add(1) }
func (m *LogMetrics) IncrementCloudflare()       { m.cloudflareBlocks.Add(1) }
func (m *LogMetrics) IncrementRateLimit()        { m.rateLimits.Add(1) }
func (m *LogMetrics) IncrementSkippedNonHTML()   { m.skippedNonHTML.Add(1) }
func (m *LogMetrics) IncrementSkippedMaxDepth()  { m.skippedMaxDepth.Add(1) }
func (m *LogMetrics) IncrementSkippedRobotsTxt() { m.skippedRobotsTxt.Add(1) }

// RecordExtracted records extraction quality for one indexed item.
func (m *LogMetrics) RecordExtracted(emptyTitle, emptyBody bool) {
	if emptyTitle {
		m.itemsExtractedEmptyTitle.Add(1)
	}
	if emptyBody {
		m.itemsExtractedEmptyBody.Add(1)
	}
}

// RecordBytes adds to the total bytes received counter.
func (m *LogMetrics) RecordBytes(n int64) { m.bytesReceived.Add(n) }

// RecordResponseTime records a response time duration, updating total, min, and max.
func (m *LogMetrics) RecordResponseTime(d time.Duration) {
	ns := d.Nanoseconds()
	m.responseTimeTotal.Add(ns)

	// Update min with CAS loop
	for {
		cur := m.responseTimeMin.Load()
		if cur != responseTimeUnset && cur <= ns {
			break
		}
		if m.responseTimeMin.CompareAndSwap(cur, ns) {
			break
		}
	}

	// Update max with CAS loop
	for {
		cur := m.responseTimeMax.Load()
		if cur >= ns {
			break
		}
		if m.responseTimeMax.CompareAndSwap(cur, ns) {
			break
		}
	}
}

// RecordErrorCategory increments the count for an error category.
func (m *LogMetrics) RecordErrorCategory(category string) {
	counter, _ := m.errorCategories.LoadOrStore(category, &atomic.Int64{})
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
		PagesDiscovered:          m.pagesDiscovered.Load(),
		PagesCrawled:             m.pagesCrawled.Load(),
		ItemsExtracted:           m.itemsExtracted.Load(),
		ErrorsCount:              m.errorsCount.Load(),
		BytesFetched:             m.bytesReceived.Load(),
		RequestsTotal:            m.requestsTotal.Load(),
		RequestsFailed:           m.requestsFailed.Load(),
		LogsEmitted:              m.logsEmitted.Load(),
		LogsThrottled:            m.logsThrottled.Load(),
		QueueMaxDepth:            m.queueMaxDepth.Load(),
		QueueEnqueued:            m.queueEnqueued.Load(),
		QueueDequeued:            m.queueDequeued.Load(),
		StatusCodes:              make(map[int]int64),
		CloudflareBlocks:         m.cloudflareBlocks.Load(),
		RateLimits:               m.rateLimits.Load(),
		SkippedNonHTML:           m.skippedNonHTML.Load(),
		SkippedMaxDepth:          m.skippedMaxDepth.Load(),
		SkippedRobotsTxt:         m.skippedRobotsTxt.Load(),
		ItemsExtractedEmptyTitle: m.itemsExtractedEmptyTitle.Load(),
		ItemsExtractedEmptyBody:  m.itemsExtractedEmptyBody.Load(),
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

	// Calculate response time stats
	m.buildResponseTimeStats(summary)

	// Collect error categories
	summary.ErrorCategories = m.buildErrorCategories()

	return summary
}

// buildResponseTimeStats populates response time avg/min/max on the summary.
func (m *LogMetrics) buildResponseTimeStats(summary *JobSummary) {
	reqTotal := summary.RequestsTotal
	if reqTotal > 0 {
		totalNs := m.responseTimeTotal.Load()
		summary.ResponseTimeAvgMs = float64(totalNs) / float64(reqTotal) / nanosPerMillisecond
		summary.ResponseTimeAvgMs = math.Round(summary.ResponseTimeAvgMs*percentageMultiplier) / percentageMultiplier
	}

	minNs := m.responseTimeMin.Load()
	if minNs != responseTimeUnset {
		summary.ResponseTimeMinMs = float64(minNs) / nanosPerMillisecond
		summary.ResponseTimeMinMs = math.Round(summary.ResponseTimeMinMs*percentageMultiplier) / percentageMultiplier
	}

	maxNs := m.responseTimeMax.Load()
	if maxNs > 0 {
		summary.ResponseTimeMaxMs = float64(maxNs) / nanosPerMillisecond
		summary.ResponseTimeMaxMs = math.Round(summary.ResponseTimeMaxMs*percentageMultiplier) / percentageMultiplier
	}
}

// buildErrorCategories collects error categories from the sync.Map.
func (m *LogMetrics) buildErrorCategories() map[string]int64 {
	categories := make(map[string]int64)
	m.errorCategories.Range(func(key, value any) bool {
		cat, catOK := key.(string)
		counter, counterOK := value.(*atomic.Int64)
		if catOK && counterOK {
			categories[cat] = counter.Load()
		}
		return true
	})
	if len(categories) == 0 {
		return nil
	}
	return categories
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

// PagesCrawled returns the current count of pages crawled.
func (m *LogMetrics) PagesCrawled() int64 {
	return m.pagesCrawled.Load()
}

// ItemsExtracted returns the current count of items extracted.
func (m *LogMetrics) ItemsExtracted() int64 {
	return m.itemsExtracted.Load()
}

// ErrorsCount returns the current count of errors.
func (m *LogMetrics) ErrorsCount() int64 {
	return m.errorsCount.Load()
}

// ItemsExtractedEmptyTitle returns the count of indexed items with empty title.
func (m *LogMetrics) ItemsExtractedEmptyTitle() int64 {
	return m.itemsExtractedEmptyTitle.Load()
}

// ItemsExtractedEmptyBody returns the count of indexed items with empty body.
func (m *LogMetrics) ItemsExtractedEmptyBody() int64 {
	return m.itemsExtractedEmptyBody.Load()
}
