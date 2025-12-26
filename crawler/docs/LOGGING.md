# Logging Best Practices

This document describes logging conventions, configuration, and best practices for the crawler service.

## Log Levels

The crawler uses structured logging with the following levels:

### Debug
**Purpose**: Detailed technical information for troubleshooting

**When to use**:
- HTTP request/response details (headers, status codes, body sizes)
- Internal state changes and processing decisions
- Configuration values and connection details
- URL processing decisions, crawl depth checks, robots.txt checks
- Detailed execution flow and intermediate steps

**Examples**:
- `logger.Debug("Starting crawler", "source", sourceName, "config", cfg)`
- `logger.Debug("Received response", "url", url, "status", statusCode, "headers", headers)`
- `logger.Debug("Indexing raw content", "index", indexName, "doc_id", docID)`

**Visibility**: Only visible when `APP_DEBUG=true` or `LOG_LEVEL=debug`

### Info
**Purpose**: Important business events and state changes for monitoring

**When to use**:
- Service startup and shutdown
- Job lifecycle events (created, started, completed, failed)
- Crawl session summaries (started, finished, pages crawled)
- Storage operations (index creation, batch indexing summaries)
- Important state transitions

**Examples**:
- `logger.Info("Job completed successfully", "job_id", jobID, "duration", duration)`
- `logger.Info("Collector finished", "source", sourceName, "pages_crawled", count)`
- `logger.Info("Created index", "index", indexName)`

**Visibility**: Always visible (default level)

### Warn
**Purpose**: Non-critical issues that should be noted

**When to use**:
- Timeouts and retries
- Fallback behaviors
- Configuration issues that don't prevent operation
- Deprecated feature usage

**Examples**:
- `logger.Warn("Timeout while crawling", "url", url, "timeout", timeout)`
- `logger.Warn("TLS certificate verification is disabled", "component", "crawler")`
- `logger.Warn("Failed to reload sources from API, using cached", "error", err)`

**Visibility**: Always visible

### Error
**Purpose**: Failures requiring attention

**When to use**:
- Operation failures
- Connection errors
- Data validation failures
- Unexpected errors in business logic

**Examples**:
- `logger.Error("Failed to start crawler", "job_id", jobID, "error", err)`
- `logger.Error("Elasticsearch returned error response", "index", index, "error", err)`
- `logger.Error("Job execution failed", "job_id", jobID, "error", err)`

**Visibility**: Always visible

### Fatal
**Purpose**: Critical errors that prevent service from continuing

**When to use**:
- Critical initialization failures
- Unrecoverable system errors

**Examples**:
- `logger.Fatal("Failed to initialize database", "error", err)`
- `logger.Fatal("Failed to create logger", "error", err)`

**Visibility**: Always visible (and terminates the application)

## Structured Logging

All log statements use structured logging with key-value pairs.

### Field Naming Conventions

**Always use snake_case** for structured log fields, following the project convention.

**Common fields**:
- `job_id`: Job identifier
- `source_name`: Source name being crawled
- `url`: URL being processed
- `status_code`: HTTP status code
- `error`: Error message or error object
- `duration`: Operation duration
- `component`: Component name (e.g., "crawler", "storage", "scheduler")
- `index`: Elasticsearch index name
- `doc_id`: Document identifier

**Examples**:
```go
// Good: snake_case fields
logger.Info("Job completed",
    "job_id", jobID,
    "source_name", sourceName,
    "duration", duration,
    "pages_crawled", count)

// Bad: camelCase or other formats
logger.Info("Job completed",
    "jobId", jobID,        // wrong
    "sourceName", sourceName,  // wrong
)
```

### Using the Logger Interface

The logger interface provides helper methods for common fields:

```go
log := logger.WithComponent("crawler")
log.WithRequestID(requestID).Info("Processing request")
log.WithDuration(duration).Info("Operation completed")
log.WithError(err).Error("Operation failed")
```

## Configuration

### Environment Variables

**Priority**: Environment variables > config file > defaults

#### `APP_DEBUG`
- **Purpose**: Controls log level (debug vs info)
- **Values**: `true` or `false`
- **Effect**: When `true`, sets `logger.level=debug` regardless of `APP_ENV`
- **Usage**: Enable debug logs in production for troubleshooting
- **Example**: `APP_DEBUG=true` enables debug logs in any environment

#### `APP_ENV`
- **Purpose**: Controls logging format and development features
- **Values**: `development`, `staging`, `production`
- **Effect when `development`**:
  - Console encoding (human-readable)
  - Color output enabled
  - Caller information included
  - Stack traces enabled
- **Effect when `production` or other**:
  - JSON encoding (machine-readable)
  - No colors
  - Minimal caller information
- **Note**: Separate from log level - you can have debug logs with production formatting

#### `LOG_LEVEL`
- **Purpose**: Directly set logger level
- **Values**: `debug`, `info`, `warn`, `error`, `fatal`
- **Effect**: Overrides default log level
- **Usage**: More direct control than `APP_DEBUG`

#### `LOG_FORMAT`
- **Purpose**: Set log encoding format
- **Values**: `json` or `console`
- **Effect**: Overrides encoding setting

### Config File

**Location**: `config.yml` or `config.example.yaml`

**Structure**:
```yaml
app:
  environment: production  # development, staging, production
  debug: false             # Enable debug mode

logger:
  level: info              # debug, info, warn, error, fatal
```

**Note**: The config file uses `logger:` section, not `log:`. The `app.debug` field is separate from `logger.level` - use `APP_DEBUG` environment variable to control debug logging.

### Configuration Priority

1. Environment variables (highest priority)
   - `APP_DEBUG`, `APP_ENV`, `LOG_LEVEL`, `LOG_FORMAT`
2. Config file (`config.yml`)
   - `app.debug`, `app.environment`, `logger.level`
3. Defaults (lowest priority)
   - `logger.level=info`, `app.environment=production`, `app.debug=false`

## Enabling Debug Logs in Production

To enable debug logs in production:

1. **Set in `.env` file**:
   ```bash
   APP_DEBUG=true
   APP_ENV=production
   ```
   
   This will:
   - Set log level to `debug` (shows Debug logs)
   - Use production formatting (JSON encoding)

2. **Or use environment variable directly**:
   ```bash
   docker-compose -f docker-compose.base.yml -f docker-compose.prod.yml up -d \
     -e APP_DEBUG=true
   ```

3. **Or set `LOG_LEVEL` directly**:
   ```bash
   LOG_LEVEL=debug
   ```

**Note**: Debug logs can be verbose. Use for troubleshooting specific issues, then disable when done.

## Production vs Development Formatting

### Production Formatting (APP_ENV=production)
- **Encoding**: JSON
- **Output**: Machine-readable, suitable for log aggregation tools
- **Example**:
  ```json
  {"level":"info","ts":1234567890,"msg":"Job completed","job_id":"123","duration":"5s"}
  ```

### Development Formatting (APP_ENV=development)
- **Encoding**: Console (human-readable)
- **Colors**: Enabled for log levels
- **Caller**: Includes file and line number
- **Example**:
  ```
  2024-01-01 12:00:00.000 | INFO | Job completed | job_id=123 | duration=5s
  ```

## Common Logging Patterns

### Service Startup
```go
logger.Info("Service starting",
    "component", "crawler",
    "version", version,
    "environment", env)
```

### Job Lifecycle
```go
logger.Info("Job started",
    "job_id", jobID,
    "source_name", sourceName,
    "url", url)

// ... execution ...

logger.Info("Job completed",
    "job_id", jobID,
    "duration", duration,
    "pages_crawled", count)
```

### Error Handling
```go
if err != nil {
    logger.Error("Operation failed",
        "component", "crawler",
        "operation", "process_url",
        "url", url,
        "error", err)
    return err
}
```

### Debugging Details
```go
logger.Debug("Processing request",
    "url", url,
    "headers", headers,
    "method", method,
    "body_size", bodySize)
```

## Best Practices

1. **Use appropriate log levels**: Debug for troubleshooting, Info for monitoring, Warn for issues, Error for failures

2. **Include context**: Always include relevant fields (job_id, url, source_name, etc.)

3. **Use structured fields**: Prefer structured logging over formatted strings

4. **Avoid sensitive data**: Don't log passwords, tokens, or full request/response bodies

5. **Be consistent**: Use the same field names across the codebase (snake_case)

6. **Production-ready**: Info logs should be useful for production monitoring without being too verbose

7. **Debug when needed**: Enable debug logs temporarily for troubleshooting, then disable

8. **Error context**: Always include the error object in Error logs

## Examples by Component

### Crawler Component
- **Debug**: URL processing, crawl depth, robots.txt checks, internal state
- **Info**: Crawl started, crawl finished, collector finished
- **Warn**: Timeouts, expected errors (max depth, already visited)
- **Error**: Crawl failures, unexpected errors

### Job Scheduler Component
- **Debug**: Job scheduling details, cron parsing, reload operations
- **Info**: Job started, job completed, scheduler started/stopped
- **Warn**: Job scheduling issues, reload failures
- **Error**: Job execution failures, scheduler errors

### Storage Component
- **Debug**: Indexing operations, document preparation, query details
- **Info**: Index created, document indexed, batch operations completed
- **Warn**: Retry operations, connection issues
- **Error**: Indexing failures, connection errors, query failures

## Troubleshooting

### Debug logs not appearing in production

1. Check `APP_DEBUG` is set to `true` in `.env` file
2. Verify `docker-compose.prod.yml` doesn't override `APP_DEBUG=false`
3. Check `LOG_LEVEL` environment variable is not set to `info` or higher
4. Verify config file has correct structure (`logger:` not `log:`)

### Too many logs in production

1. Set `APP_DEBUG=false` in `.env`
2. Ensure `LOG_LEVEL=info` or higher
3. Review Debug log usage - may need to move some to Info level

### Logs not in JSON format in production

1. Verify `APP_ENV=production` is set
2. Check `LOG_FORMAT` is not set to `console`
3. Verify config file has `app.environment=production`

