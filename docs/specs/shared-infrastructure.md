# Shared Infrastructure Specification

> Last verified: 2026-03-22 (add .layers file for layer boundary checking)

Covers the `infrastructure/` module: config loading, logging, database clients, middleware, events, and utilities used by all services.

## File Map

| File | Purpose |
|------|---------|
| `infrastructure/config/loader.go` | YAML + env config loading with generics |
| `infrastructure/config/types.go` | Config type definitions |
| `infrastructure/config/validate.go` | Config validation helpers |
| `infrastructure/logger/logger.go` | Logger interface + field helpers (zap-based) |
| `infrastructure/logger/nop.go` | No-op logger for tests |
| `infrastructure/logger/context.go` | Context-based logger propagation |
| `infrastructure/elasticsearch/client.go` | ES client with TLS, auth, retry |
| `infrastructure/redis/client.go` | Redis client wrapper with ping verification |
| `infrastructure/http/client.go` | HTTP client with configurable timeouts |
| `infrastructure/jwt/middleware.go` | JWT auth middleware for Gin |
| `infrastructure/gin/middleware.go` | Logging, CORS, recovery, request ID middleware |
| `infrastructure/pipeline/client.go` | Event emission with circuit breaker |
| `infrastructure/events/types.go` | Domain event types (source lifecycle) |
| `infrastructure/sse/broker.go` | Server-Sent Events broker |
| `infrastructure/retry/retry.go` | Exponential backoff retry logic |
| `infrastructure/profiling/pprof.go` | pprof debug endpoint setup |
| `infrastructure/profiling/pyroscope.go` | Pyroscope continuous profiling |
| `infrastructure/monitoring/health_handler.go` | Health check HTTP handler |
| `infrastructure/monitoring/memory_monitor.go` | Memory monitoring and alerts |
| `infrastructure/context/utils.go` | Timeout helper functions |
| `infrastructure/clickurl/signer.go` | Click tracking URL signing |
| `infrastructure/gin/builder.go` | Gin server builder with `WithMetrics()` option |
| `infrastructure/gin/metrics.go` | Prometheus metrics route and handler (`/metrics`) |

## Interface Signatures

### Logger (`logger/logger.go`)
```go
type Logger interface {
    Debug(msg string, fields ...Field)
    Info(msg string, fields ...Field)
    Warn(msg string, fields ...Field)
    Error(msg string, fields ...Field)
    Fatal(msg string, fields ...Field)
    With(fields ...Field) Logger
    Sync() error
}

func New(config Config) (Logger, error)
func NewFromLoggingConfig(level, format string) (Logger, error)
func NewNop() Logger  // For tests
func Must(config Config) Logger
```

### Config (`config/config.go`)
```go
func Load[T any](path string) (*T, error)
func LoadWithDefaults[T any](path string, setDefaults func(*T)) (*T, error)
func MustLoad[T any](path string) *T
func ApplyEnvOverrides(cfg any) error
func GetConfigPath(defaultPath string) string
```

### Pipeline Client (`pipeline/client.go`)
```go
func NewClient(baseURL, serviceName string) *Client  // Empty baseURL → no-op
func (c *Client) Emit(ctx context.Context, event Event) error
func (c *Client) EmitBatch(ctx context.Context, events []Event) error
func (c *Client) IsEnabled() bool
func (c *Client) CircuitOpen() bool
```

### Retry (`retry/retry.go`)
```go
type Config struct {
    MaxAttempts  int
    InitialDelay time.Duration
    MaxDelay     time.Duration
    Multiplier   float64
    IsRetryable  func(error) bool
}

func Retry(ctx context.Context, cfg Config, fn func() error) error
func RetryWithDefaults(ctx context.Context, fn func() error) error
func DefaultConfig() Config  // 3 attempts, 100ms, 30s max, 2.0 multiplier
```

### JWT (`jwt/middleware.go`)
```go
func Middleware(secret string) gin.HandlerFunc  // Skips /health, /health/*
func GetClaims(c *gin.Context) (*Claims, bool)

type Claims struct {
    Sub string `json:"sub"`
    jwt.RegisteredClaims
}
```

## Data Flow

### Service Bootstrap Pattern (all services)
```
Phase 1: Config (YAML + env overrides via Load[T]())
Phase 2: Logger (New() or NewFromLoggingConfig())
Phase 3: Database (PostgreSQL via config.DatabaseConfig)
Phase 4: External clients (ES via elasticsearch.NewClient(), Redis via redis.NewClient())
Phase 5: Service initialization (wire dependencies)
Phase 6: HTTP server (Gin + middleware stack)
Phase 7: Lifecycle (graceful shutdown with context cancellation)
```

### Gin Server Builder (`gin/builder.go`)
```go
func NewServerBuilder(cfg ServerConfig, log Logger) *ServerBuilder
func (b *ServerBuilder) WithMiddleware(mw ...gin.HandlerFunc) *ServerBuilder
func (b *ServerBuilder) WithMetrics() *ServerBuilder          // Enables /metrics endpoint
func (b *ServerBuilder) WithRoutes(fn func(*gin.Engine)) *ServerBuilder
func (b *ServerBuilder) Build() *http.Server
```

`WithMetrics()` sets `metricsEnabled` on the builder. When enabled, `Build()` calls `RegisterMetricsRoute(engine)` which adds a `GET /metrics` route serving the default Prometheus registry via `promhttp.Handler()`.

### Prometheus Metrics (`gin/metrics.go`)
```go
func RegisterMetricsRoute(engine *gin.Engine)  // GET /metrics → promhttp.Handler()
func MetricsHandler() gin.HandlerFunc          // Wraps promhttp.Handler() as Gin handler
```

Services opt in by calling `WithMetrics()` in their server builder chain. Prometheus scrape config (`prometheus.yml`) defines scrape targets per service. Current targets: classifier, publisher, search, index-manager, auth, crawler, source-manager, pipeline, click-tracker, rfp-ingestor. Not scraped: nc-http-proxy (uses raw `net/http`, not gin builder).

### Middleware Stack (typical order)
```
RequestIDMiddleware → LoggerMiddleware → CORSMiddleware → RecoveryMiddleware → JWT Middleware → Route handlers
```

### Pipeline Circuit Breaker
```
Closed → (5 consecutive failures) → Open → (30s timeout) → Half-Open → (2 successes) → Closed
Open state: Emit() returns ErrCircuitBreakerOpen immediately
```

### Config Loading Priority
```
1. ENV_FILE environment variable (if set, loads only this file)
2. .env.local (if exists, overrides .env)
3. .env (default)
Then: YAML config file → env tag overrides on struct fields
```

## Configuration

### Common Config Structs
```go
// Used by all services
type ServerConfig struct {
    Host, Port, ReadTimeout, WriteTimeout, IdleTimeout
}

type DatabaseConfig struct {
    Host, Port, User, Password, Database, SSLMode
    MaxConnections, MaxIdleConns, ConnMaxLifetime
}

type ElasticsearchConfig struct {
    URL string `env:"ELASTICSEARCH_URL"`
    Username, Password, MaxRetries, Timeout
}

type RedisConfig struct {
    URL string `env:"REDIS_URL"`
    Password, DB
}

type LoggingConfig struct {
    Level string `env:"LOG_LEVEL"`
    Format string `env:"LOG_FORMAT"`
}
```

### Import Conventions
```go
import infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
import infraconfig "github.com/jonesrussell/north-cloud/infrastructure/config"
```

### go.mod Pattern
```
module github.com/jonesrussell/north-cloud/{service}
require github.com/jonesrussell/north-cloud/infrastructure v0.0.0
replace github.com/jonesrussell/north-cloud/infrastructure => ../infrastructure
```

## Edge Cases

- **os.Getenv forbidden**: `forbidigo` linter blocks it. Use config struct with `env` tags.
- **Logger always JSON**: Never configure text output. Fields always snake_case.
- **Pipeline no-op on empty URL**: `NewClient("", "svc")` silently succeeds all Emit() calls.
- **Circuit breaker auto-recovery**: After 30s in open state, transitions to half-open. Don't manually reset.
- **Redis ping on creation**: `NewClient()` fails fast (5s timeout) if Redis is down.
- **ES retry on startup**: 10 attempts with 3s→15s backoff. Patient establishment for container startup ordering.
- **JWT skips health endpoints**: /health and /health/* bypass auth. Don't expose sensitive data there.
- **Context logger fallback**: `FromContext()` returns fallback logger (warns to stderr) if not found.
- **Request ID propagation**: `RequestIDLoggerMiddleware` stores logger with request_id in Go context. Downstream code retrieves via `logger.FromContext(ctx)`.
- **Alloy Docker labels**: Config exposes `container_name` and `compose_service` labels for Docker log discovery, enabling per-service log filtering in Grafana (e.g., `{container_name="north-cloud-crawler-1"}`).
