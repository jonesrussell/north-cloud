# Routing V2 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace per-source routing with topic-based, zero-config content distribution.

**Architecture:** Two-layer routing system. Layer 1: convention-based automatic topic→channel routing (no database). Layer 2: rule-based custom channels stored in PostgreSQL with JSONB rules. Wildcard ES discovery replaces the sources table.

**Tech Stack:** Go 1.25+, PostgreSQL, Elasticsearch 8, Redis, Vue 3 + TypeScript

**Design Document:** `docs/plans/2026-02-02-routing-v2-design.md`

---

## Task 1: Database Migration

**Files:**
- Create: `publisher/migrations/003_routing_v2.up.sql`
- Create: `publisher/migrations/003_routing_v2.down.sql`

**Step 1: Create the up migration**

```sql
-- publisher/migrations/003_routing_v2.up.sql
-- Migration: 003_routing_v2
-- Description: Replace per-source routing with topic-based routing

-- 1. Create cursor table for restart safety
CREATE TABLE IF NOT EXISTS publisher_cursor (
    id          INTEGER PRIMARY KEY DEFAULT 1,
    last_sort   JSONB NOT NULL DEFAULT '[]',
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

-- 2. Drop legacy tables (order matters due to foreign keys)
DROP TABLE IF EXISTS routes CASCADE;
DROP TABLE IF EXISTS sources CASCADE;

-- 3. Drop existing channels table and recreate with new schema
DROP TABLE IF EXISTS channels CASCADE;

CREATE TABLE channels (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            VARCHAR(255) NOT NULL,
    slug            VARCHAR(255) NOT NULL UNIQUE,
    redis_channel   VARCHAR(255) NOT NULL UNIQUE,
    description     TEXT,
    rules           JSONB NOT NULL DEFAULT '{}',
    rules_version   INTEGER NOT NULL DEFAULT 1,
    enabled         BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- 4. Create indexes
CREATE INDEX idx_channels_enabled ON channels(enabled) WHERE enabled = true;
CREATE INDEX idx_channels_slug ON channels(slug);

-- 5. Create trigger for updated_at
CREATE TRIGGER update_channels_updated_at
    BEFORE UPDATE ON channels
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 6. Seed initial channel for StreetCode
INSERT INTO channels (name, slug, redis_channel, description, rules) VALUES (
    'StreetCode Crime Feed',
    'streetcode_crime_feed',
    'streetcode:crime_feed',
    'Aggregated crime content for StreetCode',
    '{
        "include_topics": ["violent_crime", "property_crime", "drug_crime", "organized_crime", "criminal_justice"],
        "exclude_topics": [],
        "min_quality_score": 50,
        "content_types": ["article"]
    }'::jsonb
);
```

**Step 2: Create the down migration**

```sql
-- publisher/migrations/003_routing_v2.down.sql
-- Rollback: 003_routing_v2

-- Drop new tables
DROP TABLE IF EXISTS publisher_cursor CASCADE;
DROP TABLE IF EXISTS channels CASCADE;

-- Recreate original schema (from 001_initial_schema.up.sql)
CREATE TABLE sources (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL UNIQUE,
    index_pattern VARCHAR(255) NOT NULL,
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE channels (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE routes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_id UUID NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    channel_id UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    min_quality_score INT DEFAULT 50,
    topics TEXT[],
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(source_id, channel_id)
);

-- Recreate indexes
CREATE INDEX idx_sources_enabled ON sources(enabled);
CREATE INDEX idx_sources_name ON sources(name);
CREATE INDEX idx_channels_enabled ON channels(enabled);
CREATE INDEX idx_channels_name ON channels(name);
CREATE INDEX idx_routes_source ON routes(source_id);
CREATE INDEX idx_routes_channel ON routes(channel_id);
CREATE INDEX idx_routes_enabled ON routes(enabled);

-- Recreate triggers
CREATE TRIGGER update_sources_updated_at BEFORE UPDATE ON sources
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_channels_updated_at BEFORE UPDATE ON channels
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_routes_updated_at BEFORE UPDATE ON routes
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

**Step 3: Run and verify migration**

```bash
cd publisher && go run cmd/migrate/main.go up
```

Expected: Migration applies successfully.

**Step 4: Verify schema**

```bash
docker exec north-cloud-postgres-publisher-1 psql -U postgres -d publisher -c "\dt"
```

Expected: Shows `channels`, `publisher_cursor`, `publish_history` tables (no `sources`, `routes`).

**Step 5: Commit**

```bash
git add publisher/migrations/003_routing_v2.up.sql publisher/migrations/003_routing_v2.down.sql
git commit -m "feat(publisher): add routing v2 database migration

- Drop sources and routes tables (replaced by wildcard discovery)
- Add publisher_cursor table for restart safety
- Recreate channels table with rules JSONB, slug, redis_channel
- Seed initial StreetCode crime feed channel"
```

---

## Task 2: Channel Model and Rules

**Files:**
- Modify: `publisher/internal/models/channel.go`
- Create: `publisher/internal/models/rules.go`
- Delete: `publisher/internal/models/source.go`
- Delete: `publisher/internal/models/route.go`

**Step 1: Create the Rules model**

```go
// publisher/internal/models/rules.go
package models

// Rules defines the filtering rules for a custom channel
type Rules struct {
    IncludeTopics   []string `json:"include_topics"`
    ExcludeTopics   []string `json:"exclude_topics"`
    MinQualityScore int      `json:"min_quality_score"`
    ContentTypes    []string `json:"content_types"`
}

// IsEmpty returns true if no rules are defined (matches everything)
func (r *Rules) IsEmpty() bool {
    return len(r.IncludeTopics) == 0 &&
        len(r.ExcludeTopics) == 0 &&
        r.MinQualityScore == 0 &&
        len(r.ContentTypes) == 0
}

// Matches checks if an article matches the rules
func (r *Rules) Matches(qualityScore int, contentType string, topics []string) bool {
    // Fast path: empty rules match everything
    if r.IsEmpty() {
        return true
    }

    // Quality check
    if r.MinQualityScore > 0 && qualityScore < r.MinQualityScore {
        return false
    }

    // Content type check
    if len(r.ContentTypes) > 0 && !contains(r.ContentTypes, contentType) {
        return false
    }

    // Exclude topics check
    if hasAny(topics, r.ExcludeTopics) {
        return false
    }

    // Include topics check (empty = match all)
    if len(r.IncludeTopics) > 0 && !hasAny(topics, r.IncludeTopics) {
        return false
    }

    return true
}

// contains checks if slice contains value
func contains(slice []string, value string) bool {
    for _, v := range slice {
        if v == value {
            return true
        }
    }
    return false
}

// hasAny checks if any value from needles exists in haystack
func hasAny(haystack, needles []string) bool {
    for _, needle := range needles {
        if contains(haystack, needle) {
            return true
        }
    }
    return false
}
```

**Step 2: Update the Channel model**

Replace `publisher/internal/models/channel.go`:

```go
package models

import (
    "encoding/json"
    "time"

    "github.com/google/uuid"
)

// Channel represents a custom routing channel with embedded rules
type Channel struct {
    ID           uuid.UUID `db:"id"            json:"id"`
    Name         string    `db:"name"          json:"name"`
    Slug         string    `db:"slug"          json:"slug"`
    RedisChannel string    `db:"redis_channel" json:"redis_channel"`
    Description  string    `db:"description"   json:"description"`
    Rules        Rules     `db:"-"             json:"rules"`
    RulesJSON    []byte    `db:"rules"         json:"-"`
    RulesVersion int       `db:"rules_version" json:"rules_version"`
    Enabled      bool      `db:"enabled"       json:"enabled"`
    CreatedAt    time.Time `db:"created_at"    json:"created_at"`
    UpdatedAt    time.Time `db:"updated_at"    json:"updated_at"`
}

// ParseRules parses RulesJSON into Rules struct
func (c *Channel) ParseRules() error {
    if len(c.RulesJSON) == 0 {
        c.Rules = Rules{}
        return nil
    }
    return json.Unmarshal(c.RulesJSON, &c.Rules)
}

// ChannelCreateRequest represents the request payload for creating a channel
type ChannelCreateRequest struct {
    Name         string `json:"name"          binding:"required,min=1,max=255"`
    Slug         string `json:"slug"          binding:"required,min=1,max=255"`
    RedisChannel string `json:"redis_channel" binding:"required,min=1,max=255"`
    Description  string `json:"description"   binding:"max=1000"`
    Rules        *Rules `json:"rules"`
    Enabled      *bool  `json:"enabled"`
}

// ChannelUpdateRequest represents the request payload for updating a channel
type ChannelUpdateRequest struct {
    Name         *string `json:"name"          binding:"omitempty,min=1,max=255"`
    Slug         *string `json:"slug"          binding:"omitempty,min=1,max=255"`
    RedisChannel *string `json:"redis_channel" binding:"omitempty,min=1,max=255"`
    Description  *string `json:"description"   binding:"omitempty,max=1000"`
    Rules        *Rules  `json:"rules"`
    Enabled      *bool   `json:"enabled"`
}

// Validate validates the channel create request
func (r *ChannelCreateRequest) Validate() error {
    return nil
}

// Validate validates the channel update request
func (r *ChannelUpdateRequest) Validate() error {
    if r.Name == nil && r.Slug == nil && r.RedisChannel == nil &&
        r.Description == nil && r.Rules == nil && r.Enabled == nil {
        return ErrNoFieldsToUpdate
    }
    return nil
}
```

**Step 3: Delete source.go and route.go**

```bash
rm publisher/internal/models/source.go
rm publisher/internal/models/route.go
```

**Step 4: Run tests**

```bash
cd publisher && go test ./internal/models/...
```

Expected: Tests pass (or fail for deleted models - we'll fix in next task).

**Step 5: Commit**

```bash
git add -A publisher/internal/models/
git commit -m "feat(publisher): add Rules model and update Channel model

- Add Rules struct with Matches() logic for filtering
- Update Channel to include slug, redis_channel, rules JSONB
- Remove Source and Route models (replaced by wildcard discovery)"
```

---

## Task 3: Cursor Repository

**Files:**
- Create: `publisher/internal/database/cursor_repository.go`
- Create: `publisher/internal/database/cursor_repository_test.go`

**Step 1: Write the failing test**

```go
// publisher/internal/database/cursor_repository_test.go
package database

import (
    "context"
    "encoding/json"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestCursorRepository_GetAndUpdate(t *testing.T) {
    t.Helper()
    // Skip if no test database
    db := setupTestDB(t)
    repo := NewRepository(db)
    ctx := context.Background()

    // Initially should return empty cursor
    cursor, err := repo.GetCursor(ctx)
    require.NoError(t, err)
    assert.Empty(t, cursor)

    // Update cursor
    newCursor := []any{"2026-02-02T12:00:00Z", "doc123"}
    err = repo.UpdateCursor(ctx, newCursor)
    require.NoError(t, err)

    // Get should return updated value
    cursor, err = repo.GetCursor(ctx)
    require.NoError(t, err)
    assert.Len(t, cursor, 2)
}
```

**Step 2: Run test to verify it fails**

```bash
cd publisher && go test ./internal/database/... -run TestCursorRepository -v
```

Expected: FAIL with "GetCursor not defined" or similar.

**Step 3: Write the implementation**

```go
// publisher/internal/database/cursor_repository.go
package database

import (
    "context"
    "database/sql"
    "encoding/json"
    "errors"
    "fmt"
    "time"
)

// GetCursor retrieves the current polling cursor
func (r *Repository) GetCursor(ctx context.Context) ([]any, error) {
    var cursorJSON []byte
    query := `SELECT last_sort FROM publisher_cursor WHERE id = 1`

    err := r.db.GetContext(ctx, &cursorJSON, query)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return []any{}, nil
        }
        return nil, fmt.Errorf("failed to get cursor: %w", err)
    }

    var cursor []any
    if err := json.Unmarshal(cursorJSON, &cursor); err != nil {
        return nil, fmt.Errorf("failed to unmarshal cursor: %w", err)
    }

    return cursor, nil
}

// UpdateCursor updates the polling cursor
func (r *Repository) UpdateCursor(ctx context.Context, cursor []any) error {
    cursorJSON, err := json.Marshal(cursor)
    if err != nil {
        return fmt.Errorf("failed to marshal cursor: %w", err)
    }

    query := `
        INSERT INTO publisher_cursor (id, last_sort, updated_at)
        VALUES (1, $1, $2)
        ON CONFLICT (id) DO UPDATE SET
            last_sort = EXCLUDED.last_sort,
            updated_at = EXCLUDED.updated_at
    `

    _, err = r.db.ExecContext(ctx, query, cursorJSON, time.Now())
    if err != nil {
        return fmt.Errorf("failed to update cursor: %w", err)
    }

    return nil
}
```

**Step 4: Run test to verify it passes**

```bash
cd publisher && go test ./internal/database/... -run TestCursorRepository -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add publisher/internal/database/cursor_repository.go publisher/internal/database/cursor_repository_test.go
git commit -m "feat(publisher): add cursor repository for restart safety

- GetCursor returns last polling position
- UpdateCursor persists position with upsert"
```

---

## Task 4: Channel Repository Update

**Files:**
- Modify: `publisher/internal/database/repository.go`

**Step 1: Write the failing test**

```go
// Add to publisher/internal/database/repository_test.go
func TestChannelRepository_CreateWithRules(t *testing.T) {
    t.Helper()
    db := setupTestDB(t)
    repo := NewRepository(db)
    ctx := context.Background()

    rules := &models.Rules{
        IncludeTopics:   []string{"crime", "news"},
        MinQualityScore: 60,
        ContentTypes:    []string{"article"},
    }

    req := &models.ChannelCreateRequest{
        Name:         "Test Channel",
        Slug:         "test_channel",
        RedisChannel: "test:channel",
        Description:  "A test channel",
        Rules:        rules,
    }

    channel, err := repo.CreateChannel(ctx, req)
    require.NoError(t, err)
    assert.Equal(t, "test_channel", channel.Slug)
    assert.Equal(t, "test:channel", channel.RedisChannel)
    assert.Equal(t, 60, channel.Rules.MinQualityScore)
}

func TestChannelRepository_ListEnabledWithRules(t *testing.T) {
    t.Helper()
    db := setupTestDB(t)
    repo := NewRepository(db)
    ctx := context.Background()

    channels, err := repo.ListEnabledChannelsWithRules(ctx)
    require.NoError(t, err)
    // Should have seeded channel
    assert.GreaterOrEqual(t, len(channels), 1)
    // Rules should be parsed
    for _, ch := range channels {
        assert.NotNil(t, ch.Rules)
    }
}
```

**Step 2: Run test to verify it fails**

```bash
cd publisher && go test ./internal/database/... -run TestChannelRepository -v
```

Expected: FAIL

**Step 3: Update repository.go - Replace channel methods**

Remove all source-related methods and update channel methods in `repository.go`:

```go
// CreateChannel creates a new channel with rules
func (r *Repository) CreateChannel(ctx context.Context, req *models.ChannelCreateRequest) (*models.Channel, error) {
    rulesJSON := []byte("{}")
    if req.Rules != nil {
        var err error
        rulesJSON, err = json.Marshal(req.Rules)
        if err != nil {
            return nil, fmt.Errorf("failed to marshal rules: %w", err)
        }
    }

    channel := &models.Channel{
        ID:           uuid.New(),
        Name:         req.Name,
        Slug:         req.Slug,
        RedisChannel: req.RedisChannel,
        Description:  req.Description,
        RulesJSON:    rulesJSON,
        RulesVersion: 1,
        Enabled:      true,
        CreatedAt:    time.Now(),
        UpdatedAt:    time.Now(),
    }

    if req.Enabled != nil {
        channel.Enabled = *req.Enabled
    }

    query := `
        INSERT INTO channels (id, name, slug, redis_channel, description, rules, rules_version, enabled, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
        RETURNING id, name, slug, redis_channel, description, rules, rules_version, enabled, created_at, updated_at
    `

    err := r.db.QueryRowxContext(
        ctx, query,
        channel.ID, channel.Name, channel.Slug, channel.RedisChannel,
        channel.Description, channel.RulesJSON, channel.RulesVersion,
        channel.Enabled, channel.CreatedAt, channel.UpdatedAt,
    ).StructScan(channel)

    if err != nil {
        var pqErr *pq.Error
        if errors.As(err, &pqErr) && pqErr.Code == "23505" {
            return nil, models.ErrAlreadyExists
        }
        return nil, fmt.Errorf("failed to create channel: %w", err)
    }

    if parseErr := channel.ParseRules(); parseErr != nil {
        return nil, fmt.Errorf("failed to parse rules: %w", parseErr)
    }

    return channel, nil
}

// ListEnabledChannelsWithRules returns all enabled channels with parsed rules
func (r *Repository) ListEnabledChannelsWithRules(ctx context.Context) ([]models.Channel, error) {
    channels := []models.Channel{}
    query := `
        SELECT id, name, slug, redis_channel, description, rules, rules_version, enabled, created_at, updated_at
        FROM channels
        WHERE enabled = true
        ORDER BY name ASC
    `

    err := r.db.SelectContext(ctx, &channels, query)
    if err != nil {
        return nil, fmt.Errorf("failed to list channels: %w", err)
    }

    // Parse rules for each channel
    for i := range channels {
        if parseErr := channels[i].ParseRules(); parseErr != nil {
            return nil, fmt.Errorf("failed to parse rules for channel %s: %w", channels[i].Slug, parseErr)
        }
    }

    return channels, nil
}
```

**Step 4: Run test to verify it passes**

```bash
cd publisher && go test ./internal/database/... -run TestChannelRepository -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add publisher/internal/database/repository.go
git commit -m "feat(publisher): update channel repository for routing v2

- Remove source CRUD methods
- Add CreateChannel with rules JSONB support
- Add ListEnabledChannelsWithRules with automatic parsing"
```

---

## Task 5: Remove Routes Repository

**Files:**
- Delete: `publisher/internal/database/repository_routes.go`
- Modify: `publisher/internal/database/repository.go` (remove route imports if any)

**Step 1: Delete routes repository**

```bash
rm publisher/internal/database/repository_routes.go
```

**Step 2: Verify build**

```bash
cd publisher && go build ./...
```

Expected: Build succeeds (or shows which files need updating).

**Step 3: Fix any remaining references**

Search for and remove any imports or references to routes in the repository:

```bash
grep -r "Route" publisher/internal/database/
```

Fix any issues found.

**Step 4: Run all database tests**

```bash
cd publisher && go test ./internal/database/...
```

Expected: All tests pass.

**Step 5: Commit**

```bash
git add -A publisher/internal/database/
git commit -m "refactor(publisher): remove routes repository

Routes are replaced by channel rules in routing v2"
```

---

## Task 6: Index Discovery Service

**Files:**
- Create: `publisher/internal/discovery/discovery.go`
- Create: `publisher/internal/discovery/discovery_test.go`

**Step 1: Write the failing test**

```go
// publisher/internal/discovery/discovery_test.go
package discovery

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestDiscovery_ParseIndexResponse(t *testing.T) {
    t.Helper()

    // Test parsing ES cat indices response
    response := []map[string]string{
        {"index": "www_sudbury_com_classified_content"},
        {"index": "www_baytoday_ca_classified_content"},
        {"index": ".kibana_1"}, // Should be filtered out
    }

    indexes := filterClassifiedIndexes(response)

    require.Len(t, indexes, 2)
    assert.Contains(t, indexes, "www_sudbury_com_classified_content")
    assert.Contains(t, indexes, "www_baytoday_ca_classified_content")
}
```

**Step 2: Run test to verify it fails**

```bash
cd publisher && go test ./internal/discovery/... -v
```

Expected: FAIL

**Step 3: Write the implementation**

```go
// publisher/internal/discovery/discovery.go
package discovery

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"
    "time"

    "github.com/elastic/go-elasticsearch/v8"
    infralogger "github.com/north-cloud/infrastructure/logger"
)

const classifiedContentSuffix = "_classified_content"

// Service handles Elasticsearch index discovery
type Service struct {
    esClient *elasticsearch.Client
    logger   infralogger.Logger
    indexes  []string
    lastSync time.Time
}

// NewService creates a new discovery service
func NewService(esClient *elasticsearch.Client, logger infralogger.Logger) *Service {
    return &Service{
        esClient: esClient,
        logger:   logger,
        indexes:  []string{},
    }
}

// DiscoverIndexes fetches all classified content indexes from Elasticsearch
func (s *Service) DiscoverIndexes(ctx context.Context) ([]string, error) {
    res, err := s.esClient.Cat.Indices(
        s.esClient.Cat.Indices.WithContext(ctx),
        s.esClient.Cat.Indices.WithIndex("*"+classifiedContentSuffix),
        s.esClient.Cat.Indices.WithFormat("json"),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to discover indexes: %w", err)
    }
    defer res.Body.Close()

    if res.IsError() {
        return nil, fmt.Errorf("elasticsearch error: %s", res.String())
    }

    var response []map[string]string
    if decodeErr := json.NewDecoder(res.Body).Decode(&response); decodeErr != nil {
        return nil, fmt.Errorf("failed to decode response: %w", decodeErr)
    }

    indexes := filterClassifiedIndexes(response)
    s.indexes = indexes
    s.lastSync = time.Now()

    s.logger.Info("Discovered classified content indexes",
        infralogger.Int("count", len(indexes)),
    )

    return indexes, nil
}

// GetIndexes returns the cached list of indexes
func (s *Service) GetIndexes() []string {
    return s.indexes
}

// filterClassifiedIndexes extracts classified content index names
func filterClassifiedIndexes(response []map[string]string) []string {
    indexes := make([]string, 0, len(response))
    for _, item := range response {
        indexName, ok := item["index"]
        if !ok {
            continue
        }
        // Skip system indexes (start with .)
        if strings.HasPrefix(indexName, ".") {
            continue
        }
        // Only include classified content indexes
        if strings.HasSuffix(indexName, classifiedContentSuffix) {
            indexes = append(indexes, indexName)
        }
    }
    return indexes
}
```

**Step 4: Run test to verify it passes**

```bash
cd publisher && go test ./internal/discovery/... -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add publisher/internal/discovery/
git commit -m "feat(publisher): add index discovery service

- DiscoverIndexes queries ES for *_classified_content indexes
- Filters out system indexes
- Caches discovered indexes for reuse"
```

---

## Task 7: Router Service Rewrite

**Files:**
- Rewrite: `publisher/internal/router/service.go`
- Create: `publisher/internal/router/service_test.go`

**Step 1: Write failing tests for new router**

```go
// publisher/internal/router/service_test.go
package router

import (
    "testing"

    "github.com/jonesrussell/north-cloud/publisher/internal/models"
    "github.com/stretchr/testify/assert"
)

func TestRouter_Layer1_TopicToChannel(t *testing.T) {
    t.Helper()

    article := &Article{
        Topics: []string{"violent_crime", "local_news"},
    }

    channels := generateLayer1Channels(article)

    assert.Len(t, channels, 2)
    assert.Contains(t, channels, "articles:violent_crime")
    assert.Contains(t, channels, "articles:local_news")
}

func TestRouter_Layer2_RulesMatch(t *testing.T) {
    t.Helper()

    article := &Article{
        QualityScore: 75,
        ContentType:  "article",
        Topics:       []string{"violent_crime", "local_news"},
    }

    rules := models.Rules{
        IncludeTopics:   []string{"violent_crime", "property_crime"},
        ExcludeTopics:   []string{"criminal_justice"},
        MinQualityScore: 60,
        ContentTypes:    []string{"article"},
    }

    assert.True(t, rules.Matches(article.QualityScore, article.ContentType, article.Topics))
}

func TestRouter_Layer2_RulesNoMatch_Quality(t *testing.T) {
    t.Helper()

    article := &Article{
        QualityScore: 40,
        ContentType:  "article",
        Topics:       []string{"violent_crime"},
    }

    rules := models.Rules{
        MinQualityScore: 60,
    }

    assert.False(t, rules.Matches(article.QualityScore, article.ContentType, article.Topics))
}

func TestRouter_Layer2_RulesNoMatch_ExcludedTopic(t *testing.T) {
    t.Helper()

    article := &Article{
        QualityScore: 75,
        ContentType:  "article",
        Topics:       []string{"criminal_justice", "violent_crime"},
    }

    rules := models.Rules{
        IncludeTopics: []string{"violent_crime"},
        ExcludeTopics: []string{"criminal_justice"},
    }

    assert.False(t, rules.Matches(article.QualityScore, article.ContentType, article.Topics))
}
```

**Step 2: Run tests to verify they fail**

```bash
cd publisher && go test ./internal/router/... -v
```

Expected: FAIL

**Step 3: Rewrite router service**

Replace `publisher/internal/router/service.go` with the new implementation. This is a large file - key sections:

```go
// publisher/internal/router/service.go
package router

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "time"

    "github.com/elastic/go-elasticsearch/v8"
    "github.com/jonesrussell/north-cloud/publisher/internal/database"
    "github.com/jonesrussell/north-cloud/publisher/internal/discovery"
    "github.com/jonesrussell/north-cloud/publisher/internal/models"
    infralogger "github.com/north-cloud/infrastructure/logger"
    "github.com/redis/go-redis/v9"
)

// Config holds router service configuration
type Config struct {
    PollInterval      time.Duration
    DiscoveryInterval time.Duration
    BatchSize         int
}

// Service handles routing articles to Redis channels
type Service struct {
    repo        *database.Repository
    discovery   *discovery.Service
    esClient    *elasticsearch.Client
    redisClient *redis.Client
    logger      infralogger.Logger
    config      Config
    lastSort    []any
}

// NewService creates a new router service
func NewService(
    repo *database.Repository,
    disc *discovery.Service,
    esClient *elasticsearch.Client,
    redisClient *redis.Client,
    cfg Config,
    logger infralogger.Logger,
) *Service {
    // Apply defaults
    if cfg.PollInterval == 0 {
        cfg.PollInterval = 30 * time.Second
    }
    if cfg.DiscoveryInterval == 0 {
        cfg.DiscoveryInterval = 5 * time.Minute
    }
    if cfg.BatchSize == 0 {
        cfg.BatchSize = 100
    }

    return &Service{
        repo:        repo,
        discovery:   disc,
        esClient:    esClient,
        redisClient: redisClient,
        logger:      logger,
        config:      cfg,
        lastSort:    []any{},
    }
}

// Start begins the router service loop
func (s *Service) Start(ctx context.Context) error {
    s.logger.Info("Router service starting (routing v2)...")

    // Load cursor from database
    cursor, err := s.repo.GetCursor(ctx)
    if err != nil {
        s.logger.Warn("Failed to load cursor, starting fresh", infralogger.Error(err))
    } else {
        s.lastSort = cursor
    }

    // Initial discovery
    if _, err := s.discovery.DiscoverIndexes(ctx); err != nil {
        s.logger.Error("Initial index discovery failed", infralogger.Error(err))
    }

    discoveryTicker := time.NewTicker(s.config.DiscoveryInterval)
    pollTicker := time.NewTicker(s.config.PollInterval)
    defer discoveryTicker.Stop()
    defer pollTicker.Stop()

    // Run immediately
    s.pollAndRoute(ctx)

    for {
        select {
        case <-ctx.Done():
            s.logger.Info("Router service stopping...")
            return ctx.Err()

        case <-discoveryTicker.C:
            if _, err := s.discovery.DiscoverIndexes(ctx); err != nil {
                s.logger.Error("Index discovery failed", infralogger.Error(err))
            }

        case <-pollTicker.C:
            s.pollAndRoute(ctx)
        }
    }
}

// pollAndRoute fetches new articles and routes them
func (s *Service) pollAndRoute(ctx context.Context) {
    indexes := s.discovery.GetIndexes()
    if len(indexes) == 0 {
        s.logger.Debug("No indexes discovered, skipping poll")
        return
    }

    // Load custom channels (Layer 2)
    channels, err := s.repo.ListEnabledChannelsWithRules(ctx)
    if err != nil {
        s.logger.Error("Failed to load channels", infralogger.Error(err))
    }

    // Loop until we've drained the queue
    for {
        articles, err := s.fetchArticles(ctx, indexes)
        if err != nil {
            s.logger.Error("Failed to fetch articles", infralogger.Error(err))
            return
        }

        if len(articles) == 0 {
            return
        }

        s.logger.Debug("Processing articles",
            infralogger.Int("count", len(articles)),
        )

        for i := range articles {
            s.routeArticle(ctx, &articles[i], channels)
        }

        // Update cursor
        lastArticle := articles[len(articles)-1]
        s.lastSort = lastArticle.Sort
        if persistErr := s.repo.UpdateCursor(ctx, s.lastSort); persistErr != nil {
            s.logger.Error("Failed to persist cursor", infralogger.Error(persistErr))
        }

        // If we got less than batch size, we're done
        if len(articles) < s.config.BatchSize {
            return
        }
    }
}

// routeArticle routes a single article to Layer 1 and Layer 2 channels
func (s *Service) routeArticle(ctx context.Context, article *Article, channels []models.Channel) {
    // Layer 1: Automatic topic channels
    for _, topic := range article.Topics {
        channel := fmt.Sprintf("articles:%s", topic)
        s.publishToChannel(ctx, article, channel, nil)
    }

    // Layer 2: Custom channels
    for i := range channels {
        ch := &channels[i]
        if ch.Rules.Matches(article.QualityScore, article.ContentType, article.Topics) {
            s.publishToChannel(ctx, article, ch.RedisChannel, &ch.ID)
        }
    }
}

// generateLayer1Channels returns topic-based channel names for an article
func generateLayer1Channels(article *Article) []string {
    channels := make([]string, len(article.Topics))
    for i, topic := range article.Topics {
        channels[i] = fmt.Sprintf("articles:%s", topic)
    }
    return channels
}

// ... (fetchArticles and publishToChannel methods similar to current implementation
// but using search_after instead of time-based filtering)
```

**Step 4: Run tests**

```bash
cd publisher && go test ./internal/router/... -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add publisher/internal/router/
git commit -m "feat(publisher): rewrite router for routing v2

Layer 1: Convention-based topic→channel routing
Layer 2: Rule-based custom channels with filtering
- Uses search_after for reliable incremental polling
- Persists cursor for restart safety
- Wildcard index discovery"
```

---

## Task 8: Remove Sources/Routes API Handlers

**Files:**
- Delete: `publisher/internal/api/sources_handler.go`
- Delete: `publisher/internal/api/routes_handler.go`
- Modify: `publisher/internal/api/router.go` (remove routes)

**Step 1: Delete handler files**

```bash
rm publisher/internal/api/sources_handler.go
rm publisher/internal/api/routes_handler.go
```

**Step 2: Update API router**

Modify `publisher/internal/api/router.go` to remove source and route endpoints:

```go
// Remove these route groups:
// sources := api.Group("/sources")
// routes := api.Group("/routes")

// Keep channels group with updated handlers
```

**Step 3: Verify build**

```bash
cd publisher && go build ./...
```

**Step 4: Commit**

```bash
git add -A publisher/internal/api/
git commit -m "refactor(publisher): remove sources and routes API handlers

Replaced by wildcard discovery (sources) and channel rules (routes)"
```

---

## Task 9: Update Channels API Handler

**Files:**
- Modify: `publisher/internal/api/channels_handler.go`

**Step 1: Update channel handlers for new schema**

The handlers need to:
- Accept rules in create/update requests
- Return slug and redis_channel fields
- Add preview endpoint

```go
// Add to channels_handler.go

// PreviewChannel previews articles matching channel rules
func (h *Handler) PreviewChannel(c *gin.Context) {
    id, err := uuid.Parse(c.Param("id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid channel ID"})
        return
    }

    channel, err := h.repo.GetChannelByID(c.Request.Context(), id)
    if err != nil {
        if errors.Is(err, models.ErrNotFound) {
            c.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    // Query ES for matching articles
    articles, err := h.previewArticles(c.Request.Context(), &channel.Rules)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "channel":         channel,
        "matching_count":  len(articles),
        "sample_articles": articles,
    })
}
```

**Step 2: Add route in router.go**

```go
channels.GET("/:id/preview", h.PreviewChannel)
```

**Step 3: Run API tests**

```bash
cd publisher && go test ./internal/api/... -v
```

**Step 4: Commit**

```bash
git add publisher/internal/api/
git commit -m "feat(publisher): update channels API for routing v2

- Accept rules JSON in create/update
- Add preview endpoint for testing rules
- Return slug and redis_channel fields"
```

---

## Task 10: Add Topics and Indexes API Endpoints

**Files:**
- Modify: `publisher/internal/api/handlers.go`
- Modify: `publisher/internal/api/router.go`

**Step 1: Add topics endpoint**

```go
// GetTopics returns all unique topics from classified content
func (h *Handler) GetTopics(c *gin.Context) {
    ctx := c.Request.Context()

    // Query ES for topic aggregation across all classified indexes
    query := map[string]any{
        "size": 0,
        "aggs": map[string]any{
            "topics": map[string]any{
                "terms": map[string]any{
                    "field": "topics",
                    "size":  1000,
                },
            },
        },
    }

    queryJSON, _ := json.Marshal(query)

    res, err := h.esClient.Search(
        h.esClient.Search.WithContext(ctx),
        h.esClient.Search.WithIndex("*_classified_content"),
        h.esClient.Search.WithBody(bytes.NewReader(queryJSON)),
    )
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer res.Body.Close()

    // Parse aggregation response
    var result struct {
        Aggregations struct {
            Topics struct {
                Buckets []struct {
                    Key      string `json:"key"`
                    DocCount int    `json:"doc_count"`
                } `json:"buckets"`
            } `json:"topics"`
        } `json:"aggregations"`
    }

    if decodeErr := json.NewDecoder(res.Body).Decode(&result); decodeErr != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": decodeErr.Error()})
        return
    }

    topics := make([]map[string]any, len(result.Aggregations.Topics.Buckets))
    for i, bucket := range result.Aggregations.Topics.Buckets {
        topics[i] = map[string]any{
            "topic": bucket.Key,
            "count": bucket.DocCount,
        }
    }

    c.JSON(http.StatusOK, gin.H{"topics": topics})
}

// GetIndexes returns discovered classified content indexes
func (h *Handler) GetIndexes(c *gin.Context) {
    indexes := h.discovery.GetIndexes()
    c.JSON(http.StatusOK, gin.H{
        "indexes": indexes,
        "count":   len(indexes),
    })
}
```

**Step 2: Add routes**

```go
api.GET("/topics", h.GetTopics)
api.GET("/indexes", h.GetIndexes)
```

**Step 3: Test endpoints**

```bash
curl http://localhost:8070/api/v1/topics
curl http://localhost:8070/api/v1/indexes
```

**Step 4: Commit**

```bash
git add publisher/internal/api/
git commit -m "feat(publisher): add topics and indexes API endpoints

- GET /topics returns all topics from ES aggregation
- GET /indexes returns discovered classified content indexes"
```

---

## Task 11: Dashboard - Update TypeScript Types

**Files:**
- Modify: `dashboard/src/types/publisher.ts`

**Step 1: Update types**

```typescript
// dashboard/src/types/publisher.ts

// Remove Source and Route types (no longer used by publisher)
// Keep Channel type but update it

export interface ChannelRules {
  include_topics: string[]
  exclude_topics: string[]
  min_quality_score: number
  content_types: string[]
}

export interface Channel {
  id: string
  name: string
  slug: string
  redis_channel: string
  description?: string
  rules: ChannelRules
  rules_version: number
  enabled: boolean
  created_at: string
  updated_at?: string
}

export interface CreateChannelRequest {
  name: string
  slug: string
  redis_channel: string
  description?: string
  rules?: ChannelRules
  enabled?: boolean
}

export interface UpdateChannelRequest {
  name?: string
  slug?: string
  redis_channel?: string
  description?: string
  rules?: ChannelRules
  enabled?: boolean
}

export interface TopicInfo {
  topic: string
  count: number
}

export interface TopicsResponse {
  topics: TopicInfo[]
}

export interface IndexesResponse {
  indexes: string[]
  count: number
}

export interface ChannelPreviewResponse {
  channel: Channel
  matching_count: number
  sample_articles: PreviewArticle[]
}
```

**Step 2: Commit**

```bash
git add dashboard/src/types/publisher.ts
git commit -m "feat(dashboard): update publisher types for routing v2

- Remove Source and Route types
- Add ChannelRules interface
- Update Channel with slug, redis_channel, rules
- Add Topics and Indexes response types"
```

---

## Task 12: Dashboard - Update API Client

**Files:**
- Modify: `dashboard/src/api/client.ts`

**Step 1: Update publisherApi**

Remove sources and routes, update channels, add topics/indexes:

```typescript
// Publisher API
export const publisherApi = {
  // ... keep health, stats, articles, history

  // Channels CRUD (updated)
  channels: {
    list: (enabledOnly = false): Promise<AxiosResponse<ChannelsListResponse>> =>
      publisherClient.get(`/channels${enabledOnly ? '?enabled_only=true' : ''}`),
    get: (id: string): Promise<AxiosResponse<{ channel: Channel }>> =>
      publisherClient.get(`/channels/${id}`),
    create: (data: CreateChannelRequest): Promise<AxiosResponse<{ channel: Channel }>> =>
      publisherClient.post('/channels', data),
    update: (id: string, data: UpdateChannelRequest): Promise<AxiosResponse<{ channel: Channel }>> =>
      publisherClient.put(`/channels/${id}`, data),
    delete: (id: string): Promise<AxiosResponse<void>> =>
      publisherClient.delete(`/channels/${id}`),
    preview: (id: string): Promise<AxiosResponse<ChannelPreviewResponse>> =>
      publisherClient.get(`/channels/${id}/preview`),
  },

  // New endpoints
  topics: {
    list: (): Promise<AxiosResponse<TopicsResponse>> =>
      publisherClient.get('/topics'),
  },

  indexes: {
    list: (): Promise<AxiosResponse<IndexesResponse>> =>
      publisherClient.get('/indexes'),
  },

  // REMOVED: sources, routes
}
```

**Step 2: Commit**

```bash
git add dashboard/src/api/client.ts
git commit -m "feat(dashboard): update API client for routing v2

- Remove sources and routes API calls
- Update channels API with preview endpoint
- Add topics and indexes endpoints"
```

---

## Task 13: Dashboard - Create ChannelsView

**Files:**
- Create: `dashboard/src/views/distribution/ChannelsView.vue` (rewrite from RoutesView)

**Step 1: Create new ChannelsView**

This is a significant rewrite. Key changes:
- Remove source dropdown
- Add slug and redis_channel inputs
- Add rules editor with include/exclude topics, quality score, content types
- Add live preview panel

```vue
<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { Loader2, Hash, Plus, Pencil, Trash2, X, Eye } from 'lucide-vue-next'
import { publisherApi } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import type {
  Channel,
  ChannelRules,
  CreateChannelRequest,
  TopicInfo,
  PreviewArticle,
} from '@/types/publisher'

// State
const loading = ref(true)
const error = ref<string | null>(null)
const channels = ref<Channel[]>([])
const availableTopics = ref<TopicInfo[]>([])

// Modal state
const showModal = ref(false)
const isEditing = ref(false)
const modalError = ref<string | null>(null)
const saving = ref(false)
const currentChannel = ref<Channel | null>(null)

// Preview state
const previewArticles = ref<PreviewArticle[]>([])
const previewLoading = ref(false)

// Form data
const formData = ref<CreateChannelRequest>({
  name: '',
  slug: '',
  redis_channel: '',
  description: '',
  rules: {
    include_topics: [],
    exclude_topics: [],
    min_quality_score: 50,
    content_types: ['article'],
  },
  enabled: true,
})

// Auto-generate slug from name
watch(() => formData.value.name, (name) => {
  if (!isEditing.value) {
    formData.value.slug = name.toLowerCase().replace(/[^a-z0-9]+/g, '_')
    formData.value.redis_channel = formData.value.slug.replace(/_/g, ':')
  }
})

// Load data
const loadChannels = async () => {
  try {
    loading.value = true
    error.value = null
    const response = await publisherApi.channels.list()
    channels.value = response.data?.channels || []
  } catch (err) {
    error.value = 'Unable to load channels.'
  } finally {
    loading.value = false
  }
}

const loadTopics = async () => {
  try {
    const response = await publisherApi.topics.list()
    availableTopics.value = response.data?.topics || []
  } catch (err) {
    console.error('Failed to load topics:', err)
  }
}

// Preview rules
const loadPreview = async () => {
  if (!currentChannel.value?.id) return

  try {
    previewLoading.value = true
    const response = await publisherApi.channels.preview(currentChannel.value.id)
    previewArticles.value = response.data?.sample_articles || []
  } catch (err) {
    console.error('Failed to load preview:', err)
  } finally {
    previewLoading.value = false
  }
}

// Modal handlers
const openCreateModal = () => {
  isEditing.value = false
  formData.value = {
    name: '',
    slug: '',
    redis_channel: '',
    description: '',
    rules: {
      include_topics: [],
      exclude_topics: [],
      min_quality_score: 50,
      content_types: ['article'],
    },
    enabled: true,
  }
  currentChannel.value = null
  previewArticles.value = []
  modalError.value = null
  showModal.value = true
}

const openEditModal = (channel: Channel) => {
  isEditing.value = true
  formData.value = {
    name: channel.name,
    slug: channel.slug,
    redis_channel: channel.redis_channel,
    description: channel.description || '',
    rules: { ...channel.rules },
    enabled: channel.enabled,
  }
  currentChannel.value = channel
  modalError.value = null
  showModal.value = true
  loadPreview()
}

const saveChannel = async () => {
  saving.value = true
  modalError.value = null

  try {
    if (isEditing.value && currentChannel.value) {
      await publisherApi.channels.update(currentChannel.value.id, formData.value)
    } else {
      await publisherApi.channels.create(formData.value)
    }
    showModal.value = false
    await loadChannels()
  } catch (err) {
    const axiosError = err as { response?: { data?: { error?: string } } }
    modalError.value = axiosError.response?.data?.error || 'Failed to save channel'
  } finally {
    saving.value = false
  }
}

const deleteChannel = async (id: string) => {
  if (!confirm('Delete this channel?')) return
  try {
    await publisherApi.channels.delete(id)
    channels.value = channels.value.filter((c) => c.id !== id)
  } catch (err) {
    console.error('Error deleting channel:', err)
  }
}

onMounted(() => {
  loadChannels()
  loadTopics()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">Custom Channels</h1>
        <p class="text-muted-foreground">
          Configure rule-based channels for content distribution
        </p>
      </div>
      <Button @click="openCreateModal">
        <Plus class="mr-2 h-4 w-4" />
        New Channel
      </Button>
    </div>

    <!-- Loading state -->
    <div v-if="loading" class="flex items-center justify-center py-12">
      <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
    </div>

    <!-- Error state -->
    <Card v-else-if="error" class="border-destructive">
      <CardContent class="pt-6">
        <p class="text-destructive">{{ error }}</p>
      </CardContent>
    </Card>

    <!-- Empty state -->
    <Card v-else-if="channels.length === 0">
      <CardContent class="flex flex-col items-center justify-center py-12">
        <Hash class="h-12 w-12 text-muted-foreground mb-4" />
        <h3 class="text-lg font-medium mb-2">No custom channels</h3>
        <p class="text-muted-foreground mb-4">
          Layer 1 automatic routing is active. Create custom channels for filtered feeds.
        </p>
        <Button @click="openCreateModal">
          <Plus class="mr-2 h-4 w-4" />
          New Channel
        </Button>
      </CardContent>
    </Card>

    <!-- Channels table -->
    <Card v-else>
      <CardContent class="p-0">
        <table class="w-full">
          <thead class="border-b bg-muted/50">
            <tr>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">Name</th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">Redis Channel</th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">Topics</th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">Quality</th>
              <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">Status</th>
              <th class="px-6 py-3 text-right text-xs font-medium text-muted-foreground uppercase">Actions</th>
            </tr>
          </thead>
          <tbody class="divide-y">
            <tr v-for="channel in channels" :key="channel.id" class="hover:bg-muted/50">
              <td class="px-6 py-4">
                <div class="font-medium">{{ channel.name }}</div>
                <div class="text-sm text-muted-foreground">{{ channel.slug }}</div>
              </td>
              <td class="px-6 py-4 text-sm font-mono text-primary">
                {{ channel.redis_channel }}
              </td>
              <td class="px-6 py-4">
                <div class="flex gap-1 flex-wrap">
                  <Badge
                    v-for="topic in channel.rules.include_topics?.slice(0, 3)"
                    :key="topic"
                    variant="outline"
                    class="text-xs"
                  >
                    {{ topic }}
                  </Badge>
                  <Badge
                    v-if="(channel.rules.include_topics?.length || 0) > 3"
                    variant="outline"
                    class="text-xs"
                  >
                    +{{ channel.rules.include_topics!.length - 3 }}
                  </Badge>
                  <span v-if="!channel.rules.include_topics?.length" class="text-muted-foreground text-sm">
                    All topics
                  </span>
                </div>
              </td>
              <td class="px-6 py-4 text-sm text-muted-foreground">
                ≥{{ channel.rules.min_quality_score || 0 }}
              </td>
              <td class="px-6 py-4">
                <Badge :variant="channel.enabled ? 'success' : 'secondary'">
                  {{ channel.enabled ? 'Active' : 'Inactive' }}
                </Badge>
              </td>
              <td class="px-6 py-4 text-right">
                <div class="flex justify-end gap-2">
                  <Button variant="ghost" size="icon" @click="openEditModal(channel)">
                    <Pencil class="h-4 w-4" />
                  </Button>
                  <Button variant="ghost" size="icon" @click="deleteChannel(channel.id)">
                    <Trash2 class="h-4 w-4 text-destructive" />
                  </Button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </CardContent>
    </Card>

    <!-- Modal (simplified for plan - full implementation would include topic multi-select, preview panel) -->
    <!-- ... modal template ... -->
  </div>
</template>
```

**Step 2: Delete old RoutesView**

```bash
rm dashboard/src/views/distribution/RoutesView.vue
```

**Step 3: Update router**

Update `dashboard/src/router/index.ts` to use ChannelsView instead of RoutesView.

**Step 4: Commit**

```bash
git add -A dashboard/src/views/distribution/
git add dashboard/src/router/index.ts
git commit -m "feat(dashboard): create ChannelsView for routing v2

- Rule-based channel editor with topic selection
- Live preview of matching articles
- Auto-generate slug and redis_channel from name
- Remove old RoutesView"
```

---

## Task 14: Integration Testing

**Files:**
- Create: `publisher/internal/router/integration_test.go`

**Step 1: Write integration test**

```go
// publisher/internal/router/integration_test.go
//go:build integration

package router

import (
    "context"
    "testing"
    "time"

    "github.com/stretchr/testify/require"
)

func TestRouter_Integration_Layer1(t *testing.T) {
    t.Helper()
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    // Setup test infrastructure
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Create router with real ES and Redis
    router := setupTestRouter(t)

    // Insert test article into ES
    article := insertTestArticle(t, ctx, map[string]any{
        "title":        "Test Crime Article",
        "topics":       []string{"violent_crime", "local_news"},
        "quality_score": 75,
        "content_type": "article",
    })

    // Subscribe to expected channels
    sub1 := subscribeToChannel(t, ctx, "articles:violent_crime")
    sub2 := subscribeToChannel(t, ctx, "articles:local_news")

    // Run one poll cycle
    router.pollAndRoute(ctx)

    // Verify messages received
    msg1 := receiveMessage(t, ctx, sub1)
    require.Contains(t, msg1, article.ID)

    msg2 := receiveMessage(t, ctx, sub2)
    require.Contains(t, msg2, article.ID)
}

func TestRouter_Integration_Layer2(t *testing.T) {
    t.Helper()
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    router := setupTestRouter(t)

    // Create custom channel
    createTestChannel(t, ctx, "streetcode:test", map[string]any{
        "include_topics":   []string{"violent_crime"},
        "min_quality_score": 60,
        "content_types":    []string{"article"},
    })

    // Insert matching article
    insertTestArticle(t, ctx, map[string]any{
        "title":        "High Quality Crime",
        "topics":       []string{"violent_crime"},
        "quality_score": 80,
        "content_type": "article",
    })

    // Insert non-matching article (low quality)
    insertTestArticle(t, ctx, map[string]any{
        "title":        "Low Quality Crime",
        "topics":       []string{"violent_crime"},
        "quality_score": 40,
        "content_type": "article",
    })

    // Subscribe
    sub := subscribeToChannel(t, ctx, "streetcode:test")

    // Run poll
    router.pollAndRoute(ctx)

    // Should only receive high quality article
    msg := receiveMessage(t, ctx, sub)
    require.Contains(t, msg, "High Quality Crime")
}
```

**Step 2: Run integration tests**

```bash
cd publisher && go test ./internal/router/... -tags=integration -v
```

**Step 3: Commit**

```bash
git add publisher/internal/router/integration_test.go
git commit -m "test(publisher): add integration tests for routing v2

- Test Layer 1 automatic topic routing
- Test Layer 2 custom channel filtering"
```

---

## Task 15: Update Publisher Entry Point

**Files:**
- Modify: `publisher/cmd_router.go`
- Modify: `publisher/main.go`

**Step 1: Update router command**

Update `cmd_router.go` to initialize the new router with discovery service:

```go
func runRouter(ctx context.Context) error {
    // ... existing setup ...

    // Create discovery service
    disc := discovery.NewService(esClient, logger)

    // Create router with new config
    routerSvc := router.NewService(
        repo,
        disc,
        esClient,
        redisClient,
        router.Config{
            PollInterval:      cfg.Router.PollInterval,
            DiscoveryInterval: cfg.Router.DiscoveryInterval,
            BatchSize:         cfg.Router.BatchSize,
        },
        logger,
    )

    return routerSvc.Start(ctx)
}
```

**Step 2: Update config**

Add discovery interval to config:

```go
type RouterConfig struct {
    PollInterval      time.Duration `yaml:"poll_interval" env:"PUBLISHER_ROUTER_POLL_INTERVAL"`
    DiscoveryInterval time.Duration `yaml:"discovery_interval" env:"PUBLISHER_ROUTER_DISCOVERY_INTERVAL"`
    BatchSize         int           `yaml:"batch_size" env:"PUBLISHER_ROUTER_BATCH_SIZE"`
}
```

**Step 3: Build and test**

```bash
cd publisher && go build -o bin/publisher .
./bin/publisher router --help
```

**Step 4: Commit**

```bash
git add publisher/cmd_router.go publisher/internal/config/config.go publisher/main.go
git commit -m "feat(publisher): update entry point for routing v2

- Initialize discovery service
- Add discovery_interval config
- Use new router with two-layer routing"
```

---

## Task 16: Run Migrations on Dev Environment

**Step 1: Start dev environment**

```bash
task docker:dev:up
```

**Step 2: Run migration**

```bash
cd publisher && go run cmd/migrate/main.go up
```

**Step 3: Verify schema**

```bash
docker exec north-cloud-postgres-publisher-1 psql -U postgres -d publisher -c "\d channels"
```

Expected: Shows new schema with slug, redis_channel, rules columns.

**Step 4: Verify seed data**

```bash
docker exec north-cloud-postgres-publisher-1 psql -U postgres -d publisher -c "SELECT name, slug, redis_channel FROM channels"
```

Expected: Shows StreetCode Crime Feed channel.

---

## Task 17: End-to-End Smoke Test

**Step 1: Start publisher**

```bash
cd publisher && go run . both
```

**Step 2: Check logs for index discovery**

Expected: "Discovered classified content indexes" with count.

**Step 3: Check Layer 1 routing**

Subscribe to a topic channel and verify articles arrive:

```bash
redis-cli SUBSCRIBE articles:violent_crime
```

**Step 4: Check Layer 2 routing**

Subscribe to custom channel:

```bash
redis-cli SUBSCRIBE streetcode:crime_feed
```

**Step 5: Verify dashboard**

Open `http://localhost:3002/dashboard/distribution/channels` and verify:
- Channels list loads
- StreetCode Crime Feed shows with rules
- Create new channel works
- Preview shows matching articles

---

## Summary

**Files Created:**
- `publisher/migrations/003_routing_v2.up.sql`
- `publisher/migrations/003_routing_v2.down.sql`
- `publisher/internal/models/rules.go`
- `publisher/internal/database/cursor_repository.go`
- `publisher/internal/database/cursor_repository_test.go`
- `publisher/internal/discovery/discovery.go`
- `publisher/internal/discovery/discovery_test.go`
- `publisher/internal/router/service_test.go`
- `publisher/internal/router/integration_test.go`
- `dashboard/src/views/distribution/ChannelsView.vue`

**Files Modified:**
- `publisher/internal/models/channel.go`
- `publisher/internal/database/repository.go`
- `publisher/internal/router/service.go`
- `publisher/internal/api/router.go`
- `publisher/internal/api/channels_handler.go`
- `publisher/internal/api/handlers.go`
- `publisher/internal/config/config.go`
- `publisher/cmd_router.go`
- `dashboard/src/types/publisher.ts`
- `dashboard/src/api/client.ts`
- `dashboard/src/router/index.ts`

**Files Deleted:**
- `publisher/internal/models/source.go`
- `publisher/internal/models/route.go`
- `publisher/internal/database/repository_routes.go`
- `publisher/internal/api/sources_handler.go`
- `publisher/internal/api/routes_handler.go`
- `dashboard/src/views/distribution/RoutesView.vue`

**Total Tasks:** 17
**Estimated Commits:** 17
