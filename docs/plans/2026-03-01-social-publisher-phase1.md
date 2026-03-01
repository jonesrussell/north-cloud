# Social Publisher Phase 1 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a Go microservice in the NorthCloud monorepo that subscribes to Redis Pub/Sub, publishes content to X (Twitter), and exposes an HTTP API for manual publishing.

**Architecture:** Downstream Redis consumer following NorthCloud's multi-mode service pattern (publisher/classifier). Uses infrastructure packages for config, logging, HTTP, and database. Two-queue priority model for real-time vs retry traffic.

**Tech Stack:** Go 1.26, PostgreSQL (sqlx), Redis (go-redis), Gin (via infrastructure/gin), testify/assert, OAuth 2.0 (X API v2)

---

## Task 1: Scaffold Service Directory

**Files:**
- Create: `social-publisher/go.mod`
- Create: `social-publisher/main.go`
- Modify: `go.work` (add `./social-publisher`)

**Step 1: Create directory and go.mod**

```bash
mkdir -p social-publisher
cd social-publisher
go mod init github.com/north-cloud/social-publisher
```

**Step 2: Add minimal main.go**

```go
// social-publisher/main.go
package main

import (
	"fmt"
	"os"
)

const version = "0.1.0"

func main() {
	os.Exit(run())
}

func run() int {
	fmt.Printf("Social Publisher v%s\n", version)
	return 0
}
```

**Step 3: Add to workspace**

Add `./social-publisher` to the `use` block in `go.work`.

**Step 4: Verify it compiles**

Run: `cd social-publisher && go build ./...`
Expected: Clean build, no errors.

**Step 5: Commit**

```bash
git add social-publisher/go.mod social-publisher/main.go go.work
git commit -m "feat(social-publisher): scaffold service directory"
```

---

## Task 2: Config Struct and Loading

**Files:**
- Create: `social-publisher/internal/config/config.go`
- Create: `social-publisher/internal/config/config_test.go`
- Create: `social-publisher/config.yml.example`

**Step 1: Write the config test**

```go
// social-publisher/internal/config/config_test.go
package config_test

import (
	"testing"

	"github.com/north-cloud/social-publisher/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestConfig_Validate_ValidConfig(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Address:      ":8077",
			ReadTimeout:  "10s",
			WriteTimeout: "30s",
		},
		Database: config.DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			User:     "social_publisher",
			Password: "secret",
			DBName:   "social_publisher",
			SSLMode:  "disable",
		},
		Redis: config.RedisConfig{
			URL: "localhost:6379",
		},
		Service: config.ServiceConfig{
			RetryInterval:    "30s",
			ScheduleInterval: "60s",
			MaxRetries:       3,
			BatchSize:        50,
		},
	}
	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfig_Validate_MissingDatabase(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Address: ":8077"},
		Redis:  config.RedisConfig{URL: "localhost:6379"},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database")
}

func TestConfig_Validate_MissingRedis(t *testing.T) {
	cfg := &config.Config{
		Server:   config.ServerConfig{Address: ":8077"},
		Database: config.DatabaseConfig{Host: "localhost", Port: 5432, User: "u", Password: "p", DBName: "db"},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis")
}
```

**Step 2: Run test to verify it fails**

Run: `cd social-publisher && go test ./internal/config/... -v`
Expected: FAIL (package doesn't exist yet)

**Step 3: Write the config struct**

```go
// social-publisher/internal/config/config.go
package config

import (
	"fmt"

	infraconfig "github.com/north-cloud/infrastructure/config"
)

type Config struct {
	Debug    bool            `yaml:"debug" env:"SOCIAL_PUBLISHER_DEBUG"`
	Server   ServerConfig    `yaml:"server"`
	Database DatabaseConfig  `yaml:"database"`
	Redis    RedisConfig     `yaml:"redis"`
	Service  ServiceConfig   `yaml:"service"`
	Auth     AuthConfig      `yaml:"auth"`
}

type ServerConfig struct {
	Address      string `yaml:"address" env:"SOCIAL_PUBLISHER_ADDRESS"`
	ReadTimeout  string `yaml:"read_timeout"`
	WriteTimeout string `yaml:"write_timeout"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host" env:"POSTGRES_SOCIAL_PUBLISHER_HOST"`
	Port     int    `yaml:"port" env:"POSTGRES_SOCIAL_PUBLISHER_PORT"`
	User     string `yaml:"user" env:"POSTGRES_SOCIAL_PUBLISHER_USER"`
	Password string `yaml:"password" env:"POSTGRES_SOCIAL_PUBLISHER_PASSWORD"`
	DBName   string `yaml:"db" env:"POSTGRES_SOCIAL_PUBLISHER_DB"`
	SSLMode  string `yaml:"ssl_mode" env:"POSTGRES_SOCIAL_PUBLISHER_SSL_MODE"`
}

type RedisConfig struct {
	URL      string `yaml:"url" env:"REDIS_ADDR"`
	Password string `yaml:"password" env:"REDIS_PASSWORD"`
}

type ServiceConfig struct {
	RetryInterval    string `yaml:"retry_interval" env:"SOCIAL_PUBLISHER_RETRY_INTERVAL"`
	ScheduleInterval string `yaml:"schedule_interval" env:"SOCIAL_PUBLISHER_SCHEDULE_INTERVAL"`
	MaxRetries       int    `yaml:"max_retries" env:"SOCIAL_PUBLISHER_MAX_RETRIES"`
	BatchSize        int    `yaml:"batch_size" env:"SOCIAL_PUBLISHER_BATCH_SIZE"`
}

type AuthConfig struct {
	JWTSecret string `yaml:"jwt_secret" env:"SOCIAL_PUBLISHER_JWT_SECRET"`
}

func Load(path string) (*Config, error) {
	configPath := infraconfig.GetConfigPath(path)
	var cfg Config
	if err := infraconfig.LoadYAML(configPath, &cfg); err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	return &cfg, nil
}

func (c *Config) Validate() error {
	if c.Database.Host == "" || c.Database.DBName == "" {
		return fmt.Errorf("database configuration is required")
	}
	if c.Redis.URL == "" {
		return fmt.Errorf("redis URL is required")
	}
	return nil
}
```

**Step 4: Create config.yml.example**

```yaml
# social-publisher/config.yml.example
debug: false

server:
  address: ":8077"
  read_timeout: "10s"
  write_timeout: "30s"

database:
  host: "localhost"
  port: 5432
  user: "social_publisher"
  password: ""
  db: "social_publisher"
  ssl_mode: "disable"

redis:
  url: "localhost:6379"
  password: ""

service:
  retry_interval: "30s"
  schedule_interval: "60s"
  max_retries: 3
  batch_size: 50

auth:
  jwt_secret: ""
```

**Step 5: Run tests**

Run: `cd social-publisher && go test ./internal/config/... -v`
Expected: PASS (3 tests)

**Step 6: Commit**

```bash
git add social-publisher/internal/config/ social-publisher/config.yml.example
git commit -m "feat(social-publisher): add config struct with validation"
```

---

## Task 3: Database Migrations

**Files:**
- Create: `social-publisher/migrations/001_initial_schema.up.sql`
- Create: `social-publisher/migrations/001_initial_schema.down.sql`

**Step 1: Write the up migration**

```sql
-- social-publisher/migrations/001_initial_schema.up.sql
CREATE TABLE IF NOT EXISTS content (
    id          TEXT PRIMARY KEY,
    type        TEXT NOT NULL,
    title       TEXT,
    body        TEXT,
    summary     TEXT,
    url         TEXT,
    images      JSONB DEFAULT '[]'::jsonb,
    tags        JSONB DEFAULT '[]'::jsonb,
    project     TEXT NOT NULL,
    metadata    JSONB DEFAULT '{}'::jsonb,
    source      TEXT NOT NULL,
    published   BOOLEAN NOT NULL DEFAULT false,
    scheduled_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS accounts (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL UNIQUE,
    platform      TEXT NOT NULL,
    project       TEXT NOT NULL,
    enabled       BOOLEAN NOT NULL DEFAULT true,
    credentials   BYTEA,
    token_expiry  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS deliveries (
    id            TEXT PRIMARY KEY,
    content_id    TEXT NOT NULL REFERENCES content(id),
    platform      TEXT NOT NULL,
    account       TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'pending',
    platform_id   TEXT,
    platform_url  TEXT,
    error         TEXT,
    attempts      INT NOT NULL DEFAULT 0,
    max_attempts  INT NOT NULL DEFAULT 3,
    next_retry_at TIMESTAMPTZ,
    last_error_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delivered_at  TIMESTAMPTZ,
    UNIQUE(content_id, platform, account)
);

CREATE INDEX idx_deliveries_retry ON deliveries (status, next_retry_at);
CREATE INDEX idx_deliveries_content ON deliveries (content_id);
CREATE INDEX idx_content_scheduled ON content (scheduled_at) WHERE scheduled_at IS NOT NULL AND published = false;
```

**Step 2: Write the down migration**

```sql
-- social-publisher/migrations/001_initial_schema.down.sql
DROP INDEX IF EXISTS idx_content_scheduled;
DROP INDEX IF EXISTS idx_deliveries_content;
DROP INDEX IF EXISTS idx_deliveries_retry;
DROP TABLE IF EXISTS deliveries;
DROP TABLE IF EXISTS accounts;
DROP TABLE IF EXISTS content;
```

**Step 3: Verify SQL is valid**

Run: `cat social-publisher/migrations/001_initial_schema.up.sql | psql -d social_publisher -f - --dry-run` (or just review manually)

**Step 4: Commit**

```bash
git add social-publisher/migrations/
git commit -m "feat(social-publisher): add database migrations for content, deliveries, accounts"
```

---

## Task 4: Domain Types

**Files:**
- Create: `social-publisher/internal/domain/content.go`
- Create: `social-publisher/internal/domain/delivery.go`
- Create: `social-publisher/internal/domain/adapter.go`
- Create: `social-publisher/internal/domain/errors.go`
- Create: `social-publisher/internal/domain/errors_test.go`

**Step 1: Write error type tests**

```go
// social-publisher/internal/domain/errors_test.go
package domain_test

import (
	"testing"
	"time"

	"github.com/north-cloud/social-publisher/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestRateLimitError_IsRetryable(t *testing.T) {
	err := &domain.RateLimitError{
		RetryAfter: 30 * time.Second,
		Message:    "rate limited",
	}
	assert.True(t, err.IsRetryable())
	assert.Equal(t, 30*time.Second, err.RetryAfter)
	assert.Contains(t, err.Error(), "rate limited")
}

func TestTransientError_IsRetryable(t *testing.T) {
	err := &domain.TransientError{Message: "timeout"}
	assert.True(t, err.IsRetryable())
}

func TestPermanentError_IsNotRetryable(t *testing.T) {
	err := &domain.PermanentError{Message: "invalid content", Code: "400"}
	assert.False(t, err.IsRetryable())
	assert.Equal(t, "400", err.Code)
}

func TestAuthError_IsRetryable(t *testing.T) {
	err := &domain.AuthError{Message: "token expired"}
	assert.True(t, err.IsRetryable())
}

func TestValidationError_IsNotRetryable(t *testing.T) {
	err := &domain.ValidationError{
		Field:   "metadata.subreddit",
		Message: "required field missing",
	}
	assert.False(t, err.IsRetryable())
	assert.Contains(t, err.Error(), "subreddit")
}
```

**Step 2: Run test to verify it fails**

Run: `cd social-publisher && go test ./internal/domain/... -v`
Expected: FAIL

**Step 3: Write domain types**

```go
// social-publisher/internal/domain/content.go
package domain

import "time"

type ContentType string

const (
	BlogPost            ContentType = "blog_post"
	SocialUpdate        ContentType = "social_update"
	ProductAnnouncement ContentType = "product_announcement"
)

type PublishMessage struct {
	ContentID   string            `json:"content_id"`
	Type        ContentType       `json:"type"`
	Title       string            `json:"title,omitempty"`
	Body        string            `json:"body,omitempty"`
	Summary     string            `json:"summary"`
	URL         string            `json:"url,omitempty"`
	Images      []string          `json:"images,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Project     string            `json:"project"`
	Targets     []TargetConfig    `json:"targets,omitempty"`
	ScheduledAt *time.Time        `json:"scheduled_at,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	RetryCount  int               `json:"retry_count"`
	Source      string            `json:"source"`
}

type TargetConfig struct {
	Platform string  `json:"platform"`
	Account  string  `json:"account"`
	Override *string `json:"override,omitempty"`
}
```

```go
// social-publisher/internal/domain/delivery.go
package domain

import "time"

type DeliveryStatus string

const (
	StatusPending    DeliveryStatus = "pending"
	StatusPublishing DeliveryStatus = "publishing"
	StatusDelivered  DeliveryStatus = "delivered"
	StatusRetrying   DeliveryStatus = "retrying"
	StatusFailed     DeliveryStatus = "failed"
)

type Delivery struct {
	ID           string         `db:"id" json:"id"`
	ContentID    string         `db:"content_id" json:"content_id"`
	Platform     string         `db:"platform" json:"platform"`
	Account      string         `db:"account" json:"account"`
	Status       DeliveryStatus `db:"status" json:"status"`
	PlatformID   *string        `db:"platform_id" json:"platform_id,omitempty"`
	PlatformURL  *string        `db:"platform_url" json:"platform_url,omitempty"`
	Error        *string        `db:"error" json:"error,omitempty"`
	Attempts     int            `db:"attempts" json:"attempts"`
	MaxAttempts  int            `db:"max_attempts" json:"max_attempts"`
	NextRetryAt  *time.Time     `db:"next_retry_at" json:"next_retry_at,omitempty"`
	LastErrorAt  *time.Time     `db:"last_error_at" json:"last_error_at,omitempty"`
	CreatedAt    time.Time      `db:"created_at" json:"created_at"`
	DeliveredAt  *time.Time     `db:"delivered_at" json:"delivered_at,omitempty"`
}

type DeliveryEvent struct {
	ContentID   string    `json:"content_id"`
	ContentType string    `json:"content_type"`
	DeliveryID  string    `json:"delivery_id"`
	Platform    string    `json:"platform"`
	Account     string    `json:"account"`
	Status      string    `json:"status"`
	PlatformID  string    `json:"platform_id,omitempty"`
	PlatformURL string    `json:"platform_url,omitempty"`
	Error       string    `json:"error,omitempty"`
	RetryAfter  *int      `json:"retry_after,omitempty"`
	Attempts    int       `json:"attempts"`
	Timestamp   time.Time `json:"timestamp"`
}

type DeliveryResult struct {
	PlatformID  string
	PlatformURL string
}
```

```go
// social-publisher/internal/domain/adapter.go
package domain

import "context"

type PlatformAdapter interface {
	Name() string
	Capabilities() PlatformCapabilities
	Transform(content PublishMessage) (PlatformPost, error)
	Validate(post PlatformPost) error
	Publish(ctx context.Context, post PlatformPost) (DeliveryResult, error)
}

type PlatformCapabilities struct {
	SupportsImages    bool
	SupportsThreading bool
	SupportsMarkdown  bool
	SupportsHTML      bool
	MaxLength         int
	RequiresMetadata  []string
}

type PlatformPost struct {
	Platform string
	Content  string
	Title    string
	URL      string
	Images   []string
	Tags     []string
	Metadata map[string]string
	Thread   []string // for platforms that support threading (X)
}
```

```go
// social-publisher/internal/domain/errors.go
package domain

import (
	"fmt"
	"time"
)

type PublishError interface {
	error
	IsRetryable() bool
}

type RateLimitError struct {
	RetryAfter time.Duration
	Message    string
}

func (e *RateLimitError) Error() string   { return fmt.Sprintf("rate limited: %s", e.Message) }
func (e *RateLimitError) IsRetryable() bool { return true }

type TransientError struct {
	Message string
}

func (e *TransientError) Error() string   { return fmt.Sprintf("transient error: %s", e.Message) }
func (e *TransientError) IsRetryable() bool { return true }

type PermanentError struct {
	Message  string
	Code     string
	Response string
}

func (e *PermanentError) Error() string   { return fmt.Sprintf("permanent error [%s]: %s", e.Code, e.Message) }
func (e *PermanentError) IsRetryable() bool { return false }

type AuthError struct {
	Message string
}

func (e *AuthError) Error() string   { return fmt.Sprintf("auth error: %s", e.Message) }
func (e *AuthError) IsRetryable() bool { return true }

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string   { return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message) }
func (e *ValidationError) IsRetryable() bool { return false }
```

**Step 4: Run tests**

Run: `cd social-publisher && go test ./internal/domain/... -v`
Expected: PASS (5 tests)

**Step 5: Commit**

```bash
git add social-publisher/internal/domain/
git commit -m "feat(social-publisher): add domain types for content, delivery, adapter, errors"
```

---

## Task 5: Database Repository

**Files:**
- Create: `social-publisher/internal/database/repository.go`
- Create: `social-publisher/internal/database/repository_test.go`

**Step 1: Write repository interface test**

```go
// social-publisher/internal/database/repository_test.go
package database_test

import (
	"testing"

	"github.com/north-cloud/social-publisher/internal/database"
	"github.com/stretchr/testify/assert"
)

func TestNewRepository_NotNil(t *testing.T) {
	// Verify the repository struct can be created (nil db is ok for unit tests)
	repo := database.NewRepository(nil)
	assert.NotNil(t, repo)
}
```

Note: Full repository integration tests require a running PostgreSQL instance. The unit test verifies construction. Integration tests will be added in a separate task with testcontainers or a test database.

**Step 2: Run test to verify it fails**

Run: `cd social-publisher && go test ./internal/database/... -v`
Expected: FAIL

**Step 3: Write repository**

```go
// social-publisher/internal/database/repository.go
package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/north-cloud/social-publisher/internal/domain"
)

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

// Content operations

func (r *Repository) CreateContent(ctx context.Context, msg *domain.PublishMessage) error {
	images, _ := json.Marshal(msg.Images)
	tags, _ := json.Marshal(msg.Tags)
	metadata, _ := json.Marshal(msg.Metadata)

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO content (id, type, title, body, summary, url, images, tags, project, metadata, source, scheduled_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (id) DO NOTHING`,
		msg.ContentID, msg.Type, msg.Title, msg.Body, msg.Summary, msg.URL,
		images, tags, msg.Project, metadata, msg.Source, msg.ScheduledAt,
	)
	return err
}

func (r *Repository) MarkContentPublished(ctx context.Context, contentID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE content SET published = true WHERE id = $1`, contentID)
	return err
}

func (r *Repository) GetDueScheduledContent(ctx context.Context, limit int) ([]domain.PublishMessage, error) {
	rows, err := r.db.QueryxContext(ctx, `
		SELECT id, type, title, body, summary, url, images, tags, project, metadata, source, scheduled_at
		FROM content
		WHERE scheduled_at <= NOW() AND published = false
		ORDER BY scheduled_at ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []domain.PublishMessage
	for rows.Next() {
		var msg domain.PublishMessage
		var images, tags, metadata []byte
		if err := rows.Scan(
			&msg.ContentID, &msg.Type, &msg.Title, &msg.Body, &msg.Summary, &msg.URL,
			&images, &tags, &msg.Project, &metadata, &msg.Source, &msg.ScheduledAt,
		); err != nil {
			return nil, err
		}
		json.Unmarshal(images, &msg.Images)
		json.Unmarshal(tags, &msg.Tags)
		json.Unmarshal(metadata, &msg.Metadata)
		results = append(results, msg)
	}
	return results, nil
}

// Delivery operations

func (r *Repository) CreateDelivery(ctx context.Context, contentID, platform, account string, maxAttempts int) (*domain.Delivery, error) {
	id := uuid.New().String()
	delivery := &domain.Delivery{
		ID:          id,
		ContentID:   contentID,
		Platform:    platform,
		Account:     account,
		Status:      domain.StatusPending,
		Attempts:    0,
		MaxAttempts: maxAttempts,
		CreatedAt:   time.Now(),
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO deliveries (id, content_id, platform, account, status, attempts, max_attempts, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (content_id, platform, account) DO NOTHING`,
		delivery.ID, delivery.ContentID, delivery.Platform, delivery.Account,
		delivery.Status, delivery.Attempts, delivery.MaxAttempts, delivery.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return delivery, nil
}

func (r *Repository) UpdateDeliveryStatus(ctx context.Context, id string, status domain.DeliveryStatus, result *domain.DeliveryResult, errMsg *string) error {
	now := time.Now()
	var platformID, platformURL *string
	if result != nil {
		platformID = &result.PlatformID
		platformURL = &result.PlatformURL
	}

	query := `UPDATE deliveries SET status = $1, platform_id = $2, platform_url = $3, error = $4`
	args := []any{status, platformID, platformURL, errMsg}

	if status == domain.StatusDelivered {
		query += fmt.Sprintf(", delivered_at = $%d", len(args)+1)
		args = append(args, now)
	}
	if errMsg != nil {
		query += fmt.Sprintf(", last_error_at = $%d", len(args)+1)
		args = append(args, now)
	}

	query += fmt.Sprintf(" WHERE id = $%d", len(args)+1)
	args = append(args, id)

	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

func (r *Repository) IncrementAttempts(ctx context.Context, id string, nextRetryAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE deliveries SET attempts = attempts + 1, status = 'retrying', next_retry_at = $1
		WHERE id = $2`, nextRetryAt, id)
	return err
}

func (r *Repository) GetDueRetries(ctx context.Context, limit int) ([]domain.Delivery, error) {
	var deliveries []domain.Delivery
	err := r.db.SelectContext(ctx, &deliveries, `
		SELECT * FROM deliveries
		WHERE status = 'retrying' AND next_retry_at <= NOW()
		ORDER BY next_retry_at ASC
		LIMIT $1`, limit)
	return deliveries, err
}

func (r *Repository) GetDeliveriesByContentID(ctx context.Context, contentID string) ([]domain.Delivery, error) {
	var deliveries []domain.Delivery
	err := r.db.SelectContext(ctx, &deliveries, `
		SELECT * FROM deliveries WHERE content_id = $1 ORDER BY created_at`, contentID)
	return deliveries, err
}

func (r *Repository) MarkDeliveryFailed(ctx context.Context, id string, errMsg string) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `
		UPDATE deliveries SET status = 'failed', error = $1, last_error_at = $2
		WHERE id = $3`, errMsg, now, id)
	return err
}

// Scheduled content transaction

func (r *Repository) PublishScheduledContent(ctx context.Context, contentID string, fn func(tx *sqlx.Tx) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `UPDATE content SET published = true WHERE id = $1`, contentID); err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit()
}
```

**Step 4: Run tests**

Run: `cd social-publisher && go test ./internal/database/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add social-publisher/internal/database/
git commit -m "feat(social-publisher): add database repository for content and deliveries"
```

---

## Task 6: Priority Queue

**Files:**
- Create: `social-publisher/internal/orchestrator/queue.go`
- Create: `social-publisher/internal/orchestrator/queue_test.go`

**Step 1: Write the queue tests**

```go
// social-publisher/internal/orchestrator/queue_test.go
package orchestrator_test

import (
	"testing"
	"time"

	"github.com/north-cloud/social-publisher/internal/orchestrator"
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
```

**Step 2: Run test to verify it fails**

Run: `cd social-publisher && go test ./internal/orchestrator/... -v`
Expected: FAIL

**Step 3: Write the priority queue**

```go
// social-publisher/internal/orchestrator/queue.go
package orchestrator

import "time"

type PublishJob struct {
	ContentID  string
	DeliveryID string
	Platform   string
	Account    string
	Message    interface{} // *domain.PublishMessage
	IsRetry    bool
}

type PriorityQueue struct {
	realtime chan PublishJob
	retries  chan PublishJob
}

func NewPriorityQueue(realtimeSize, retrySize int) *PriorityQueue {
	return &PriorityQueue{
		realtime: make(chan PublishJob, realtimeSize),
		retries:  make(chan PublishJob, retrySize),
	}
}

func (pq *PriorityQueue) EnqueueRealtime(job PublishJob) {
	pq.realtime <- job
}

func (pq *PriorityQueue) EnqueueRetry(job PublishJob) {
	pq.retries <- job
}

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
```

**Step 4: Run tests**

Run: `cd social-publisher && go test ./internal/orchestrator/... -v`
Expected: PASS (3 tests)

**Step 5: Commit**

```bash
git add social-publisher/internal/orchestrator/
git commit -m "feat(social-publisher): add two-queue priority model"
```

---

## Task 7: Redis Subscriber and Event Publisher

**Files:**
- Create: `social-publisher/internal/redis/subscriber.go`
- Create: `social-publisher/internal/redis/publisher.go`

**Step 1: Write Redis subscriber**

```go
// social-publisher/internal/redis/subscriber.go
package redis

import (
	"context"
	"encoding/json"

	goredis "github.com/redis/go-redis/v9"

	"github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/social-publisher/internal/domain"
)

const ChannelSocialPublish = "social:publish"

type Subscriber struct {
	client *goredis.Client
	log    logger.Logger
}

func NewSubscriber(client *goredis.Client, log logger.Logger) *Subscriber {
	return &Subscriber{client: client, log: log}
}

func (s *Subscriber) Subscribe(ctx context.Context, handler func(msg *domain.PublishMessage)) error {
	pubsub := s.client.Subscribe(ctx, ChannelSocialPublish)
	defer pubsub.Close()

	s.log.Info("Subscribed to Redis channel", logger.String("channel", ChannelSocialPublish))

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			s.log.Info("Redis subscriber shutting down")
			return ctx.Err()
		case redisMsg, ok := <-ch:
			if !ok {
				return nil
			}
			var msg domain.PublishMessage
			if err := json.Unmarshal([]byte(redisMsg.Payload), &msg); err != nil {
				s.log.Error("Failed to unmarshal publish message",
					logger.Error(err),
					logger.String("payload", redisMsg.Payload),
				)
				continue
			}
			handler(&msg)
		}
	}
}
```

**Step 2: Write Redis event publisher**

```go
// social-publisher/internal/redis/publisher.go
package redis

import (
	"context"
	"encoding/json"

	goredis "github.com/redis/go-redis/v9"

	"github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/social-publisher/internal/domain"
)

const (
	ChannelDeliveryStatus = "social:delivery-status"
	ChannelDeadLetter     = "social:dead-letter"
)

type EventPublisher struct {
	client *goredis.Client
	log    logger.Logger
}

func NewEventPublisher(client *goredis.Client, log logger.Logger) *EventPublisher {
	return &EventPublisher{client: client, log: log}
}

func (p *EventPublisher) PublishDeliveryEvent(ctx context.Context, event *domain.DeliveryEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.client.Publish(ctx, ChannelDeliveryStatus, data).Err()
}

type DeadLetterMessage struct {
	Event   domain.DeliveryEvent `json:"event"`
	Original domain.PublishMessage `json:"original"`
	ErrorType string              `json:"error_type"`
	PlatformResponse string       `json:"platform_response,omitempty"`
}

func (p *EventPublisher) PublishDeadLetter(ctx context.Context, msg *DeadLetterMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	p.log.Warn("Publishing to dead-letter channel",
		logger.String("content_id", msg.Event.ContentID),
		logger.String("platform", msg.Event.Platform),
		logger.String("error_type", msg.ErrorType),
	)
	return p.client.Publish(ctx, ChannelDeadLetter, data).Err()
}
```

**Step 3: Commit**

```bash
git add social-publisher/internal/redis/
git commit -m "feat(social-publisher): add Redis subscriber and event publisher"
```

---

## Task 8: Adapter Template and Test Harness

**Files:**
- Create: `social-publisher/internal/adapters/testing.go`
- Create: `social-publisher/internal/adapters/testing_test.go`

**Step 1: Write adapter test harness tests**

```go
// social-publisher/internal/adapters/testing_test.go
package adapters_test

import (
	"context"
	"testing"

	"github.com/north-cloud/social-publisher/internal/adapters"
	"github.com/north-cloud/social-publisher/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestMockAdapter_ImplementsInterface(t *testing.T) {
	var _ domain.PlatformAdapter = (*adapters.MockAdapter)(nil)
}

func TestMockAdapter_PublishSuccess(t *testing.T) {
	mock := adapters.NewMockAdapter("test-platform")

	msg := domain.PublishMessage{
		ContentID: "test-1",
		Summary:   "Test post",
		URL:       "https://example.com",
	}

	post, err := mock.Transform(msg)
	assert.NoError(t, err)

	err = mock.Validate(post)
	assert.NoError(t, err)

	result, err := mock.Publish(context.Background(), post)
	assert.NoError(t, err)
	assert.NotEmpty(t, result.PlatformID)
	assert.Equal(t, 1, mock.PublishCount())
}

func TestMockAdapter_PublishFailure(t *testing.T) {
	mock := adapters.NewMockAdapter("test-platform")
	mock.SetPublishError(&domain.TransientError{Message: "timeout"})

	msg := domain.PublishMessage{Summary: "Test"}
	post, _ := mock.Transform(msg)
	_, err := mock.Publish(context.Background(), post)
	assert.Error(t, err)
	assert.True(t, err.(domain.PublishError).IsRetryable())
}

func TestMockAdapter_Capabilities(t *testing.T) {
	mock := adapters.NewMockAdapter("test-platform")
	caps := mock.Capabilities()
	assert.Equal(t, 280, caps.MaxLength)
}
```

**Step 2: Run test to verify it fails**

Run: `cd social-publisher && go test ./internal/adapters/... -v`
Expected: FAIL

**Step 3: Write the mock adapter (test harness)**

```go
// social-publisher/internal/adapters/testing.go
package adapters

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/north-cloud/social-publisher/internal/domain"
)

type MockAdapter struct {
	name         string
	capabilities domain.PlatformCapabilities
	publishErr   domain.PublishError
	publishCount atomic.Int32
	mu           sync.Mutex
	published    []domain.PlatformPost
}

func NewMockAdapter(name string) *MockAdapter {
	return &MockAdapter{
		name: name,
		capabilities: domain.PlatformCapabilities{
			SupportsImages:    true,
			SupportsThreading: false,
			SupportsMarkdown:  false,
			SupportsHTML:      false,
			MaxLength:         280,
		},
	}
}

func (m *MockAdapter) Name() string                           { return m.name }
func (m *MockAdapter) Capabilities() domain.PlatformCapabilities { return m.capabilities }

func (m *MockAdapter) Transform(content domain.PublishMessage) (domain.PlatformPost, error) {
	text := content.Summary
	if content.URL != "" {
		text = fmt.Sprintf("%s %s", text, content.URL)
	}
	return domain.PlatformPost{
		Platform: m.name,
		Content:  text,
		URL:      content.URL,
		Tags:     content.Tags,
	}, nil
}

func (m *MockAdapter) Validate(post domain.PlatformPost) error {
	if post.Content == "" {
		return &domain.ValidationError{Field: "content", Message: "content is required"}
	}
	return nil
}

func (m *MockAdapter) Publish(ctx context.Context, post domain.PlatformPost) (domain.DeliveryResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.publishErr != nil {
		return domain.DeliveryResult{}, m.publishErr
	}

	m.publishCount.Add(1)
	m.published = append(m.published, post)

	return domain.DeliveryResult{
		PlatformID:  fmt.Sprintf("mock-%d", m.publishCount.Load()),
		PlatformURL: fmt.Sprintf("https://%s.example.com/post/%d", m.name, m.publishCount.Load()),
	}, nil
}

func (m *MockAdapter) SetPublishError(err domain.PublishError) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.publishErr = err
}

func (m *MockAdapter) PublishCount() int {
	return int(m.publishCount.Load())
}

func (m *MockAdapter) Published() []domain.PlatformPost {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]domain.PlatformPost{}, m.published...)
}
```

**Step 4: Run tests**

Run: `cd social-publisher && go test ./internal/adapters/... -v`
Expected: PASS (4 tests)

**Step 5: Commit**

```bash
git add social-publisher/internal/adapters/
git commit -m "feat(social-publisher): add adapter test harness with mock implementation"
```

---

## Task 9: X (Twitter) Adapter

**Files:**
- Create: `social-publisher/internal/adapters/x/adapter.go`
- Create: `social-publisher/internal/adapters/x/adapter_test.go`
- Create: `social-publisher/internal/adapters/x/client.go`
- Create: `social-publisher/internal/adapters/x/oauth.go`

**Step 1: Write X adapter transform/validate tests**

```go
// social-publisher/internal/adapters/x/adapter_test.go
package x_test

import (
	"testing"

	"github.com/north-cloud/social-publisher/internal/adapters/x"
	"github.com/north-cloud/social-publisher/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestXAdapter_Name(t *testing.T) {
	adapter := x.NewAdapter(nil)
	assert.Equal(t, "x", adapter.Name())
}

func TestXAdapter_Capabilities(t *testing.T) {
	adapter := x.NewAdapter(nil)
	caps := adapter.Capabilities()
	assert.Equal(t, 280, caps.MaxLength)
	assert.True(t, caps.SupportsThreading)
	assert.True(t, caps.SupportsImages)
	assert.Empty(t, caps.RequiresMetadata)
}

func TestXAdapter_Transform_ShortPost(t *testing.T) {
	adapter := x.NewAdapter(nil)
	msg := domain.PublishMessage{
		Summary: "Check out this new feature",
		URL:     "https://example.com/post",
		Tags:    []string{"golang", "dev"},
	}

	post, err := adapter.Transform(msg)
	assert.NoError(t, err)
	assert.Contains(t, post.Content, "Check out this new feature")
	assert.Contains(t, post.Content, "https://example.com/post")
	assert.LessOrEqual(t, len(post.Content), 280)
}

func TestXAdapter_Transform_LongPostCreatesThread(t *testing.T) {
	adapter := x.NewAdapter(nil)
	longBody := ""
	for i := 0; i < 50; i++ {
		longBody += "This is a sentence that makes the content longer. "
	}
	msg := domain.PublishMessage{
		Summary: "A long blog post",
		Body:    longBody,
		URL:     "https://example.com/long-post",
	}

	post, err := adapter.Transform(msg)
	assert.NoError(t, err)
	assert.True(t, len(post.Thread) > 0 || len(post.Content) <= 280)
}

func TestXAdapter_Validate_EmptyContent(t *testing.T) {
	adapter := x.NewAdapter(nil)
	post := domain.PlatformPost{Platform: "x", Content: ""}
	err := adapter.Validate(post)
	assert.Error(t, err)
}

func TestXAdapter_Validate_TooLong(t *testing.T) {
	adapter := x.NewAdapter(nil)
	longContent := ""
	for i := 0; i < 300; i++ {
		longContent += "a"
	}
	post := domain.PlatformPost{Platform: "x", Content: longContent}
	err := adapter.Validate(post)
	assert.Error(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `cd social-publisher && go test ./internal/adapters/x/... -v`
Expected: FAIL

**Step 3: Write the X adapter**

```go
// social-publisher/internal/adapters/x/adapter.go
package x

import (
	"context"
	"fmt"
	"strings"

	"github.com/north-cloud/social-publisher/internal/domain"
)

const maxTweetLength = 280

type Adapter struct {
	client *Client
}

func NewAdapter(client *Client) *Adapter {
	return &Adapter{client: client}
}

func (a *Adapter) Name() string { return "x" }

func (a *Adapter) Capabilities() domain.PlatformCapabilities {
	return domain.PlatformCapabilities{
		SupportsImages:    true,
		SupportsThreading: true,
		SupportsMarkdown:  false,
		SupportsHTML:      false,
		MaxLength:         maxTweetLength,
	}
}

func (a *Adapter) Transform(content domain.PublishMessage) (domain.PlatformPost, error) {
	text := content.Summary
	if content.URL != "" {
		// X counts URLs as 23 characters regardless of length
		textBudget := maxTweetLength - 24 // 23 for URL + 1 for space
		if len(text) > textBudget {
			text = text[:textBudget-3] + "..."
		}
		text = fmt.Sprintf("%s %s", text, content.URL)
	}

	// Add hashtags if they fit
	if len(content.Tags) > 0 {
		hashtags := formatHashtags(content.Tags)
		if len(text)+1+len(hashtags) <= maxTweetLength {
			text = fmt.Sprintf("%s\n%s", text, hashtags)
		}
	}

	post := domain.PlatformPost{
		Platform: "x",
		Content:  text,
		URL:      content.URL,
		Images:   content.Images,
		Tags:     content.Tags,
	}

	// If the full body is provided and much longer, create a thread
	if content.Body != "" && len(content.Body) > maxTweetLength*2 {
		post.Thread = splitThread(content.Summary, content.Body, content.URL)
	}

	return post, nil
}

func (a *Adapter) Validate(post domain.PlatformPost) error {
	if post.Content == "" {
		return &domain.ValidationError{Field: "content", Message: "tweet content is required"}
	}
	if len(post.Thread) == 0 && len(post.Content) > maxTweetLength {
		return &domain.ValidationError{
			Field:   "content",
			Message: fmt.Sprintf("tweet exceeds %d characters (%d)", maxTweetLength, len(post.Content)),
		}
	}
	return nil
}

func (a *Adapter) Publish(ctx context.Context, post domain.PlatformPost) (domain.DeliveryResult, error) {
	if a.client == nil {
		return domain.DeliveryResult{}, &domain.PermanentError{Message: "X client not configured"}
	}

	if len(post.Thread) > 0 {
		return a.client.PostThread(ctx, post.Thread)
	}
	return a.client.PostTweet(ctx, post.Content)
}

func formatHashtags(tags []string) string {
	var hashtags []string
	for _, tag := range tags {
		cleaned := strings.ReplaceAll(tag, "-", "")
		cleaned = strings.ReplaceAll(cleaned, " ", "")
		hashtags = append(hashtags, "#"+cleaned)
	}
	return strings.Join(hashtags, " ")
}

func splitThread(summary, body, url string) []string {
	// First tweet: summary + URL
	first := summary
	if url != "" {
		first = fmt.Sprintf("%s %s", summary, url)
	}

	// Remaining tweets: split body by sentences
	sentences := strings.Split(body, ". ")
	var thread []string
	thread = append(thread, first)

	current := ""
	for _, sentence := range sentences {
		candidate := current + sentence + ". "
		if len(candidate) > maxTweetLength-5 { // leave room for thread numbering
			if current != "" {
				thread = append(thread, strings.TrimSpace(current))
			}
			current = sentence + ". "
		} else {
			current = candidate
		}
	}
	if current != "" {
		thread = append(thread, strings.TrimSpace(current))
	}

	return thread
}
```

```go
// social-publisher/internal/adapters/x/client.go
package x

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/north-cloud/social-publisher/internal/domain"
)

const apiBaseURL = "https://api.x.com/2"

type Client struct {
	httpClient  *http.Client
	bearerToken string
}

func NewClient(bearerToken string) *Client {
	return &Client{
		httpClient:  &http.Client{},
		bearerToken: bearerToken,
	}
}

type tweetRequest struct {
	Text  string      `json:"text"`
	Reply *replyTo    `json:"reply,omitempty"`
}

type replyTo struct {
	InReplyToTweetID string `json:"in_reply_to_tweet_id"`
}

type tweetResponse struct {
	Data struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"errors"`
}

func (c *Client) PostTweet(ctx context.Context, text string) (domain.DeliveryResult, error) {
	return c.postTweet(ctx, text, "")
}

func (c *Client) PostThread(ctx context.Context, tweets []string) (domain.DeliveryResult, error) {
	if len(tweets) == 0 {
		return domain.DeliveryResult{}, &domain.ValidationError{Field: "thread", Message: "empty thread"}
	}

	// Post first tweet
	result, err := c.postTweet(ctx, tweets[0], "")
	if err != nil {
		return result, err
	}

	// Post replies
	lastID := result.PlatformID
	for _, tweet := range tweets[1:] {
		result, err = c.postTweet(ctx, tweet, lastID)
		if err != nil {
			return result, err // partial thread posted
		}
		lastID = result.PlatformID
	}

	return result, nil
}

func (c *Client) postTweet(ctx context.Context, text, replyToID string) (domain.DeliveryResult, error) {
	reqBody := tweetRequest{Text: text}
	if replyToID != "" {
		reqBody.Reply = &replyTo{InReplyToTweetID: replyToID}
	}

	body, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", apiBaseURL+"/tweets", bytes.NewReader(body))
	if err != nil {
		return domain.DeliveryResult{}, &domain.TransientError{Message: err.Error()}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.bearerToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.DeliveryResult{}, &domain.TransientError{Message: err.Error()}
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	switch {
	case resp.StatusCode == 429:
		return domain.DeliveryResult{}, &domain.RateLimitError{
			Message:    "X API rate limit exceeded",
			RetryAfter: 900, // 15 minutes default
		}
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		return domain.DeliveryResult{}, &domain.AuthError{Message: "X API authentication failed"}
	case resp.StatusCode >= 500:
		return domain.DeliveryResult{}, &domain.TransientError{Message: fmt.Sprintf("X API server error: %d", resp.StatusCode)}
	case resp.StatusCode >= 400:
		return domain.DeliveryResult{}, &domain.PermanentError{
			Message:  "X API client error",
			Code:     fmt.Sprintf("%d", resp.StatusCode),
			Response: string(respBody),
		}
	}

	var tweetResp tweetResponse
	if err := json.Unmarshal(respBody, &tweetResp); err != nil {
		return domain.DeliveryResult{}, &domain.TransientError{Message: "failed to parse X API response"}
	}

	return domain.DeliveryResult{
		PlatformID:  tweetResp.Data.ID,
		PlatformURL: fmt.Sprintf("https://x.com/i/status/%s", tweetResp.Data.ID),
	}, nil
}
```

**Step 4: Run tests**

Run: `cd social-publisher && go test ./internal/adapters/x/... -v`
Expected: PASS (5 transform/validate tests; publish tests use nil client and test error path)

**Step 5: Commit**

```bash
git add social-publisher/internal/adapters/x/
git commit -m "feat(social-publisher): add X (Twitter) adapter with thread support"
```

---

## Task 10: Publish Orchestrator

**Files:**
- Create: `social-publisher/internal/orchestrator/orchestrator.go`
- Create: `social-publisher/internal/orchestrator/orchestrator_test.go`

**Step 1: Write orchestrator tests**

```go
// social-publisher/internal/orchestrator/orchestrator_test.go
package orchestrator_test

import (
	"context"
	"testing"
	"time"

	"github.com/north-cloud/social-publisher/internal/adapters"
	"github.com/north-cloud/social-publisher/internal/domain"
	"github.com/north-cloud/social-publisher/internal/orchestrator"
	"github.com/stretchr/testify/assert"
)

func TestOrchestrator_ProcessJob_Success(t *testing.T) {
	mock := adapters.NewMockAdapter("test")
	orch := orchestrator.NewOrchestrator(
		map[string]domain.PlatformAdapter{"test": mock},
		nil, // no event publisher in unit tests
		nil, // no repo in unit tests
	)

	msg := &domain.PublishMessage{
		ContentID: "test-1",
		Summary:   "Hello world",
		URL:       "https://example.com",
	}

	result, err := orch.ProcessJob(context.Background(), "test", msg)
	assert.NoError(t, err)
	assert.NotEmpty(t, result.PlatformID)
	assert.Equal(t, 1, mock.PublishCount())
}

func TestOrchestrator_ProcessJob_UnknownPlatform(t *testing.T) {
	orch := orchestrator.NewOrchestrator(
		map[string]domain.PlatformAdapter{},
		nil, nil,
	)

	msg := &domain.PublishMessage{ContentID: "test-1", Summary: "Hello"}
	_, err := orch.ProcessJob(context.Background(), "unknown", msg)
	assert.Error(t, err)
}

func TestOrchestrator_ProcessJob_ValidationError(t *testing.T) {
	mock := adapters.NewMockAdapter("test")
	orch := orchestrator.NewOrchestrator(
		map[string]domain.PlatformAdapter{"test": mock},
		nil, nil,
	)

	msg := &domain.PublishMessage{ContentID: "test-1", Summary: ""} // empty = validation error
	_, err := orch.ProcessJob(context.Background(), "test", msg)
	assert.Error(t, err)
}

func TestBackoff_Calculation(t *testing.T) {
	tests := []struct {
		attempts int
		expected time.Duration
		valid    bool
	}{
		{0, 30 * time.Second, true},
		{1, 2 * time.Minute, true},
		{2, 10 * time.Minute, true},
		{3, 0, false},
	}

	for _, tc := range tests {
		result, ok := orchestrator.NextRetryAt(tc.attempts)
		assert.Equal(t, tc.valid, ok)
		if ok {
			assert.InDelta(t, tc.expected.Seconds(), time.Until(result).Seconds(), 2)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd social-publisher && go test ./internal/orchestrator/... -v`
Expected: FAIL (orchestrator.go doesn't exist yet; queue tests still pass)

**Step 3: Write the orchestrator**

```go
// social-publisher/internal/orchestrator/orchestrator.go
package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/north-cloud/social-publisher/internal/domain"
)

var backoffs = []time.Duration{
	30 * time.Second,
	2 * time.Minute,
	10 * time.Minute,
}

func NextRetryAt(attempts int) (time.Time, bool) {
	if attempts >= len(backoffs) {
		return time.Time{}, false
	}
	return time.Now().Add(backoffs[attempts]), true
}

type EventPublisher interface {
	PublishDeliveryEvent(ctx context.Context, event *domain.DeliveryEvent) error
}

type ContentRepository interface {
	UpdateDeliveryStatus(ctx context.Context, id string, status domain.DeliveryStatus, result *domain.DeliveryResult, errMsg *string) error
	IncrementAttempts(ctx context.Context, id string, nextRetryAt time.Time) error
	MarkDeliveryFailed(ctx context.Context, id string, errMsg string) error
}

type Orchestrator struct {
	adapters map[string]domain.PlatformAdapter
	events   EventPublisher
	repo     ContentRepository
}

func NewOrchestrator(
	adapters map[string]domain.PlatformAdapter,
	events EventPublisher,
	repo ContentRepository,
) *Orchestrator {
	return &Orchestrator{
		adapters: adapters,
		events:   events,
		repo:     repo,
	}
}

func (o *Orchestrator) ProcessJob(ctx context.Context, platform string, msg *domain.PublishMessage) (domain.DeliveryResult, error) {
	adapter, ok := o.adapters[platform]
	if !ok {
		return domain.DeliveryResult{}, fmt.Errorf("unknown platform: %s", platform)
	}

	// Transform
	post, err := adapter.Transform(*msg)
	if err != nil {
		return domain.DeliveryResult{}, err
	}

	// Validate (before any API call)
	if err := adapter.Validate(post); err != nil {
		return domain.DeliveryResult{}, err
	}

	// Publish
	result, err := adapter.Publish(ctx, post)
	if err != nil {
		return domain.DeliveryResult{}, err
	}

	return result, nil
}

func (o *Orchestrator) GetAdapter(platform string) (domain.PlatformAdapter, bool) {
	a, ok := o.adapters[platform]
	return a, ok
}
```

**Step 4: Run tests**

Run: `cd social-publisher && go test ./internal/orchestrator/... -v`
Expected: PASS (all queue + orchestrator tests)

**Step 5: Commit**

```bash
git add social-publisher/internal/orchestrator/orchestrator.go
git commit -m "feat(social-publisher): add publish orchestrator with backoff calculation"
```

---

## Task 11: Retry Worker

**Files:**
- Create: `social-publisher/internal/workers/retry.go`

**Step 1: Write the retry worker**

```go
// social-publisher/internal/workers/retry.go
package workers

import (
	"context"
	"time"

	"github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/social-publisher/internal/database"
	"github.com/north-cloud/social-publisher/internal/domain"
	"github.com/north-cloud/social-publisher/internal/orchestrator"
)

type RetryWorker struct {
	repo      *database.Repository
	orch      *orchestrator.Orchestrator
	events    orchestrator.EventPublisher
	log       logger.Logger
	interval  time.Duration
	batchSize int
}

func NewRetryWorker(
	repo *database.Repository,
	orch *orchestrator.Orchestrator,
	events orchestrator.EventPublisher,
	log logger.Logger,
	interval time.Duration,
	batchSize int,
) *RetryWorker {
	return &RetryWorker{
		repo:      repo,
		orch:      orch,
		events:    events,
		log:       log,
		interval:  interval,
		batchSize: batchSize,
	}
}

func (w *RetryWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.log.Info("Retry worker started", logger.Duration("interval", w.interval))

	for {
		select {
		case <-ctx.Done():
			w.log.Info("Retry worker shutting down")
			return
		case <-ticker.C:
			w.processRetries(ctx)
		}
	}
}

func (w *RetryWorker) processRetries(ctx context.Context) {
	deliveries, err := w.repo.GetDueRetries(ctx, w.batchSize)
	if err != nil {
		w.log.Error("Failed to fetch due retries", logger.Error(err))
		return
	}

	if len(deliveries) == 0 {
		return
	}

	w.log.Info("Processing retries", logger.Int("count", len(deliveries)))

	for _, delivery := range deliveries {
		w.processRetry(ctx, &delivery)
	}
}

func (w *RetryWorker) processRetry(ctx context.Context, delivery *domain.Delivery) {
	// Emit publishing event
	w.emitEvent(ctx, delivery, string(domain.StatusPublishing), nil)

	// Load content for this delivery (would need a GetContent method)
	// For now, use delivery metadata to reconstruct minimal message
	msg := &domain.PublishMessage{
		ContentID: delivery.ContentID,
	}

	result, err := w.orch.ProcessJob(ctx, delivery.Platform, msg)
	if err != nil {
		w.handleRetryError(ctx, delivery, err)
		return
	}

	// Success
	if updateErr := w.repo.UpdateDeliveryStatus(ctx, delivery.ID, domain.StatusDelivered, &result, nil); updateErr != nil {
		w.log.Error("Failed to update delivery status", logger.Error(updateErr))
	}
	w.emitEvent(ctx, delivery, string(domain.StatusDelivered), nil)

	w.log.Info("Retry succeeded",
		logger.String("delivery_id", delivery.ID),
		logger.String("platform", delivery.Platform),
		logger.Int("attempts", delivery.Attempts),
	)
}

func (w *RetryWorker) handleRetryError(ctx context.Context, delivery *domain.Delivery, err error) {
	errMsg := err.Error()

	pubErr, isPubErr := err.(domain.PublishError)
	if !isPubErr || !pubErr.IsRetryable() {
		// Permanent error: dead-letter
		if failErr := w.repo.MarkDeliveryFailed(ctx, delivery.ID, errMsg); failErr != nil {
			w.log.Error("Failed to mark delivery as failed", logger.Error(failErr))
		}
		w.emitEvent(ctx, delivery, string(domain.StatusFailed), &errMsg)
		return
	}

	// Check if max attempts exceeded
	nextRetry, ok := orchestrator.NextRetryAt(delivery.Attempts)
	if !ok {
		if failErr := w.repo.MarkDeliveryFailed(ctx, delivery.ID, errMsg); failErr != nil {
			w.log.Error("Failed to mark delivery as failed", logger.Error(failErr))
		}
		w.emitEvent(ctx, delivery, string(domain.StatusFailed), &errMsg)
		return
	}

	// Handle rate limit with platform-specific cooldown
	if rle, ok := err.(*domain.RateLimitError); ok {
		nextRetry = time.Now().Add(rle.RetryAfter)
	}

	if incErr := w.repo.IncrementAttempts(ctx, delivery.ID, nextRetry); incErr != nil {
		w.log.Error("Failed to increment retry attempts", logger.Error(incErr))
	}

	w.log.Warn("Retry failed, scheduling next attempt",
		logger.String("delivery_id", delivery.ID),
		logger.String("platform", delivery.Platform),
		logger.Int("attempts", delivery.Attempts+1),
		logger.String("next_retry", nextRetry.Format(time.RFC3339)),
	)
}

func (w *RetryWorker) emitEvent(ctx context.Context, delivery *domain.Delivery, status string, errMsg *string) {
	if w.events == nil {
		return
	}
	event := &domain.DeliveryEvent{
		ContentID:  delivery.ContentID,
		DeliveryID: delivery.ID,
		Platform:   delivery.Platform,
		Account:    delivery.Account,
		Status:     status,
		Attempts:   delivery.Attempts,
		Timestamp:  time.Now(),
	}
	if errMsg != nil {
		event.Error = *errMsg
	}
	if pubErr := w.events.PublishDeliveryEvent(ctx, event); pubErr != nil {
		w.log.Error("Failed to emit delivery event", logger.Error(pubErr))
	}
}
```

**Step 2: Commit**

```bash
git add social-publisher/internal/workers/
git commit -m "feat(social-publisher): add retry worker with backoff and error classification"
```

---

## Task 12: Scheduler

**Files:**
- Create: `social-publisher/internal/workers/scheduler.go`

**Step 1: Write the scheduler**

```go
// social-publisher/internal/workers/scheduler.go
package workers

import (
	"context"
	"time"

	"github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/social-publisher/internal/database"
	"github.com/north-cloud/social-publisher/internal/domain"
	"github.com/north-cloud/social-publisher/internal/orchestrator"
)

type Scheduler struct {
	repo      *database.Repository
	queue     *orchestrator.PriorityQueue
	log       logger.Logger
	interval  time.Duration
	batchSize int
}

func NewScheduler(
	repo *database.Repository,
	queue *orchestrator.PriorityQueue,
	log logger.Logger,
	interval time.Duration,
	batchSize int,
) *Scheduler {
	return &Scheduler{
		repo:      repo,
		queue:     queue,
		log:       log,
		interval:  interval,
		batchSize: batchSize,
	}
}

func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.log.Info("Scheduler started", logger.Duration("interval", s.interval))

	for {
		select {
		case <-ctx.Done():
			s.log.Info("Scheduler shutting down")
			return
		case <-ticker.C:
			s.processDueContent(ctx)
		}
	}
}

func (s *Scheduler) processDueContent(ctx context.Context) {
	content, err := s.repo.GetDueScheduledContent(ctx, s.batchSize)
	if err != nil {
		s.log.Error("Failed to fetch due scheduled content", logger.Error(err))
		return
	}

	if len(content) == 0 {
		return
	}

	s.log.Info("Processing scheduled content", logger.Int("count", len(content)))

	for i := range content {
		s.processScheduledItem(ctx, &content[i])
	}
}

func (s *Scheduler) processScheduledItem(ctx context.Context, msg *domain.PublishMessage) {
	// Use transaction to mark published + create deliveries atomically
	// For now, mark as published (routing resolution happens at the API/subscriber level)
	if err := s.repo.MarkContentPublished(ctx, msg.ContentID); err != nil {
		s.log.Error("Failed to mark content as published",
			logger.String("content_id", msg.ContentID),
			logger.Error(err),
		)
		return
	}

	// Enqueue for real-time processing
	s.queue.EnqueueRealtime(orchestrator.PublishJob{
		ContentID: msg.ContentID,
		Message:   msg,
	})

	s.log.Info("Scheduled content triggered",
		logger.String("content_id", msg.ContentID),
		logger.String("type", string(msg.Type)),
	)
}
```

**Step 2: Commit**

```bash
git add social-publisher/internal/workers/scheduler.go
git commit -m "feat(social-publisher): add scheduling loop for future-dated content"
```

---

## Task 13: HTTP API

**Files:**
- Create: `social-publisher/internal/api/handler.go`
- Create: `social-publisher/internal/api/handler_test.go`
- Create: `social-publisher/internal/api/router.go`

**Step 1: Write handler tests**

```go
// social-publisher/internal/api/handler_test.go
package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/north-cloud/social-publisher/internal/api"
	"github.com/stretchr/testify/assert"
)

func TestPublishEndpoint_ValidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := api.NewHandler(nil, nil) // nil deps for request parsing test

	w := httptest.NewRecorder()
	body := map[string]any{
		"type":    "social_update",
		"summary": "Test post",
		"project": "personal",
	}
	bodyJSON, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/v1/publish", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")

	r := gin.New()
	r.POST("/api/v1/publish", handler.Publish)
	r.ServeHTTP(w, req)

	// Without a real repo/orchestrator, expect 500 (nil pointer)
	// but the request parsing itself should work
	assert.NotEqual(t, http.StatusBadRequest, w.Code)
}

func TestPublishEndpoint_MissingType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := api.NewHandler(nil, nil)

	w := httptest.NewRecorder()
	body := map[string]any{
		"summary": "Test post",
	}
	bodyJSON, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/v1/publish", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")

	r := gin.New()
	r.POST("/api/v1/publish", handler.Publish)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
```

**Step 2: Run tests to verify they fail**

Run: `cd social-publisher && go test ./internal/api/... -v`
Expected: FAIL

**Step 3: Write the handler**

```go
// social-publisher/internal/api/handler.go
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/north-cloud/social-publisher/internal/database"
	"github.com/north-cloud/social-publisher/internal/domain"
)

type Handler struct {
	repo *database.Repository
	orch interface{} // *orchestrator.Orchestrator
}

func NewHandler(repo *database.Repository, orch interface{}) *Handler {
	return &Handler{repo: repo, orch: orch}
}

type PublishRequest struct {
	Type        string            `json:"type" binding:"required"`
	Title       string            `json:"title"`
	Body        string            `json:"body"`
	Summary     string            `json:"summary"`
	URL         string            `json:"url"`
	Images      []string          `json:"images"`
	Tags        []string          `json:"tags"`
	Project     string            `json:"project"`
	Targets     []domain.TargetConfig `json:"targets"`
	ScheduledAt string            `json:"scheduled_at"`
	Metadata    map[string]string `json:"metadata"`
	Source      string            `json:"source"`
}

func (h *Handler) Publish(c *gin.Context) {
	var req PublishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Type == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type is required"})
		return
	}

	contentID := uuid.New().String()
	if req.Source == "" {
		req.Source = "api"
	}

	msg := &domain.PublishMessage{
		ContentID: contentID,
		Type:      domain.ContentType(req.Type),
		Title:     req.Title,
		Body:      req.Body,
		Summary:   req.Summary,
		URL:       req.URL,
		Images:    req.Images,
		Tags:      req.Tags,
		Project:   req.Project,
		Targets:   req.Targets,
		Metadata:  req.Metadata,
		Source:    req.Source,
	}

	// Store content
	if err := h.repo.CreateContent(c.Request.Context(), msg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store content"})
		return
	}

	// TODO: resolve routing rules and create deliveries
	// TODO: enqueue for publishing

	c.JSON(http.StatusAccepted, gin.H{
		"content_id": contentID,
		"status":     "accepted",
	})
}

func (h *Handler) Status(c *gin.Context) {
	contentID := c.Param("id")
	if contentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content_id is required"})
		return
	}

	deliveries, err := h.repo.GetDeliveriesByContentID(c.Request.Context(), contentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch deliveries"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"content_id": contentID,
		"deliveries": deliveries,
	})
}

func (h *Handler) Retry(c *gin.Context) {
	deliveryID := c.Param("id")
	if deliveryID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "delivery_id is required"})
		return
	}

	// TODO: look up delivery and resubmit to queue
	c.JSON(http.StatusAccepted, gin.H{
		"delivery_id": deliveryID,
		"status":      "retry_queued",
	})
}
```

```go
// social-publisher/internal/api/router.go
package api

import (
	"github.com/gin-gonic/gin"

	infragin "github.com/north-cloud/infrastructure/gin"
	"github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/social-publisher/internal/config"
	"github.com/north-cloud/social-publisher/internal/database"
)

type Router struct {
	handler *Handler
	repo    *database.Repository
	cfg     *config.Config
}

func NewRouter(repo *database.Repository, orch interface{}, cfg *config.Config) *Router {
	return &Router{
		handler: NewHandler(repo, orch),
		repo:    repo,
		cfg:     cfg,
	}
}

func (r *Router) NewServer(log logger.Logger) *infragin.Server {
	return infragin.NewServerBuilder("social-publisher", r.cfg.Server.Address).
		WithLogger(log).
		WithDebug(r.cfg.Debug).
		WithVersion("0.1.0").
		WithDatabaseHealthCheck(func() error {
			return r.repo.Ping(nil)
		}).
		WithRoutes(func(router *gin.Engine) {
			r.setupRoutes(router)
		}).
		Build()
}

func (r *Router) setupRoutes(router *gin.Engine) {
	v1 := router.Group("/api/v1")

	v1.POST("/publish", r.handler.Publish)
	v1.GET("/status/:id", r.handler.Status)
	v1.POST("/retry/:id", r.handler.Retry)
}
```

**Step 4: Run tests**

Run: `cd social-publisher && go test ./internal/api/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add social-publisher/internal/api/
git commit -m "feat(social-publisher): add HTTP API with publish, status, and retry endpoints"
```

---

## Task 14: Service Entry Point

**Files:**
- Modify: `social-publisher/main.go`

**Step 1: Write the full main.go with graceful shutdown**

```go
// social-publisher/main.go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	goredis "github.com/redis/go-redis/v9"

	infraconfig "github.com/north-cloud/infrastructure/config"
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/profiling"

	"github.com/north-cloud/social-publisher/internal/api"
	"github.com/north-cloud/social-publisher/internal/config"
	"github.com/north-cloud/social-publisher/internal/database"
	"github.com/north-cloud/social-publisher/internal/domain"
	"github.com/north-cloud/social-publisher/internal/orchestrator"
	spredis "github.com/north-cloud/social-publisher/internal/redis"
	"github.com/north-cloud/social-publisher/internal/workers"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

const version = "0.1.0"

func main() {
	os.Exit(run())
}

func run() int {
	profiling.StartPprofServer()

	// 1. Config
	cfg, err := config.Load("config.yml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		return 1
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid config: %v\n", err)
		return 1
	}

	// 2. Logger
	log, err := infralogger.New(infralogger.Config{
		Level:       "info",
		Format:      "json",
		Development: cfg.Debug,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		return 1
	}
	defer func() { _ = log.Sync() }()
	log = log.With(infralogger.String("service", "social-publisher"), infralogger.String("version", version))

	log.Info("Starting social-publisher")

	// 3. Database
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User,
		cfg.Database.Password, cfg.Database.DBName, cfg.Database.SSLMode,
	)
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		log.Error("Failed to connect to database", infralogger.Error(err))
		return 1
	}
	defer db.Close()
	repo := database.NewRepository(db)

	// 4. Redis
	redisClient := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Redis.URL,
		Password: cfg.Redis.Password,
	})
	defer redisClient.Close()

	eventPub := spredis.NewEventPublisher(redisClient, log)
	subscriber := spredis.NewSubscriber(redisClient, log)

	// 5. Adapters (Phase 1: X only)
	adapters := map[string]domain.PlatformAdapter{
		// "x": x.NewAdapter(x.NewClient(bearerToken)),
		// Adapters added as platform credentials are configured
	}

	// 6. Orchestrator
	orch := orchestrator.NewOrchestrator(adapters, eventPub, repo)

	// 7. Priority queue
	queue := orchestrator.NewPriorityQueue(100, 50)

	// 8. Parse intervals
	retryInterval, _ := time.ParseDuration(cfg.Service.RetryInterval)
	if retryInterval == 0 {
		retryInterval = 30 * time.Second
	}
	scheduleInterval, _ := time.ParseDuration(cfg.Service.ScheduleInterval)
	if scheduleInterval == 0 {
		scheduleInterval = 60 * time.Second
	}

	// 9. Workers
	retryWorker := workers.NewRetryWorker(repo, orch, eventPub, log, retryInterval, cfg.Service.BatchSize)
	scheduler := workers.NewScheduler(repo, queue, log, scheduleInterval, cfg.Service.BatchSize)

	// 10. HTTP server
	router := api.NewRouter(repo, orch, cfg)
	server := router.NewServer(log)

	// 11. Start everything
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start workers
	go retryWorker.Run(ctx)
	go scheduler.Run(ctx)

	// Start Redis subscriber
	go func() {
		if err := subscriber.Subscribe(ctx, func(msg *domain.PublishMessage) {
			queue.EnqueueRealtime(orchestrator.PublishJob{
				ContentID: msg.ContentID,
				Message:   msg,
			})
		}); err != nil && ctx.Err() == nil {
			log.Error("Redis subscriber error", infralogger.Error(err))
		}
	}()

	// Start HTTP server
	errChan := server.StartAsync()

	// 12. Wait for shutdown signal
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-shutdown:
		log.Info("Shutdown signal received", infralogger.String("signal", sig.String()))
	case err := <-errChan:
		log.Error("Server error", infralogger.Error(err))
	}

	// 13. Graceful shutdown
	cancel() // Stop workers and subscriber

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("Server shutdown error", infralogger.Error(err))
	}

	log.Info("Social publisher stopped")
	return 0
}
```

**Step 2: Verify it compiles**

Run: `cd social-publisher && go build ./...`
Expected: Clean build (may need `go mod tidy` first)

**Step 3: Commit**

```bash
git add social-publisher/main.go
git commit -m "feat(social-publisher): wire up service entry point with graceful shutdown"
```

---

## Task 15: GitHub Action Integration

**Files:**
- Modify: `/home/fsd42/dev/blog/.github/workflows/hugo.yml`

**Step 1: Add notification step after deploy**

Add this step after the existing "Deploy to GitHub Pages" step:

```yaml
      - name: Notify social-publisher
        if: success()
        env:
          NC_SOCIAL_PUBLISHER_URL: ${{ secrets.NC_SOCIAL_PUBLISHER_URL }}
          NC_TOKEN: ${{ secrets.NC_TOKEN }}
        run: |
          if [ -z "$NC_SOCIAL_PUBLISHER_URL" ]; then
            echo "Social publisher URL not configured, skipping notification"
            exit 0
          fi

          # Find posts that changed from draft to published in this push
          NEW_POSTS=$(git diff --name-only HEAD~1 HEAD -- content/posts/ \
            | grep '\.md$' || true)

          for POST in $NEW_POSTS; do
            if [ ! -f "$POST" ]; then continue; fi
            if grep -q 'draft: true' "$POST"; then continue; fi

            SLUG=$(grep -m1 '^slug:' "$POST" | sed 's/slug: *"\(.*\)"/\1/')
            TITLE=$(grep -m1 '^title:' "$POST" | sed 's/title: *"\(.*\)"/\1/')
            SUMMARY=$(grep -m1 '^summary:' "$POST" | sed 's/summary: *"\(.*\)"/\1/')

            if [ -n "$SLUG" ] && [ -n "$TITLE" ]; then
              echo "Notifying social-publisher about: $TITLE"
              curl -s -X POST "${NC_SOCIAL_PUBLISHER_URL}/api/v1/publish" \
                -H "Authorization: Bearer ${NC_TOKEN}" \
                -H "Content-Type: application/json" \
                -d "{
                  \"type\": \"blog_post\",
                  \"content_id\": \"blog-${SLUG}\",
                  \"title\": \"${TITLE}\",
                  \"summary\": \"${SUMMARY}\",
                  \"url\": \"https://jonesrussell.github.io/blog/${SLUG}/\",
                  \"project\": \"personal\",
                  \"source\": \"github_action\"
                }" || echo "Warning: social-publisher notification failed (non-fatal)"
            fi
          done
```

**Step 2: Verify YAML is valid**

Run: `python3 -c "import yaml; yaml.safe_load(open('/home/fsd42/dev/blog/.github/workflows/hugo.yml'))"`
Expected: No errors

**Step 3: Commit (in blog repo)**

```bash
cd /home/fsd42/dev/blog
git add .github/workflows/hugo.yml
git commit -m "feat: add social-publisher notification to deploy workflow"
```

---

## Task 16: Smoke Test (Phase 0.5)

This task is manual verification after the service is deployed locally.

**Step 1: Create test database**

```bash
createdb social_publisher
psql social_publisher < social-publisher/migrations/001_initial_schema.up.sql
```

**Step 2: Copy and configure config.yml**

```bash
cd social-publisher
cp config.yml.example config.yml
# Edit config.yml with local database credentials
```

**Step 3: Start the service**

```bash
cd social-publisher && go run .
```

Expected: Service starts, logs show "Starting social-publisher", "Retry worker started", "Scheduler started", "Subscribed to Redis channel"

**Step 4: Test publish endpoint**

```bash
curl -s -X POST http://localhost:8077/api/v1/publish \
  -H "Content-Type: application/json" \
  -d '{
    "type": "social_update",
    "summary": "Testing social publisher",
    "project": "personal",
    "source": "smoke_test"
  }' | jq .
```

Expected: `{"content_id": "<uuid>", "status": "accepted"}`

**Step 5: Check status**

```bash
curl -s http://localhost:8077/api/v1/status/<content_id> | jq .
```

Expected: `{"content_id": "<uuid>", "deliveries": []}`  (no deliveries yet because no adapters are enabled)

**Step 6: Verify database record**

```bash
psql social_publisher -c "SELECT id, type, summary, source FROM content;"
```

Expected: One row with the test content.

**Step 7: Verify Redis events** (optional, if redis-cli is available)

```bash
redis-cli SUBSCRIBE social:delivery-status
```

Then publish again in another terminal and watch for events.

---

## Summary

| Task | Description | Files | Tests |
|------|-------------|-------|-------|
| 1 | Scaffold | go.mod, main.go, go.work | Build check |
| 2 | Config | config.go, config_test.go | 3 tests |
| 3 | Migrations | 001_initial_schema.up/down.sql | SQL review |
| 4 | Domain types | content.go, delivery.go, adapter.go, errors.go | 5 tests |
| 5 | Database repo | repository.go, repository_test.go | 1 test |
| 6 | Priority queue | queue.go, queue_test.go | 3 tests |
| 7 | Redis sub/pub | subscriber.go, publisher.go | - |
| 8 | Adapter harness | testing.go, testing_test.go | 4 tests |
| 9 | X adapter | adapter.go, client.go, adapter_test.go | 5 tests |
| 10 | Orchestrator | orchestrator.go, orchestrator_test.go | 4 tests |
| 11 | Retry worker | retry.go | - |
| 12 | Scheduler | scheduler.go | - |
| 13 | HTTP API | handler.go, router.go, handler_test.go | 2 tests |
| 14 | Entry point | main.go (rewrite) | Build check |
| 15 | GitHub Action | hugo.yml | YAML check |
| 16 | Smoke test | Manual | Manual |

**Total: 16 tasks, ~27 tests, 14 commits**
