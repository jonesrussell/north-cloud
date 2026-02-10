# Pipeline Service Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the Pipeline Service (Phase 1) and shared client library (Phase 2) from the design in `docs/PIPELINE.md`.

**Architecture:** New Go microservice (`pipeline/`) with PostgreSQL backend, append-only event table partitioned by month, and HTTP API for event ingest + funnel/stats queries. Shared client library in `infrastructure/pipeline/` for fire-and-forget event emission with circuit breaker.

**Tech Stack:** Go 1.25, PostgreSQL 16, Gin HTTP framework, infrastructure packages (config, logger, gin, jwt, profiling), Docker, Air hot reload.

**Design doc:** `docs/PIPELINE.md` (Sections 1-7)

---

## Task 1: Initialize Go Module

**Files:**
- Create: `pipeline/go.mod`
- Create: `pipeline/go.sum`

**Step 1: Create go.mod**

```bash
cd /home/fsd42/dev/north-cloud/pipeline && go mod init github.com/jonesrussell/north-cloud/pipeline
```

**Step 2: Add infrastructure replace directive and core dependencies**

Edit `pipeline/go.mod` to match project conventions:

```
module github.com/jonesrussell/north-cloud/pipeline

go 1.25

require (
	github.com/gin-gonic/gin v1.11.0
	github.com/lib/pq v1.11.0
	github.com/north-cloud/infrastructure v0.0.0
	github.com/stretchr/testify v1.11.1
)

replace github.com/north-cloud/infrastructure => ../infrastructure
```

**Step 3: Download dependencies**

Run: `cd /home/fsd42/dev/north-cloud/pipeline && go mod tidy`

**Step 4: Commit**

```bash
git add pipeline/go.mod pipeline/go.sum
git commit -m "feat(pipeline): initialize Go module"
```

---

## Task 2: Config Package

**Files:**
- Create: `pipeline/internal/config/config.go`
- Create: `pipeline/config.yml`

**Step 1: Write config struct**

Create `pipeline/internal/config/config.go`:

```go
package config

import (
	"time"

	infraconfig "github.com/north-cloud/infrastructure/config"
)

const (
	defaultServiceName    = "pipeline"
	defaultServiceVersion = "1.0.0"
	defaultServicePort    = 8075
	defaultDBHost         = "localhost"
	defaultDBPort         = 5432
	defaultDBUser         = "postgres"
	defaultDBName         = "pipeline"
	defaultDBSSLMode      = "disable"
	defaultDBMaxConns     = 25
	defaultDBMaxIdle      = 5
	defaultDBConnLifetime = time.Hour
	defaultLogLevel       = "info"
	defaultLogFormat      = "json"
)

// Config holds all configuration for the pipeline service.
type Config struct {
	Service  ServiceConfig  `yaml:"service"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
	Logging  LoggingConfig  `yaml:"logging"`
}

// ServiceConfig holds service-level configuration.
type ServiceConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Port    int    `env:"PIPELINE_PORT" yaml:"port"`
	Debug   bool   `env:"APP_DEBUG"     yaml:"debug"`
}

// DatabaseConfig holds database connection configuration.
type DatabaseConfig struct {
	Host                  string        `env:"POSTGRES_PIPELINE_HOST"     yaml:"host"`
	Port                  int           `env:"POSTGRES_PIPELINE_PORT"     yaml:"port"`
	User                  string        `env:"POSTGRES_PIPELINE_USER"     yaml:"user"`
	Password              string        `env:"POSTGRES_PIPELINE_PASSWORD" yaml:"password"`
	Database              string        `env:"POSTGRES_PIPELINE_DB"       yaml:"database"`
	SSLMode               string        `yaml:"sslmode"`
	MaxConnections        int           `yaml:"max_connections"`
	MaxIdleConns          int           `yaml:"max_idle_connections"`
	ConnectionMaxLifetime time.Duration `yaml:"connection_max_lifetime"`
}

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	JWTSecret string `env:"AUTH_JWT_SECRET" yaml:"jwt_secret"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Level  string `env:"LOG_LEVEL"  yaml:"level"`
	Format string `env:"LOG_FORMAT" yaml:"format"`
}

// Load reads config from a YAML file with env overrides.
func Load(path string) (*Config, error) {
	return infraconfig.LoadWithDefaults[Config](path, setDefaults)
}

func setDefaults(cfg *Config) {
	setServiceDefaults(&cfg.Service)
	setDatabaseDefaults(&cfg.Database)
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
}

func setDatabaseDefaults(d *DatabaseConfig) {
	if d.Host == "" {
		d.Host = defaultDBHost
	}
	if d.Port == 0 {
		d.Port = defaultDBPort
	}
	if d.User == "" {
		d.User = defaultDBUser
	}
	if d.Database == "" {
		d.Database = defaultDBName
	}
	if d.SSLMode == "" {
		d.SSLMode = defaultDBSSLMode
	}
	if d.MaxConnections == 0 {
		d.MaxConnections = defaultDBMaxConns
	}
	if d.MaxIdleConns == 0 {
		d.MaxIdleConns = defaultDBMaxIdle
	}
	if d.ConnectionMaxLifetime == 0 {
		d.ConnectionMaxLifetime = defaultDBConnLifetime
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

// Validate checks that required config fields are present.
func (c *Config) Validate() error {
	if validateErr := infraconfig.ValidatePort("service.port", c.Service.Port); validateErr != nil {
		return validateErr
	}
	if c.Database.Host == "" {
		return &infraconfig.ValidationError{Field: "database.host", Message: "is required"}
	}
	return nil
}
```

**Step 2: Write config.yml**

Create `pipeline/config.yml`:

```yaml
service:
  name: "pipeline"
  version: "1.0.0"
  port: 8075
  debug: true

database:
  host: "postgres-pipeline"
  port: 5432
  user: "postgres"
  password: "postgres"
  database: "pipeline"
  sslmode: "disable"
  max_connections: 25
  max_idle_connections: 5
  connection_max_lifetime: "1h"

auth:
  jwt_secret: ""

logging:
  level: "info"
  format: "json"
```

**Step 3: Run `go mod tidy`**

Run: `cd /home/fsd42/dev/north-cloud/pipeline && go mod tidy`
Expected: Success, no errors.

**Step 4: Commit**

```bash
git add pipeline/internal/config/config.go pipeline/config.yml pipeline/go.mod pipeline/go.sum
git commit -m "feat(pipeline): add config package and config.yml"
```

---

## Task 3: Database Connection and Migrations

**Files:**
- Create: `pipeline/internal/database/connection.go`
- Create: `pipeline/migrations/001_create_pipeline_events.up.sql`
- Create: `pipeline/migrations/001_create_pipeline_events.down.sql`

**Step 1: Write database connection**

Create `pipeline/internal/database/connection.go`:

```go
package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
)

const dbConnectionTimeout = 5 * time.Second

// Config holds database connection configuration.
type Config struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string
	MaxConnections  int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// Connection wraps a database/sql.DB with convenience methods.
type Connection struct {
	DB *sql.DB
}

// NewConnection creates a new database connection with the given config.
func NewConnection(cfg *Config) (*Connection, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxConnections)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	ctx, cancel := context.WithTimeout(context.Background(), dbConnectionTimeout)
	defer cancel()

	if pingErr := db.PingContext(ctx); pingErr != nil {
		return nil, fmt.Errorf("ping database: %w", pingErr)
	}

	return &Connection{DB: db}, nil
}

// Ping checks that the database is reachable.
func (c *Connection) Ping(ctx context.Context) error {
	return c.DB.PingContext(ctx)
}

// Close closes the database connection.
func (c *Connection) Close() error {
	return c.DB.Close()
}
```

**Step 2: Write up migration**

Create `pipeline/migrations/001_create_pipeline_events.up.sql`:

```sql
-- Migration: Create pipeline events schema
-- Version: 001
-- Date: 2026-02-10

-- articles: stable lookup table for cross-service analytics
CREATE TABLE IF NOT EXISTS articles (
    url             TEXT PRIMARY KEY,
    url_hash        CHAR(64) NOT NULL,
    domain          TEXT NOT NULL,
    source_name     TEXT NOT NULL,
    first_seen_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(url_hash)
);

CREATE INDEX IF NOT EXISTS idx_articles_source ON articles(source_name);
CREATE INDEX IF NOT EXISTS idx_articles_hash ON articles(url_hash);
CREATE INDEX IF NOT EXISTS idx_articles_domain ON articles(domain);

-- pipeline stage enum
CREATE TYPE pipeline_stage AS ENUM (
    'crawled',
    'indexed',
    'classified',
    'routed',
    'published'
);

-- stage ordering lookup (avoids CASE statements as enum grows)
CREATE TABLE IF NOT EXISTS stage_ordering (
    stage       pipeline_stage PRIMARY KEY,
    sort_order  SMALLINT NOT NULL UNIQUE
);

INSERT INTO stage_ordering (stage, sort_order) VALUES
    ('crawled', 1),
    ('indexed', 2),
    ('classified', 3),
    ('routed', 4),
    ('published', 5);

-- pipeline_events: append-only canonical event stream, partitioned by month
CREATE TABLE IF NOT EXISTS pipeline_events (
    id                      BIGSERIAL,
    article_url             TEXT NOT NULL REFERENCES articles(url),
    stage                   pipeline_stage NOT NULL,
    occurred_at             TIMESTAMPTZ NOT NULL,
    received_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    service_name            TEXT NOT NULL,
    metadata                JSONB,
    metadata_schema_version SMALLINT NOT NULL DEFAULT 1,
    idempotency_key         TEXT,
    PRIMARY KEY (id, occurred_at),
    UNIQUE (idempotency_key, occurred_at)
) PARTITION BY RANGE (occurred_at);

-- Create partitions for current and next months
CREATE TABLE pipeline_events_2026_01 PARTITION OF pipeline_events
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE pipeline_events_2026_02 PARTITION OF pipeline_events
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');
CREATE TABLE pipeline_events_2026_03 PARTITION OF pipeline_events
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');
CREATE TABLE pipeline_events_2026_04 PARTITION OF pipeline_events
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');

CREATE INDEX IF NOT EXISTS idx_events_stage_time ON pipeline_events(stage, occurred_at);
CREATE INDEX IF NOT EXISTS idx_events_article ON pipeline_events(article_url);
CREATE INDEX IF NOT EXISTS idx_events_occurred ON pipeline_events(occurred_at);
```

**Step 3: Write down migration**

Create `pipeline/migrations/001_create_pipeline_events.down.sql`:

```sql
-- Rollback: Drop pipeline events schema
-- WARNING: This will permanently delete all pipeline event data

DROP INDEX IF EXISTS idx_events_occurred;
DROP INDEX IF EXISTS idx_events_article;
DROP INDEX IF EXISTS idx_events_stage_time;

DROP TABLE IF EXISTS pipeline_events_2026_04;
DROP TABLE IF EXISTS pipeline_events_2026_03;
DROP TABLE IF EXISTS pipeline_events_2026_02;
DROP TABLE IF EXISTS pipeline_events_2026_01;
DROP TABLE IF EXISTS pipeline_events;
DROP TABLE IF EXISTS stage_ordering;
DROP TYPE IF EXISTS pipeline_stage;

DROP INDEX IF EXISTS idx_articles_domain;
DROP INDEX IF EXISTS idx_articles_hash;
DROP INDEX IF EXISTS idx_articles_source;
DROP TABLE IF EXISTS articles;
```

**Step 4: Commit**

```bash
git add pipeline/internal/database/connection.go pipeline/migrations/
git commit -m "feat(pipeline): add database connection and migration schema"
```

---

## Task 4: Domain Models

**Files:**
- Create: `pipeline/internal/domain/models.go`

**Step 1: Write domain models**

Create `pipeline/internal/domain/models.go`:

```go
package domain

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Stage represents a step in the content pipeline.
type Stage string

const (
	StageCrawled    Stage = "crawled"
	StageIndexed    Stage = "indexed"
	StageClassified Stage = "classified"
	StageRouted     Stage = "routed"
	StagePublished  Stage = "published"
)

// validStages is the set of allowed pipeline stages.
var validStages = map[Stage]bool{
	StageCrawled:    true,
	StageIndexed:    true,
	StageClassified: true,
	StageRouted:     true,
	StagePublished:  true,
}

// IsValid returns true if the stage is a recognized pipeline stage.
func (s Stage) IsValid() bool {
	return validStages[s]
}

// Article represents an article tracked in the pipeline.
type Article struct {
	URL         string    `json:"url"`
	URLHash     string    `json:"url_hash"`
	Domain      string    `json:"domain"`
	SourceName  string    `json:"source_name"`
	FirstSeenAt time.Time `json:"first_seen_at"`
}

// PipelineEvent represents a single stage transition in the pipeline.
type PipelineEvent struct {
	ID                    int64          `json:"id"`
	ArticleURL            string         `json:"article_url"`
	Stage                 Stage          `json:"stage"`
	OccurredAt            time.Time      `json:"occurred_at"`
	ReceivedAt            time.Time      `json:"received_at"`
	ServiceName           string         `json:"service_name"`
	Metadata              map[string]any `json:"metadata,omitempty"`
	MetadataSchemaVersion int            `json:"metadata_schema_version"`
	IdempotencyKey        string         `json:"idempotency_key,omitempty"`
}

// IngestRequest is the payload for POST /api/v1/events.
type IngestRequest struct {
	ArticleURL     string         `json:"article_url"  binding:"required"`
	SourceName     string         `json:"source_name"  binding:"required"`
	Stage          Stage          `json:"stage"         binding:"required"`
	OccurredAt     time.Time      `json:"occurred_at"   binding:"required"`
	ServiceName    string         `json:"service_name"  binding:"required"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// BatchIngestRequest is the payload for POST /api/v1/events/batch.
type BatchIngestRequest struct {
	Events []IngestRequest `json:"events" binding:"required,min=1"`
}

// FunnelStage represents one stage in the funnel response.
type FunnelStage struct {
	Name           string `json:"name"`
	Count          int64  `json:"count"`
	UniqueArticles int64  `json:"unique_articles"`
}

// FunnelResponse is the response for GET /api/v1/funnel.
type FunnelResponse struct {
	Period      string        `json:"period"`
	Timezone    string        `json:"timezone"`
	From        time.Time     `json:"from"`
	To          time.Time     `json:"to"`
	Stages      []FunnelStage `json:"stages"`
	GeneratedAt time.Time     `json:"generated_at"`
}

// URLHash computes the SHA-256 hash of a URL.
func URLHash(rawURL string) string {
	h := sha256.Sum256([]byte(rawURL))
	return fmt.Sprintf("%x", h)
}

// URLHashShort returns the first 8 characters of the URL hash.
func URLHashShort(rawURL string) string {
	hash := URLHash(rawURL)
	const shortHashLen = 8
	return hash[:shortHashLen]
}

// ExtractDomain extracts the hostname from a URL.
func ExtractDomain(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return "unknown"
	}
	return strings.TrimPrefix(parsed.Hostname(), "www.")
}

// GenerateIdempotencyKey builds the default idempotency key.
func GenerateIdempotencyKey(serviceName string, stage Stage, articleURL string, occurredAt time.Time) string {
	return fmt.Sprintf("%s:%s:%s:%s",
		serviceName,
		string(stage),
		URLHashShort(articleURL),
		occurredAt.UTC().Format(time.RFC3339),
	)
}
```

**Step 2: Write tests for domain helpers**

Create `pipeline/internal/domain/models_test.go`:

```go
package domain

import (
	"testing"
	"time"
)

func TestStage_IsValid(t *testing.T) {
	t.Helper()

	testCases := []struct {
		name  string
		stage Stage
		want  bool
	}{
		{"crawled is valid", StageCrawled, true},
		{"indexed is valid", StageIndexed, true},
		{"classified is valid", StageClassified, true},
		{"routed is valid", StageRouted, true},
		{"published is valid", StagePublished, true},
		{"empty is invalid", Stage(""), false},
		{"unknown is invalid", Stage("unknown"), false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.stage.IsValid(); got != tc.want {
				t.Errorf("Stage(%q).IsValid() = %v, want %v", tc.stage, got, tc.want)
			}
		})
	}
}

func TestExtractDomain(t *testing.T) {
	t.Helper()

	testCases := []struct {
		name string
		url  string
		want string
	}{
		{"simple domain", "https://example.com/article", "example.com"},
		{"www prefix stripped", "https://www.example.com/article", "example.com"},
		{"with port", "https://example.com:8080/article", "example.com"},
		{"invalid url", "not-a-url", "unknown"},
		{"empty url", "", "unknown"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExtractDomain(tc.url); got != tc.want {
				t.Errorf("ExtractDomain(%q) = %q, want %q", tc.url, got, tc.want)
			}
		})
	}
}

func TestURLHashShort(t *testing.T) {
	t.Helper()

	hash := URLHashShort("https://example.com/article")

	const expectedLen = 8
	if len(hash) != expectedLen {
		t.Errorf("URLHashShort() length = %d, want %d", len(hash), expectedLen)
	}

	// Deterministic
	hash2 := URLHashShort("https://example.com/article")
	if hash != hash2 {
		t.Errorf("URLHashShort() not deterministic: %q != %q", hash, hash2)
	}

	// Different URLs produce different hashes
	hash3 := URLHashShort("https://example.com/other")
	if hash == hash3 {
		t.Errorf("URLHashShort() collision: %q == %q", hash, hash3)
	}
}

func TestGenerateIdempotencyKey(t *testing.T) {
	t.Helper()

	ts := time.Date(2026, 2, 10, 14, 30, 0, 0, time.UTC)
	key := GenerateIdempotencyKey("classifier", StageClassified, "https://example.com/article", ts)

	if key == "" {
		t.Fatal("GenerateIdempotencyKey() returned empty string")
	}

	// Deterministic
	key2 := GenerateIdempotencyKey("classifier", StageClassified, "https://example.com/article", ts)
	if key != key2 {
		t.Errorf("GenerateIdempotencyKey() not deterministic: %q != %q", key, key2)
	}

	// Different inputs produce different keys
	key3 := GenerateIdempotencyKey("crawler", StageCrawled, "https://example.com/article", ts)
	if key == key3 {
		t.Errorf("GenerateIdempotencyKey() collision with different service/stage")
	}
}
```

**Step 3: Run tests to verify they pass**

Run: `cd /home/fsd42/dev/north-cloud/pipeline && go test ./internal/domain/ -v`
Expected: All 4 tests PASS.

**Step 4: Lint**

Run: `cd /home/fsd42/dev/north-cloud/pipeline && golangci-lint run --config ../.golangci.yml ./internal/domain/`
Expected: No issues.

**Step 5: Commit**

```bash
git add pipeline/internal/domain/
git commit -m "feat(pipeline): add domain models with URL hashing and idempotency"
```

---

## Task 5: Event Repository

**Files:**
- Create: `pipeline/internal/database/repository.go`
- Create: `pipeline/internal/database/repository_test.go`

**Step 1: Write the repository test**

Create `pipeline/internal/database/repository_test.go`:

```go
//nolint:testpackage // Testing internal repository requires same package access
package database

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jonesrussell/north-cloud/pipeline/internal/domain"
)

func TestRepository_UpsertArticle(t *testing.T) {
	t.Helper()

	db, mock, setupErr := sqlmock.New()
	if setupErr != nil {
		t.Fatalf("failed to create sqlmock: %v", setupErr)
	}
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	mock.ExpectExec("INSERT INTO articles").
		WithArgs("https://example.com/article", sqlmock.AnyArg(), "example.com", "example_com").
		WillReturnResult(sqlmock.NewResult(0, 1))

	upsertErr := repo.UpsertArticle(ctx, &domain.Article{
		URL:        "https://example.com/article",
		URLHash:    domain.URLHash("https://example.com/article"),
		Domain:     "example.com",
		SourceName: "example_com",
	})

	if upsertErr != nil {
		t.Errorf("UpsertArticle() error = %v", upsertErr)
	}

	if expectErr := mock.ExpectationsWereMet(); expectErr != nil {
		t.Errorf("unfulfilled expectations: %v", expectErr)
	}
}

func TestRepository_InsertEvent(t *testing.T) {
	t.Helper()

	db, mock, setupErr := sqlmock.New()
	if setupErr != nil {
		t.Fatalf("failed to create sqlmock: %v", setupErr)
	}
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	occurredAt := time.Date(2026, 2, 10, 14, 30, 0, 0, time.UTC)

	mock.ExpectQuery("INSERT INTO pipeline_events").
		WithArgs(
			"https://example.com/article",
			"classified",
			occurredAt,
			"classifier",
			sqlmock.AnyArg(), // metadata JSONB
			1,                // schema version
			sqlmock.AnyArg(), // idempotency key
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	event := &domain.PipelineEvent{
		ArticleURL:            "https://example.com/article",
		Stage:                 domain.StageClassified,
		OccurredAt:            occurredAt,
		ServiceName:           "classifier",
		Metadata:              map[string]any{"quality_score": 78},
		MetadataSchemaVersion: 1,
		IdempotencyKey:        "test-key",
	}

	insertErr := repo.InsertEvent(ctx, event)
	if insertErr != nil {
		t.Errorf("InsertEvent() error = %v", insertErr)
	}

	if expectErr := mock.ExpectationsWereMet(); expectErr != nil {
		t.Errorf("unfulfilled expectations: %v", expectErr)
	}
}

func TestRepository_GetFunnel(t *testing.T) {
	t.Helper()

	db, mock, setupErr := sqlmock.New()
	if setupErr != nil {
		t.Fatalf("failed to create sqlmock: %v", setupErr)
	}
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	from := time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 2, 11, 0, 0, 0, 0, time.UTC)

	rows := sqlmock.NewRows([]string{"stage", "count", "unique_articles"}).
		AddRow("crawled", 100, 90).
		AddRow("indexed", 90, 85).
		AddRow("classified", 80, 70)

	mock.ExpectQuery("SELECT").
		WithArgs(from, to).
		WillReturnRows(rows)

	stages, queryErr := repo.GetFunnel(ctx, from, to)
	if queryErr != nil {
		t.Fatalf("GetFunnel() error = %v", queryErr)
	}

	const expectedStages = 3
	if len(stages) != expectedStages {
		t.Errorf("GetFunnel() returned %d stages, want %d", len(stages), expectedStages)
	}

	if stages[0].Name != "crawled" {
		t.Errorf("stages[0].Name = %q, want %q", stages[0].Name, "crawled")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /home/fsd42/dev/north-cloud/pipeline && go test ./internal/database/ -v`
Expected: FAIL (repository.go doesn't exist yet)

**Step 3: Write the repository**

Create `pipeline/internal/database/repository.go`:

```go
package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/jonesrussell/north-cloud/pipeline/internal/domain"
)

// Repository handles database operations for pipeline events.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new repository with the given database connection.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Ping checks database connectivity.
func (r *Repository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

// UpsertArticle inserts or updates an article record.
func (r *Repository) UpsertArticle(ctx context.Context, article *domain.Article) error {
	query := `
		INSERT INTO articles (url, url_hash, domain, source_name)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (url) DO NOTHING
	`

	_, err := r.db.ExecContext(ctx, query,
		article.URL,
		article.URLHash,
		article.Domain,
		article.SourceName,
	)
	if err != nil {
		return fmt.Errorf("upsert article: %w", err)
	}

	return nil
}

// InsertEvent inserts a pipeline event. Returns nil on idempotent duplicate.
func (r *Repository) InsertEvent(ctx context.Context, event *domain.PipelineEvent) error {
	metadataJSON, marshalErr := json.Marshal(event.Metadata)
	if marshalErr != nil {
		return fmt.Errorf("marshal metadata: %w", marshalErr)
	}

	query := `
		INSERT INTO pipeline_events
			(article_url, stage, occurred_at, service_name, metadata, metadata_schema_version, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (idempotency_key, occurred_at) DO NOTHING
		RETURNING id
	`

	var id int64
	scanErr := r.db.QueryRowContext(ctx, query,
		event.ArticleURL,
		string(event.Stage),
		event.OccurredAt,
		event.ServiceName,
		metadataJSON,
		event.MetadataSchemaVersion,
		event.IdempotencyKey,
	).Scan(&id)

	if scanErr == sql.ErrNoRows {
		// Idempotent duplicate — not an error
		return nil
	}
	if scanErr != nil {
		return fmt.Errorf("insert event: %w", scanErr)
	}

	event.ID = id
	return nil
}

// GetFunnel returns aggregated funnel stages for a time range.
func (r *Repository) GetFunnel(ctx context.Context, from, to time.Time) ([]domain.FunnelStage, error) {
	query := `
		SELECT
			pe.stage,
			COUNT(*) AS count,
			COUNT(DISTINCT pe.article_url) AS unique_articles
		FROM pipeline_events pe
		JOIN stage_ordering so ON pe.stage = so.stage
		WHERE pe.occurred_at >= $1 AND pe.occurred_at < $2
		GROUP BY pe.stage, so.sort_order
		ORDER BY so.sort_order
	`

	rows, queryErr := r.db.QueryContext(ctx, query, from, to)
	if queryErr != nil {
		return nil, fmt.Errorf("query funnel: %w", queryErr)
	}
	defer rows.Close()

	var stages []domain.FunnelStage
	for rows.Next() {
		var s domain.FunnelStage
		if scanErr := rows.Scan(&s.Name, &s.Count, &s.UniqueArticles); scanErr != nil {
			return nil, fmt.Errorf("scan funnel row: %w", scanErr)
		}
		stages = append(stages, s)
	}

	if closeErr := rows.Err(); closeErr != nil {
		return nil, fmt.Errorf("funnel rows: %w", closeErr)
	}

	return stages, nil
}
```

Note: You'll need to add `"time"` to the imports in repository.go for the `GetFunnel` method.

**Step 4: Add go-sqlmock dependency**

Run: `cd /home/fsd42/dev/north-cloud/pipeline && go get github.com/DATA-DOG/go-sqlmock@v1.5.2 && go mod tidy`

**Step 5: Run tests to verify they pass**

Run: `cd /home/fsd42/dev/north-cloud/pipeline && go test ./internal/database/ -v`
Expected: All 3 tests PASS.

**Step 6: Lint**

Run: `cd /home/fsd42/dev/north-cloud/pipeline && golangci-lint run --config ../.golangci.yml ./internal/database/`
Expected: No issues.

**Step 7: Commit**

```bash
git add pipeline/internal/database/ pipeline/go.mod pipeline/go.sum
git commit -m "feat(pipeline): add event repository with upsert, insert, and funnel queries"
```

---

## Task 6: Pipeline Service Layer

**Files:**
- Create: `pipeline/internal/service/pipeline.go`
- Create: `pipeline/internal/service/pipeline_test.go`

**Step 1: Write the failing test**

Create `pipeline/internal/service/pipeline_test.go`:

```go
//nolint:testpackage // Testing internal service requires same package access
package service

import (
	"context"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/pipeline/internal/domain"
)

type mockRepository struct {
	upsertArticleFunc func(ctx context.Context, article *domain.Article) error
	insertEventFunc   func(ctx context.Context, event *domain.PipelineEvent) error
	getFunnelFunc     func(ctx context.Context, from, to time.Time) ([]domain.FunnelStage, error)
}

func (m *mockRepository) UpsertArticle(ctx context.Context, article *domain.Article) error {
	if m.upsertArticleFunc != nil {
		return m.upsertArticleFunc(ctx, article)
	}
	return nil
}

func (m *mockRepository) InsertEvent(ctx context.Context, event *domain.PipelineEvent) error {
	if m.insertEventFunc != nil {
		return m.insertEventFunc(ctx, event)
	}
	return nil
}

func (m *mockRepository) GetFunnel(ctx context.Context, from, to time.Time) ([]domain.FunnelStage, error) {
	if m.getFunnelFunc != nil {
		return m.getFunnelFunc(ctx, from, to)
	}
	return nil, nil
}

func (m *mockRepository) Ping(_ context.Context) error { return nil }

type mockLogger struct{}

func (m *mockLogger) Debug(_ string, _ ...any) {}
func (m *mockLogger) Info(_ string, _ ...any)  {}
func (m *mockLogger) Warn(_ string, _ ...any)  {}
func (m *mockLogger) Error(_ string, _ ...any) {}
func (m *mockLogger) With(_ ...any) Logger     { return m }
func (m *mockLogger) Sync() error              { return nil }

func TestPipelineService_Ingest_ValidEvent(t *testing.T) {
	t.Helper()

	var upsertCalled, insertCalled bool
	repo := &mockRepository{
		upsertArticleFunc: func(_ context.Context, article *domain.Article) error {
			upsertCalled = true
			if article.Domain != "example.com" {
				t.Errorf("article.Domain = %q, want %q", article.Domain, "example.com")
			}
			return nil
		},
		insertEventFunc: func(_ context.Context, event *domain.PipelineEvent) error {
			insertCalled = true
			if event.IdempotencyKey == "" {
				t.Error("expected idempotency key to be auto-generated")
			}
			return nil
		},
	}

	svc := NewPipelineService(repo, &mockLogger{})
	ctx := context.Background()

	req := &domain.IngestRequest{
		ArticleURL:  "https://example.com/article",
		SourceName:  "example_com",
		Stage:       domain.StageCrawled,
		OccurredAt:  time.Now().UTC(),
		ServiceName: "crawler",
	}

	ingestErr := svc.Ingest(ctx, req)
	if ingestErr != nil {
		t.Fatalf("Ingest() error = %v", ingestErr)
	}

	if !upsertCalled {
		t.Error("expected UpsertArticle to be called")
	}
	if !insertCalled {
		t.Error("expected InsertEvent to be called")
	}
}

func TestPipelineService_Ingest_RejectsNonUTC(t *testing.T) {
	t.Helper()

	svc := NewPipelineService(&mockRepository{}, &mockLogger{})
	ctx := context.Background()

	est, _ := time.LoadLocation("America/New_York")
	req := &domain.IngestRequest{
		ArticleURL:  "https://example.com/article",
		SourceName:  "example_com",
		Stage:       domain.StageCrawled,
		OccurredAt:  time.Now().In(est),
		ServiceName: "crawler",
	}

	ingestErr := svc.Ingest(ctx, req)
	if ingestErr == nil {
		t.Fatal("Ingest() expected error for non-UTC timestamp, got nil")
	}
}

func TestPipelineService_Ingest_RejectsInvalidStage(t *testing.T) {
	t.Helper()

	svc := NewPipelineService(&mockRepository{}, &mockLogger{})
	ctx := context.Background()

	req := &domain.IngestRequest{
		ArticleURL:  "https://example.com/article",
		SourceName:  "example_com",
		Stage:       domain.Stage("invalid"),
		OccurredAt:  time.Now().UTC(),
		ServiceName: "crawler",
	}

	ingestErr := svc.Ingest(ctx, req)
	if ingestErr == nil {
		t.Fatal("Ingest() expected error for invalid stage, got nil")
	}
}

func TestPipelineService_Ingest_RejectsFutureTimestamp(t *testing.T) {
	t.Helper()

	svc := NewPipelineService(&mockRepository{}, &mockLogger{})
	ctx := context.Background()

	req := &domain.IngestRequest{
		ArticleURL:  "https://example.com/article",
		SourceName:  "example_com",
		Stage:       domain.StageCrawled,
		OccurredAt:  time.Now().UTC().Add(time.Hour),
		ServiceName: "crawler",
	}

	ingestErr := svc.Ingest(ctx, req)
	if ingestErr == nil {
		t.Fatal("Ingest() expected error for future timestamp, got nil")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /home/fsd42/dev/north-cloud/pipeline && go test ./internal/service/ -v`
Expected: FAIL (service file doesn't exist yet)

**Step 3: Write the service**

Create `pipeline/internal/service/pipeline.go`:

```go
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jonesrussell/north-cloud/pipeline/internal/domain"
)

const maxEventAge = 24 * time.Hour

// Logger is the minimal logging interface needed by the service.
type Logger interface {
	Debug(msg string, fields ...any)
	Info(msg string, fields ...any)
	Warn(msg string, fields ...any)
	Error(msg string, fields ...any)
	With(fields ...any) Logger
	Sync() error
}

// Repository is the data access interface for pipeline events.
type Repository interface {
	UpsertArticle(ctx context.Context, article *domain.Article) error
	InsertEvent(ctx context.Context, event *domain.PipelineEvent) error
	GetFunnel(ctx context.Context, from, to time.Time) ([]domain.FunnelStage, error)
	Ping(ctx context.Context) error
}

// PipelineService handles pipeline event ingestion and querying.
type PipelineService struct {
	repo   Repository
	logger Logger
}

// NewPipelineService creates a new pipeline service.
func NewPipelineService(repo Repository, logger Logger) *PipelineService {
	return &PipelineService{
		repo:   repo,
		logger: logger,
	}
}

// Ingest validates and stores a single pipeline event.
func (s *PipelineService) Ingest(ctx context.Context, req *domain.IngestRequest) error {
	if validateErr := s.validateIngestRequest(req); validateErr != nil {
		return validateErr
	}

	// Auto-generate idempotency key if not provided
	if req.IdempotencyKey == "" {
		req.IdempotencyKey = domain.GenerateIdempotencyKey(
			req.ServiceName, req.Stage, req.ArticleURL, req.OccurredAt,
		)
	}

	// Upsert article
	article := &domain.Article{
		URL:        req.ArticleURL,
		URLHash:    domain.URLHash(req.ArticleURL),
		Domain:     domain.ExtractDomain(req.ArticleURL),
		SourceName: req.SourceName,
	}
	if upsertErr := s.repo.UpsertArticle(ctx, article); upsertErr != nil {
		return fmt.Errorf("upsert article: %w", upsertErr)
	}

	// Insert event
	event := &domain.PipelineEvent{
		ArticleURL:            req.ArticleURL,
		Stage:                 req.Stage,
		OccurredAt:            req.OccurredAt,
		ServiceName:           req.ServiceName,
		Metadata:              req.Metadata,
		MetadataSchemaVersion: 1,
		IdempotencyKey:        req.IdempotencyKey,
	}
	if insertErr := s.repo.InsertEvent(ctx, event); insertErr != nil {
		return fmt.Errorf("insert event: %w", insertErr)
	}

	return nil
}

// IngestBatch validates and stores a batch of pipeline events.
func (s *PipelineService) IngestBatch(ctx context.Context, req *domain.BatchIngestRequest) (int, error) {
	ingested := 0
	for i := range req.Events {
		if ingestErr := s.Ingest(ctx, &req.Events[i]); ingestErr != nil {
			return ingested, fmt.Errorf("event %d: %w", i, ingestErr)
		}
		ingested++
	}
	return ingested, nil
}

// GetFunnel returns the pipeline funnel for a time range.
func (s *PipelineService) GetFunnel(ctx context.Context, from, to time.Time) (*domain.FunnelResponse, error) {
	stages, queryErr := s.repo.GetFunnel(ctx, from, to)
	if queryErr != nil {
		return nil, fmt.Errorf("get funnel: %w", queryErr)
	}

	return &domain.FunnelResponse{
		From:        from,
		To:          to,
		Stages:      stages,
		GeneratedAt: time.Now().UTC(),
	}, nil
}

func (s *PipelineService) validateIngestRequest(req *domain.IngestRequest) error {
	if !req.Stage.IsValid() {
		return errors.New("invalid pipeline stage")
	}

	// Enforce UTC
	if req.OccurredAt.Location() != time.UTC {
		return errors.New("occurred_at must be in UTC")
	}

	now := time.Now().UTC()

	// Reject future timestamps
	if req.OccurredAt.After(now) {
		return errors.New("occurred_at must not be in the future")
	}

	// Reject timestamps older than 24h
	if now.Sub(req.OccurredAt) > maxEventAge {
		return errors.New("occurred_at must not be more than 24 hours in the past")
	}

	return nil
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /home/fsd42/dev/north-cloud/pipeline && go test ./internal/service/ -v`
Expected: All 4 tests PASS.

**Step 5: Lint**

Run: `cd /home/fsd42/dev/north-cloud/pipeline && golangci-lint run --config ../.golangci.yml ./internal/service/`
Expected: No issues.

**Step 6: Commit**

```bash
git add pipeline/internal/service/
git commit -m "feat(pipeline): add pipeline service with ingest validation and funnel queries"
```

---

## Task 7: API Handlers — Ingest

**Files:**
- Create: `pipeline/internal/api/ingest_handler.go`
- Create: `pipeline/internal/api/ingest_handler_test.go`

**Step 1: Write the failing test**

Create `pipeline/internal/api/ingest_handler_test.go`:

```go
package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/pipeline/internal/api"
	"github.com/jonesrussell/north-cloud/pipeline/internal/domain"
)

type mockPipelineService struct {
	ingestFunc      func(req *domain.IngestRequest) error
	ingestBatchFunc func(req *domain.BatchIngestRequest) (int, error)
}

func (m *mockPipelineService) Ingest(_ context.Context, req *domain.IngestRequest) error {
	if m.ingestFunc != nil {
		return m.ingestFunc(req)
	}
	return nil
}

func (m *mockPipelineService) IngestBatch(_ context.Context, req *domain.BatchIngestRequest) (int, error) {
	if m.ingestBatchFunc != nil {
		return m.ingestBatchFunc(req)
	}
	return len(req.Events), nil
}

func (m *mockPipelineService) GetFunnel(_ context.Context, _, _ time.Time) (*domain.FunnelResponse, error) {
	return nil, nil
}

func setupTestRouter(handler *api.IngestHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")
	v1.POST("/events", handler.IngestEvent)
	v1.POST("/events/batch", handler.IngestBatch)
	return router
}

func TestIngestHandler_IngestEvent_Success(t *testing.T) {
	t.Helper()

	svc := &mockPipelineService{}
	handler := api.NewIngestHandler(svc)
	router := setupTestRouter(handler)

	body := map[string]any{
		"article_url":  "https://example.com/article",
		"source_name":  "example_com",
		"stage":        "crawled",
		"occurred_at":  time.Now().UTC().Format(time.RFC3339),
		"service_name": "crawler",
	}
	bodyJSON, marshalErr := json.Marshal(body)
	if marshalErr != nil {
		t.Fatalf("failed to marshal body: %v", marshalErr)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/events", bytes.NewBuffer(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
}

func TestIngestHandler_IngestEvent_BadRequest(t *testing.T) {
	t.Helper()

	svc := &mockPipelineService{}
	handler := api.NewIngestHandler(svc)
	router := setupTestRouter(handler)

	// Missing required fields
	body := map[string]any{"article_url": "https://example.com"}
	bodyJSON, marshalErr := json.Marshal(body)
	if marshalErr != nil {
		t.Fatalf("failed to marshal body: %v", marshalErr)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/events", bytes.NewBuffer(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
```

Note: The test file needs `"context"` imported — add it to the import block for the mock methods.

**Step 2: Run tests to verify they fail**

Run: `cd /home/fsd42/dev/north-cloud/pipeline && go test ./internal/api/ -v`
Expected: FAIL (handler doesn't exist yet)

**Step 3: Write the ingest handler**

Create `pipeline/internal/api/ingest_handler.go`:

```go
package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/pipeline/internal/domain"
)

// Ingester defines the ingest operations needed by the handler.
type Ingester interface {
	Ingest(ctx context.Context, req *domain.IngestRequest) error
	IngestBatch(ctx context.Context, req *domain.BatchIngestRequest) (int, error)
	GetFunnel(ctx context.Context, from, to time.Time) (*domain.FunnelResponse, error)
}

// IngestHandler handles event ingestion HTTP requests.
type IngestHandler struct {
	svc Ingester
}

// NewIngestHandler creates a new ingest handler.
func NewIngestHandler(svc Ingester) *IngestHandler {
	return &IngestHandler{svc: svc}
}

// IngestEvent handles POST /api/v1/events.
func (h *IngestHandler) IngestEvent(c *gin.Context) {
	var req domain.IngestRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": bindErr.Error()})
		return
	}

	if ingestErr := h.svc.Ingest(c.Request.Context(), &req); ingestErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": ingestErr.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "ingested"})
}

// IngestBatch handles POST /api/v1/events/batch.
func (h *IngestHandler) IngestBatch(c *gin.Context) {
	var req domain.BatchIngestRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": bindErr.Error()})
		return
	}

	ingested, ingestErr := h.svc.IngestBatch(c.Request.Context(), &req)
	if ingestErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":    ingestErr.Error(),
			"ingested": ingested,
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"status":   "ingested",
		"ingested": ingested,
	})
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /home/fsd42/dev/north-cloud/pipeline && go test ./internal/api/ -v`
Expected: All 2 tests PASS.

**Step 5: Lint**

Run: `cd /home/fsd42/dev/north-cloud/pipeline && golangci-lint run --config ../.golangci.yml ./internal/api/`

**Step 6: Commit**

```bash
git add pipeline/internal/api/
git commit -m "feat(pipeline): add ingest handler with single and batch endpoints"
```

---

## Task 8: API Handlers — Funnel

**Files:**
- Create: `pipeline/internal/api/funnel_handler.go`
- Create: `pipeline/internal/api/funnel_handler_test.go`

**Step 1: Write the failing test**

Create `pipeline/internal/api/funnel_handler_test.go`:

```go
package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/pipeline/internal/api"
	"github.com/jonesrussell/north-cloud/pipeline/internal/domain"
)

func setupFunnelRouter(handler *api.FunnelHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")
	v1.GET("/funnel", handler.GetFunnel)
	return router
}

type mockFunnelService struct {
	getFunnelFunc func(from, to time.Time) (*domain.FunnelResponse, error)
}

func (m *mockFunnelService) GetFunnel(_ context.Context, from, to time.Time) (*domain.FunnelResponse, error) {
	if m.getFunnelFunc != nil {
		return m.getFunnelFunc(from, to)
	}
	return &domain.FunnelResponse{
		Stages:      []domain.FunnelStage{},
		GeneratedAt: time.Now().UTC(),
	}, nil
}

func TestFunnelHandler_GetFunnel_DefaultPeriod(t *testing.T) {
	t.Helper()

	svc := &mockFunnelService{
		getFunnelFunc: func(_, _ time.Time) (*domain.FunnelResponse, error) {
			return &domain.FunnelResponse{
				Period:   "today",
				Timezone: "UTC",
				Stages: []domain.FunnelStage{
					{Name: "crawled", Count: 100, UniqueArticles: 90},
				},
				GeneratedAt: time.Now().UTC(),
			}, nil
		},
	}

	handler := api.NewFunnelHandler(svc)
	router := setupFunnelRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/funnel", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}
```

Note: Add `"context"` to imports for the mock method.

**Step 2: Run tests to verify they fail**

Run: `cd /home/fsd42/dev/north-cloud/pipeline && go test ./internal/api/ -v -run TestFunnelHandler`
Expected: FAIL (funnel_handler.go doesn't exist yet)

**Step 3: Write the funnel handler**

Create `pipeline/internal/api/funnel_handler.go`:

```go
package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/pipeline/internal/domain"
)

// FunnelQuerier defines the funnel query operations needed by the handler.
type FunnelQuerier interface {
	GetFunnel(ctx context.Context, from, to time.Time) (*domain.FunnelResponse, error)
}

// FunnelHandler handles funnel query HTTP requests.
type FunnelHandler struct {
	svc FunnelQuerier
}

// NewFunnelHandler creates a new funnel handler.
func NewFunnelHandler(svc FunnelQuerier) *FunnelHandler {
	return &FunnelHandler{svc: svc}
}

// GetFunnel handles GET /api/v1/funnel.
func (h *FunnelHandler) GetFunnel(c *gin.Context) {
	period := c.DefaultQuery("period", "today")

	from, to := resolvePeriod(period)

	response, queryErr := h.svc.GetFunnel(c.Request.Context(), from, to)
	if queryErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": queryErr.Error()})
		return
	}

	response.Period = period
	response.Timezone = "UTC"

	c.JSON(http.StatusOK, response)
}

// resolvePeriod converts a period string to a time range.
func resolvePeriod(period string) (from, to time.Time) {
	now := time.Now().UTC()
	to = now

	switch period {
	case "24h":
		from = now.Add(-24 * time.Hour)
	case "7d":
		from = now.AddDate(0, 0, -7)
	case "30d":
		from = now.AddDate(0, 0, -30)
	default: // "today"
		from = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	}

	return from, to
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /home/fsd42/dev/north-cloud/pipeline && go test ./internal/api/ -v`
Expected: All tests PASS.

**Step 5: Lint**

Run: `cd /home/fsd42/dev/north-cloud/pipeline && golangci-lint run --config ../.golangci.yml ./internal/api/`

**Step 6: Commit**

```bash
git add pipeline/internal/api/funnel_handler.go pipeline/internal/api/funnel_handler_test.go
git commit -m "feat(pipeline): add funnel handler with period-based time ranges"
```

---

## Task 9: HTTP Server, Routes, and Bootstrap

**Files:**
- Create: `pipeline/internal/api/routes.go`
- Create: `pipeline/internal/bootstrap/app.go`
- Create: `pipeline/internal/bootstrap/config.go`
- Create: `pipeline/internal/bootstrap/database.go`
- Create: `pipeline/internal/bootstrap/server.go`
- Create: `pipeline/main.go`

**Step 1: Write routes**

Create `pipeline/internal/api/routes.go`:

```go
package api

import (
	"github.com/gin-gonic/gin"
	infragin "github.com/north-cloud/infrastructure/gin"
)

// SetupRoutes configures all API routes.
func SetupRoutes(router *gin.Engine, ingestHandler *IngestHandler, funnelHandler *FunnelHandler, jwtSecret string) {
	v1 := infragin.ProtectedGroup(router, "/api/v1", jwtSecret)

	// Event ingest (write path)
	v1.POST("/events", ingestHandler.IngestEvent)
	v1.POST("/events/batch", ingestHandler.IngestBatch)

	// Funnel (read path)
	v1.GET("/funnel", funnelHandler.GetFunnel)
}
```

**Step 2: Write bootstrap config**

Create `pipeline/internal/bootstrap/config.go`:

```go
package bootstrap

import (
	"fmt"

	"github.com/jonesrussell/north-cloud/pipeline/internal/config"
	infraconfig "github.com/north-cloud/infrastructure/config"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// LoadConfig loads and validates the service configuration.
func LoadConfig() (*config.Config, error) {
	configPath := infraconfig.GetConfigPath("config.yml")

	cfg, loadErr := config.Load(configPath)
	if loadErr != nil {
		return nil, fmt.Errorf("load config: %w", loadErr)
	}

	if validateErr := cfg.Validate(); validateErr != nil {
		return nil, fmt.Errorf("validate config: %w", validateErr)
	}

	return cfg, nil
}

// CreateLogger creates a structured logger for the service.
func CreateLogger(cfg *config.Config) (infralogger.Logger, error) {
	log, logErr := infralogger.New(infralogger.Config{
		Level:       cfg.Logging.Level,
		Format:      cfg.Logging.Format,
		Development: cfg.Service.Debug,
	})
	if logErr != nil {
		return nil, fmt.Errorf("create logger: %w", logErr)
	}

	return log.With(infralogger.String("service", "pipeline")), nil
}
```

**Step 3: Write bootstrap database**

Create `pipeline/internal/bootstrap/database.go`:

```go
package bootstrap

import (
	"fmt"

	"github.com/jonesrussell/north-cloud/pipeline/internal/config"
	"github.com/jonesrussell/north-cloud/pipeline/internal/database"
)

// SetupDatabase creates a database connection from config.
func SetupDatabase(cfg *config.Config) (*database.Connection, error) {
	dbCfg := &database.Config{
		Host:            cfg.Database.Host,
		Port:            cfg.Database.Port,
		User:            cfg.Database.User,
		Password:        cfg.Database.Password,
		Database:        cfg.Database.Database,
		SSLMode:         cfg.Database.SSLMode,
		MaxConnections:  cfg.Database.MaxConnections,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnectionMaxLifetime,
	}

	db, connErr := database.NewConnection(dbCfg)
	if connErr != nil {
		return nil, fmt.Errorf("database connection: %w", connErr)
	}

	return db, nil
}
```

**Step 4: Write bootstrap server**

Create `pipeline/internal/bootstrap/server.go`:

```go
package bootstrap

import (
	"context"
	"time"

	"github.com/jonesrussell/north-cloud/pipeline/internal/api"
	"github.com/jonesrussell/north-cloud/pipeline/internal/config"
	"github.com/jonesrussell/north-cloud/pipeline/internal/database"
	"github.com/jonesrussell/north-cloud/pipeline/internal/service"
	infragin "github.com/north-cloud/infrastructure/gin"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
	defaultReadTimeout  = 30 * time.Second
	defaultWriteTimeout = 60 * time.Second
	defaultIdleTimeout  = 120 * time.Second
	healthCheckTimeout  = 2 * time.Second
	serviceVersion      = "1.0.0"
)

// SetupHTTPServer creates the HTTP server with all handlers wired.
func SetupHTTPServer(
	cfg *config.Config,
	db *database.Connection,
	log infralogger.Logger,
) *infragin.Server {
	repo := database.NewRepository(db.DB)
	pipelineSvc := service.NewPipelineService(repo, log)

	ingestHandler := api.NewIngestHandler(pipelineSvc)
	funnelHandler := api.NewFunnelHandler(pipelineSvc)

	server := infragin.NewServerBuilder("pipeline", cfg.Service.Port).
		WithLogger(log).
		WithDebug(cfg.Service.Debug).
		WithVersion(serviceVersion).
		WithTimeouts(defaultReadTimeout, defaultWriteTimeout, defaultIdleTimeout).
		WithDatabaseHealthCheck(func() error {
			ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)
			defer cancel()
			return db.Ping(ctx)
		}).
		WithRoutes(func(router *gin.Engine) {
			api.SetupRoutes(router, ingestHandler, funnelHandler, cfg.Auth.JWTSecret)
		}).
		Build()

	return server
}
```

Note: Add `"github.com/gin-gonic/gin"` to the imports for the `*gin.Engine` parameter in `WithRoutes`.

**Step 5: Write bootstrap app**

Create `pipeline/internal/bootstrap/app.go`:

```go
package bootstrap

import (
	"fmt"

	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/profiling"
)

// Start initializes and runs the pipeline service.
func Start() error {
	profiling.StartPprofServer()

	cfg, configErr := LoadConfig()
	if configErr != nil {
		return fmt.Errorf("config: %w", configErr)
	}

	log, logErr := CreateLogger(cfg)
	if logErr != nil {
		return fmt.Errorf("logger: %w", logErr)
	}
	defer func() { _ = log.Sync() }()

	log.Info("Starting Pipeline Service",
		infralogger.String("name", cfg.Service.Name),
		infralogger.String("version", cfg.Service.Version),
		infralogger.Int("port", cfg.Service.Port),
	)

	db, dbErr := SetupDatabase(cfg)
	if dbErr != nil {
		return fmt.Errorf("database: %w", dbErr)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			log.Error("Failed to close database", infralogger.Error(closeErr))
		}
	}()
	log.Info("Database connection established")

	server := SetupHTTPServer(cfg, db, log)

	if runErr := server.Run(); runErr != nil {
		log.Error("Server error", infralogger.Error(runErr))
		return fmt.Errorf("server: %w", runErr)
	}

	log.Info("Pipeline Service stopped")
	return nil
}
```

**Step 6: Write main.go**

Create `pipeline/main.go`:

```go
package main

import (
	"fmt"
	"os"

	"github.com/jonesrussell/north-cloud/pipeline/internal/bootstrap"
)

func main() {
	if err := bootstrap.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
```

**Step 7: Run `go mod tidy` and verify compilation**

Run: `cd /home/fsd42/dev/north-cloud/pipeline && go mod tidy && go build -o /dev/null .`
Expected: Compiles successfully.

**Step 8: Lint everything**

Run: `cd /home/fsd42/dev/north-cloud/pipeline && golangci-lint run --config ../.golangci.yml ./...`
Expected: No issues (or only minor ones to fix).

**Step 9: Commit**

```bash
git add pipeline/main.go pipeline/internal/bootstrap/ pipeline/internal/api/routes.go pipeline/go.mod pipeline/go.sum
git commit -m "feat(pipeline): add HTTP server, routes, and bootstrap"
```

---

## Task 10: Docker and Infrastructure

**Files:**
- Create: `pipeline/Dockerfile`
- Create: `pipeline/Dockerfile.dev`
- Create: `pipeline/.air.toml`
- Create: `pipeline/Taskfile.yml`
- Modify: `docker-compose.base.yml` (add postgres-pipeline, pipeline service)
- Modify: `docker-compose.dev.yml` (add pipeline dev override)
- Modify: `infrastructure/nginx/nginx.dev.conf` (add pipeline upstream and route)
- Modify: `.env.example` (add pipeline env vars)
- Modify: `scripts/run-migration.sh` (add pipeline case)

**Step 1: Write Dockerfile**

Create `pipeline/Dockerfile`:

```dockerfile
# Build stage
FROM golang:1.25.7-alpine AS builder

RUN apk add --no-cache ca-certificates

WORKDIR /build
COPY infrastructure ./infrastructure

WORKDIR /build/pipeline

COPY pipeline/go.mod pipeline/go.sum ./
RUN go mod download

COPY pipeline/ .

RUN CGO_ENABLED=0 go build \
    -mod=mod \
    -ldflags="-w -s" \
    -trimpath \
    -o pipeline \
    .

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata curl wget && \
    addgroup -g 1000 pipeline && \
    adduser -D -u 1000 -G pipeline pipeline

WORKDIR /app

COPY --from=builder /build/pipeline/pipeline .
COPY --from=builder /build/pipeline/config.yml ./config.yml

USER pipeline

EXPOSE 8075

CMD ["/app/pipeline"]
```

**Step 2: Write Dockerfile.dev**

Create `pipeline/Dockerfile.dev`:

```dockerfile
FROM golang:1.25.7-alpine

ARG UID=1000
ARG GID=1000

WORKDIR /app

RUN apk add --no-cache git curl wget

RUN go install github.com/air-verse/air@latest

RUN addgroup -g ${GID} appuser && \
    adduser -D -u ${UID} -G appuser appuser

RUN mkdir -p /tmp/go-mod-cache \
             /tmp/go-build-cache \
             /go/pkg/sumdb && \
    chmod -R 1777 /tmp/go-mod-cache \
                  /tmp/go-build-cache \
                  /go/pkg/sumdb

COPY go.mod go.sum ./

EXPOSE 8075

ENV GOCACHE=/tmp/go-build-cache
ENV GOMODCACHE=/tmp/go-mod-cache

CMD ["sh", "-c", "go mod download && air -c .air.toml"]
```

**Step 3: Write .air.toml**

Create `pipeline/.air.toml`:

```toml
root = "."
testdata_dir = "testdata"
tmp_dir = "tmp"

[build]
  args_bin = []
  entrypoint = "./tmp/pipeline"
  cmd = "go build -mod=mod -o ./tmp/pipeline ."
  delay = 1000
  exclude_dir = ["assets", "tmp", "vendor", "testdata", "bin", "dist", ".git", "node_modules"]
  exclude_file = []
  exclude_regex = ["_test.go"]
  exclude_unchanged = true
  follow_symlink = false
  include_ext = ["go", "tpl", "tmpl", "html", "yaml", "yml"]
  kill_delay = "2s"
  log = "build-errors.log"
  send_interrupt = true
  stop_on_error = false

[log]
  main_only = false
  time = false

[misc]
  clean_on_exit = false

[screen]
  clear_on_rebuild = false
  keep_scroll = true
```

**Step 4: Write Taskfile.yml**

Create `pipeline/Taskfile.yml`:

```yaml
version: '3'

dotenv: ['../.env']

vars:
  BINARY_NAME: pipeline
  MAIN_PATH: .

tasks:
  default:
    desc: "Show available tasks"
    cmds:
      - task --list

  build:
    desc: "Build the pipeline binary"
    cmds:
      - go build -o bin/{{.BINARY_NAME}} {{.MAIN_PATH}}

  run:
    desc: "Run the pipeline service"
    cmds:
      - go run {{.MAIN_PATH}}

  test:
    desc: "Run all tests"
    cmds:
      - go test -v ./...

  test:coverage:
    desc: "Run tests with coverage"
    cmds:
      - go test -coverprofile=coverage.out ./...
      - go tool cover -html=coverage.out -o coverage.html

  lint:
    desc: "Lint the Go code"
    cmds:
      - go fmt ./...
      - go vet ./...
      - golangci-lint run --config ../.golangci.yml ./...

  tidy:
    desc: "Tidy go.mod and go.sum"
    cmds:
      - go mod tidy

  clean:
    desc: "Clean build artifacts"
    cmds:
      - rm -rf bin/ tmp/
      - rm -f coverage.out coverage.html

  migrate:up:
    desc: "Run database migrations up"
    silent: true
    cmds:
      - ../scripts/run-migration.sh pipeline up

  migrate:down:
    desc: "Rollback last migration"
    silent: true
    cmds:
      - ../scripts/run-migration.sh pipeline down

  dev:
    desc: "Run in development mode with hot-reload"
    cmds:
      - air
```

**Step 5: Add pipeline to docker-compose.base.yml**

Add after the `postgres-publisher` block (around line 88):

```yaml
  postgres-pipeline:
    <<: *postgres-defaults
    environment:
      POSTGRES_USER: ${POSTGRES_PIPELINE_USER:-postgres}
      POSTGRES_PASSWORD: ${POSTGRES_PIPELINE_PASSWORD:-postgres}
      POSTGRES_DB: ${POSTGRES_PIPELINE_DB:-pipeline}
    volumes:
      - postgres_pipeline_data:/var/lib/postgresql/data
      - ./pipeline/migrations:/migrations:ro
```

Add `postgres_pipeline_data:` to the volumes section.

**Step 6: Add pipeline to docker-compose.dev.yml**

Add before the Pyroscope section:

```yaml
  pipeline:
    <<: *go-dev-defaults
    deploy:
      resources:
        limits:
          cpus: "0.5"
          memory: 256M
    pull_policy: build
    build:
      context: ./pipeline
      dockerfile: Dockerfile.dev
      args:
        UID: "${UID:-1000}"
        GID: "${GID:-1000}"
    ports:
      - "${PIPELINE_PORT:-8075}:8075"
      - "${PIPELINE_PPROF_PORT:-6067}:6060"
    depends_on:
      postgres-pipeline:
        condition: service_healthy
    environment:
      <<: *go-dev-environment
      POSTGRES_PIPELINE_HOST: postgres-pipeline
      POSTGRES_PIPELINE_PORT: 5432
      POSTGRES_PIPELINE_USER: "${POSTGRES_PIPELINE_USER:-postgres}"
      POSTGRES_PIPELINE_PASSWORD: "${POSTGRES_PIPELINE_PASSWORD:-postgres}"
      POSTGRES_PIPELINE_DB: "${POSTGRES_PIPELINE_DB:-pipeline}"
      POSTGRES_PIPELINE_SSLMODE: disable
      PIPELINE_PORT: 8075
      GIN_MODE: debug
      PPROF_PORT: 6060
    volumes:
      - ./pipeline:/app
      - pipeline_go_mod_cache:/tmp/go-mod-cache
      - pipeline_go_build_cache:/tmp/go-build-cache
      - ./infrastructure:/infrastructure
    command: ["sh", "-c", "go mod download && air -c .air.toml"]
    healthcheck:
      test: ["CMD", "wget", "-q", "-O", "/dev/null", "http://localhost:8075/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 25s
```

Add `pipeline_go_mod_cache:` and `pipeline_go_build_cache:` to the dev volumes section.

**Step 7: Add nginx upstream and route**

Add upstream variable in `infrastructure/nginx/nginx.dev.conf` after the `index_manager_api` map:

```nginx
    map "" $pipeline_api {
        default "pipeline:8075";
    }
```

Add location block after the index-manager API block:

```nginx
        # Pipeline API
        location /api/pipeline {
            rewrite ^/api/pipeline(/.*)?$ /api/v1$1 break;
            proxy_pass http://$pipeline_api;
            proxy_http_version 1.1;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            proxy_set_header Authorization $http_authorization;
            proxy_read_timeout 300s;
            proxy_connect_timeout 75s;
        }
```

Add health check block:

```nginx
        location /api/health/pipeline {
            proxy_pass http://$pipeline_api/health;
            proxy_http_version 1.1;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
        }
```

**Step 8: Add pipeline to .env.example**

Add a new section:

```bash
# ============================================
# Pipeline Service
# ============================================
PIPELINE_PORT=8075

# Pipeline Database
POSTGRES_PIPELINE_HOST=localhost
POSTGRES_PIPELINE_PORT=5438
POSTGRES_PIPELINE_USER=postgres
POSTGRES_PIPELINE_PASSWORD=postgres
POSTGRES_PIPELINE_DB=pipeline
POSTGRES_PIPELINE_SSLMODE=disable
```

Add to the profiling section:

```bash
PIPELINE_PPROF_PORT=6067
```

**Step 9: Add pipeline to run-migration.sh**

Add a new case in the `scripts/run-migration.sh` case statement (after `index-manager`):

```bash
    pipeline)
        ENV_PREFIX="POSTGRES_PIPELINE"
        DB_DEFAULT="pipeline"
        PORT_DEFAULT="5438"
        CONTAINER_PATTERN="postgres-pipeline"
        SWARM_SERVICE="northcloud_postgres-pipeline"
        MIGRATION_PATH="./migrations"
        ;;
```

Update the usage/error messages to include `pipeline` in the valid services list.

**Step 10: Run all tests**

Run: `cd /home/fsd42/dev/north-cloud/pipeline && go test ./... -v`
Expected: All tests PASS.

**Step 11: Lint**

Run: `cd /home/fsd42/dev/north-cloud/pipeline && golangci-lint run --config ../.golangci.yml ./...`
Expected: No issues.

**Step 12: Commit**

```bash
git add pipeline/Dockerfile pipeline/Dockerfile.dev pipeline/.air.toml pipeline/Taskfile.yml \
    docker-compose.base.yml docker-compose.dev.yml \
    infrastructure/nginx/nginx.dev.conf \
    .env.example scripts/run-migration.sh
git commit -m "feat(pipeline): add Docker, compose, nginx, and infrastructure integration"
```

---

## Task 11: Smoke Test — Deploy and Verify

**Step 1: Start the pipeline service**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d postgres-pipeline pipeline`

**Step 2: Check health**

Run: `curl -s http://localhost:8075/health | jq .`
Expected: `{"status": "ok", "service": "pipeline", ...}`

**Step 3: Run migrations**

Run: `cd /home/fsd42/dev/north-cloud && ./scripts/run-migration.sh pipeline up`
Expected: Migration 001 applied successfully.

**Step 4: Test event ingestion**

Get auth token and POST a test event:

```bash
TOKEN=$(curl -s http://localhost:8040/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}' | jq -r '.token')

curl -s -X POST http://localhost:8075/api/v1/events \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "article_url": "https://example.com/test-article",
    "source_name": "example_com",
    "stage": "crawled",
    "occurred_at": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
    "service_name": "smoke-test"
  }'
```

Expected: `201 Created` with `{"status": "ingested"}`

**Step 5: Test funnel**

```bash
curl -s http://localhost:8075/api/v1/funnel \
  -H "Authorization: Bearer $TOKEN" | jq .
```

Expected: Response with one "crawled" stage showing count=1, unique_articles=1.

**Step 6: Commit (no code changes, just verification)**

No commit needed — this task is verification only.

---

## Task 12: Shared Pipeline Client Library

**Files:**
- Create: `infrastructure/pipeline/client.go`
- Create: `infrastructure/pipeline/client_test.go`

**Step 1: Write the failing test**

Create `infrastructure/pipeline/client_test.go`:

```go
package pipeline

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestClient_Emit_Success(t *testing.T) {
	t.Helper()

	var received atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Store(true)

		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/events" {
			t.Errorf("path = %s, want /api/v1/events", r.URL.Path)
		}

		var req map[string]any
		if decodeErr := json.NewDecoder(r.Body).Decode(&req); decodeErr != nil {
			t.Errorf("decode error: %v", decodeErr)
		}

		if req["stage"] != "crawled" {
			t.Errorf("stage = %v, want crawled", req["stage"])
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-service")
	ctx := context.Background()

	emitErr := client.Emit(ctx, Event{
		ArticleURL:  "https://example.com/article",
		SourceName:  "example_com",
		Stage:       "crawled",
		OccurredAt:  time.Now().UTC(),
		ServiceName: "test-service",
	})

	if emitErr != nil {
		t.Errorf("Emit() error = %v", emitErr)
	}

	if !received.Load() {
		t.Error("expected server to receive the event")
	}
}

func TestClient_Emit_NoopWhenURLEmpty(t *testing.T) {
	t.Helper()

	client := NewClient("", "test-service")
	ctx := context.Background()

	emitErr := client.Emit(ctx, Event{
		ArticleURL:  "https://example.com/article",
		SourceName:  "example_com",
		Stage:       "crawled",
		OccurredAt:  time.Now().UTC(),
		ServiceName: "test-service",
	})

	if emitErr != nil {
		t.Errorf("Emit() should be no-op when URL is empty, got error: %v", emitErr)
	}
}

func TestClient_EmitBatch_SingleRequest(t *testing.T) {
	t.Helper()

	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.URL.Path != "/api/v1/events/batch" {
			t.Errorf("path = %s, want /api/v1/events/batch", r.URL.Path)
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-service")
	ctx := context.Background()

	events := []Event{
		{ArticleURL: "https://example.com/1", SourceName: "test", Stage: "crawled", OccurredAt: time.Now().UTC(), ServiceName: "test"},
		{ArticleURL: "https://example.com/2", SourceName: "test", Stage: "crawled", OccurredAt: time.Now().UTC(), ServiceName: "test"},
		{ArticleURL: "https://example.com/3", SourceName: "test", Stage: "crawled", OccurredAt: time.Now().UTC(), ServiceName: "test"},
	}

	batchErr := client.EmitBatch(ctx, events)
	if batchErr != nil {
		t.Errorf("EmitBatch() error = %v", batchErr)
	}

	if requestCount.Load() != 1 {
		t.Errorf("EmitBatch() made %d requests, want 1", requestCount.Load())
	}
}

func TestClient_CircuitBreaker_Opens(t *testing.T) {
	t.Helper()

	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-service")
	ctx := context.Background()

	event := Event{
		ArticleURL:  "https://example.com/article",
		SourceName:  "example_com",
		Stage:       "crawled",
		OccurredAt:  time.Now().UTC(),
		ServiceName: "test-service",
	}

	// Trigger enough failures to trip the circuit breaker
	const tripThreshold = 6
	for i := 0; i < tripThreshold; i++ {
		_ = client.Emit(ctx, event)
	}

	requestsBefore := requestCount.Load()

	// Next call should be blocked by circuit breaker (no HTTP request)
	_ = client.Emit(ctx, event)

	if requestCount.Load() != requestsBefore {
		t.Error("expected circuit breaker to block the request")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /home/fsd42/dev/north-cloud/infrastructure && go test ./pipeline/ -v`
Expected: FAIL (client.go doesn't exist yet)

**Step 3: Write the client**

Create `infrastructure/pipeline/client.go`:

```go
package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const (
	defaultTimeout            = 2 * time.Second
	circuitBreakerThreshold   = 5
	circuitBreakerHalfOpenAge = 30 * time.Second
	circuitBreakerCloseAfter  = 2
)

// Event represents a pipeline event to emit.
type Event struct {
	ArticleURL     string         `json:"article_url"`
	SourceName     string         `json:"source_name"`
	Stage          string         `json:"stage"`
	OccurredAt     time.Time      `json:"occurred_at"`
	ServiceName    string         `json:"service_name"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type batchRequest struct {
	Events []Event `json:"events"`
}

type circuitState int

const (
	circuitClosed   circuitState = iota
	circuitOpen     circuitState = iota
	circuitHalfOpen circuitState = iota
)

type circuitBreaker struct {
	mu                 sync.Mutex
	state              circuitState
	consecutiveFailures int
	lastFailure        time.Time
	successesSinceOpen int
}

// Client is a fire-and-forget pipeline event emitter.
type Client struct {
	baseURL     string
	serviceName string
	httpClient  *http.Client
	breaker     *circuitBreaker
}

// NewClient creates a new pipeline client. If baseURL is empty, all methods are no-ops.
func NewClient(baseURL, serviceName string) *Client {
	return &Client{
		baseURL:     baseURL,
		serviceName: serviceName,
		httpClient:  &http.Client{Timeout: defaultTimeout},
		breaker:     &circuitBreaker{},
	}
}

// IsEnabled returns true if the client is configured with a URL.
func (c *Client) IsEnabled() bool {
	return c.baseURL != ""
}

// CircuitOpen returns true if the circuit breaker is open.
func (c *Client) CircuitOpen() bool {
	c.breaker.mu.Lock()
	defer c.breaker.mu.Unlock()
	return c.breaker.state == circuitOpen
}

// Emit sends a single event to the Pipeline Service. Fire-and-forget: errors are returned
// but should be logged as warnings, not treated as fatal.
func (c *Client) Emit(ctx context.Context, event Event) error {
	if !c.IsEnabled() {
		return nil
	}

	if !c.breakerAllow() {
		return fmt.Errorf("pipeline circuit breaker open")
	}

	body, marshalErr := json.Marshal(event)
	if marshalErr != nil {
		return fmt.Errorf("marshal event: %w", marshalErr)
	}

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/events", bytes.NewReader(body))
	if reqErr != nil {
		c.breakerRecordFailure()
		return fmt.Errorf("create request: %w", reqErr)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, doErr := c.httpClient.Do(req)
	if doErr != nil {
		c.breakerRecordFailure()
		return fmt.Errorf("send event: %w", doErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		c.breakerRecordFailure()
		return fmt.Errorf("pipeline service error: status %d", resp.StatusCode)
	}

	c.breakerRecordSuccess()
	return nil
}

// EmitBatch sends multiple events in a single HTTP request.
func (c *Client) EmitBatch(ctx context.Context, events []Event) error {
	if !c.IsEnabled() || len(events) == 0 {
		return nil
	}

	if !c.breakerAllow() {
		return fmt.Errorf("pipeline circuit breaker open")
	}

	batch := batchRequest{Events: events}
	body, marshalErr := json.Marshal(batch)
	if marshalErr != nil {
		return fmt.Errorf("marshal batch: %w", marshalErr)
	}

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/events/batch", bytes.NewReader(body))
	if reqErr != nil {
		c.breakerRecordFailure()
		return fmt.Errorf("create request: %w", reqErr)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, doErr := c.httpClient.Do(req)
	if doErr != nil {
		c.breakerRecordFailure()
		return fmt.Errorf("send batch: %w", doErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		c.breakerRecordFailure()
		return fmt.Errorf("pipeline service error: status %d", resp.StatusCode)
	}

	c.breakerRecordSuccess()
	return nil
}

func (c *Client) breakerAllow() bool {
	c.breaker.mu.Lock()
	defer c.breaker.mu.Unlock()

	switch c.breaker.state {
	case circuitClosed:
		return true
	case circuitOpen:
		if time.Since(c.breaker.lastFailure) > circuitBreakerHalfOpenAge {
			c.breaker.state = circuitHalfOpen
			c.breaker.successesSinceOpen = 0
			return true
		}
		return false
	case circuitHalfOpen:
		return true
	}
	return true
}

func (c *Client) breakerRecordFailure() {
	c.breaker.mu.Lock()
	defer c.breaker.mu.Unlock()

	c.breaker.consecutiveFailures++
	c.breaker.lastFailure = time.Now()
	c.breaker.successesSinceOpen = 0

	if c.breaker.consecutiveFailures >= circuitBreakerThreshold {
		c.breaker.state = circuitOpen
	}
}

func (c *Client) breakerRecordSuccess() {
	c.breaker.mu.Lock()
	defer c.breaker.mu.Unlock()

	if c.breaker.state == circuitHalfOpen {
		c.breaker.successesSinceOpen++
		if c.breaker.successesSinceOpen >= circuitBreakerCloseAfter {
			c.breaker.state = circuitClosed
			c.breaker.consecutiveFailures = 0
		}
	} else {
		c.breaker.consecutiveFailures = 0
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /home/fsd42/dev/north-cloud/infrastructure && go test ./pipeline/ -v`
Expected: All 4 tests PASS.

**Step 5: Lint**

Run: `cd /home/fsd42/dev/north-cloud/infrastructure && golangci-lint run --config ../.golangci.yml ./pipeline/`

**Step 6: Commit**

```bash
git add infrastructure/pipeline/
git commit -m "feat(infrastructure): add pipeline client library with circuit breaker"
```

---

## Summary

| Task | What | Files |
|------|------|-------|
| 1 | Go module init | `go.mod` |
| 2 | Config package | `internal/config/config.go`, `config.yml` |
| 3 | DB connection + migrations | `internal/database/connection.go`, `migrations/` |
| 4 | Domain models + tests | `internal/domain/models.go`, `models_test.go` |
| 5 | Event repository + tests | `internal/database/repository.go`, `repository_test.go` |
| 6 | Service layer + tests | `internal/service/pipeline.go`, `pipeline_test.go` |
| 7 | Ingest handler + tests | `internal/api/ingest_handler.go`, `ingest_handler_test.go` |
| 8 | Funnel handler + tests | `internal/api/funnel_handler.go`, `funnel_handler_test.go` |
| 9 | Server, routes, bootstrap | `internal/bootstrap/`, `internal/api/routes.go`, `main.go` |
| 10 | Docker + compose + nginx | Dockerfiles, compose, nginx, env, migration script |
| 11 | Smoke test | Verify deployment end-to-end |
| 12 | Shared client library | `infrastructure/pipeline/client.go`, `client_test.go` |

After completing these 12 tasks, Phase 1 and Phase 2 from the design doc are done. The Pipeline Service is deployed and the shared client library is ready for Phase 3 (wiring up services).
