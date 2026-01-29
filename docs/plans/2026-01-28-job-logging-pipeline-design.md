# Job Logging Pipeline Design

**Date:** 2026-01-28
**Status:** Draft
**Author:** Claude (with user requirements)

## Overview

Design a production-grade job logging pipeline that routes all crawler logs through the job log writer, with configurable verbosity, buffered SSE streaming, and log throttling.

## Goals

1. **Configurable verbosity levels** - quiet, normal, debug, trace
2. **Unified job log writer** - All crawler logs route through job writer → SSE + MinIO + stdout
3. **Buffered SSE streaming** - Replay last N lines on connect, then stream live
4. **Crawler instrumentation** - `JobLogger` interface injected into crawler
5. **Structured fields** - job_id, url, duration_ms on every log line
6. **Log throttling** - Rate limiting for debug/trace modes
7. **Job summary** - Append final stats at job completion

---

## Section 1: Verbosity Levels

### Configuration Model

```go
// crawler/internal/logs/verbosity.go

// Verbosity defines the log detail level for job execution.
type Verbosity string

const (
    // VerbosityQuiet - Only scheduler + major crawler milestones
    // ~5-10 lines per job
    VerbosityQuiet Verbosity = "quiet"

    // VerbosityNormal - Pages discovered, pages crawled, items extracted, errors
    // ~50-100 lines per job
    VerbosityNormal Verbosity = "normal"

    // VerbosityDebug - Every URL visited, timing info, extraction details
    // ~500-2000 lines per job
    VerbosityDebug Verbosity = "debug"

    // VerbosityTrace - Extremely detailed (local debugging only)
    // Unlimited, includes internal state changes
    VerbosityTrace Verbosity = "trace"
)

// VerbosityConfig defines what gets logged at each level.
type VerbosityConfig struct {
    // Level is the current verbosity level
    Level Verbosity `env:"JOB_LOGS_VERBOSITY" yaml:"verbosity"`

    // MaxLogsPerSecond limits log rate (0 = unlimited)
    // Only applies to debug/trace levels
    MaxLogsPerSecond int `env:"JOB_LOGS_MAX_PER_SEC" yaml:"max_logs_per_sec"`

    // AlsoLogToStdout mirrors job logs to infrastructure logger
    AlsoLogToStdout bool `env:"JOB_LOGS_ALSO_STDOUT" yaml:"also_log_to_stdout"`
}
```

### What Gets Logged at Each Level

| Event | quiet | normal | debug | trace |
|-------|-------|--------|-------|-------|
| Job started/completed | ✓ | ✓ | ✓ | ✓ |
| Job failed with error | ✓ | ✓ | ✓ | ✓ |
| Page discovered | | ✓ | ✓ | ✓ |
| Page crawled successfully | | ✓ | ✓ | ✓ |
| Content item extracted | | ✓ | ✓ | ✓ |
| Error (timeout, 404, etc) | | ✓ | ✓ | ✓ |
| URL visited (every request) | | | ✓ | ✓ |
| Link skipped (domain/depth) | | | ✓ | ✓ |
| Response timing | | | ✓ | ✓ |
| Extraction field details | | | ✓ | ✓ |
| Internal state changes | | | | ✓ |
| Colly callbacks fired | | | | ✓ |

### Event Categories

Event categories augment verbosity levels, enabling UI filtering and archive searchability.

```go
// crawler/internal/logs/category.go

// Category defines the type of log event for filtering.
type Category string

const (
    // CategoryLifecycle - start, stop, shutdown, summary
    CategoryLifecycle Category = "crawler.lifecycle"

    // CategoryFetch - URL fetch attempts, retries, status codes
    CategoryFetch Category = "crawler.fetch"

    // CategoryExtract - content extraction, selectors, parse results
    CategoryExtract Category = "crawler.extract"

    // CategoryError - network errors, parse failures, unexpected conditions
    CategoryError Category = "crawler.error"

    // CategoryRateLimit - throttling, backoff, delays
    CategoryRateLimit Category = "crawler.rate_limit"

    // CategoryQueue - enqueue/dequeue events, depth changes
    CategoryQueue Category = "crawler.queue"

    // CategoryMetrics - timing, durations, counts
    CategoryMetrics Category = "crawler.metrics"
)
```

### Log Line Examples

```
[info]  crawler.lifecycle  Job started                    job_id=abc123 source_id=xyz
[info]  crawler.fetch      Page fetched                   url="https://..." status=200 duration_ms=142
[info]  crawler.extract    Content extracted              url="https://..." items=3
[warn]  crawler.error      Request timeout                url="https://..." error="context deadline exceeded"
[debug] crawler.queue      Link enqueued                  url="https://..." depth=2
[debug] crawler.rate_limit Throttling request             delay_ms=2000
[info]  crawler.lifecycle  Job completed                  pages=47 items=12 errors=2 duration_ms=45230
```

### Environment Variables

```bash
# New variables to add to .env.example
JOB_LOGS_VERBOSITY=normal       # quiet|normal|debug|trace
JOB_LOGS_MAX_PER_SEC=50         # Rate limit for debug/trace (0=unlimited)
JOB_LOGS_ALSO_STDOUT=false      # Also mirror to container stdout
```

---

## Section 2: JobLogger Interface

### Core Interface

The `JobLogger` interface is injected into the crawler at runtime. It writes to SSE, MinIO archive buffer, and optionally stdout.

```go
// crawler/internal/logs/job_logger.go

// JobLogger provides structured logging for job execution.
// All methods are safe for concurrent use.
type JobLogger interface {
    // Core logging methods
    Info(category Category, msg string, fields ...Field)
    Warn(category Category, msg string, fields ...Field)
    Error(category Category, msg string, fields ...Field)
    Debug(category Category, msg string, fields ...Field)

    // Lifecycle events (always logged regardless of verbosity)
    JobStarted(sourceID, url string)
    JobCompleted(summary *JobSummary)
    JobFailed(err error)

    // Verbosity check (for expensive operations)
    IsDebugEnabled() bool
    IsTraceEnabled() bool

    // WithFields returns a scoped logger with pre-set fields.
    // Useful for reducing noise in loops:
    //   pageLogger := jobLogger.WithFields(logs.URL(pageURL))
    //   pageLogger.Info(CategoryFetch, "fetch started")
    WithFields(fields ...Field) JobLogger

    // Flush pending logs (called at job end)
    Flush() error
}

// Field represents a structured log field.
type Field struct {
    Key   string
    Value any
}

// Helper constructors for fields
func String(key, value string) Field           { return Field{Key: key, Value: value} }
func Int(key string, value int) Field          { return Field{Key: key, Value: value} }
func Int64(key string, value int64) Field      { return Field{Key: key, Value: value} }
func Duration(key string, d time.Duration) Field { return Field{Key: key, Value: d.Milliseconds()} }
func Err(err error) Field                      { return Field{Key: "error", Value: err.Error()} }
func URL(url string) Field                     { return Field{Key: "url", Value: url} }
```

### JobSummary Structure

```go
// JobSummary contains final statistics appended to job logs.
type JobSummary struct {
    PagesDiscovered int64         `json:"pages_discovered"`
    PagesCrawled    int64         `json:"pages_crawled"`
    ItemsExtracted  int64         `json:"items_extracted"`
    ErrorsCount     int64         `json:"errors_count"`
    Duration        time.Duration `json:"duration_ms"`
    BytesFetched    int64         `json:"bytes_fetched"`

    // Breakdown by status code
    StatusCodes map[int]int64 `json:"status_codes,omitempty"`

    // Top errors (if any)
    TopErrors []ErrorSummary `json:"top_errors,omitempty"`
}

type ErrorSummary struct {
    Message string `json:"message"`
    Count   int    `json:"count"`
}
```

### Implementation: jobLogger

```go
// crawler/internal/logs/job_logger_impl.go

type jobLogger struct {
    jobID       string
    executionID string
    verbosity   Verbosity
    category    Category

    // Output targets
    buffer    Buffer      // In-memory buffer for archive
    publisher Publisher   // SSE publisher
    stdout    infralogger.Logger // Optional stdout mirror

    // Throttling (for debug/trace)
    throttler *rateLimiter

    // Metrics collection
    metrics   *logMetrics

    mu sync.Mutex
}

// NewJobLogger creates a logger for a specific job execution.
func NewJobLogger(
    jobID, executionID string,
    verbosity Verbosity,
    buffer Buffer,
    publisher Publisher,
    stdout infralogger.Logger, // nil to disable stdout
    maxLogsPerSec int,
) JobLogger {
    return &jobLogger{
        jobID:       jobID,
        executionID: executionID,
        verbosity:   verbosity,
        buffer:      buffer,
        publisher:   publisher,
        stdout:      stdout,
        throttler:   newRateLimiter(maxLogsPerSec),
        metrics:     newLogMetrics(),
    }
}

func (l *jobLogger) Info(category Category, msg string, fields ...Field) {
    l.log("info", category, msg, fields)
}

func (l *jobLogger) Debug(category Category, msg string, fields ...Field) {
    if l.verbosity != VerbosityDebug && l.verbosity != VerbosityTrace {
        return // Skip debug logs unless verbosity allows
    }

    // Apply throttling for debug level
    if !l.throttler.Allow() {
        l.metrics.IncrementThrottled()
        return
    }

    l.log("debug", category, msg, fields)
}

func (l *jobLogger) log(level string, category Category, msg string, fields []Field) {
    entry := LogEntry{
        Timestamp: time.Now(),
        Level:     level,
        Message:   msg,
        Category:  string(category),
        JobID:     l.jobID,
        ExecID:    l.executionID,
        Fields:    l.fieldsToMap(fields),
    }

    // Write to buffer (for archive)
    l.buffer.Write(entry)

    // Publish to SSE
    if l.publisher != nil {
        l.publisher.PublishLogLine(context.Background(), entry)
    }

    // Mirror to stdout if enabled
    if l.stdout != nil {
        l.logToStdout(level, category, msg, fields)
    }

    l.metrics.IncrementWritten()
}
```

### Crawler Integration Point

```go
// crawler/internal/crawler/crawler.go

type Crawler struct {
    // ... existing fields ...

    // Job-scoped logger (set per execution, nil when not running)
    jobLogger logs.JobLogger
}

// SetJobLogger sets the job-scoped logger for the current execution.
// Must be called before Start() by the scheduler.
func (c *Crawler) SetJobLogger(logger logs.JobLogger) {
    c.jobLogger = logger
}

// log writes to the job logger if available, otherwise to infrastructure logger.
func (c *Crawler) log(level string, category logs.Category, msg string, fields ...logs.Field) {
    if c.jobLogger != nil {
        switch level {
        case "info":
            c.jobLogger.Info(category, msg, fields...)
        case "warn":
            c.jobLogger.Warn(category, msg, fields...)
        case "error":
            c.jobLogger.Error(category, msg, fields...)
        case "debug":
            c.jobLogger.Debug(category, msg, fields...)
        }
    }
    // Note: Don't fallback to c.logger - the scheduler handles stdout mirroring
}
```

---

## Section 3: Crawler Instrumentation

### Overview

Replace all `c.logger.*` calls in the crawler package with `jobLogger.*` calls. The crawler currently has **~45 log statements** across 6 files that need conversion.

### File-by-File Conversion Map

#### collector.go (12 statements)

| Line | Current | New Category | Verbosity |
|------|---------|--------------|-----------|
| 39 | `c.logger.Debug("Setting up collector")` | `CategoryLifecycle` | debug |
| 64 | `c.logger.Error("Failed to parse rate limit")` | `CategoryError` | normal |
| 80 | `c.logger.Warn("TLS certificate verification disabled")` | `CategoryError` | quiet |
| 87 | `c.logger.Debug("Collector configured")` | `CategoryLifecycle` | debug |
| 102 | `c.logger.Debug("Received response")` | `CategoryFetch` | debug |
| 110 | `c.logger.Warn("Cloudflare challenge detected")` | `CategoryError` | normal |
| 131 | `c.logger.Warn("Failed to archive HTML")` | `CategoryError` | normal |
| 149 | `c.logger.Debug("Visiting URL")` | `CategoryFetch` | debug |
| 182 | `c.logger.Debug("Finished processing page")` | `CategoryFetch` | debug |
| 210 | `c.logger.Debug("Expected error while crawling")` | `CategoryQueue` | debug |
| 226 | `c.logger.Warn("Timeout while crawling")` | `CategoryError` | normal |
| 236 | `c.logger.Error("Error while crawling")` | `CategoryError` | normal |

#### start.go (14 statements)

| Line | Current | New Category | Verbosity |
|------|---------|--------------|-----------|
| 17 | `c.logger.Debug("Starting crawler")` | `CategoryLifecycle` | quiet |
| 61-68 | Initial page wait logs | `CategoryLifecycle` | debug |
| 82 | `c.logger.Info("Collector finished")` | `CategoryLifecycle` | quiet |
| 84-90 | Context cancellation logs | `CategoryLifecycle` | normal |
| 134 | `c.logger.Debug("Starting to wait for collector")` | `CategoryLifecycle` | debug |
| 199 | `c.logger.Debug("Initial page scraped")` | `CategoryQueue` | debug |
| 223 | `c.logger.Warn("No links discovered on initial page")` | `CategoryError` | normal |
| 232-252 | Cleanup resource logs | `CategoryLifecycle` | debug |

#### link_handler.go (12 statements)

| Line | Current | New Category | Verbosity |
|------|---------|--------------|-----------|
| 37 | `Debug("Skipping link")` | `CategoryQueue` | debug |
| 47 | `Debug("Skipping link")` | `CategoryQueue` | debug |
| 56 | `Debug("Skipping link")` | `CategoryQueue` | debug |
| 65 | `Debug("Discovered link")` | `CategoryQueue` | normal |
| 130-135 | `Debug("Successfully queued link")` | `CategoryQueue` | debug |
| 141 | `Debug("Skipping link visit")` | `CategoryQueue` | debug |
| 152 | `Debug("Failed to visit link, retrying")` | `CategoryRateLimit` | debug |
| 166 | `Error("Failed to visit link after retries")` | `CategoryError` | normal |
| 272-298 | Link saving logs | `CategoryQueue` | debug |

#### processing.go (4 statements)

| Line | Current | New Category | Verbosity |
|------|---------|--------------|-----------|
| 38 | `Debug("Raw content processor not available")` | `CategoryExtract` | debug |
| 51 | `Debug("Content processing not implemented")` | `CategoryExtract` | debug |
| 55 | `Error("Failed to process raw content")` | `CategoryError` | normal |
| 62 | `Debug("Successfully processed raw content")` | `CategoryExtract` | normal |

### Code Transformation Examples

#### Before (collector.go:98-136)

```go
c.collector.OnResponse(func(r *colly.Response) {
    pageURL := r.Request.URL.String()
    c.logger.Debug("Received response",
        infralogger.String("url", pageURL),
        infralogger.Int("status", r.StatusCode),
        infralogger.Any("headers", r.Headers),
    )

    if c.isCloudflareChallenge(r) {
        c.logger.Warn("Cloudflare challenge detected - JavaScript execution required",
            infralogger.String("url", pageURL),
            infralogger.Int("status", r.StatusCode),
            // ... more fields
        )
    }
    // ...
})
```

#### After (collector.go:98-136)

```go
c.collector.OnResponse(func(r *colly.Response) {
    pageURL := r.Request.URL.String()

    // Create page-scoped logger to reduce field repetition
    pageLog := c.jobLog().WithFields(
        logs.URL(pageURL),
        logs.Int("status", r.StatusCode),
    )

    pageLog.Debug(logs.CategoryFetch, "Response received",
        logs.Int64("content_length", r.Headers.Get("Content-Length")),
    )

    if c.isCloudflareChallenge(r) {
        pageLog.Warn(logs.CategoryError, "Cloudflare challenge detected",
            logs.String("note", "JavaScript execution required"),
            logs.String("suggestion", "Consider browser-based crawler"),
        )
    }
    // ...
})
```

#### Before (link_handler.go:119-173)

```go
func (h *LinkHandler) visitWithRetries(e *colly.HTMLElement, absLink string) {
    var lastErr error
    totalAttempts := h.crawler.cfg.MaxRetries + 1

    for attempt := 0; attempt < totalAttempts; attempt++ {
        err := e.Request.Visit(absLink)
        if err == nil {
            h.crawler.logger.Debug("Successfully queued link for visiting",
                infralogger.String("url", absLink),
            )
            return
        }
        // ... retry logic with many logger calls
    }

    h.crawler.logger.Error("Failed to visit link after retries",
        infralogger.String("url", absLink),
        infralogger.Error(lastErr),
        // ... more fields
    )
}
```

#### After (link_handler.go:119-173)

```go
func (h *LinkHandler) visitWithRetries(e *colly.HTMLElement, absLink string) {
    linkLog := h.crawler.jobLog().WithFields(
        logs.URL(absLink),
        logs.String("page_url", e.Request.URL.String()),
        logs.Int("depth", e.Request.Depth),
    )

    var lastErr error
    totalAttempts := h.crawler.cfg.MaxRetries + 1

    for attempt := 0; attempt < totalAttempts; attempt++ {
        err := e.Request.Visit(absLink)
        if err == nil {
            linkLog.Debug(logs.CategoryQueue, "Link queued",
                logs.Int("attempt", attempt+1),
            )
            return
        }

        if h.isNonRetryableError(err) {
            linkLog.Debug(logs.CategoryQueue, "Link skipped",
                logs.String("reason", h.getErrorReason(err)),
            )
            return
        }

        lastErr = err
        if attempt < totalAttempts-1 {
            linkLog.Debug(logs.CategoryRateLimit, "Retry scheduled",
                logs.Int("attempt", attempt+1),
                logs.Duration("delay", h.crawler.cfg.RetryDelay),
            )
            time.Sleep(h.crawler.cfg.RetryDelay)
        }
    }

    linkLog.Error(logs.CategoryError, "Link failed after retries",
        logs.Err(lastErr),
        logs.Int("total_attempts", totalAttempts),
    )
}
```

### Helper Method on Crawler

```go
// crawler/internal/crawler/crawler.go

// jobLog returns the job logger, or a no-op logger if not set.
// Safe to call even when jobLogger is nil.
func (c *Crawler) jobLog() logs.JobLogger {
    if c.jobLogger != nil {
        return c.jobLogger
    }
    return logs.NoopJobLogger()
}
```

### LinkHandler Access Pattern

The `LinkHandler` accesses the crawler's logger via `h.crawler.jobLog()`:

```go
// link_handler.go

func (h *LinkHandler) HandleLink(e *colly.HTMLElement) {
    linkLog := h.crawler.jobLog().WithFields(
        logs.URL(absLink),
        logs.Int("depth", e.Request.Depth),
    )
    // ... use linkLog throughout
}
```

### Metrics Collection During Logging

The `jobLogger` implementation automatically tracks metrics as logs are written:

```go
type logMetrics struct {
    pagesDiscovered atomic.Int64
    pagesCrawled    atomic.Int64
    itemsExtracted  atomic.Int64
    errorsCount     atomic.Int64
    bytesReceived   atomic.Int64
    statusCodes     sync.Map // map[int]*atomic.Int64
}

func (l *jobLogger) log(level string, category Category, msg string, fields []Field) {
    // ... write to buffer/SSE/stdout ...

    // Auto-collect metrics from category
    switch category {
    case CategoryQueue:
        if msg == "Link discovered" || msg == "Link queued" {
            l.metrics.pagesDiscovered.Add(1)
        }
    case CategoryFetch:
        if msg == "Response received" || msg == "Page fetched" {
            l.metrics.pagesCrawled.Add(1)
            // Extract status code from fields
            for _, f := range fields {
                if f.Key == "status" {
                    if status, ok := f.Value.(int); ok {
                        l.incrementStatusCode(status)
                    }
                }
            }
        }
    case CategoryExtract:
        if msg == "Content extracted" || strings.Contains(msg, "processed") {
            l.metrics.itemsExtracted.Add(1)
        }
    case CategoryError:
        l.metrics.errorsCount.Add(1)
    }
}
```

### Edge Cases

#### A) Multi-URL Fan-Out

When a page enqueues multiple child URLs, each must get a fresh scoped logger:

```go
// CORRECT: Fresh logger per URL
for _, link := range discoveredLinks {
    linkLog := c.jobLog().WithFields(logs.URL(link.URL), logs.Int("depth", link.Depth))
    linkLog.Debug(CategoryQueue, "Link enqueued")
}

// WRONG: Reusing scoped logger across siblings (fields would bleed)
pageLog := c.jobLog().WithFields(logs.URL(pageURL))
for _, link := range discoveredLinks {
    pageLog.Debug(CategoryQueue, "Link enqueued", logs.URL(link.URL)) // URL field collision!
}
```

#### B) Throttling After Scoping

Throttling must apply after `WithFields()` is resolved, not before:

```go
func (l *jobLogger) Debug(category Category, msg string, fields ...Field) {
    // 1. First: check verbosity
    if l.verbosity != VerbosityDebug && l.verbosity != VerbosityTrace {
        return
    }

    // 2. Second: merge scoped fields with call-site fields
    allFields := append(l.scopedFields, fields...)

    // 3. Third: apply throttling (after fields are ready)
    if !l.throttler.Allow() {
        l.metrics.IncrementThrottled()
        return
    }

    // 4. Fourth: emit log
    l.log("debug", category, msg, allFields)
}
```

#### C) Guaranteed Summary Emission

The summary must be emitted even on early exit, failure, or cancellation:

```go
// scheduler/interval_scheduler.go

func (s *IntervalScheduler) runJob(jobExec *JobExecution) {
    job := jobExec.Job
    execution := jobExec.Execution
    startTime := time.Now()

    // Create job logger
    jobLogger := s.createJobLogger(jobExec)
    jobLogger.JobStarted(job.SourceID, job.URL)

    // CRITICAL: Defer summary emission BEFORE any early returns
    defer func() {
        summary := jobLogger.BuildSummary()
        summary.Duration = time.Since(startTime)
        jobLogger.JobCompleted(summary) // Always emitted
        jobLogger.Flush()
        s.stopLogCapture(jobExec, jobLogger)
    }()

    // ... rest of job execution (may return early, panic, or succeed)
}
```

---

## Section 4: Buffered SSE Streaming

### Problem

Currently, when a client connects to `/api/v1/jobs/:id/logs/stream`:
1. SSE connection opens
2. Client receives only *new* events from that moment forward
3. If the job started before connection, client sees "Waiting for log output..."
4. Fast jobs (~5 seconds) complete before the UI even loads

### Solution: Replay Buffer

When a client connects, immediately replay the last N log lines from the in-memory buffer, then continue with live streaming.

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Log Service                              │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐      │
│  │   Buffer     │───▶│  Publisher   │───▶│  SSE Broker  │      │
│  │ (ring buffer)│    │ (publishes   │    │ (broadcasts  │      │
│  │  last 200    │    │  to broker)  │    │  to clients) │      │
│  │  lines/job)  │    └──────────────┘    └──────────────┘      │
│  └──────────────┘                               │               │
│         │                                       │               │
│         │ GetRecentLogs(jobID, n)               │               │
│         ▼                                       ▼               │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                   SSE Handler                             │  │
│  │  1. Client connects to /jobs/:id/logs/stream              │  │
│  │  2. Fetch last 200 lines from buffer (if job still runs)  │  │
│  │  3. Send replay batch as "log:replay" event               │  │
│  │  4. Subscribe to broker for live "log:line" events        │  │
│  │  5. Stream until client disconnects or job completes      │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### Buffer Implementation

```go
// crawler/internal/logs/buffer.go

const (
    DefaultReplayBufferSize = 200 // Lines to replay on connect
)

// Buffer is a thread-safe ring buffer for log entries.
type Buffer interface {
    // Write appends an entry (overwrites oldest if full)
    Write(entry LogEntry)

    // ReadAll returns all entries in chronological order
    ReadAll() []LogEntry

    // ReadLast returns the last n entries
    ReadLast(n int) []LogEntry

    // Size returns current entry count
    Size() int

    // Bytes returns gzipped content for archival
    Bytes() []byte

    // LineCount returns total lines written (may exceed buffer size)
    LineCount() int
}

type ringBuffer struct {
    entries   []LogEntry
    head      int  // Next write position
    size      int  // Current number of entries
    capacity  int  // Max entries
    lineCount int  // Total lines written (for archive metadata)
    mu        sync.RWMutex
}

func NewBuffer(capacity int) Buffer {
    return &ringBuffer{
        entries:  make([]LogEntry, capacity),
        capacity: capacity,
    }
}

func (b *ringBuffer) Write(entry LogEntry) {
    b.mu.Lock()
    defer b.mu.Unlock()

    b.entries[b.head] = entry
    b.head = (b.head + 1) % b.capacity
    if b.size < b.capacity {
        b.size++
    }
    b.lineCount++
}

func (b *ringBuffer) ReadLast(n int) []LogEntry {
    b.mu.RLock()
    defer b.mu.RUnlock()

    if n > b.size {
        n = b.size
    }

    result := make([]LogEntry, n)
    start := (b.head - n + b.capacity) % b.capacity

    for i := 0; i < n; i++ {
        result[i] = b.entries[(start+i)%b.capacity]
    }

    return result
}
```

### Log Service: GetRecentLogs

```go
// crawler/internal/logs/service.go

// GetRecentLogs returns the last n log lines for a running job.
// Returns nil if job is not currently being captured.
func (s *logService) GetRecentLogs(executionID string, n int) []LogEntry {
    s.mu.RLock()
    aw, exists := s.activeWriters[executionID]
    s.mu.RUnlock()

    if !exists {
        return nil
    }

    return aw.writer.GetBuffer().ReadLast(n)
}

// IsCapturing returns true if logs are being captured for the execution.
func (s *logService) IsCapturing(executionID string) bool {
    s.mu.RLock()
    defer s.mu.RUnlock()
    _, exists := s.activeWriters[executionID]
    return exists
}
```

### SSE Handler: Replay on Connect

```go
// crawler/internal/api/logs_handler.go

const (
    replayBufferSize = 200 // Lines to replay on connect
)

// StreamLogs handles GET /api/v1/jobs/:id/logs/stream
func (h *LogsHandler) StreamLogs(c *gin.Context) {
    jobID := c.Param("id")

    // Set SSE headers
    sse.SetSSEHeaders(c.Writer)
    c.Writer.Flush()

    // Get the latest execution for this job
    execution, err := h.executionRepo.GetLatestByJobID(c.Request.Context(), jobID)
    if err != nil {
        h.sendSSEError(c.Writer, "No executions found")
        return
    }

    // Send connected event
    h.sendConnectedEvent(c.Writer, jobID)

    // REPLAY: If job is still running, send buffered logs immediately
    if execution.Status == "running" && h.logService.IsCapturing(execution.ID) {
        recentLogs := h.logService.GetRecentLogs(execution.ID, replayBufferSize)
        if len(recentLogs) > 0 {
            h.sendReplayBatch(c.Writer, recentLogs)
            h.logger.Debug("Sent replay batch",
                infralogger.String("job_id", jobID),
                infralogger.Int("lines", len(recentLogs)),
            )
        }
    }

    // Subscribe to live events (filtered by job ID)
    filter := func(event sse.Event) bool {
        // ... existing filter logic ...
    }

    eventChan, cleanup := h.sseBroker.Subscribe(c.Request.Context(), sse.WithFilter(filter))
    defer cleanup()

    // Stream live events
    for {
        select {
        case event, ok := <-eventChan:
            if !ok {
                return
            }
            if err := sse.WriteEventDirect(c.Writer, event); err != nil {
                return
            }
        case <-c.Request.Context().Done():
            return
        }
    }
}

// sendReplayBatch sends buffered logs as a single "log:replay" event
func (h *LogsHandler) sendReplayBatch(w http.ResponseWriter, logs []LogEntry) {
    event := sse.Event{
        Type: "log:replay",
        Data: map[string]any{
            "lines": logs,
            "count": len(logs),
        },
    }
    sse.WriteEventDirect(w, event)
}
```

### Frontend: Handle Replay Event

```typescript
// dashboard/src/components/crawler/JobLogsViewer.vue

eventSource.onmessage = (event) => {
  try {
    const data = JSON.parse(event.data)

    switch (data.type) {
      case 'log:replay':
        // Replay batch: prepend to displayed logs
        const replayLines = data.data.lines as LogLine[]
        displayedLogs.value = [...replayLines, ...displayedLogs.value]
        console.log(`[JobLogsViewer] Replayed ${replayLines.length} buffered lines`)
        if (autoScroll.value) {
          scrollToBottom()
        }
        break

      case 'log:line':
        // Live log line: append
        addLogLine(data.data as LogLine)
        break

      case 'log:archived':
        // Job finished, reload metadata
        loadLogsMetadata()
        break
    }
  } catch (err) {
    console.error('[JobLogsViewer] Error parsing SSE event:', err)
  }
}
```

### Configuration

```bash
# New env vars
JOB_LOGS_REPLAY_BUFFER_SIZE=200   # Lines to replay on SSE connect
```

---

## Section 5: Log Throttling

### Problem

In `debug` and `trace` verbosity modes, the crawler can generate thousands of log lines per second:
- Every URL visited
- Every link discovered
- Every extraction attempt
- Internal state changes

Without throttling:
- SSE broker buffer overflow → dropped events
- MinIO archive files become gigabytes
- Frontend UI freezes rendering thousands of lines
- Network bandwidth saturation

### Solution: Token Bucket Rate Limiter

Apply rate limiting to `debug` and `trace` level logs only. `quiet` and `normal` logs are never throttled.

### Implementation

```go
// crawler/internal/logs/throttle.go

// rateLimiter implements a token bucket rate limiter.
type rateLimiter struct {
    maxPerSecond int
    tokens       float64
    lastRefill   time.Time
    mu           sync.Mutex
}

func newRateLimiter(maxPerSecond int) *rateLimiter {
    if maxPerSecond <= 0 {
        return nil // Disabled
    }
    return &rateLimiter{
        maxPerSecond: maxPerSecond,
        tokens:       float64(maxPerSecond),
        lastRefill:   time.Now(),
    }
}

// Allow returns true if a log can be emitted, false if throttled.
func (r *rateLimiter) Allow() bool {
    if r == nil {
        return true // No rate limiting
    }

    r.mu.Lock()
    defer r.mu.Unlock()

    // Refill tokens based on elapsed time
    now := time.Now()
    elapsed := now.Sub(r.lastRefill).Seconds()
    r.tokens += elapsed * float64(r.maxPerSecond)
    if r.tokens > float64(r.maxPerSecond) {
        r.tokens = float64(r.maxPerSecond) // Cap at max
    }
    r.lastRefill = now

    // Check if we have a token
    if r.tokens >= 1 {
        r.tokens--
        return true
    }

    return false
}

// Stats returns current throttler state for diagnostics.
func (r *rateLimiter) Stats() (tokens float64, maxPerSec int) {
    if r == nil {
        return 0, 0
    }
    r.mu.Lock()
    defer r.mu.Unlock()
    return r.tokens, r.maxPerSecond
}
```

### Integration with JobLogger

```go
// crawler/internal/logs/job_logger_impl.go

type jobLogger struct {
    // ... existing fields ...

    // Throttling (only for debug/trace)
    throttler       *rateLimiter
    throttledCount  atomic.Int64
    lastThrottleLog time.Time
    throttleLogMu   sync.Mutex
}

func (l *jobLogger) Debug(category Category, msg string, fields ...Field) {
    // Skip if verbosity doesn't allow debug
    if l.verbosity != VerbosityDebug && l.verbosity != VerbosityTrace {
        return
    }

    // Apply throttling
    if l.throttler != nil && !l.throttler.Allow() {
        l.throttledCount.Add(1)
        l.maybeLogThrottleWarning()
        return
    }

    l.log("debug", category, msg, fields)
}

// maybeLogThrottleWarning emits a periodic warning when logs are being throttled.
// Prevents log spam about throttling itself.
func (l *jobLogger) maybeLogThrottleWarning() {
    l.throttleLogMu.Lock()
    defer l.throttleLogMu.Unlock()

    const throttleWarnInterval = 10 * time.Second

    if time.Since(l.lastThrottleLog) < throttleWarnInterval {
        return
    }

    l.lastThrottleLog = time.Now()
    count := l.throttledCount.Load()

    // Emit warning via Info (never throttled)
    l.log("warn", CategoryMetrics, "Logs being throttled",
        []Field{
            Int64("throttled_count", count),
            Int("max_per_sec", l.throttler.maxPerSecond),
        },
    )
}
```

### Throttling Rules

| Verbosity | Level | Throttled? |
|-----------|-------|------------|
| quiet | info/warn/error | Never |
| quiet | debug | Not emitted |
| normal | info/warn/error | Never |
| normal | debug | Not emitted |
| debug | info/warn/error | Never |
| debug | debug | **Yes** (rate limited) |
| trace | info/warn/error | Never |
| trace | debug | **Yes** (rate limited) |
| trace | trace | **Yes** (rate limited) |

### Summary Includes Throttle Stats

```go
type JobSummary struct {
    // ... existing fields ...

    // Throttling stats
    LogsThrottled int64 `json:"logs_throttled,omitempty"`
}

func (l *jobLogger) BuildSummary() *JobSummary {
    return &JobSummary{
        PagesDiscovered: l.metrics.pagesDiscovered.Load(),
        PagesCrawled:    l.metrics.pagesCrawled.Load(),
        ItemsExtracted:  l.metrics.itemsExtracted.Load(),
        ErrorsCount:     l.metrics.errorsCount.Load(),
        LogsThrottled:   l.throttledCount.Load(),
        // ... status codes, top errors ...
    }
}
```

### Configuration

```bash
# Existing (from Section 1)
JOB_LOGS_VERBOSITY=normal       # quiet|normal|debug|trace
JOB_LOGS_MAX_PER_SEC=50         # Rate limit for debug/trace (0=unlimited)
```

### Behavior Examples

**Normal mode (default):**
- `JOB_LOGS_VERBOSITY=normal`
- Debug logs not emitted → no throttling needed
- All info/warn/error logs pass through

**Debug mode with throttling:**
- `JOB_LOGS_VERBOSITY=debug`
- `JOB_LOGS_MAX_PER_SEC=50`
- First 50 debug logs per second pass through
- Excess debug logs dropped with periodic warning
- Info/warn/error never dropped

**Debug mode unlimited (local dev only):**
- `JOB_LOGS_VERBOSITY=debug`
- `JOB_LOGS_MAX_PER_SEC=0`
- All logs pass through (may overwhelm UI)

---

## Section 6: Job Summary

### Structure

The job summary is emitted as the final log line, capturing aggregate statistics for the entire job execution.

```go
// crawler/internal/logs/summary.go

// JobSummary contains final statistics appended to job logs.
type JobSummary struct {
    // Core metrics
    PagesDiscovered int64         `json:"pages_discovered"`
    PagesCrawled    int64         `json:"pages_crawled"`
    ItemsExtracted  int64         `json:"items_extracted"`
    ErrorsCount     int64         `json:"errors_count"`

    // Duration breakdown
    Duration          time.Duration `json:"duration_ms"`
    CrawlDuration     time.Duration `json:"crawl_duration_ms"`
    ExtractDuration   time.Duration `json:"extract_duration_ms"`
    BackoffDuration   time.Duration `json:"backoff_duration_ms,omitempty"`

    // Network stats
    BytesFetched    int64 `json:"bytes_fetched"`
    RequestsTotal   int64 `json:"requests_total"`
    RequestsFailed  int64 `json:"requests_failed"`

    // Queue behavior
    QueueMaxDepth     int64 `json:"queue_max_depth,omitempty"`
    QueueAvgDepth     int64 `json:"queue_avg_depth,omitempty"`
    QueueEnqueued     int64 `json:"queue_enqueued,omitempty"`
    QueueDequeued     int64 `json:"queue_dequeued,omitempty"`

    // Status code breakdown
    StatusCodes map[int]int64 `json:"status_codes,omitempty"`

    // Top errors (deduplicated, max 5)
    TopErrors []ErrorSummary `json:"top_errors,omitempty"`

    // Throttling stats
    LogsEmitted       int64   `json:"logs_emitted"`
    LogsThrottled     int64   `json:"logs_throttled,omitempty"`
    ThrottlePercent   float64 `json:"throttle_percent,omitempty"`
}

type ErrorSummary struct {
    Message string `json:"message"`
    Count   int    `json:"count"`
    LastURL string `json:"last_url,omitempty"`
}
```

### Metrics Collection

```go
// crawler/internal/logs/metrics.go

type logMetrics struct {
    pagesDiscovered atomic.Int64
    pagesCrawled    atomic.Int64
    itemsExtracted  atomic.Int64
    errorsCount     atomic.Int64
    bytesReceived   atomic.Int64
    requestsTotal   atomic.Int64
    requestsFailed  atomic.Int64
    logsEmitted     atomic.Int64

    statusCodes sync.Map // map[int]*atomic.Int64
    errorCounts sync.Map // map[string]*errorTracker
}

type errorTracker struct {
    count   atomic.Int64
    lastURL atomic.Value // string
}

func (m *logMetrics) RecordStatusCode(code int) {
    counter, _ := m.statusCodes.LoadOrStore(code, &atomic.Int64{})
    counter.(*atomic.Int64).Add(1)
}

func (m *logMetrics) RecordError(msg, url string) {
    // Normalize error message (strip URLs, IDs, etc.)
    normalized := normalizeErrorMessage(msg)

    tracker, _ := m.errorCounts.LoadOrStore(normalized, &errorTracker{})
    t := tracker.(*errorTracker)
    t.count.Add(1)
    t.lastURL.Store(url)
}

func (m *logMetrics) BuildSummary() *JobSummary {
    summary := &JobSummary{
        PagesDiscovered: m.pagesDiscovered.Load(),
        PagesCrawled:    m.pagesCrawled.Load(),
        ItemsExtracted:  m.itemsExtracted.Load(),
        ErrorsCount:     m.errorsCount.Load(),
        BytesFetched:    m.bytesReceived.Load(),
        RequestsTotal:   m.requestsTotal.Load(),
        RequestsFailed:  m.requestsFailed.Load(),
        LogsEmitted:     m.logsEmitted.Load(),
        StatusCodes:     make(map[int]int64),
    }

    // Collect status codes
    m.statusCodes.Range(func(key, value any) bool {
        summary.StatusCodes[key.(int)] = value.(*atomic.Int64).Load()
        return true
    })

    // Collect top 5 errors by count
    summary.TopErrors = m.getTopErrors(5)

    return summary
}

func (m *logMetrics) getTopErrors(n int) []ErrorSummary {
    var errors []ErrorSummary

    m.errorCounts.Range(func(key, value any) bool {
        t := value.(*errorTracker)
        errors = append(errors, ErrorSummary{
            Message: key.(string),
            Count:   int(t.count.Load()),
            LastURL: t.lastURL.Load().(string),
        })
        return true
    })

    // Sort by count descending
    sort.Slice(errors, func(i, j int) bool {
        return errors[i].Count > errors[j].Count
    })

    if len(errors) > n {
        errors = errors[:n]
    }

    return errors
}
```

### Summary Emission

```go
// crawler/internal/logs/job_logger_impl.go

func (l *jobLogger) JobCompleted(summary *JobSummary) {
    // Add throttle stats
    summary.LogsThrottled = l.throttledCount.Load()
    summary.LogsEmitted = l.metrics.logsEmitted.Load()

    // Emit as final log line (always at info level, never throttled)
    l.log("info", CategoryLifecycle, "Job completed", []Field{
        Int64("pages_discovered", summary.PagesDiscovered),
        Int64("pages_crawled", summary.PagesCrawled),
        Int64("items_extracted", summary.ItemsExtracted),
        Int64("errors_count", summary.ErrorsCount),
        Duration("duration", summary.Duration),
        Int64("bytes_fetched", summary.BytesFetched),
        Any("status_codes", summary.StatusCodes),
        Any("top_errors", summary.TopErrors),
    })
}

func (l *jobLogger) JobFailed(err error) {
    summary := l.metrics.BuildSummary()
    summary.LogsThrottled = l.throttledCount.Load()

    l.log("error", CategoryLifecycle, "Job failed", []Field{
        Err(err),
        Int64("pages_discovered", summary.PagesDiscovered),
        Int64("pages_crawled", summary.PagesCrawled),
        Int64("errors_count", summary.ErrorsCount),
        Duration("duration", summary.Duration),
    })
}
```

### Example Summary Log Line

```json
{
  "timestamp": "2026-01-28T15:30:45.123Z",
  "level": "info",
  "category": "crawler.lifecycle",
  "message": "Job completed",
  "job_id": "aab0aa01-8f9f-4ddd-ae15-05aa33e8dea1",
  "execution_id": "fb810c29-2abb-497e-8eae-3829ae1a0014",
  "fields": {
    "pages_discovered": 156,
    "pages_crawled": 47,
    "items_extracted": 12,
    "errors_count": 3,
    "duration_ms": 45230,
    "bytes_fetched": 2456789,
    "status_codes": {
      "200": 44,
      "404": 2,
      "503": 1
    },
    "top_errors": [
      {"message": "context deadline exceeded", "count": 2, "last_url": "https://..."},
      {"message": "connection refused", "count": 1, "last_url": "https://..."}
    ],
    "logs_emitted": 234,
    "logs_throttled": 0
  }
}
```

---

## Section 7: Frontend Changes

### Overview

The Vue.js dashboard needs updates to:
1. Handle the new `log:replay` SSE event
2. Display log categories with filtering
3. Show the job summary card
4. Handle throttling indicators

### Updated TypeScript Types

```typescript
// dashboard/src/types/logs.ts

export type LogCategory =
  | 'crawler.lifecycle'
  | 'crawler.fetch'
  | 'crawler.extract'
  | 'crawler.error'
  | 'crawler.rate_limit'
  | 'crawler.queue'
  | 'crawler.metrics'

export type LogLevel = 'debug' | 'info' | 'warn' | 'error'

export interface LogLine {
  timestamp: string
  level: LogLevel
  category: LogCategory
  message: string
  job_id: string
  execution_id: string
  fields?: Record<string, unknown>
}

export interface JobSummary {
  pages_discovered: number
  pages_crawled: number
  items_extracted: number
  errors_count: number
  duration_ms: number
  crawl_duration_ms: number
  extract_duration_ms: number
  backoff_duration_ms?: number
  bytes_fetched: number
  requests_total: number
  requests_failed: number
  queue_max_depth?: number
  queue_avg_depth?: number
  status_codes?: Record<number, number>
  top_errors?: ErrorSummary[]
  logs_emitted: number
  logs_throttled?: number
  throttle_percent?: number
}

export interface ErrorSummary {
  message: string
  count: number
  last_url?: string
}

export interface LogReplayEvent {
  type: 'log:replay'
  data: {
    lines: LogLine[]
    count: number
  }
}

export interface LogLineEvent {
  type: 'log:line'
  data: LogLine
}

export interface LogArchivedEvent {
  type: 'log:archived'
  data: {
    job_id: string
    execution_id: string
    object_key: string
  }
}

export type LogSSEEvent = LogReplayEvent | LogLineEvent | LogArchivedEvent
```

### Updated JobLogsViewer Component

```vue
<!-- dashboard/src/components/crawler/JobLogsViewer.vue -->

<template>
  <div class="bg-white shadow rounded-lg overflow-hidden">
    <!-- Header with filters -->
    <div class="px-6 py-4 border-b border-gray-200">
      <div class="flex items-center justify-between">
        <h2 class="text-lg font-medium text-gray-900">Job Logs</h2>
        <div class="flex items-center space-x-3">
          <!-- Category Filter -->
          <select
            v-model="categoryFilter"
            class="text-sm border-gray-300 rounded-md"
          >
            <option value="">All Categories</option>
            <option value="crawler.lifecycle">Lifecycle</option>
            <option value="crawler.fetch">Fetch</option>
            <option value="crawler.extract">Extract</option>
            <option value="crawler.error">Errors</option>
            <option value="crawler.queue">Queue</option>
          </select>

          <!-- Level Filter -->
          <select
            v-model="levelFilter"
            class="text-sm border-gray-300 rounded-md"
          >
            <option value="">All Levels</option>
            <option value="error">Errors Only</option>
            <option value="warn">Warnings+</option>
            <option value="info">Info+</option>
            <option value="debug">Debug+</option>
          </select>

          <!-- Live indicator -->
          <span
            v-if="isLiveStreaming"
            class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800"
          >
            <span class="w-2 h-2 mr-1.5 bg-green-500 rounded-full animate-pulse" />
            Live
          </span>

          <!-- Throttle warning -->
          <span
            v-if="throttledCount > 0"
            class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-yellow-100 text-yellow-800"
            :title="`${throttledCount} logs throttled`"
          >
            <ExclamationTriangleIcon class="w-3 h-3 mr-1" />
            Throttled
          </span>
        </div>
      </div>
    </div>

    <!-- Log container -->
    <div
      ref="logContainer"
      class="bg-gray-900 text-gray-100 font-mono text-sm overflow-y-auto"
      style="height: 500px"
    >
      <div class="p-4 space-y-0.5">
        <div
          v-for="(line, index) in filteredLogs"
          :key="index"
          class="flex items-start hover:bg-gray-800 px-2 py-0.5 rounded"
        >
          <!-- Timestamp -->
          <span class="w-20 flex-shrink-0 text-gray-500 text-xs">
            {{ formatTimestamp(line.timestamp) }}
          </span>

          <!-- Level badge -->
          <span
            :class="getLevelClass(line.level)"
            class="w-12 flex-shrink-0 uppercase text-xs font-semibold"
          >
            {{ line.level }}
          </span>

          <!-- Category badge -->
          <span
            class="w-24 flex-shrink-0 text-xs text-gray-400 truncate"
            :title="line.category"
          >
            {{ formatCategory(line.category) }}
          </span>

          <!-- Message -->
          <span class="flex-1 break-all whitespace-pre-wrap">
            {{ line.message }}
            <span
              v-if="line.fields && Object.keys(line.fields).length > 0"
              class="text-gray-500"
            >
              {{ formatFields(line.fields) }}
            </span>
          </span>
        </div>

        <!-- Empty state -->
        <div
          v-if="isLiveStreaming && filteredLogs.length === 0"
          class="text-gray-500 italic"
        >
          Waiting for log output...
        </div>

        <!-- Replay indicator -->
        <div
          v-if="replayedCount > 0 && filteredLogs.length > 0"
          class="text-center text-gray-500 text-xs py-2 border-b border-gray-700 mb-2"
        >
          ↑ {{ replayedCount }} buffered logs replayed ↑
        </div>
      </div>
    </div>

    <!-- Summary card (shown when job completes) -->
    <JobSummaryCard
      v-if="summary"
      :summary="summary"
      class="border-t border-gray-200"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { ExclamationTriangleIcon } from '@heroicons/vue/24/outline'
import type { LogLine, LogCategory, LogLevel, JobSummary, LogSSEEvent } from '@/types/logs'
import JobSummaryCard from './JobSummaryCard.vue'

const props = defineProps<{
  jobId: string
  jobStatus: string
}>()

// State
const allLogs = ref<LogLine[]>([])
const categoryFilter = ref<LogCategory | ''>('')
const levelFilter = ref<LogLevel | ''>('')
const replayedCount = ref(0)
const throttledCount = ref(0)
const summary = ref<JobSummary | null>(null)
const isLiveStreaming = ref(false)

// SSE connection
let eventSource: EventSource | null = null

// Computed
const filteredLogs = computed(() => {
  return allLogs.value.filter(line => {
    // Category filter
    if (categoryFilter.value && line.category !== categoryFilter.value) {
      return false
    }

    // Level filter (hierarchical)
    if (levelFilter.value) {
      const levelOrder = ['debug', 'info', 'warn', 'error']
      const minLevel = levelOrder.indexOf(levelFilter.value)
      const lineLevel = levelOrder.indexOf(line.level)
      if (lineLevel < minLevel) {
        return false
      }
    }

    return true
  })
})

// SSE handling
const startLiveStream = () => {
  const token = localStorage.getItem('dashboard_token')
  const url = `/api/crawler/jobs/${props.jobId}/logs/stream?token=${encodeURIComponent(token || '')}`

  eventSource = new EventSource(url)
  isLiveStreaming.value = true

  eventSource.onmessage = (event) => {
    const data = JSON.parse(event.data) as LogSSEEvent

    switch (data.type) {
      case 'log:replay':
        // Prepend replayed logs
        allLogs.value = [...data.data.lines, ...allLogs.value]
        replayedCount.value = data.data.count
        break

      case 'log:line':
        // Append live log
        allLogs.value.push(data.data)

        // Track throttle warnings
        if (data.data.message.includes('Logs being throttled')) {
          throttledCount.value = (data.data.fields?.throttled_count as number) || 0
        }

        // Extract summary from job completed message
        if (data.data.category === 'crawler.lifecycle' &&
            data.data.message === 'Job completed') {
          summary.value = extractSummary(data.data.fields)
        }
        break

      case 'log:archived':
        // Job finished
        isLiveStreaming.value = false
        break
    }
  }

  eventSource.onerror = () => {
    isLiveStreaming.value = false
  }
}

// Helpers
const formatCategory = (category: LogCategory): string => {
  return category.replace('crawler.', '')
}

const formatFields = (fields: Record<string, unknown>): string => {
  return Object.entries(fields)
    .map(([k, v]) => `${k}=${JSON.stringify(v)}`)
    .join(' ')
}

const extractSummary = (fields?: Record<string, unknown>): JobSummary | null => {
  if (!fields) return null
  return fields as unknown as JobSummary
}

// Lifecycle
onMounted(() => {
  if (['running', 'pending'].includes(props.jobStatus)) {
    startLiveStream()
  }
})

onUnmounted(() => {
  eventSource?.close()
})
</script>
```

### JobSummaryCard Component

```vue
<!-- dashboard/src/components/crawler/JobSummaryCard.vue -->

<template>
  <div class="p-6 bg-gray-50">
    <h3 class="text-sm font-medium text-gray-900 mb-4">Job Summary</h3>

    <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
      <!-- Core metrics -->
      <SummaryMetric
        label="Pages Discovered"
        :value="summary.pages_discovered"
        icon="MagnifyingGlassIcon"
      />
      <SummaryMetric
        label="Pages Crawled"
        :value="summary.pages_crawled"
        icon="DocumentIcon"
      />
      <SummaryMetric
        label="Items Extracted"
        :value="summary.items_extracted"
        icon="DocumentTextIcon"
      />
      <SummaryMetric
        label="Errors"
        :value="summary.errors_count"
        icon="ExclamationCircleIcon"
        :variant="summary.errors_count > 0 ? 'error' : 'default'"
      />

      <!-- Duration breakdown -->
      <SummaryMetric
        label="Total Duration"
        :value="formatDuration(summary.duration_ms)"
        icon="ClockIcon"
      />
      <SummaryMetric
        label="Crawl Time"
        :value="formatDuration(summary.crawl_duration_ms)"
        icon="ArrowPathIcon"
      />
      <SummaryMetric
        label="Extract Time"
        :value="formatDuration(summary.extract_duration_ms)"
        icon="ScissorsIcon"
      />
      <SummaryMetric
        v-if="summary.backoff_duration_ms"
        label="Backoff Time"
        :value="formatDuration(summary.backoff_duration_ms)"
        icon="PauseIcon"
      />
    </div>

    <!-- Status codes -->
    <div v-if="summary.status_codes" class="mt-4">
      <h4 class="text-xs font-medium text-gray-500 mb-2">Status Codes</h4>
      <div class="flex flex-wrap gap-2">
        <span
          v-for="(count, code) in summary.status_codes"
          :key="code"
          :class="getStatusClass(Number(code))"
          class="px-2 py-1 rounded text-xs font-medium"
        >
          {{ code }}: {{ count }}
        </span>
      </div>
    </div>

    <!-- Top errors -->
    <div v-if="summary.top_errors?.length" class="mt-4">
      <h4 class="text-xs font-medium text-gray-500 mb-2">Top Errors</h4>
      <ul class="space-y-1">
        <li
          v-for="(error, index) in summary.top_errors"
          :key="index"
          class="text-sm text-red-600"
        >
          {{ error.message }} (×{{ error.count }})
        </li>
      </ul>
    </div>

    <!-- Throttle warning -->
    <div
      v-if="summary.logs_throttled && summary.logs_throttled > 0"
      class="mt-4 p-3 bg-yellow-50 rounded-md"
    >
      <p class="text-sm text-yellow-800">
        <ExclamationTriangleIcon class="w-4 h-4 inline mr-1" />
        {{ summary.logs_throttled.toLocaleString() }} logs were throttled
        ({{ summary.throttle_percent?.toFixed(1) }}%)
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { JobSummary } from '@/types/logs'

defineProps<{
  summary: JobSummary
}>()

const formatDuration = (ms: number): string => {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  return `${(ms / 60000).toFixed(1)}m`
}

const getStatusClass = (code: number): string => {
  if (code >= 200 && code < 300) return 'bg-green-100 text-green-800'
  if (code >= 300 && code < 400) return 'bg-blue-100 text-blue-800'
  if (code >= 400 && code < 500) return 'bg-yellow-100 text-yellow-800'
  return 'bg-red-100 text-red-800'
}
</script>
```

### API Client Updates

```typescript
// dashboard/src/api/crawler.ts

export const crawlerApi = {
  jobs: {
    // ... existing methods ...

    // Stream logs (returns EventSource URL)
    streamLogsUrl: (jobId: string, token: string): string => {
      return `/api/crawler/jobs/${jobId}/logs/stream?token=${encodeURIComponent(token)}`
    },

    // Get logs metadata (existing, unchanged)
    logs: (jobId: string) => api.get(`/api/crawler/jobs/${jobId}/logs`),

    // View archived logs (existing, unchanged)
    viewLogs: (jobId: string, executionNumber: number) =>
      api.get(`/api/crawler/jobs/${jobId}/logs/view?execution=${executionNumber}`),
  },
}
```

---

## Section 8: Migration Plan

### Phase 1: Backend Foundation (Week 1)

**Goal:** Implement core logging infrastructure without breaking existing behavior.

#### 1.1 Add New Files

```
crawler/internal/logs/
├── category.go        # Log categories
├── verbosity.go       # Verbosity levels
├── throttle.go        # Rate limiter
├── job_logger.go      # Interface
├── job_logger_impl.go # Implementation
├── metrics.go         # Auto-collected metrics
├── summary.go         # JobSummary struct
└── noop_logger.go     # No-op implementation
```

#### 1.2 Update Existing Files

- `crawler/internal/logs/types.go` - Add `Category` field to `LogEntry`
- `crawler/internal/logs/buffer.go` - Add `ReadLast(n)` method
- `crawler/internal/logs/service.go` - Add `GetRecentLogs()` method
- `crawler/internal/logs/writer.go` - Support new `LogEntry` structure

#### 1.3 Configuration

- Add new env vars to `.env.example`
- Update `crawler/internal/config/logs/config.go`

**Verification:** Existing behavior unchanged, new code compiles.

---

### Phase 2: Crawler Instrumentation (Week 2)

**Goal:** Replace all `c.logger.*` calls with `jobLogger.*` calls.

#### 2.1 Add JobLogger to Crawler

```go
// crawler/internal/crawler/crawler.go
type Crawler struct {
    // ... existing fields ...
    jobLogger logs.JobLogger
}

func (c *Crawler) SetJobLogger(logger logs.JobLogger) {
    c.jobLogger = logger
}

func (c *Crawler) jobLog() logs.JobLogger {
    if c.jobLogger != nil {
        return c.jobLogger
    }
    return logs.NoopJobLogger()
}
```

#### 2.2 Convert Files (in order)

1. `collector.go` (12 statements)
2. `start.go` (14 statements)
3. `link_handler.go` (12 statements)
4. `processing.go` (4 statements)
5. `stop.go` (4 statements)
6. `signals.go` (internal, keep as-is or convert)

#### 2.3 Update Scheduler

```go
// crawler/internal/scheduler/interval_scheduler.go

func (s *IntervalScheduler) runJob(jobExec *JobExecution) {
    // Create job logger
    jobLogger := s.logService.CreateJobLogger(
        jobExec.Job.ID,
        jobExec.Execution.ID,
        s.config.Verbosity,
    )

    // Inject into crawler
    s.crawler.SetJobLogger(jobLogger)

    // ... rest of job execution ...
}
```

**Verification:** Jobs produce categorized logs, old scheduler logs still work.

---

### Phase 3: SSE Replay Buffer (Week 3)

**Goal:** Implement buffered SSE streaming.

#### 3.1 Update SSE Handler

- Add `GetRecentLogs()` call on connect
- Send `log:replay` event before subscribing to broker
- Handle edge case: job finishes during replay

#### 3.2 Update Log Service

- Expose `GetRecentLogs(executionID, n)` method
- Ensure buffer is populated during job execution

**Verification:** Connecting to a running job shows buffered logs immediately.

---

### Phase 4: Frontend Updates (Week 4)

**Goal:** Update Vue.js dashboard to handle new events and display summary.

#### 4.1 Add New Components

- `JobSummaryCard.vue`
- `SummaryMetric.vue`

#### 4.2 Update Existing Components

- `JobLogsViewer.vue` - Handle `log:replay`, category filter, summary extraction

#### 4.3 Add Types

- `dashboard/src/types/logs.ts`

**Verification:** UI shows replayed logs, filters work, summary card displays.

---

### Phase 5: Production Rollout

#### 5.1 Staging Deployment

1. Deploy backend changes
2. Run test jobs with `JOB_LOGS_VERBOSITY=debug`
3. Verify SSE replay works
4. Verify archived logs contain categories
5. Deploy frontend changes
6. Verify UI filters and summary

#### 5.2 Production Deployment

1. Deploy with `JOB_LOGS_VERBOSITY=normal` (default)
2. Monitor SSE broker metrics
3. Monitor MinIO archive sizes
4. Gradually enable `debug` for specific jobs if needed

#### 5.3 Rollback Plan

If issues occur:
1. Set `JOB_LOGS_VERBOSITY=quiet` to reduce load
2. Disable SSE replay by skipping `GetRecentLogs()` call
3. Revert frontend to ignore unknown event types

---

### Environment Variables Summary

```bash
# Verbosity
JOB_LOGS_VERBOSITY=normal           # quiet|normal|debug|trace

# Throttling
JOB_LOGS_MAX_PER_SEC=50             # Rate limit for debug/trace (0=unlimited)

# SSE Replay
JOB_LOGS_REPLAY_BUFFER_SIZE=200     # Lines to replay on connect

# Stdout mirroring
JOB_LOGS_ALSO_STDOUT=false          # Also mirror to container stdout

# Existing (unchanged)
JOB_LOGS_ENABLED=true
JOB_LOGS_BUFFER_SIZE=1000
JOB_LOGS_SSE_ENABLED=true
JOB_LOGS_ARCHIVE_ENABLED=true
JOB_LOGS_RETENTION_DAYS=30
JOB_LOGS_MIN_LEVEL=info
JOB_LOGS_MINIO_BUCKET=crawler-logs
```

---

---

## Section 9: Safety & Resilience

### 9.1 Log Schema Version

Every log line includes a `schema_version` field for forward compatibility:

```go
type LogEntry struct {
    SchemaVersion int            `json:"schema_version"` // Always 1 for this design
    Timestamp     time.Time      `json:"timestamp"`
    Level         string         `json:"level"`
    Category      string         `json:"category"`
    Message       string         `json:"message"`
    JobID         string         `json:"job_id"`
    ExecID        string         `json:"execution_id"`
    Fields        map[string]any `json:"fields,omitempty"`
}

const CurrentSchemaVersion = 1
```

Frontend handling:
```typescript
if (logLine.schema_version > SUPPORTED_SCHEMA_VERSION) {
  console.warn(`Unknown schema version ${logLine.schema_version}, displaying raw`)
  // Fallback: show raw JSON
}
```

### 9.2 Job Metadata Event

Emitted immediately after SSE connection, before any logs:

```go
type JobMetadataEvent struct {
    Type string      `json:"type"` // "log:metadata"
    Data JobMetadata `json:"data"`
}

type JobMetadata struct {
    JobID         string    `json:"job_id"`
    ExecutionID   string    `json:"execution_id"`
    CrawlerName   string    `json:"crawler_name"`
    SourceName    string    `json:"source_name"`
    SourceURL     string    `json:"source_url"`
    Verbosity     string    `json:"verbosity"`
    StartTime     time.Time `json:"start_time"`
    Config        JobConfig `json:"config"`
}

type JobConfig struct {
    ThrottlingEnabled bool `json:"throttling_enabled"`
    MaxLogsPerSec     int  `json:"max_logs_per_sec"`
    ArchiveEnabled    bool `json:"archive_enabled"`
    ReplayBufferSize  int  `json:"replay_buffer_size"`
}
```

SSE event order:
1. `connected` - connection established
2. `log:metadata` - job context
3. `log:replay` - buffered logs (if any)
4. `log:line` - live logs
5. `log:archived` - job complete

### 9.3 Total Log Cap

Hard ceiling to prevent pathological cases:

```go
const (
    MaxLogsPerJob = 50000 // Hard cap
)

func (l *jobLogger) log(level string, category Category, msg string, fields []Field) {
    // Check hard cap
    if l.metrics.logsEmitted.Load() >= MaxLogsPerJob {
        l.emitTruncatedOnce()
        return
    }

    // ... normal logging ...
}

func (l *jobLogger) emitTruncatedOnce() {
    if l.truncatedEmitted.CompareAndSwap(false, true) {
        // Emit truncation warning (bypasses cap)
        l.logDirect("warn", CategoryLifecycle, "Log output truncated",
            []Field{
                Int64("logs_emitted", MaxLogsPerJob),
                String("reason", "max_logs_per_job reached"),
            },
        )

        // Publish SSE event
        if l.publisher != nil {
            l.publisher.PublishTruncated(context.Background(), l.jobID, l.executionID)
        }
    }
}
```

Frontend handling:
```typescript
case 'log:truncated':
  // Show warning banner
  truncatedWarning.value = true
  // Continue displaying existing logs
  break
```

### 9.4 Crawler Heartbeat

Emitted every 15 seconds during job execution:

```go
const HeartbeatInterval = 15 * time.Second

func (l *jobLogger) StartHeartbeat(ctx context.Context) {
    go func() {
        ticker := time.NewTicker(HeartbeatInterval)
        defer ticker.Stop()

        for {
            select {
            case <-ticker.C:
                l.emitHeartbeat()
            case <-ctx.Done():
                return
            }
        }
    }()
}

func (l *jobLogger) emitHeartbeat() {
    summary := l.metrics.BuildSummary()

    l.Info(CategoryLifecycle, "Heartbeat",
        Int64("pages_crawled", summary.PagesCrawled),
        Int64("items_extracted", summary.ItemsExtracted),
        Int64("errors_count", summary.ErrorsCount),
        Int64("queue_depth", l.metrics.queueDepth.Load()),
        Duration("elapsed", time.Since(l.startTime)),
    )
}
```

Example heartbeat log:
```json
{
  "schema_version": 1,
  "level": "info",
  "category": "crawler.lifecycle",
  "message": "Heartbeat",
  "fields": {
    "pages_crawled": 23,
    "items_extracted": 5,
    "errors_count": 1,
    "queue_depth": 12,
    "elapsed_ms": 45000
  }
}
```

### 9.5 Frontend SSE Fallback

When SSE disconnects, fall back to polling:

```typescript
// dashboard/src/composables/useLogStream.ts

const MAX_RECONNECT_ATTEMPTS = 5
const POLL_INTERVAL = 2000 // 2 seconds

export function useLogStream(jobId: string) {
  const logs = ref<LogLine[]>([])
  const mode = ref<'sse' | 'polling'>('sse')
  const reconnectAttempts = ref(0)

  let eventSource: EventSource | null = null
  let pollTimer: number | null = null

  const startSSE = () => {
    mode.value = 'sse'
    eventSource = new EventSource(getSSEUrl(jobId))

    eventSource.onmessage = (event) => {
      reconnectAttempts.value = 0 // Reset on success
      handleEvent(JSON.parse(event.data))
    }

    eventSource.onerror = () => {
      eventSource?.close()
      reconnectAttempts.value++

      if (reconnectAttempts.value < MAX_RECONNECT_ATTEMPTS) {
        // Retry SSE with exponential backoff
        setTimeout(startSSE, Math.pow(2, reconnectAttempts.value) * 1000)
      } else {
        // Fall back to polling
        startPolling()
      }
    }
  }

  const startPolling = () => {
    mode.value = 'polling'
    console.log('[LogStream] Falling back to polling mode')

    pollTimer = window.setInterval(async () => {
      try {
        const response = await crawlerApi.jobs.tailLogs(jobId, 200)
        logs.value = response.data.lines

        // Try to reconnect SSE periodically
        if (reconnectAttempts.value > 0) {
          reconnectAttempts.value--
          if (reconnectAttempts.value === 0) {
            stopPolling()
            startSSE()
          }
        }
      } catch (err) {
        console.error('[LogStream] Polling failed:', err)
      }
    }, POLL_INTERVAL)
  }

  const stopPolling = () => {
    if (pollTimer) {
      clearInterval(pollTimer)
      pollTimer = null
    }
  }

  onMounted(startSSE)
  onUnmounted(() => {
    eventSource?.close()
    stopPolling()
  })

  return { logs, mode }
}
```

New API endpoint for polling fallback:

```go
// GET /api/v1/jobs/:id/logs/tail?limit=200
func (h *LogsHandler) TailLogs(c *gin.Context) {
    jobID := c.Param("id")
    limit, _ := strconv.Atoi(c.DefaultQuery("limit", "200"))

    // Get latest execution
    execution, err := h.executionRepo.GetLatestByJobID(c.Request.Context(), jobID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "no executions found"})
        return
    }

    // If running, get from buffer
    if execution.Status == "running" && h.logService.IsCapturing(execution.ID) {
        lines := h.logService.GetRecentLogs(execution.ID, limit)
        c.JSON(http.StatusOK, gin.H{"lines": lines, "source": "live"})
        return
    }

    // Otherwise, get from archive
    // ... existing viewLogs logic ...
}
```

---

## Summary

This design provides:

1. **Configurable verbosity** - 4 levels from quiet to trace
2. **Event categories** - 7 categories for filtering and searchability
3. **Unified job logging** - All crawler logs route through JobLogger
4. **Buffered SSE streaming** - Replay last 200 lines on connect
5. **Log throttling** - Token bucket rate limiting for debug/trace
6. **Rich job summary** - Metrics, durations, status codes, top errors
7. **Frontend updates** - Category filters, summary card, throttle indicator
8. **Schema versioning** - Forward compatibility for future changes
9. **Job metadata event** - Context immediately on connect
10. **Total log cap** - 50k hard limit prevents pathological cases
11. **Crawler heartbeat** - Every 15s, prevents frozen UI
12. **SSE fallback** - Auto-switch to polling on disconnect

**Estimated effort:** 4-5 weeks
**Risk:** Low (backward compatible, feature flagged)

---

## Appendix: File Changes Summary

### New Files

```
crawler/internal/logs/
├── category.go          # Log categories (7 types)
├── verbosity.go         # Verbosity levels (4 types)
├── throttle.go          # Token bucket rate limiter
├── job_logger.go        # JobLogger interface
├── job_logger_impl.go   # Implementation with WithFields
├── scoped_logger.go     # Scoped logger for WithFields
├── metrics.go           # Auto-collected metrics
├── summary.go           # JobSummary struct
├── heartbeat.go         # Heartbeat goroutine
└── noop_logger.go       # No-op implementation

dashboard/src/
├── types/logs.ts        # TypeScript types
├── composables/useLogStream.ts  # SSE + polling fallback
└── components/crawler/
    ├── JobSummaryCard.vue
    └── SummaryMetric.vue
```

### Modified Files

```
crawler/internal/logs/
├── types.go             # Add SchemaVersion, Category to LogEntry
├── buffer.go            # Add ReadLast(n) method
├── service.go           # Add GetRecentLogs(), CreateJobLogger()
└── writer.go            # Support new LogEntry structure

crawler/internal/crawler/
├── crawler.go           # Add jobLogger field, SetJobLogger(), jobLog()
├── collector.go         # Convert 12 c.logger calls
├── start.go             # Convert 14 c.logger calls
├── link_handler.go      # Convert 12 c.logger calls
├── processing.go        # Convert 4 c.logger calls
└── stop.go              # Convert 4 c.logger calls

crawler/internal/scheduler/
└── interval_scheduler.go  # Create JobLogger, inject into crawler

crawler/internal/api/
└── logs_handler.go      # Add replay logic, metadata event, tail endpoint

crawler/internal/config/logs/
└── config.go            # Add verbosity, throttle, replay config

dashboard/src/components/crawler/
└── JobLogsViewer.vue    # Handle new events, filters, summary
```

### Configuration Changes

```bash
# New env vars
JOB_LOGS_VERBOSITY=normal
JOB_LOGS_MAX_PER_SEC=50
JOB_LOGS_REPLAY_BUFFER_SIZE=200
JOB_LOGS_ALSO_STDOUT=false
JOB_LOGS_MAX_PER_JOB=50000
JOB_LOGS_HEARTBEAT_INTERVAL=15s
```
