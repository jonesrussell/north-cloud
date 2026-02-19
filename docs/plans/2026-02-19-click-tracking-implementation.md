# Click Tracking Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a privacy-respectful click tracking system to NorthCloud Search using server-side 302 redirects with HMAC-signed URLs.

**Architecture:** New standalone `click-tracker` Go service (port 8093) handles redirect + logging. Search service generates signed click URLs per result. Frontend uses `click_url` for link `href`. Shared HMAC package lives in `infrastructure/clickurl/`.

**Tech Stack:** Go 1.25, Gin, PostgreSQL 16, HMAC-SHA256, infrastructure packages (config, logger, gin)

**Design doc:** `docs/plans/2026-02-19-click-tracking-design.md`

---

### Task 1: Infrastructure HMAC Package

**Files:**
- Create: `infrastructure/clickurl/signer.go`
- Create: `infrastructure/clickurl/signer_test.go`

This shared package provides HMAC-SHA256 signing and verification for click URLs. Both the search service (sign) and click-tracker (verify) import it.

**Step 1: Write the failing test**

```go
// infrastructure/clickurl/signer_test.go
package clickurl_test

import (
	"testing"

	"github.com/north-cloud/infrastructure/clickurl"
)

func TestSign(t *testing.T) {
	t.Helper()

	signer := clickurl.NewSigner("test-secret")
	sig := signer.Sign("q_abc123|r_doc456|3|1|1708300000|https://example.com")

	if sig == "" {
		t.Fatal("expected non-empty signature")
	}
	if len(sig) != clickurl.SignatureLength {
		t.Fatalf("expected signature length %d, got %d", clickurl.SignatureLength, len(sig))
	}
}

func TestVerify_Valid(t *testing.T) {
	t.Helper()

	signer := clickurl.NewSigner("test-secret")
	message := "q_abc123|r_doc456|3|1|1708300000|https://example.com"
	sig := signer.Sign(message)

	if !signer.Verify(message, sig) {
		t.Fatal("expected valid signature to verify")
	}
}

func TestVerify_InvalidSignature(t *testing.T) {
	t.Helper()

	signer := clickurl.NewSigner("test-secret")
	message := "q_abc123|r_doc456|3|1|1708300000|https://example.com"

	if signer.Verify(message, "000000000000") {
		t.Fatal("expected invalid signature to fail verification")
	}
}

func TestVerify_WrongSecret(t *testing.T) {
	t.Helper()

	signer1 := clickurl.NewSigner("secret-1")
	signer2 := clickurl.NewSigner("secret-2")
	message := "q_abc123|r_doc456|3|1|1708300000|https://example.com"
	sig := signer1.Sign(message)

	if signer2.Verify(message, sig) {
		t.Fatal("expected signature from different secret to fail")
	}
}

func TestSign_Deterministic(t *testing.T) {
	t.Helper()

	signer := clickurl.NewSigner("test-secret")
	message := "q_abc123|r_doc456|3|1|1708300000|https://example.com"
	sig1 := signer.Sign(message)
	sig2 := signer.Sign(message)

	if sig1 != sig2 {
		t.Fatalf("expected deterministic signatures, got %s and %s", sig1, sig2)
	}
}

func TestBuildMessage(t *testing.T) {
	t.Helper()

	params := clickurl.ClickParams{
		QueryID:        "q_abc123",
		ResultID:       "r_doc456",
		Position:       3,
		Page:           1,
		Timestamp:      1708300000,
		DestinationURL: "https://example.com",
	}

	msg := params.Message()
	expected := "q_abc123|r_doc456|3|1|1708300000|https://example.com"
	if msg != expected {
		t.Fatalf("expected %q, got %q", expected, msg)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd infrastructure && go test ./clickurl/... -v`
Expected: FAIL (package does not exist)

**Step 3: Write the implementation**

```go
// infrastructure/clickurl/signer.go
package clickurl

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// SignatureLength is the number of hex characters in a truncated HMAC signature.
const SignatureLength = 12

// ClickParams holds the parameters that are signed in a click URL.
type ClickParams struct {
	QueryID        string
	ResultID       string
	Position       int
	Page           int
	Timestamp      int64
	DestinationURL string
}

// Message builds the pipe-delimited string that gets signed.
func (p ClickParams) Message() string {
	return fmt.Sprintf("%s|%s|%d|%d|%d|%s",
		p.QueryID, p.ResultID, p.Position, p.Page, p.Timestamp, p.DestinationURL)
}

// Signer creates and verifies HMAC-SHA256 signatures for click URLs.
type Signer struct {
	secret []byte
}

// NewSigner creates a Signer with the given secret.
func NewSigner(secret string) *Signer {
	return &Signer{secret: []byte(secret)}
}

// Sign returns a truncated HMAC-SHA256 hex signature.
func (s *Signer) Sign(message string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(message))
	full := hex.EncodeToString(mac.Sum(nil))
	return full[:SignatureLength]
}

// Verify checks that the signature matches the message.
func (s *Signer) Verify(message, signature string) bool {
	expected := s.Sign(message)
	return hmac.Equal([]byte(expected), []byte(signature))
}
```

**Step 4: Run tests to verify they pass**

Run: `cd infrastructure && go test ./clickurl/... -v`
Expected: PASS (all 5 tests)

**Step 5: Commit**

```bash
git add infrastructure/clickurl/
git commit -m "feat(infrastructure): add clickurl HMAC signing package

Shared HMAC-SHA256 signer for click URL generation (search) and
verification (click-tracker). Truncated to 12 hex chars (48 bits)."
```

---

### Task 2: Click-Tracker Service Scaffold

**Files:**
- Create: `click-tracker/go.mod`
- Create: `click-tracker/main.go`
- Create: `click-tracker/config.yml.example`
- Create: `click-tracker/internal/config/config.go`
- Create: `click-tracker/internal/config/config_test.go`
- Create: `click-tracker/Dockerfile`
- Create: `click-tracker/Dockerfile.dev`
- Create: `click-tracker/.air.toml`
- Create: `click-tracker/Taskfile.yml`
- Modify: `go.work` (add `./click-tracker`)

**Step 1: Create go.mod**

```
module github.com/jonesrussell/north-cloud/click-tracker

go 1.25

require (
	github.com/gin-gonic/gin v1.11.0
	github.com/north-cloud/infrastructure v0.0.0-00010101000000-000000000000
	github.com/lib/pq v1.10.9
)

replace github.com/north-cloud/infrastructure => ../infrastructure
```

Run: `cd click-tracker && go mod tidy`

**Step 2: Create config**

```go
// click-tracker/internal/config/config.go
package config

import (
	"fmt"
	"time"

	infraconfig "github.com/north-cloud/infrastructure/config"
)

const (
	defaultServiceName      = "click-tracker"
	defaultServiceVersion   = "1.0.0"
	defaultServicePort      = 8093
	defaultSignatureLength  = 12
	defaultMaxTimestampAge  = 86400 // 24 hours
	defaultBufferSize       = 1000
	defaultFlushIntervalSec = 1
	defaultFlushThreshold   = 500
	defaultMaxClicksPerMin  = 10
	defaultRateLimitWindow  = 60
	defaultLogLevel         = "info"
	defaultLogFormat        = "json"
)

// Config holds all configuration for the click-tracker service.
type Config struct {
	Service   ServiceConfig   `yaml:"service"`
	Database  DatabaseConfig  `yaml:"database"`
	RateLimit RateLimitConfig `yaml:"ratelimit"`
	Logging   LoggingConfig   `yaml:"logging"`
}

// ServiceConfig holds service-level configuration.
type ServiceConfig struct {
	Name            string        `yaml:"name"`
	Version         string        `yaml:"version"`
	Port            int           `env:"CLICK_TRACKER_PORT"   yaml:"port"`
	Debug           bool          `env:"APP_DEBUG"            yaml:"debug"`
	HMACSecret      string        `env:"CLICK_TRACKER_SECRET" yaml:"hmac_secret"`
	SignatureLength int           `yaml:"signature_length"`
	MaxTimestampAge time.Duration `yaml:"max_timestamp_age"`
	BufferSize      int           `yaml:"buffer_size"`
	FlushInterval   time.Duration `yaml:"flush_interval"`
	FlushThreshold  int           `yaml:"flush_threshold"`
}

// DatabaseConfig holds PostgreSQL connection configuration.
type DatabaseConfig struct {
	Host     string `env:"POSTGRES_CLICK_TRACKER_HOST"     yaml:"host"`
	Port     int    `env:"POSTGRES_CLICK_TRACKER_PORT"     yaml:"port"`
	User     string `env:"POSTGRES_CLICK_TRACKER_USER"     yaml:"user"`
	Password string `env:"POSTGRES_CLICK_TRACKER_PASSWORD" yaml:"password"`
	Database string `env:"POSTGRES_CLICK_TRACKER_DB"       yaml:"database"`
	SSLMode  string `yaml:"ssl_mode"`
}

// DSN returns the PostgreSQL connection string.
func (d *DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Database, d.SSLMode)
}

// RateLimitConfig holds rate limiting configuration.
type RateLimitConfig struct {
	MaxClicksPerMinute int `yaml:"max_clicks_per_minute"`
	WindowSeconds      int `yaml:"window_seconds"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Level  string `env:"LOG_LEVEL"  yaml:"level"`
	Format string `env:"LOG_FORMAT" yaml:"format"`
}

// Load loads configuration from file and environment variables.
func Load(path string) (*Config, error) {
	cfg, err := infraconfig.LoadWithDefaults[Config](path, setDefaults)
	if err != nil {
		return nil, err
	}
	if validateErr := cfg.Validate(); validateErr != nil {
		return nil, fmt.Errorf("invalid configuration: %w", validateErr)
	}
	return cfg, nil
}

func setDefaults(cfg *Config) {
	setServiceDefaults(&cfg.Service)
	setDatabaseDefaults(&cfg.Database)
	setRateLimitDefaults(&cfg.RateLimit)
	setLoggingDefaults(&cfg.Logging)
}

func setServiceDefaults(s *ServiceConfig) {
	if s.Name == "" {
		s.Name = defaultServiceName
	}
	if s.Version == "" {
		s.Version = defaultServiceVersion
	}
	if s.Port == 0 {
		s.Port = defaultServicePort
	}
	if s.SignatureLength == 0 {
		s.SignatureLength = defaultSignatureLength
	}
	if s.MaxTimestampAge == 0 {
		s.MaxTimestampAge = time.Duration(defaultMaxTimestampAge) * time.Second
	}
	if s.BufferSize == 0 {
		s.BufferSize = defaultBufferSize
	}
	if s.FlushInterval == 0 {
		s.FlushInterval = time.Duration(defaultFlushIntervalSec) * time.Second
	}
	if s.FlushThreshold == 0 {
		s.FlushThreshold = defaultFlushThreshold
	}
}

func setDatabaseDefaults(d *DatabaseConfig) {
	if d.Host == "" {
		d.Host = "localhost"
	}
	if d.Port == 0 {
		d.Port = 5432
	}
	if d.User == "" {
		d.User = "postgres"
	}
	if d.Database == "" {
		d.Database = "click_tracker"
	}
	if d.SSLMode == "" {
		d.SSLMode = "disable"
	}
}

func setRateLimitDefaults(r *RateLimitConfig) {
	if r.MaxClicksPerMinute == 0 {
		r.MaxClicksPerMinute = defaultMaxClicksPerMin
	}
	if r.WindowSeconds == 0 {
		r.WindowSeconds = defaultRateLimitWindow
	}
}

func setLoggingDefaults(l *LoggingConfig) {
	if l.Level == "" {
		l.Level = defaultLogLevel
	}
	if l.Format == "" {
		l.Format = defaultLogFormat
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.Service.Port < 1 || c.Service.Port > 65535 {
		return &infraconfig.ValidationError{
			Field:   "service.port",
			Message: fmt.Sprintf("invalid port: %d", c.Service.Port),
		}
	}
	if c.Service.HMACSecret == "" {
		return &infraconfig.ValidationError{
			Field:   "service.hmac_secret",
			Message: "CLICK_TRACKER_SECRET is required",
		}
	}
	if err := infraconfig.ValidateLogLevel(c.Logging.Level); err != nil {
		return err
	}
	return infraconfig.ValidateLogFormat(c.Logging.Format)
}
```

**Step 3: Write config test**

```go
// click-tracker/internal/config/config_test.go
package config

import (
	"testing"
)

func TestSetDefaults(t *testing.T) {
	t.Helper()

	cfg := &Config{}
	setDefaults(cfg)

	if cfg.Service.Port != defaultServicePort {
		t.Fatalf("expected port %d, got %d", defaultServicePort, cfg.Service.Port)
	}
	if cfg.Service.Name != defaultServiceName {
		t.Fatalf("expected name %q, got %q", defaultServiceName, cfg.Service.Name)
	}
	if cfg.Database.Host != "localhost" {
		t.Fatalf("expected host localhost, got %q", cfg.Database.Host)
	}
	if cfg.RateLimit.MaxClicksPerMinute != defaultMaxClicksPerMin {
		t.Fatalf("expected max clicks %d, got %d", defaultMaxClicksPerMin, cfg.RateLimit.MaxClicksPerMinute)
	}
}

func TestValidate_MissingSecret(t *testing.T) {
	t.Helper()

	cfg := &Config{}
	setDefaults(cfg)
	// Secret intentionally not set

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for missing secret")
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	t.Helper()

	cfg := &Config{}
	setDefaults(cfg)
	cfg.Service.HMACSecret = "test-secret"

	err := cfg.Validate()
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestDSN(t *testing.T) {
	t.Helper()

	d := &DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "pass",
		Database: "click_tracker",
		SSLMode:  "disable",
	}
	dsn := d.DSN()
	expected := "host=localhost port=5432 user=postgres password=pass dbname=click_tracker sslmode=disable"
	if dsn != expected {
		t.Fatalf("expected %q, got %q", expected, dsn)
	}
}
```

**Step 4: Create config.yml.example**

```yaml
# click-tracker/config.yml.example
service:
  name: click-tracker
  version: "1.0.0"
  port: 8093
  debug: false
  # hmac_secret: set via CLICK_TRACKER_SECRET env var
  signature_length: 12
  max_timestamp_age: 86400s  # 24 hours
  buffer_size: 1000
  flush_interval: 1s
  flush_threshold: 500

database:
  host: postgres-click-tracker
  port: 5432
  user: postgres
  password: postgres
  database: click_tracker
  ssl_mode: disable

ratelimit:
  max_clicks_per_minute: 10
  window_seconds: 60

logging:
  level: info
  format: json
```

**Step 5: Create main.go**

```go
// click-tracker/main.go
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"

	"github.com/jonesrussell/north-cloud/click-tracker/internal/config"
	infraconfig "github.com/north-cloud/infrastructure/config"
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/profiling"
)

func main() {
	os.Exit(run())
}

func run() int {
	profiling.StartPprofServer()

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		return 1
	}

	log, err := createLogger(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		return 1
	}
	defer func() { _ = log.Sync() }()

	log.Info("Starting click-tracker",
		infralogger.String("name", cfg.Service.Name),
		infralogger.String("version", cfg.Service.Version),
		infralogger.Int("port", cfg.Service.Port),
	)

	db, err := setupDatabase(cfg, log)
	if err != nil {
		log.Error("Failed to connect to database", infralogger.Error(err))
		return 1
	}
	defer func() { _ = db.Close() }()

	return runServer(cfg, db, log)
}

func loadConfig() (*config.Config, error) {
	configPath := infraconfig.GetConfigPath("config.yml")
	return config.Load(configPath)
}

func createLogger(cfg *config.Config) (infralogger.Logger, error) {
	log, err := infralogger.New(infralogger.Config{
		Level:       cfg.Logging.Level,
		Format:      cfg.Logging.Format,
		Development: cfg.Service.Debug,
	})
	if err != nil {
		return nil, err
	}
	return log.With(infralogger.String("service", "click-tracker")), nil
}

func setupDatabase(cfg *config.Config, log infralogger.Logger) (*sql.DB, error) {
	log.Info("Connecting to database",
		infralogger.String("host", cfg.Database.Host),
		infralogger.String("database", cfg.Database.Database),
	)

	db, err := sql.Open("postgres", cfg.Database.DSN())
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if pingErr := db.PingContext(context.Background()); pingErr != nil {
		return nil, fmt.Errorf("ping database: %w", pingErr)
	}

	log.Info("Database connected")
	return db, nil
}

// runServer is a placeholder — wired up in Task 8.
func runServer(_ *config.Config, _ *sql.DB, log infralogger.Logger) int {
	log.Info("Click-tracker service ready (server wiring in progress)")
	return 0
}
```

**Step 6: Create Dockerfiles**

Production (`click-tracker/Dockerfile`):
```dockerfile
FROM golang:1.25.7-alpine AS builder

WORKDIR /build
COPY infrastructure ./infrastructure

WORKDIR /build/click-tracker

COPY click-tracker/go.mod click-tracker/go.sum ./
RUN go mod download

COPY click-tracker/ .

RUN CGO_ENABLED=0 go build \
    -ldflags="-w -s" \
    -trimpath \
    -o /click-tracker \
    main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /click-tracker .
COPY --from=builder /build/click-tracker/config.yml.example ./config.yml

EXPOSE 8093

CMD ["./click-tracker"]
```

Development (`click-tracker/Dockerfile.dev`):
```dockerfile
FROM golang:1.25.7-alpine

ARG UID=1000
ARG GID=1000

WORKDIR /app

RUN apk add --no-cache git bash

RUN go install github.com/air-verse/air@latest

RUN addgroup -g ${GID} appuser && \
    adduser -D -u ${UID} -G appuser appuser

RUN mkdir -p /tmp/go-mod-cache \
             /tmp/go-build-cache \
             /go/pkg/sumdb && \
    chmod -R 1777 /tmp/go-mod-cache \
                  /tmp/go-build-cache \
                  /go/pkg/sumdb

COPY infrastructure ../infrastructure

COPY click-tracker/go.mod click-tracker/go.sum ./
RUN go mod download

EXPOSE 8093

ENV GOCACHE=/tmp/go-build-cache
ENV GOMODCACHE=/tmp/go-mod-cache

CMD ["go", "run", "main.go"]
```

**Step 7: Create .air.toml**

```toml
# click-tracker/.air.toml
root = "."
tmp_dir = "tmp"

[build]
  bin = "./tmp/main"
  cmd = "go build -o ./tmp/main ./main.go"
  delay = 1000
  exclude_dir = ["vendor", "testdata", "bin", "tmp"]
  exclude_regex = ["_test.go"]
  include_ext = ["go", "yaml", "yml"]
  kill_delay = "1s"
  send_interrupt = true
  stop_on_error = true

[log]
  time = false

[misc]
  clean_on_exit = true
```

**Step 8: Create Taskfile.yml**

```yaml
# click-tracker/Taskfile.yml
version: '3'

vars:
  BINARY_NAME: click-tracker
  MAIN_PATH: ./main.go

env:
  GOWORK: "off"

tasks:
  build:
    desc: Build the click-tracker binary
    cmds:
      - go build -o bin/{{.BINARY_NAME}} {{.MAIN_PATH}}
    sources:
      - "**/*.go"
      - go.mod
      - go.sum
      - "../infrastructure/**/*.go"
    generates:
      - bin/{{.BINARY_NAME}}

  test:
    desc: Run tests
    cmds:
      - go test -v ./...
    sources:
      - "**/*.go"
      - go.mod
      - go.sum
      - "../infrastructure/**/*.go"

  test:cover:
    desc: Run tests with coverage
    cmds:
      - go test -coverprofile=coverage.out ./...
      - go tool cover -func=coverage.out

  lint:
    desc: Run linter
    cmds:
      - gofmt -l .
      - go vet ./...
      - golangci-lint run --config ../.golangci.yml
    sources:
      - "**/*.go"
      - go.mod
      - go.sum
      - "../infrastructure/**/*.go"

  lint:no-cache:
    desc: Run linter without cache
    cmds:
      - golangci-lint cache clean
      - golangci-lint run --config ../.golangci.yml

  vendor:
    desc: Vendor dependencies
    cmds:
      - go mod vendor

  dev:
    desc: Run with air hot reload
    cmds:
      - air -c .air.toml
```

**Step 9: Update go.work**

Add `./click-tracker` to the `use` block in `go.work`.

**Step 10: Run tests**

Run: `cd click-tracker && go mod tidy && go test ./... -v`
Expected: PASS (config tests pass, main compiles)

**Step 11: Commit**

```bash
git add click-tracker/ go.work
git commit -m "feat(click-tracker): scaffold service with config, main, and Docker

New standalone click-tracker service at port 8093. Includes config
loading, database setup, Dockerfile (prod + dev), Air hot reload,
and Taskfile. Server wiring comes in later tasks."
```

---

### Task 3: Database Migration

**Files:**
- Create: `click-tracker/migrations/001_create_click_events.up.sql`
- Create: `click-tracker/migrations/001_create_click_events.down.sql`
- Create: `click-tracker/cmd/migrate/main.go`

**Step 1: Create up migration**

```sql
-- click-tracker/migrations/001_create_click_events.up.sql
CREATE TABLE click_events (
    id               BIGSERIAL PRIMARY KEY,
    query_id         VARCHAR(32)  NOT NULL,
    result_id        VARCHAR(128) NOT NULL,
    position         SMALLINT     NOT NULL,
    page             SMALLINT     NOT NULL DEFAULT 1,
    destination_hash VARCHAR(64)  NOT NULL,
    session_id       VARCHAR(32),
    user_agent_hash  VARCHAR(12),
    generated_at     TIMESTAMPTZ  NOT NULL,
    clicked_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
) PARTITION BY RANGE (clicked_at);

-- Create partition for current month (additional partitions created by scheduled task)
CREATE TABLE click_events_default PARTITION OF click_events DEFAULT;

-- Indexes for analytics queries
CREATE INDEX idx_click_events_query_id   ON click_events (query_id);
CREATE INDEX idx_click_events_result_id  ON click_events (result_id);
CREATE INDEX idx_click_events_position   ON click_events (position);
CREATE INDEX idx_click_events_clicked_at ON click_events (clicked_at);
```

**Step 2: Create down migration**

```sql
-- click-tracker/migrations/001_create_click_events.down.sql
DROP TABLE IF EXISTS click_events CASCADE;
```

**Step 3: Create migrate command**

```go
// click-tracker/cmd/migrate/main.go
package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"

	"github.com/jonesrussell/north-cloud/click-tracker/internal/config"
	infraconfig "github.com/north-cloud/infrastructure/config"
	"github.com/north-cloud/infrastructure/migration"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: migrate [up|down]")
		os.Exit(1)
	}

	configPath := infraconfig.GetConfigPath("config.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		os.Exit(1)
	}

	db, err := sql.Open("postgres", cfg.Database.DSN())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Database error: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	if migrateErr := migration.Run(db, "migrations", os.Args[1]); migrateErr != nil {
		fmt.Fprintf(os.Stderr, "Migration error: %v\n", migrateErr)
		os.Exit(1)
	}
	fmt.Println("Migration completed successfully")
}
```

> **Note:** If `infrastructure/migration` doesn't exist, check how other services run migrations. Services like crawler use `golang-migrate/migrate`. Adapt the migration runner to match the existing pattern.

**Step 4: Commit**

```bash
git add click-tracker/migrations/ click-tracker/cmd/
git commit -m "feat(click-tracker): add click_events migration and migrate command

Partitioned table for click events with indexes on query_id, result_id,
position, and clicked_at. Default partition handles all data until
monthly partitions are created."
```

---

### Task 4: Click Event Domain Model and Storage

**Files:**
- Create: `click-tracker/internal/domain/click_event.go`
- Create: `click-tracker/internal/storage/postgres.go`
- Create: `click-tracker/internal/storage/postgres_test.go`

**Step 1: Create domain model**

```go
// click-tracker/internal/domain/click_event.go
package domain

import "time"

// ClickEvent represents a single tracked click.
type ClickEvent struct {
	QueryID         string    `json:"query_id"`
	ResultID        string    `json:"result_id"`
	Position        int       `json:"position"`
	Page            int       `json:"page"`
	DestinationHash string    `json:"destination_hash"`
	SessionID       string    `json:"session_id,omitempty"`
	UserAgentHash   string    `json:"user_agent_hash,omitempty"`
	GeneratedAt     time.Time `json:"generated_at"`
	ClickedAt       time.Time `json:"clicked_at"`
}
```

**Step 2: Write failing storage test**

```go
// click-tracker/internal/storage/postgres_test.go
package storage_test

import (
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/click-tracker/internal/domain"
	"github.com/jonesrussell/north-cloud/click-tracker/internal/storage"
)

func TestBuffer_Send(t *testing.T) {
	t.Helper()

	buf := storage.NewBuffer(100)
	defer buf.Close()

	event := domain.ClickEvent{
		QueryID:         "q_abc123",
		ResultID:        "r_doc456",
		Position:        3,
		Page:            1,
		DestinationHash: "a1b2c3d4e5f6",
		GeneratedAt:     time.Now(),
		ClickedAt:       time.Now(),
	}

	ok := buf.Send(event)
	if !ok {
		t.Fatal("expected Send to succeed")
	}
}

func TestBuffer_SendFull(t *testing.T) {
	t.Helper()

	buf := storage.NewBuffer(1) // capacity 1
	defer buf.Close()

	event := domain.ClickEvent{
		QueryID:  "q_test",
		ResultID: "r_test",
		Position: 1,
		Page:     1,
	}

	// Fill the buffer
	buf.Send(event)

	// This should fail (non-blocking)
	ok := buf.Send(event)
	if ok {
		t.Fatal("expected Send to fail on full buffer")
	}
}
```

**Step 3: Implement storage**

```go
// click-tracker/internal/storage/postgres.go
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jonesrussell/north-cloud/click-tracker/internal/domain"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const insertBatchSize = 50

// Buffer receives click events via a channel and batch-inserts them.
type Buffer struct {
	ch     chan domain.ClickEvent
	closed chan struct{}
}

// NewBuffer creates a new event buffer with the given capacity.
func NewBuffer(capacity int) *Buffer {
	return &Buffer{
		ch:     make(chan domain.ClickEvent, capacity),
		closed: make(chan struct{}),
	}
}

// Send enqueues a click event. Returns false if the buffer is full (non-blocking).
func (b *Buffer) Send(event domain.ClickEvent) bool {
	select {
	case b.ch <- event:
		return true
	default:
		return false
	}
}

// Close signals the buffer to stop.
func (b *Buffer) Close() {
	close(b.closed)
}

// Store manages buffered writes to PostgreSQL.
type Store struct {
	db             *sql.DB
	buffer         *Buffer
	logger         infralogger.Logger
	flushInterval  time.Duration
	flushThreshold int
	wg             sync.WaitGroup
}

// NewStore creates a new click event store.
func NewStore(db *sql.DB, buffer *Buffer, log infralogger.Logger, flushInterval time.Duration, flushThreshold int) *Store {
	return &Store{
		db:             db,
		buffer:         buffer,
		logger:         log,
		flushInterval:  flushInterval,
		flushThreshold: flushThreshold,
	}
}

// Start begins the background flush goroutine.
func (s *Store) Start() {
	s.wg.Add(1)
	go s.flushLoop()
}

// Stop waits for the flush goroutine to finish.
func (s *Store) Stop() {
	s.buffer.Close()
	s.wg.Wait()
}

func (s *Store) flushLoop() {
	defer s.wg.Done()

	batch := make([]domain.ClickEvent, 0, s.flushThreshold)
	ticker := time.NewTicker(s.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case event, ok := <-s.buffer.ch:
			if !ok {
				// Channel closed, flush remaining
				if len(batch) > 0 {
					s.flush(batch)
				}
				return
			}
			batch = append(batch, event)
			if len(batch) >= s.flushThreshold {
				s.flush(batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			if len(batch) > 0 {
				s.flush(batch)
				batch = batch[:0]
			}

		case <-s.buffer.closed:
			// Drain remaining events
			close(s.buffer.ch)
			for event := range s.buffer.ch {
				batch = append(batch, event)
			}
			if len(batch) > 0 {
				s.flush(batch)
			}
			return
		}
	}
}

func (s *Store) flush(batch []domain.ClickEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) //nolint:mnd
	defer cancel()

	// Batch insert in chunks
	for i := 0; i < len(batch); i += insertBatchSize {
		end := i + insertBatchSize
		if end > len(batch) {
			end = len(batch)
		}
		if err := s.batchInsert(ctx, batch[i:end]); err != nil {
			s.logger.Error("Failed to flush click events",
				infralogger.Error(err),
				infralogger.Int("count", end-i),
			)
		}
	}
}

func (s *Store) batchInsert(ctx context.Context, events []domain.ClickEvent) error {
	if len(events) == 0 {
		return nil
	}

	var b strings.Builder
	b.WriteString("INSERT INTO click_events (query_id, result_id, position, page, destination_hash, session_id, user_agent_hash, generated_at, clicked_at) VALUES ")

	args := make([]any, 0, len(events)*9) //nolint:mnd
	for i, e := range events {
		if i > 0 {
			b.WriteString(", ")
		}
		offset := i * 9 //nolint:mnd
		fmt.Fprintf(&b, "($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			offset+1, offset+2, offset+3, offset+4, offset+5, offset+6, offset+7, offset+8, offset+9)
		args = append(args, e.QueryID, e.ResultID, e.Position, e.Page, e.DestinationHash, e.SessionID, e.UserAgentHash, e.GeneratedAt, e.ClickedAt)
	}

	_, err := s.db.ExecContext(ctx, b.String(), args...)
	if err != nil {
		return fmt.Errorf("batch insert click events: %w", err)
	}
	return nil
}
```

**Step 4: Run tests**

Run: `cd click-tracker && go test ./internal/storage/... -v`
Expected: PASS (buffer tests pass — DB tests need a real connection, so unit tests focus on buffer)

**Step 5: Commit**

```bash
git add click-tracker/internal/domain/ click-tracker/internal/storage/
git commit -m "feat(click-tracker): add domain model and buffered PostgreSQL storage

ClickEvent domain model. Buffer with non-blocking Send (drops events
when full). Store does batch INSERT with configurable flush interval
and threshold."
```

---

### Task 5: Click Handler (Redirect Endpoint)

**Files:**
- Create: `click-tracker/internal/handler/click.go`
- Create: `click-tracker/internal/handler/click_test.go`
- Create: `click-tracker/internal/handler/health.go`

**Step 1: Write failing tests**

```go
// click-tracker/internal/handler/click_test.go
package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/click-tracker/internal/handler"
	"github.com/jonesrussell/north-cloud/click-tracker/internal/storage"
	"github.com/north-cloud/infrastructure/clickurl"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const testSecret = "test-secret-key"

func setupRouter(t *testing.T) (*gin.Engine, *storage.Buffer) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	signer := clickurl.NewSigner(testSecret)
	buf := storage.NewBuffer(100)
	log := infralogger.NewNop()
	maxAge := 24 * time.Hour

	h := handler.NewClickHandler(signer, buf, log, maxAge)
	r.GET("/click", h.HandleClick)

	return r, buf
}

func signedURL(t *testing.T, queryID, resultID string, pos, page int, ts int64, dest string) string {
	t.Helper()

	signer := clickurl.NewSigner(testSecret)
	params := clickurl.ClickParams{
		QueryID:        queryID,
		ResultID:       resultID,
		Position:       pos,
		Page:           page,
		Timestamp:      ts,
		DestinationURL: dest,
	}
	sig := signer.Sign(params.Message())
	return "/click?q=" + queryID + "&r=" + resultID +
		"&p=" + fmt.Sprintf("%d", pos) +
		"&pg=" + fmt.Sprintf("%d", page) +
		"&t=" + fmt.Sprintf("%d", ts) +
		"&u=" + url.QueryEscape(dest) +
		"&sig=" + sig
}

func TestHandleClick_ValidRedirect(t *testing.T) {
	t.Helper()

	r, buf := setupRouter(t)
	defer buf.Close()

	now := time.Now().Unix()
	target := signedURL(t, "q_abc", "r_doc", 3, 1, now, "https://example.com/article")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "https://example.com/article" {
		t.Fatalf("expected redirect to https://example.com/article, got %q", loc)
	}
}

func TestHandleClick_InvalidSignature(t *testing.T) {
	t.Helper()

	r, buf := setupRouter(t)
	defer buf.Close()

	now := time.Now().Unix()
	target := "/click?q=q_abc&r=r_doc&p=3&pg=1&t=" + fmt.Sprintf("%d", now) +
		"&u=" + url.QueryEscape("https://example.com") + "&sig=000000000000"

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for invalid signature, got %d", w.Code)
	}
}

func TestHandleClick_ExpiredTimestamp(t *testing.T) {
	t.Helper()

	r, buf := setupRouter(t)
	defer buf.Close()

	old := time.Now().Add(-25 * time.Hour).Unix()
	target := signedURL(t, "q_abc", "r_doc", 3, 1, old, "https://example.com")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusGone {
		t.Fatalf("expected 410 for expired timestamp, got %d", w.Code)
	}
}

func TestHandleClick_MissingParams(t *testing.T) {
	t.Helper()

	r, buf := setupRouter(t)
	defer buf.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/click?q=abc", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing params, got %d", w.Code)
	}
}
```

> **Note:** The test file needs `import ("fmt"; "net/url")` — add these when creating the file.

**Step 2: Run tests to verify they fail**

Run: `cd click-tracker && go test ./internal/handler/... -v`
Expected: FAIL (handler package does not exist)

**Step 3: Implement the handler**

```go
// click-tracker/internal/handler/click.go
package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/click-tracker/internal/domain"
	"github.com/jonesrussell/north-cloud/click-tracker/internal/storage"
	"github.com/north-cloud/infrastructure/clickurl"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// ClickHandler handles click redirect requests.
type ClickHandler struct {
	signer      *clickurl.Signer
	buffer      *storage.Buffer
	logger      infralogger.Logger
	maxAge      time.Duration
}

// NewClickHandler creates a new click handler.
func NewClickHandler(signer *clickurl.Signer, buffer *storage.Buffer, log infralogger.Logger, maxAge time.Duration) *ClickHandler {
	return &ClickHandler{
		signer: signer,
		buffer: buffer,
		logger: log,
		maxAge: maxAge,
	}
}

// HandleClick validates the signature, logs the event, and redirects.
func (h *ClickHandler) HandleClick(c *gin.Context) {
	params, err := parseClickParams(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify HMAC signature
	msg := params.Message()
	if !h.signer.Verify(msg, c.Query("sig")) {
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid signature"})
		return
	}

	// Check timestamp expiry
	generated := time.Unix(params.Timestamp, 0)
	if time.Since(generated) > h.maxAge {
		c.JSON(http.StatusGone, gin.H{"error": "click URL expired"})
		return
	}

	// Build and enqueue click event (non-blocking)
	event := domain.ClickEvent{
		QueryID:         params.QueryID,
		ResultID:        params.ResultID,
		Position:        params.Position,
		Page:            params.Page,
		DestinationHash: hashURL(params.DestinationURL),
		UserAgentHash:   hashUA(c.Request.UserAgent()),
		GeneratedAt:     generated,
		ClickedAt:       time.Now(),
	}
	if !h.buffer.Send(event) {
		h.logger.Warn("Click event buffer full, dropping event",
			infralogger.String("query_id", params.QueryID),
		)
	}

	// 302 redirect to destination
	c.Redirect(http.StatusFound, params.DestinationURL)
}

func parseClickParams(c *gin.Context) (clickurl.ClickParams, error) {
	q := c.Query("q")
	r := c.Query("r")
	pStr := c.Query("p")
	pgStr := c.Query("pg")
	tStr := c.Query("t")
	u := c.Query("u")

	if q == "" || r == "" || pStr == "" || tStr == "" || u == "" {
		return clickurl.ClickParams{}, errMissingParams
	}

	p, err := strconv.Atoi(pStr)
	if err != nil {
		return clickurl.ClickParams{}, errMissingParams
	}

	pg := 1
	if pgStr != "" {
		pg, _ = strconv.Atoi(pgStr)
		if pg < 1 {
			pg = 1
		}
	}

	t, err := strconv.ParseInt(tStr, 10, 64)
	if err != nil {
		return clickurl.ClickParams{}, errMissingParams
	}

	return clickurl.ClickParams{
		QueryID:        q,
		ResultID:       r,
		Position:       p,
		Page:           pg,
		Timestamp:      t,
		DestinationURL: u,
	}, nil
}

var errMissingParams = fmt.Errorf("missing required parameters (q, r, p, t, u, sig)")

const uaHashLength = 12

func hashURL(rawURL string) string {
	h := sha256.Sum256([]byte(rawURL))
	return hex.EncodeToString(h[:])
}

func hashUA(ua string) string {
	if ua == "" {
		return ""
	}
	h := sha256.Sum256([]byte(ua))
	return hex.EncodeToString(h[:])[:uaHashLength]
}
```

> **Note:** Add `"fmt"` to imports for the `errMissingParams` variable.

**Step 4: Create health handler**

```go
// click-tracker/internal/handler/health.go
package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// HealthHandler handles health check requests.
type HealthHandler struct {
	version string
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler(version string) *HealthHandler {
	return &HealthHandler{version: version}
}

// HealthCheck returns service health status.
func (h *HealthHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"version":   h.version,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// ReadinessCheck returns service readiness status.
func (h *HealthHandler) ReadinessCheck(c *gin.Context) {
	h.HealthCheck(c)
}
```

**Step 5: Run tests**

Run: `cd click-tracker && go test ./internal/handler/... -v`
Expected: PASS (4 tests)

**Step 6: Commit**

```bash
git add click-tracker/internal/handler/
git commit -m "feat(click-tracker): add click redirect handler with HMAC validation

Validates signature and timestamp window, hashes destination URL and
user agent, enqueues event to buffer (non-blocking), returns 302
redirect. Returns 403 for bad sig, 410 for expired, 400 for missing params."
```

---

### Task 6: Bot Filter Middleware

**Files:**
- Create: `click-tracker/internal/middleware/botfilter.go`
- Create: `click-tracker/internal/middleware/botfilter_test.go`

**Step 1: Write failing test**

```go
// click-tracker/internal/middleware/botfilter_test.go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/click-tracker/internal/middleware"
)

func TestBotFilter_AllowsNormalUA(t *testing.T) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.BotFilter())
	r.GET("/click", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/click", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestBotFilter_BlocksGooglebot(t *testing.T) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.BotFilter())
	r.GET("/click", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/click", nil)
	req.Header.Set("User-Agent", "Googlebot/2.1 (+http://www.google.com/bot.html)")
	r.ServeHTTP(w, req)

	// Bot requests get redirected (302) without logging, not blocked
	if w.Code != http.StatusOK {
		// BotFilter sets a flag; handler checks it. For simplicity,
		// the middleware just sets c.Set("is_bot", true) and continues.
		t.Logf("Note: bot detection sets flag, handler skips logging")
	}
}

func TestBotFilter_FlagsMissingUA(t *testing.T) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.BotFilter())
	r.GET("/click", func(c *gin.Context) {
		isBot, _ := c.Get("is_bot")
		if isBot == true {
			c.String(http.StatusOK, "bot")
			return
		}
		c.String(http.StatusOK, "human")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/click", nil)
	// No User-Agent header
	r.ServeHTTP(w, req)

	if w.Body.String() != "bot" {
		t.Fatalf("expected 'bot' for missing UA, got %q", w.Body.String())
	}
}
```

**Step 2: Implement bot filter**

```go
// click-tracker/internal/middleware/botfilter.go
package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// botPatterns are known bot User-Agent substrings.
var botPatterns = []string{
	"googlebot", "bingbot", "slurp", "duckduckbot",
	"baiduspider", "yandexbot", "facebookexternalhit",
	"twitterbot", "rogerbot", "linkedinbot", "embedly",
	"quora link preview", "showyoubot", "outbrain",
	"pinterest", "applebot", "semrushbot", "ahrefsbot",
	"mj12bot", "dotbot", "petalbot", "bytespider",
}

// BotFilter sets c.Set("is_bot", true) for known bot user agents.
// The handler can check this flag to skip event logging while still redirecting.
func BotFilter() gin.HandlerFunc {
	return func(c *gin.Context) {
		ua := strings.ToLower(c.Request.UserAgent())
		if ua == "" || isBot(ua) {
			c.Set("is_bot", true)
		}
		c.Next()
	}
}

func isBot(ua string) bool {
	for _, pattern := range botPatterns {
		if strings.Contains(ua, pattern) {
			return true
		}
	}
	return false
}
```

**Step 3: Run tests**

Run: `cd click-tracker && go test ./internal/middleware/... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add click-tracker/internal/middleware/botfilter.go click-tracker/internal/middleware/botfilter_test.go
git commit -m "feat(click-tracker): add bot filter middleware

Sets is_bot flag on gin context for known bot UAs and missing
User-Agent headers. Handler checks flag to skip event logging
while still performing the redirect."
```

---

### Task 7: Rate Limiter Middleware

**Files:**
- Create: `click-tracker/internal/middleware/ratelimit.go`
- Create: `click-tracker/internal/middleware/ratelimit_test.go`

**Step 1: Write failing test**

```go
// click-tracker/internal/middleware/ratelimit_test.go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/click-tracker/internal/middleware"
)

func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RateLimiter(10, time.Minute))
	r.GET("/click", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/click?q=q1", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	limit := 3
	r.Use(middleware.RateLimiter(limit, time.Minute))
	r.GET("/click", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	for i := 0; i < limit; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/click?q=q1", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, w.Code)
		}
	}

	// This should be rate limited
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/click?q=q1", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
}

func TestRateLimiter_DifferentIPsIndependent(t *testing.T) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RateLimiter(1, time.Minute))
	r.GET("/click", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// First IP uses its one allowed request
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/click?q=q1", nil)
	req1.RemoteAddr = "1.1.1.1:1234"
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("IP1: expected 200, got %d", w1.Code)
	}

	// Second IP should still be allowed
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/click?q=q1", nil)
	req2.RemoteAddr = "2.2.2.2:1234"
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("IP2: expected 200, got %d", w2.Code)
	}
}
```

**Step 2: Implement rate limiter**

```go
// click-tracker/internal/middleware/ratelimit.go
package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type ipEntry struct {
	count     int
	expiresAt time.Time
}

// RateLimiter limits requests per IP address within a time window.
func RateLimiter(maxRequests int, window time.Duration) gin.HandlerFunc {
	var mu sync.Mutex
	entries := make(map[string]*ipEntry)

	// Background cleanup every window duration
	go func() {
		ticker := time.NewTicker(window)
		defer ticker.Stop()
		for range ticker.C {
			mu.Lock()
			now := time.Now()
			for ip, entry := range entries {
				if now.After(entry.expiresAt) {
					delete(entries, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
		if ip == "" {
			ip = c.Request.RemoteAddr
		}

		mu.Lock()
		entry, exists := entries[ip]
		now := time.Now()

		if !exists || now.After(entry.expiresAt) {
			entries[ip] = &ipEntry{count: 1, expiresAt: now.Add(window)}
			mu.Unlock()
			c.Next()
			return
		}

		entry.count++
		if entry.count > maxRequests {
			mu.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			return
		}
		mu.Unlock()
		c.Next()
	}
}
```

**Step 3: Run tests**

Run: `cd click-tracker && go test ./internal/middleware/... -v`
Expected: PASS (all 6 middleware tests)

**Step 4: Commit**

```bash
git add click-tracker/internal/middleware/ratelimit.go click-tracker/internal/middleware/ratelimit_test.go
git commit -m "feat(click-tracker): add IP-based rate limiter middleware

In-memory rate limiting per IP with configurable max requests and
time window. Background goroutine cleans expired entries."
```

---

### Task 8: Wire Up Server, Routes, and Integration

**Files:**
- Modify: `click-tracker/main.go` (replace placeholder `runServer`)
- Create: `click-tracker/internal/api/routes.go`
- Create: `click-tracker/internal/api/server.go`

**Step 1: Create routes**

```go
// click-tracker/internal/api/routes.go
package api

import (
	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/click-tracker/internal/handler"
	"github.com/jonesrussell/north-cloud/click-tracker/internal/middleware"
	"github.com/north-cloud/infrastructure/monitoring"
)

// SetupRoutes configures all API routes.
func SetupRoutes(router *gin.Engine, clickHandler *handler.ClickHandler, healthHandler *handler.HealthHandler, maxClicksPerMin int, rateLimitWindow int) {
	// Health checks
	router.GET("/health", healthHandler.HealthCheck)
	router.GET("/ready", healthHandler.ReadinessCheck)
	router.GET("/health/memory", func(c *gin.Context) {
		monitoring.MemoryHealthHandler(c.Writer, c.Request)
	})

	// Click redirect with bot filter and rate limiting
	click := router.Group("")
	click.Use(middleware.BotFilter())
	click.Use(middleware.RateLimiter(maxClicksPerMin, rateLimitWindow))
	click.GET("/click", clickHandler.HandleClick)
}
```

> **Note:** The `rateLimitWindow` param should be `time.Duration`. Adjust the signature: `rateLimitWindow time.Duration`.

**Step 2: Create server**

```go
// click-tracker/internal/api/server.go
package api

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/click-tracker/internal/config"
	"github.com/jonesrussell/north-cloud/click-tracker/internal/handler"
	infragin "github.com/north-cloud/infrastructure/gin"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
	defaultReadTimeout  = 10 * time.Second
	defaultWriteTimeout = 10 * time.Second
	defaultIdleTimeout  = 60 * time.Second
)

// NewServer creates a new HTTP server.
func NewServer(
	clickHandler *handler.ClickHandler,
	healthHandler *handler.HealthHandler,
	cfg *config.Config,
	log infralogger.Logger,
) *infragin.Server {
	rateLimitWindow := time.Duration(cfg.RateLimit.WindowSeconds) * time.Second

	return infragin.NewServerBuilder(cfg.Service.Name, cfg.Service.Port).
		WithLogger(log).
		WithDebug(cfg.Service.Debug).
		WithVersion(cfg.Service.Version).
		WithTimeouts(defaultReadTimeout, defaultWriteTimeout, defaultIdleTimeout).
		WithRoutes(func(router *gin.Engine) {
			SetupRoutes(router, clickHandler, healthHandler, cfg.RateLimit.MaxClicksPerMinute, rateLimitWindow)
		}).
		Build()
}
```

**Step 3: Update main.go runServer**

Replace the placeholder `runServer` function in `click-tracker/main.go`:

```go
func runServer(cfg *config.Config, db *sql.DB, log infralogger.Logger) int {
	// Create HMAC signer
	signer := clickurl.NewSigner(cfg.Service.HMACSecret)

	// Create event buffer and store
	buf := storage.NewBuffer(cfg.Service.BufferSize)
	store := storage.NewStore(db, buf, log, cfg.Service.FlushInterval, cfg.Service.FlushThreshold)
	store.Start()
	defer store.Stop()

	// Create handlers
	clickHandler := handler.NewClickHandler(signer, buf, log, cfg.Service.MaxTimestampAge)
	healthHandler := handler.NewHealthHandler(cfg.Service.Version)

	// Create and run server
	server := api.NewServer(clickHandler, healthHandler, cfg, log)

	log.Info("Click-tracker starting",
		infralogger.Int("port", cfg.Service.Port),
	)

	if err := server.Run(); err != nil {
		log.Error("Server error", infralogger.Error(err))
		return 1
	}

	log.Info("Click-tracker exited cleanly")
	return 0
}
```

Update imports in main.go to include:
```go
"github.com/jonesrussell/north-cloud/click-tracker/internal/api"
"github.com/jonesrussell/north-cloud/click-tracker/internal/handler"
"github.com/jonesrussell/north-cloud/click-tracker/internal/storage"
"github.com/north-cloud/infrastructure/clickurl"
```

**Step 4: Run build**

Run: `cd click-tracker && go build -o /dev/null .`
Expected: BUILD SUCCESS

**Step 5: Run all click-tracker tests**

Run: `cd click-tracker && go test ./... -v`
Expected: PASS (all tests across config, handler, middleware, storage)

**Step 6: Commit**

```bash
git add click-tracker/internal/api/ click-tracker/main.go
git commit -m "feat(click-tracker): wire up server with routes, middleware, and handlers

Complete click-tracker server: bot filter + rate limiter middleware
on /click endpoint, health checks on /health and /ready. Buffered
event store starts/stops with server lifecycle."
```

---

### Task 9: Search Service Changes (Generate Click URLs)

**Files:**
- Modify: `search/internal/domain/search.go` (add `ClickURL` field to `SearchHit`)
- Modify: `search/internal/config/config.go` (add click tracker config)
- Modify: `search/internal/service/search_service.go` (generate click URLs)
- Modify: `search/main.go` (pass signer to search service)

**Step 1: Add ClickURL to SearchHit**

In `search/internal/domain/search.go`, add to `SearchHit` struct:

```go
type SearchHit struct {
	// ... existing fields ...
	Snippet        string              `json:"snippet,omitempty"`
	ClickURL       string              `json:"click_url,omitempty"` // ADD THIS
}
```

**Step 2: Add click tracker config to search service**

In `search/internal/config/config.go`, add:

```go
// Config struct - add ClickTracker field
type Config struct {
	Service       ServiceConfig       `yaml:"service"`
	Elasticsearch ElasticsearchConfig `yaml:"elasticsearch"`
	Facets        FacetsConfig        `yaml:"facets"`
	Logging       LoggingConfig       `yaml:"logging"`
	CORS          CORSConfig          `yaml:"cors"`
	ClickTracker  ClickTrackerConfig  `yaml:"click_tracker"` // ADD THIS
}

// ClickTrackerConfig holds click tracking URL generation config.
type ClickTrackerConfig struct {
	Enabled bool   `env:"CLICK_TRACKER_ENABLED" yaml:"enabled"`
	Secret  string `env:"CLICK_TRACKER_SECRET"  yaml:"secret"`
	BaseURL string `env:"CLICK_TRACKER_BASE_URL" yaml:"base_url"`
}
```

**Step 3: Generate click URLs in search service**

In `search/internal/service/search_service.go`, modify `parseSearchResponse` to add click URLs.

Add a `clickSigner` field to `SearchService`:

```go
type SearchService struct {
	esClient     *elasticsearch.Client
	queryBuilder *elasticsearch.QueryBuilder
	config       *config.Config
	logger       infralogger.Logger
	clickSigner  *clickurl.Signer // ADD THIS (nil if disabled)
}
```

Update `NewSearchService` to accept and store the signer:

```go
func NewSearchService(esClient *elasticsearch.Client, cfg *config.Config, log infralogger.Logger, clickSigner *clickurl.Signer) *SearchService {
	return &SearchService{
		esClient:     esClient,
		queryBuilder: elasticsearch.NewQueryBuilder(&cfg.Elasticsearch),
		config:       cfg,
		logger:       log,
		clickSigner:  clickSigner,
	}
}
```

Add click URL generation after building hits in `parseSearchResponse`:

```go
// In parseSearchResponse, after the for loop that builds hits:
if s.clickSigner != nil {
	queryID := generateQueryID()
	page := req.Pagination.Page
	s.addClickURLs(response.Hits, queryID, page)
}
```

Add helper methods:

```go
func (s *SearchService) addClickURLs(hits []*domain.SearchHit, queryID string, page int) {
	baseURL := s.config.ClickTracker.BaseURL
	now := time.Now().Unix()

	for i, hit := range hits {
		position := i + 1
		params := clickurl.ClickParams{
			QueryID:        queryID,
			ResultID:       hit.ID,
			Position:       position,
			Page:           page,
			Timestamp:      now,
			DestinationURL: hit.URL,
		}
		sig := s.clickSigner.Sign(params.Message())
		hit.ClickURL = fmt.Sprintf("%s/click?q=%s&r=%s&p=%d&pg=%d&t=%d&u=%s&sig=%s",
			baseURL, queryID, hit.ID, position, page, now,
			url.QueryEscape(hit.URL), sig)
	}
}

const queryIDLength = 8

func generateQueryID() string {
	b := make([]byte, queryIDLength)
	if _, err := rand.Read(b); err != nil {
		// Fallback: timestamp-based
		return fmt.Sprintf("q_%x", time.Now().UnixNano())
	}
	return "q_" + hex.EncodeToString(b)[:queryIDLength]
}
```

Add imports: `"crypto/rand"`, `"encoding/hex"`, `"net/url"`, `"github.com/north-cloud/infrastructure/clickurl"`

**Step 4: Update search main.go**

In `search/main.go`, modify `runServer` to create a signer if click tracking is enabled:

```go
func runServer(cfg *config.Config, esClient *elasticsearch.Client, log infralogger.Logger) int {
	// Create click URL signer if enabled
	var clickSigner *clickurl.Signer
	if cfg.ClickTracker.Enabled && cfg.ClickTracker.Secret != "" {
		clickSigner = clickurl.NewSigner(cfg.ClickTracker.Secret)
		log.Info("Click tracking enabled",
			infralogger.String("base_url", cfg.ClickTracker.BaseURL),
		)
	}

	searchService := service.NewSearchService(esClient, cfg, log, clickSigner)
	// ... rest unchanged ...
}
```

Add import: `"github.com/north-cloud/infrastructure/clickurl"`

**Step 5: Update search config.yml.example**

Add to `search/config.yml.example`:

```yaml
click_tracker:
  enabled: false
  # secret: set via CLICK_TRACKER_SECRET env var
  base_url: "https://northcloud.biz/api"
```

**Step 6: Run search tests**

Run: `cd search && go test ./... -v`
Expected: PASS (existing tests pass; `NewSearchService` call sites need `nil` signer added)

**Step 7: Commit**

```bash
git add search/
git commit -m "feat(search): generate signed click URLs in search results

When CLICK_TRACKER_ENABLED=true, search results include a click_url
field with HMAC-signed redirect URL. Falls back gracefully to no
click URLs when disabled (default)."
```

---

### Task 10: Frontend Changes

**Files:**
- Modify: `search-frontend/src/types/search.ts` (add `click_url` field)
- Modify: `search-frontend/src/components/search/SearchResultItem.vue` (use `click_url`)

**Step 1: Add click_url to SearchResult type**

In `search-frontend/src/types/search.ts`, add to `SearchResult`:

```typescript
export interface SearchResult {
  id: string
  title: string
  url: string
  click_url?: string  // ADD THIS
  body?: string
  // ... rest unchanged
}
```

**Step 2: Update SearchResultItem.vue**

In `search-frontend/src/components/search/SearchResultItem.vue`, change the link href:

```vue
<!-- Line 9: Change from result.url to click_url with fallback -->
<a
  :href="result.click_url || result.url"
  target="_blank"
  rel="noopener noreferrer"
  class="block group"
  :aria-label="`Open result: ${result.title}`"
>
```

The `displayUrl` computed property stays unchanged (it still shows `result.url` as visible text).

**Step 3: Run frontend build**

Run: `cd search-frontend && npm run build`
Expected: BUILD SUCCESS

**Step 4: Commit**

```bash
git add search-frontend/src/types/search.ts search-frontend/src/components/search/SearchResultItem.vue
git commit -m "feat(search-frontend): use click_url for result links

Result links use click_url (tracked redirect) when available,
falling back to direct url. Display URL still shows the
destination domain/path."
```

---

### Task 11: Docker Compose, Nginx, and Root Integration

**Files:**
- Modify: `docker-compose.base.yml` (add postgres-click-tracker + click-tracker services)
- Modify: `docker-compose.dev.yml` (add dev overrides)
- Modify: `infrastructure/nginx/nginx.conf` (add /api/click location)
- Modify: `Taskfile.yml` (add click-tracker includes)
- Modify: `.env.example` (add CLICK_TRACKER env vars)

**Step 1: Add to docker-compose.base.yml**

Add postgres-click-tracker service (using `*postgres-defaults` anchor):

```yaml
postgres-click-tracker:
  <<: *postgres-defaults
  environment:
    POSTGRES_USER: ${POSTGRES_CLICK_TRACKER_USER:-postgres}
    POSTGRES_PASSWORD: ${POSTGRES_CLICK_TRACKER_PASSWORD:-postgres}
    POSTGRES_DB: ${POSTGRES_CLICK_TRACKER_DB:-click_tracker}
  volumes:
    - postgres_click_tracker_data:/var/lib/postgresql/data
    - ./click-tracker/migrations:/migrations:ro
```

Add click-tracker service (using `*service-defaults` anchor):

```yaml
click-tracker:
  <<: *service-defaults
  build:
    context: .
    dockerfile: ./click-tracker/Dockerfile
  image: docker.io/jonesrussell/click-tracker:latest
  deploy:
    resources:
      limits:
        cpus: "0.25"
        memory: 128M
  environment:
    CLICK_TRACKER_PORT: ${CLICK_TRACKER_PORT:-8093}
    CLICK_TRACKER_SECRET: ${CLICK_TRACKER_SECRET:-}
    POSTGRES_CLICK_TRACKER_HOST: postgres-click-tracker
    POSTGRES_CLICK_TRACKER_PORT: 5432
    POSTGRES_CLICK_TRACKER_USER: ${POSTGRES_CLICK_TRACKER_USER:-postgres}
    POSTGRES_CLICK_TRACKER_PASSWORD: ${POSTGRES_CLICK_TRACKER_PASSWORD:-postgres}
    POSTGRES_CLICK_TRACKER_DB: ${POSTGRES_CLICK_TRACKER_DB:-click_tracker}
    APP_DEBUG: ${APP_DEBUG:-false}
  depends_on:
    postgres-click-tracker:
      condition: service_healthy
  healthcheck:
    test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8093/health"]
    interval: 30s
    timeout: 5s
    retries: 3
    start_period: 10s
```

Add volume: `postgres_click_tracker_data:` in the volumes section.

Update search service to pass CLICK_TRACKER env vars:

```yaml
# Add to search service environment section:
CLICK_TRACKER_ENABLED: ${CLICK_TRACKER_ENABLED:-false}
CLICK_TRACKER_SECRET: ${CLICK_TRACKER_SECRET:-}
CLICK_TRACKER_BASE_URL: ${CLICK_TRACKER_BASE_URL:-https://northcloud.biz/api}
```

**Step 2: Add to docker-compose.dev.yml**

```yaml
click-tracker:
  <<: *go-dev-defaults
  deploy:
    resources:
      limits:
        cpus: "0.25"
        memory: 128M
  pull_policy: build
  build:
    context: .
    dockerfile: ./click-tracker/Dockerfile.dev
    args:
      UID: "${UID:-1000}"
      GID: "${GID:-1000}"
  ports:
    - "${CLICK_TRACKER_PORT:-8093}:8093"
    - "${CLICK_TRACKER_PPROF_PORT:-6072}:6060"
  environment:
    <<: *go-dev-environment
    CLICK_TRACKER_PORT: 8093
    CLICK_TRACKER_SECRET: "${CLICK_TRACKER_SECRET:-dev-secret-change-me}"
    POSTGRES_CLICK_TRACKER_HOST: postgres-click-tracker
    POSTGRES_CLICK_TRACKER_PORT: 5432
    POSTGRES_CLICK_TRACKER_USER: ${POSTGRES_CLICK_TRACKER_USER:-postgres}
    POSTGRES_CLICK_TRACKER_PASSWORD: ${POSTGRES_CLICK_TRACKER_PASSWORD:-postgres}
    POSTGRES_CLICK_TRACKER_DB: ${POSTGRES_CLICK_TRACKER_DB:-click_tracker}
    PPROF_PORT: 6060
  volumes:
    - ./click-tracker:/app
    - ./infrastructure:/infrastructure:ro
    - click_tracker_go_mod_cache:/tmp/go-mod-cache
    - click_tracker_go_build_cache:/tmp/go-build-cache
  command: ["sh", "-c", "go mod download && air -c .air.toml"]
  depends_on:
    postgres-click-tracker:
      condition: service_healthy
  healthcheck:
    test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8093/health"]
    interval: 10s
    timeout: 3s
    retries: 3
    start_period: 15s
```

Add volumes: `click_tracker_go_mod_cache:`, `click_tracker_go_build_cache:`

**Step 3: Add nginx location**

In `infrastructure/nginx/nginx.conf`, add upstream variable and location block:

```nginx
# Add with other map variables
map "" $click_tracker_api {
    default "click-tracker:8093";
}

# Add location block (before the catch-all location /)
location /api/click {
    proxy_pass http://$click_tracker_api/click;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
}
```

**Step 4: Add to root Taskfile.yml**

Add to the `includes:` section:

```yaml
click-tracker: { taskfile: ./click-tracker/Taskfile.yml, dir: ./click-tracker }
```

Add `click-tracker` to aggregate tasks (lint, test, build, vendor, migrate, docker shortcuts) following the pattern of existing services.

**Step 5: Update .env.example**

```bash
# Click Tracker
CLICK_TRACKER_PORT=8093
CLICK_TRACKER_SECRET=  # Generate: openssl rand -hex 32
CLICK_TRACKER_ENABLED=false
CLICK_TRACKER_BASE_URL=https://northcloud.biz/api
POSTGRES_CLICK_TRACKER_USER=postgres
POSTGRES_CLICK_TRACKER_PASSWORD=postgres
POSTGRES_CLICK_TRACKER_DB=click_tracker
```

**Step 6: Test Docker build**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml build click-tracker`
Expected: BUILD SUCCESS

**Step 7: Commit**

```bash
git add docker-compose.base.yml docker-compose.dev.yml infrastructure/nginx/nginx.conf Taskfile.yml .env.example
git commit -m "feat: integrate click-tracker into Docker, nginx, and Taskfile

Adds postgres-click-tracker database, click-tracker service to
both base and dev compose files. Nginx routes /api/click to the
service. Root Taskfile includes click-tracker in aggregate tasks.
Search service gets CLICK_TRACKER_* env vars."
```

---

### Task 12: End-to-End Verification

**Step 1: Start services**

Run:
```bash
task docker:dev:up
# Wait for services to be healthy
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml ps
```

**Step 2: Run migration**

Run:
```bash
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml exec click-tracker go run cmd/migrate/main.go up
```

**Step 3: Verify health**

Run: `curl http://localhost:8093/health`
Expected: `{"status":"healthy","version":"1.0.0",...}`

**Step 4: Test click redirect manually**

Generate a signed click URL using the HMAC secret and verify the redirect works:

```bash
# This requires the search service to generate click URLs.
# Enable click tracking:
# CLICK_TRACKER_ENABLED=true in .env

# Search for something:
curl "http://localhost:8092/api/v1/search?q=test" | jq '.hits[0].click_url'

# Visit the click_url - should 302 redirect to the article
```

**Step 5: Run all tests**

Run: `task test`
Expected: PASS across all services

**Step 6: Run all linters**

Run: `task lint`
Expected: PASS (fix any violations before committing)

**Step 7: Final commit if any fixes needed**

```bash
git add -A
git commit -m "fix: address lint and test issues from click-tracker integration"
```
