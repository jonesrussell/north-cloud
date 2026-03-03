# Social Publisher Backend Additions — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add content list, accounts CRUD, JWT auth, scheduled_at fix, retry endpoint, and X adapter wiring to make social-publisher frontend-ready.

**Architecture:** Six independent changes to the existing social-publisher service. Each task adds a vertical slice (domain type → repo → handler → route → test). The crypto package is the only new package; everything else extends existing files.

**Tech Stack:** Go 1.26, Gin, sqlx, PostgreSQL, AES-256-GCM (stdlib `crypto/aes` + `crypto/cipher`), testify

---

### Task 1: Add Database Indexes Migration

**Files:**
- Create: `social-publisher/migrations/002_add_content_list_indexes.up.sql`
- Create: `social-publisher/migrations/002_add_content_list_indexes.down.sql`

**Step 1: Create the up migration**

```sql
CREATE INDEX idx_content_type ON content (type);
CREATE INDEX idx_content_created ON content (created_at DESC);
CREATE INDEX idx_content_source ON content (source);
```

**Step 2: Create the down migration**

```sql
DROP INDEX IF EXISTS idx_content_source;
DROP INDEX IF EXISTS idx_content_created;
DROP INDEX IF EXISTS idx_content_type;
```

**Step 3: Commit**

```bash
git add social-publisher/migrations/
git commit -m "feat(social-publisher): add content list indexes migration"
```

---

### Task 2: Wire JWT Auth Middleware

This must come before new endpoints so all subsequent routes are protected.

**Files:**
- Modify: `social-publisher/internal/api/router.go:48-54`

**Step 1: Write the test**

Add to `social-publisher/internal/api/handler_test.go`:

```go
func TestProtectedRoutes_RejectUnauthenticated(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Auth: config.AuthConfig{JWTSecret: "test-secret-key-for-testing"},
	}
	router := api.NewRouter(nil, nil, cfg, infralogger.NewNop())

	r := gin.New()
	r.Use(gin.Recovery())
	// Use a simplified approach: just test that setupRoutes adds JWT middleware
	// by trying to hit a route without a token
	server := router.NewServer(infralogger.NewNop(), 0)
	_ = server // Server builds successfully with JWT config

	// Create a request without JWT token
	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/api/v1/status/test-id", nil)
	assert.NoError(t, err)

	// Build a test engine through the router
	testEngine := router.TestEngine()
	testEngine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
```

**Step 2: Run test to verify it fails**

Run: `cd social-publisher && go test ./internal/api/ -run TestProtectedRoutes -v`
Expected: FAIL (TestEngine method doesn't exist, routes are unprotected)

**Step 3: Implement JWT middleware in router**

Modify `social-publisher/internal/api/router.go`. Add a `TestEngine` method and change `setupRoutes` to use `ProtectedGroup`:

```go
package api

import (
	"context"

	"github.com/gin-gonic/gin"

	infragin "github.com/north-cloud/infrastructure/gin"
	"github.com/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/config"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/database"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/orchestrator"
)

// Router wires API routes to the infrastructure HTTP server.
type Router struct {
	handler *Handler
	repo    *database.Repository
	cfg     *config.Config
}

// NewRouter creates a new API router.
func NewRouter(
	repo *database.Repository, orch *orchestrator.Orchestrator, cfg *config.Config, log logger.Logger,
) *Router {
	return &Router{
		handler: NewHandler(repo, orch, log),
		repo:    repo,
		cfg:     cfg,
	}
}

// NewServer builds an infrastructure gin.Server with routes and health checks.
func (r *Router) NewServer(log logger.Logger, port int) *infragin.Server {
	return infragin.NewServerBuilder("social-publisher", port).
		WithLogger(log).
		WithDebug(r.cfg.Debug).
		WithVersion("0.1.0").
		WithDatabaseHealthCheck(func() error {
			return r.repo.Ping(context.TODO())
		}).
		WithRoutes(func(router *gin.Engine) {
			r.setupRoutes(router)
		}).
		Build()
}

// TestEngine creates a Gin engine with routes configured, for use in tests.
func (r *Router) TestEngine() *gin.Engine {
	engine := gin.New()
	engine.Use(gin.Recovery())
	r.setupRoutes(engine)
	return engine
}

func (r *Router) setupRoutes(router *gin.Engine) {
	v1 := infragin.ProtectedGroup(router, "/api/v1", r.cfg.Auth.JWTSecret)

	v1.POST("/publish", r.handler.Publish)
	v1.GET("/status/:id", r.handler.Status)
	v1.POST("/retry/:id", r.handler.Retry)
}
```

**Step 4: Run test to verify it passes**

Run: `cd social-publisher && go test ./internal/api/ -run TestProtectedRoutes -v`
Expected: PASS

**Step 5: Update existing tests to work without JWT (use handler directly, not router)**

The existing `TestPublishEndpoint_*` tests call handlers directly on a bare gin.Engine, so they still pass. Verify:

Run: `cd social-publisher && go test ./internal/api/ -v`
Expected: All PASS

**Step 6: Commit**

```bash
git add social-publisher/internal/api/
git commit -m "feat(social-publisher): wire JWT auth middleware to API routes"
```

---

### Task 3: Fix `scheduled_at` Parsing in Publish Handler

**Files:**
- Modify: `social-publisher/internal/api/handler.go:28-41` (PublishRequest struct)
- Modify: `social-publisher/internal/api/handler.go:56-69` (Publish handler msg construction)

**Step 1: Write the test**

Add to `social-publisher/internal/api/handler_test.go`:

```go
func TestPublishEndpoint_ParsesScheduledAt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := api.NewHandler(nil, nil, infralogger.NewNop())

	w := httptest.NewRecorder()
	body := map[string]any{
		"type":         "blog_post",
		"summary":      "Scheduled post",
		"project":      "personal",
		"scheduled_at": "2026-03-15T10:00:00Z",
	}
	bodyJSON, err := json.Marshal(body)
	assert.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/api/v1/publish", bytes.NewReader(bodyJSON))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	r := gin.New()
	r.POST("/api/v1/publish", handler.Publish)
	r.ServeHTTP(w, req)

	// Without a real repo this panics/500s, but the key is it didn't return 400
	// (i.e., scheduled_at was parsed successfully, not rejected)
	assert.NotEqual(t, http.StatusBadRequest, w.Code)
}

func TestPublishEndpoint_InvalidScheduledAt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := api.NewHandler(nil, nil, infralogger.NewNop())

	w := httptest.NewRecorder()
	body := map[string]any{
		"type":         "blog_post",
		"summary":      "Bad date",
		"project":      "personal",
		"scheduled_at": "not-a-date",
	}
	bodyJSON, err := json.Marshal(body)
	assert.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/api/v1/publish", bytes.NewReader(bodyJSON))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	r := gin.New()
	r.POST("/api/v1/publish", handler.Publish)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
```

**Step 2: Run tests to verify they fail**

Run: `cd social-publisher && go test ./internal/api/ -run "TestPublishEndpoint_ParsesScheduledAt|TestPublishEndpoint_InvalidScheduledAt" -v`
Expected: FAIL (InvalidScheduledAt won't return 400 because string parsing isn't validated)

**Step 3: Fix the handler**

In `handler.go`, change `ScheduledAt` from `string` to `*time.Time` and add validation:

```go
// PublishRequest is the JSON body for the publish endpoint.
type PublishRequest struct {
	Type        string               `json:"type"         binding:"required"`
	Title       string               `json:"title"`
	Body        string               `json:"body"`
	Summary     string               `json:"summary"`
	URL         string               `json:"url"`
	Images      []string             `json:"images"`
	Tags        []string             `json:"tags"`
	Project     string               `json:"project"`
	Targets     []domain.TargetConfig `json:"targets"`
	ScheduledAt *time.Time           `json:"scheduled_at"`
	Metadata    map[string]string    `json:"metadata"`
	Source      string               `json:"source"`
}
```

And in the `Publish` method, pass `ScheduledAt` through:

```go
	msg := &domain.PublishMessage{
		ContentID:   contentID,
		Type:        domain.ContentType(req.Type),
		Title:       req.Title,
		Body:        req.Body,
		Summary:     req.Summary,
		URL:         req.URL,
		Images:      req.Images,
		Tags:        req.Tags,
		Project:     req.Project,
		Targets:     req.Targets,
		ScheduledAt: req.ScheduledAt,
		Metadata:    req.Metadata,
		Source:      req.Source,
	}
```

Add `"time"` to the import block in `handler.go`.

**Step 4: Run tests to verify they pass**

Run: `cd social-publisher && go test ./internal/api/ -v`
Expected: All PASS. Gin's JSON binding handles `*time.Time` from RFC3339 strings automatically and returns 400 on malformed dates.

**Step 5: Commit**

```bash
git add social-publisher/internal/api/
git commit -m "fix(social-publisher): parse scheduled_at in publish handler"
```

---

### Task 4: Add Content List Endpoint

**Files:**
- Create: `social-publisher/internal/domain/list.go`
- Modify: `social-publisher/internal/database/repository.go` (add ListContent, CountContent)
- Modify: `social-publisher/internal/api/handler.go` (add ListContent handler)
- Modify: `social-publisher/internal/api/router.go` (add route)
- Modify: `social-publisher/internal/api/handler_test.go` (add tests)

**Step 1: Create list domain types**

Create `social-publisher/internal/domain/list.go`:

```go
package domain

import "time"

// ContentListItem is a content record with a rolled-up delivery summary for list views.
type ContentListItem struct {
	ID              string           `json:"id"         db:"id"`
	Type            ContentType      `json:"type"       db:"type"`
	Title           string           `json:"title"      db:"title"`
	Summary         string           `json:"summary"    db:"summary"`
	URL             string           `json:"url"        db:"url"`
	Project         string           `json:"project"    db:"project"`
	Source          string           `json:"source"     db:"source"`
	Published       bool             `json:"published"  db:"published"`
	ScheduledAt     *time.Time       `json:"scheduled_at,omitempty" db:"scheduled_at"`
	CreatedAt       time.Time        `json:"created_at" db:"created_at"`
	DeliverySummary *DeliverySummary `json:"delivery_summary,omitempty"`
}

// DeliverySummary is a count of deliveries by status for a single content item.
type DeliverySummary struct {
	Total     int `json:"total"     db:"total"`
	Pending   int `json:"pending"   db:"pending"`
	Delivered int `json:"delivered" db:"delivered"`
	Failed    int `json:"failed"    db:"failed"`
	Retrying  int `json:"retrying"  db:"retrying"`
}

// ContentListFilter holds query parameters for listing content.
type ContentListFilter struct {
	Offset int
	Limit  int
	Status string // "pending", "delivered", "failed" — filters on delivery aggregate
	Type   string // content type filter
}
```

**Step 2: Add repository methods**

Add to `social-publisher/internal/database/repository.go`:

```go
// ListContent returns a paginated list of content items with delivery summaries.
func (r *Repository) ListContent(ctx context.Context, filter domain.ContentListFilter) ([]domain.ContentListItem, error) {
	query, args := buildListContentQuery(filter)
	rows, err := r.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing content: %w", err)
	}
	defer rows.Close()

	items := make([]domain.ContentListItem, 0, filter.Limit)
	for rows.Next() {
		item, scanErr := scanContentListItem(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// CountContent returns the total count of content items matching the filter.
func (r *Repository) CountContent(ctx context.Context, filter domain.ContentListFilter) (int, error) {
	query, args := buildCountContentQuery(filter)
	var count int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("counting content: %w", err)
	}
	return count, nil
}

func buildListContentQuery(filter domain.ContentListFilter) (string, []any) {
	query := `SELECT c.id, c.type, c.title, c.summary, c.url, c.project, c.source,
		c.published, c.scheduled_at, c.created_at,
		COALESCE(d.total, 0) AS total,
		COALESCE(d.pending, 0) AS pending,
		COALESCE(d.delivered, 0) AS delivered,
		COALESCE(d.failed, 0) AS failed,
		COALESCE(d.retrying, 0) AS retrying
	FROM content c
	LEFT JOIN (
		SELECT content_id,
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE status = 'pending' OR status = 'publishing') AS pending,
			COUNT(*) FILTER (WHERE status = 'delivered') AS delivered,
			COUNT(*) FILTER (WHERE status = 'failed') AS failed,
			COUNT(*) FILTER (WHERE status = 'retrying') AS retrying
		FROM deliveries GROUP BY content_id
	) d ON d.content_id = c.id`

	args := make([]any, 0)
	conditions := buildContentConditions(filter, &args)
	if len(conditions) > 0 {
		query += " WHERE " + joinConditions(conditions)
	}
	query += " ORDER BY c.created_at DESC"
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, filter.Limit, filter.Offset)

	return query, args
}

func buildCountContentQuery(filter domain.ContentListFilter) (string, []any) {
	query := `SELECT COUNT(*) FROM content c
	LEFT JOIN (
		SELECT content_id,
			COUNT(*) FILTER (WHERE status = 'delivered') AS delivered,
			COUNT(*) FILTER (WHERE status = 'failed') AS failed
		FROM deliveries GROUP BY content_id
	) d ON d.content_id = c.id`

	args := make([]any, 0)
	conditions := buildContentConditions(filter, &args)
	if len(conditions) > 0 {
		query += " WHERE " + joinConditions(conditions)
	}
	return query, args
}

func buildContentConditions(filter domain.ContentListFilter, args *[]any) []string {
	conditions := make([]string, 0)
	if filter.Type != "" {
		*args = append(*args, filter.Type)
		conditions = append(conditions, fmt.Sprintf("c.type = $%d", len(*args)))
	}
	if filter.Status != "" {
		switch filter.Status {
		case "delivered":
			conditions = append(conditions, "COALESCE(d.delivered, 0) > 0")
		case "failed":
			conditions = append(conditions, "COALESCE(d.failed, 0) > 0 AND COALESCE(d.delivered, 0) = 0")
		case "pending":
			conditions = append(conditions, "(COALESCE(d.total, 0) = 0 OR COALESCE(d.pending, 0) > 0)")
		}
	}
	return conditions
}

func joinConditions(conditions []string) string {
	result := conditions[0]
	for _, c := range conditions[1:] {
		result += " AND " + c
	}
	return result
}

func scanContentListItem(rows *sqlx.Rows) (domain.ContentListItem, error) {
	var item domain.ContentListItem
	var summary domain.DeliverySummary
	if err := rows.Scan(
		&item.ID, &item.Type, &item.Title, &item.Summary, &item.URL,
		&item.Project, &item.Source, &item.Published, &item.ScheduledAt, &item.CreatedAt,
		&summary.Total, &summary.Pending, &summary.Delivered, &summary.Failed, &summary.Retrying,
	); err != nil {
		return domain.ContentListItem{}, fmt.Errorf("scanning content list item: %w", err)
	}
	item.DeliverySummary = &summary
	return item, nil
}
```

**Step 3: Add the handler**

Add to `social-publisher/internal/api/handler.go`:

```go
const (
	defaultLimit = 50
	maxLimit     = 100
)

// ListContent returns a paginated list of content items with delivery summaries.
func (h *Handler) ListContent(c *gin.Context) {
	filter := domain.ContentListFilter{
		Limit:  defaultLimit,
		Offset: 0,
		Status: c.Query("status"),
		Type:   c.Query("type"),
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, parseErr := strconv.Atoi(limitStr); parseErr == nil && limit > 0 && limit <= maxLimit {
			filter.Limit = limit
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, parseErr := strconv.Atoi(offsetStr); parseErr == nil && offset >= 0 {
			filter.Offset = offset
		}
	}

	items, err := h.repo.ListContent(c.Request.Context(), filter)
	if err != nil {
		h.log.Error("Failed to list content", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list content"})
		return
	}

	total, err := h.repo.CountContent(c.Request.Context(), filter)
	if err != nil {
		h.log.Error("Failed to count content", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count content"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items":  items,
		"count":  len(items),
		"total":  total,
		"offset": filter.Offset,
		"limit":  filter.Limit,
	})
}
```

Add `"strconv"` to handler.go imports.

**Step 4: Add the route**

In `router.go` `setupRoutes`, add:

```go
v1.GET("/content", r.handler.ListContent)
```

**Step 5: Write handler test**

Add to `handler_test.go`:

```go
func TestListContent_NoRepo_Returns500(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := api.NewHandler(nil, nil, infralogger.NewNop())

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/api/v1/content", nil)
	assert.NoError(t, err)

	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/api/v1/content", handler.ListContent)
	r.ServeHTTP(w, req)

	// Without a repo, this panics/500s — validates route is wired
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestListContent_ParsesPaginationParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := api.NewHandler(nil, nil, infralogger.NewNop())

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/api/v1/content?limit=10&offset=20&status=delivered&type=blog_post", nil)
	assert.NoError(t, err)

	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/api/v1/content", handler.ListContent)
	r.ServeHTTP(w, req)

	// Without repo it 500s, but confirms query parsing doesn't 400
	assert.NotEqual(t, http.StatusBadRequest, w.Code)
}
```

**Step 6: Run tests**

Run: `cd social-publisher && go test ./internal/... -v`
Expected: All PASS

**Step 7: Run linter**

Run: `cd social-publisher && golangci-lint run`
Expected: No issues

**Step 8: Commit**

```bash
git add social-publisher/internal/domain/list.go social-publisher/internal/database/repository.go social-publisher/internal/api/handler.go social-publisher/internal/api/router.go social-publisher/internal/api/handler_test.go
git commit -m "feat(social-publisher): add content list endpoint with pagination and filters"
```

---

### Task 5: Add Crypto Package for Credential Encryption

**Files:**
- Create: `social-publisher/internal/crypto/crypto.go`
- Create: `social-publisher/internal/crypto/crypto_test.go`

**Step 1: Write the test**

Create `social-publisher/internal/crypto/crypto_test.go`:

```go
package crypto_test

import (
	"encoding/hex"
	"testing"

	"github.com/jonesrussell/north-cloud/social-publisher/internal/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	keyHex := hex.EncodeToString(key)

	plaintext := []byte(`{"api_key":"sk-test","api_secret":"secret123"}`)

	encrypted, err := crypto.Encrypt(plaintext, keyHex)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, encrypted)

	decrypted, err := crypto.Decrypt(encrypted, keyHex)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestEncrypt_DifferentNonceEachTime(t *testing.T) {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	keyHex := hex.EncodeToString(key)

	plaintext := []byte("same data")

	enc1, err := crypto.Encrypt(plaintext, keyHex)
	require.NoError(t, err)

	enc2, err := crypto.Encrypt(plaintext, keyHex)
	require.NoError(t, err)

	assert.NotEqual(t, enc1, enc2, "each encryption should produce different ciphertext due to random nonce")
}

func TestDecrypt_InvalidKey(t *testing.T) {
	t.Helper()
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	key2[0] = 0xFF
	keyHex1 := hex.EncodeToString(key1)
	keyHex2 := hex.EncodeToString(key2)

	encrypted, err := crypto.Encrypt([]byte("secret"), keyHex1)
	require.NoError(t, err)

	_, err = crypto.Decrypt(encrypted, keyHex2)
	assert.Error(t, err)
}

func TestEncrypt_InvalidKeyLength(t *testing.T) {
	t.Helper()
	_, err := crypto.Encrypt([]byte("data"), "tooshort")
	assert.Error(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `cd social-publisher && go test ./internal/crypto/ -v`
Expected: FAIL (package doesn't exist)

**Step 3: Implement the crypto package**

Create `social-publisher/internal/crypto/crypto.go`:

```go
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

const requiredKeyLen = 32

// Encrypt encrypts plaintext using AES-256-GCM with a random nonce.
// The keyHex parameter must be a 64-character hex string (32 bytes).
// Returns nonce || ciphertext.
func Encrypt(plaintext []byte, keyHex string) ([]byte, error) {
	key, err := decodeKey(keyHex)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt decrypts ciphertext produced by Encrypt.
// The keyHex parameter must be the same key used during encryption.
func Decrypt(ciphertext []byte, keyHex string) ([]byte, error) {
	key, err := decodeKey(keyHex)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, encrypted := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypting: %w", err)
	}

	return plaintext, nil
}

func decodeKey(keyHex string) ([]byte, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("decoding key: %w", err)
	}
	if len(key) != requiredKeyLen {
		return nil, fmt.Errorf("key must be %d bytes, got %d", requiredKeyLen, len(key))
	}
	return key, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `cd social-publisher && go test ./internal/crypto/ -v`
Expected: All PASS

**Step 5: Run linter**

Run: `cd social-publisher && golangci-lint run ./internal/crypto/`
Expected: No issues

**Step 6: Commit**

```bash
git add social-publisher/internal/crypto/
git commit -m "feat(social-publisher): add AES-256-GCM credential encryption package"
```

---

### Task 6: Add Accounts Domain Types and Repository

**Files:**
- Create: `social-publisher/internal/domain/account.go`
- Modify: `social-publisher/internal/database/repository.go` (add account CRUD methods)
- Modify: `social-publisher/internal/config/config.go` (add EncryptionKey)

**Step 1: Add EncryptionKey to config**

Add to `config.go` `Config` struct:

```go
type Config struct {
	Debug      bool           `env:"SOCIAL_PUBLISHER_DEBUG"  yaml:"debug"`
	Server     ServerConfig   `yaml:"server"`
	Database   DatabaseConfig `yaml:"database"`
	Redis      RedisConfig    `yaml:"redis"`
	Service    ServiceConfig  `yaml:"service"`
	Auth       AuthConfig     `yaml:"auth"`
	Encryption EncryptionConfig `yaml:"encryption"`
}

// EncryptionConfig holds credential encryption settings.
type EncryptionConfig struct {
	Key string `env:"SOCIAL_PUBLISHER_ENCRYPTION_KEY" yaml:"key"`
}
```

**Step 2: Create account domain type**

Create `social-publisher/internal/domain/account.go`:

```go
package domain

import "time"

// Account represents a social media account with credentials.
type Account struct {
	ID                    string     `json:"id"                      db:"id"`
	Name                  string     `json:"name"                    db:"name"`
	Platform              string     `json:"platform"                db:"platform"`
	Project               string     `json:"project"                 db:"project"`
	Enabled               bool       `json:"enabled"                 db:"enabled"`
	CredentialsConfigured bool       `json:"credentials_configured"  db:"-"`
	TokenExpiry           *time.Time `json:"token_expiry,omitempty"  db:"token_expiry"`
	CreatedAt             time.Time  `json:"created_at"              db:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"              db:"updated_at"`
}

// CreateAccountRequest is the input for creating a new account.
type CreateAccountRequest struct {
	Name        string         `json:"name"         binding:"required"`
	Platform    string         `json:"platform"     binding:"required"`
	Project     string         `json:"project"      binding:"required"`
	Enabled     *bool          `json:"enabled"`
	Credentials map[string]any `json:"credentials"`
	TokenExpiry *time.Time     `json:"token_expiry"`
}

// UpdateAccountRequest is the input for updating an existing account.
type UpdateAccountRequest struct {
	Name        *string        `json:"name"`
	Platform    *string        `json:"platform"`
	Project     *string        `json:"project"`
	Enabled     *bool          `json:"enabled"`
	Credentials map[string]any `json:"credentials"`
	TokenExpiry *time.Time     `json:"token_expiry"`
}
```

**Step 3: Add repository methods for accounts**

Add to `social-publisher/internal/database/repository.go`:

```go
// ListAccounts returns all configured accounts. Credentials are never returned.
func (r *Repository) ListAccounts(ctx context.Context) ([]domain.Account, error) {
	rows, err := r.db.QueryxContext(ctx, `
		SELECT id, name, platform, project, enabled, credentials IS NOT NULL AS has_creds,
			token_expiry, created_at, updated_at
		FROM accounts
		ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("listing accounts: %w", err)
	}
	defer rows.Close()

	accounts := make([]domain.Account, 0)
	for rows.Next() {
		var acct domain.Account
		var hasCreds bool
		if scanErr := rows.Scan(
			&acct.ID, &acct.Name, &acct.Platform, &acct.Project, &acct.Enabled,
			&hasCreds, &acct.TokenExpiry, &acct.CreatedAt, &acct.UpdatedAt,
		); scanErr != nil {
			return nil, fmt.Errorf("scanning account: %w", scanErr)
		}
		acct.CredentialsConfigured = hasCreds
		accounts = append(accounts, acct)
	}
	return accounts, rows.Err()
}

// GetAccountByID returns a single account by ID. Credentials are never returned.
func (r *Repository) GetAccountByID(ctx context.Context, id string) (*domain.Account, error) {
	var acct domain.Account
	var hasCreds bool
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, platform, project, enabled, credentials IS NOT NULL AS has_creds,
			token_expiry, created_at, updated_at
		FROM accounts WHERE id = $1`, id).Scan(
		&acct.ID, &acct.Name, &acct.Platform, &acct.Project, &acct.Enabled,
		&hasCreds, &acct.TokenExpiry, &acct.CreatedAt, &acct.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("getting account: %w", err)
	}
	acct.CredentialsConfigured = hasCreds
	return &acct, nil
}

// CreateAccount inserts a new account. Credentials should be pre-encrypted.
func (r *Repository) CreateAccount(
	ctx context.Context, id, name, platform, project string,
	enabled bool, encryptedCreds []byte, tokenExpiry *time.Time,
) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO accounts (id, name, platform, project, enabled, credentials, token_expiry)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, name, platform, project, enabled, encryptedCreds, tokenExpiry,
	)
	return err
}

// UpdateAccount updates account fields. Only non-nil fields are changed.
func (r *Repository) UpdateAccount(
	ctx context.Context, id string,
	name, platform, project *string, enabled *bool,
	encryptedCreds []byte, tokenExpiry *time.Time,
) error {
	query := "UPDATE accounts SET updated_at = NOW()"
	args := make([]any, 0)

	if name != nil {
		args = append(args, *name)
		query += fmt.Sprintf(", name = $%d", len(args))
	}
	if platform != nil {
		args = append(args, *platform)
		query += fmt.Sprintf(", platform = $%d", len(args))
	}
	if project != nil {
		args = append(args, *project)
		query += fmt.Sprintf(", project = $%d", len(args))
	}
	if enabled != nil {
		args = append(args, *enabled)
		query += fmt.Sprintf(", enabled = $%d", len(args))
	}
	if encryptedCreds != nil {
		args = append(args, encryptedCreds)
		query += fmt.Sprintf(", credentials = $%d", len(args))
	}
	if tokenExpiry != nil {
		args = append(args, *tokenExpiry)
		query += fmt.Sprintf(", token_expiry = $%d", len(args))
	}

	args = append(args, id)
	query += fmt.Sprintf(" WHERE id = $%d", len(args))

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("updating account: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("account not found: %s", id)
	}
	return nil
}

// DeleteAccount removes an account by ID.
func (r *Repository) DeleteAccount(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM accounts WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting account: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("account not found: %s", id)
	}
	return nil
}

// GetAccountCredentials returns the raw encrypted credentials for an account.
// Used internally by the orchestrator to load credentials at publish time.
func (r *Repository) GetAccountCredentials(ctx context.Context, accountName string) ([]byte, error) {
	var creds []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT credentials FROM accounts WHERE name = $1 AND enabled = true`, accountName).Scan(&creds)
	if err != nil {
		return nil, fmt.Errorf("getting account credentials: %w", err)
	}
	return creds, nil
}
```

**Step 4: Run tests + linter**

Run: `cd social-publisher && go test ./internal/... -v && golangci-lint run`
Expected: All PASS, no lint issues

**Step 5: Commit**

```bash
git add social-publisher/internal/domain/account.go social-publisher/internal/database/repository.go social-publisher/internal/config/config.go
git commit -m "feat(social-publisher): add account domain types and repository methods"
```

---

### Task 7: Add Accounts HTTP Handlers and Routes

**Files:**
- Create: `social-publisher/internal/api/accounts_handler.go`
- Modify: `social-publisher/internal/api/router.go` (add account routes)
- Create: `social-publisher/internal/api/accounts_handler_test.go`

**Step 1: Write the test**

Create `social-publisher/internal/api/accounts_handler_test.go`:

```go
package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/api"
	"github.com/stretchr/testify/assert"
)

func TestCreateAccount_MissingRequiredFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := api.NewAccountsHandler(nil, "", infralogger.NewNop())

	w := httptest.NewRecorder()
	body := map[string]any{"name": "test-x"}
	bodyJSON, err := json.Marshal(body)
	assert.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/api/v1/accounts", bytes.NewReader(bodyJSON))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	r := gin.New()
	r.POST("/api/v1/accounts", handler.Create)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateAccount_ValidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := api.NewAccountsHandler(nil, "", infralogger.NewNop())

	w := httptest.NewRecorder()
	body := map[string]any{
		"name":     "personal-x",
		"platform": "x",
		"project":  "personal",
	}
	bodyJSON, err := json.Marshal(body)
	assert.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/api/v1/accounts", bytes.NewReader(bodyJSON))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	r := gin.New()
	r.Use(gin.Recovery())
	r.POST("/api/v1/accounts", handler.Create)
	r.ServeHTTP(w, req)

	// Without a real repo, panics/500s — but not 400 (proves validation passed)
	assert.NotEqual(t, http.StatusBadRequest, w.Code)
}
```

**Step 2: Run test to verify it fails**

Run: `cd social-publisher && go test ./internal/api/ -run TestCreateAccount -v`
Expected: FAIL (NewAccountsHandler doesn't exist)

**Step 3: Implement the accounts handler**

Create `social-publisher/internal/api/accounts_handler.go`:

```go
package api

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/crypto"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/database"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/domain"
)

// AccountsHandler implements account management endpoints.
type AccountsHandler struct {
	repo          *database.Repository
	encryptionKey string
	log           infralogger.Logger
}

// NewAccountsHandler creates a new accounts handler.
func NewAccountsHandler(repo *database.Repository, encryptionKey string, log infralogger.Logger) *AccountsHandler {
	return &AccountsHandler{repo: repo, encryptionKey: encryptionKey, log: log}
}

// List returns all configured accounts with credentials masked.
func (h *AccountsHandler) List(c *gin.Context) {
	accounts, err := h.repo.ListAccounts(c.Request.Context())
	if err != nil {
		h.log.Error("Failed to list accounts", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list accounts"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": accounts,
		"count": len(accounts),
	})
}

// Get returns a single account by ID.
func (h *AccountsHandler) Get(c *gin.Context) {
	id := c.Param("id")
	account, err := h.repo.GetAccountByID(c.Request.Context(), id)
	if err != nil {
		h.log.Error("Failed to get account", infralogger.Error(err), infralogger.String("account_id", id))
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}

	c.JSON(http.StatusOK, account)
}

// Create adds a new social media account.
func (h *AccountsHandler) Create(c *gin.Context) {
	var req domain.CreateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	var encryptedCreds []byte
	if req.Credentials != nil {
		credsJSON, marshalErr := json.Marshal(req.Credentials)
		if marshalErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid credentials format"})
			return
		}
		var encErr error
		encryptedCreds, encErr = crypto.Encrypt(credsJSON, h.encryptionKey)
		if encErr != nil {
			h.log.Error("Failed to encrypt credentials", infralogger.Error(encErr))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encrypt credentials"})
			return
		}
	}

	id := uuid.New().String()
	if err := h.repo.CreateAccount(
		c.Request.Context(), id, req.Name, req.Platform, req.Project,
		enabled, encryptedCreds, req.TokenExpiry,
	); err != nil {
		h.log.Error("Failed to create account", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create account"})
		return
	}

	account, err := h.repo.GetAccountByID(c.Request.Context(), id)
	if err != nil {
		h.log.Error("Failed to retrieve created account", infralogger.Error(err))
		c.JSON(http.StatusCreated, gin.H{"id": id})
		return
	}

	c.JSON(http.StatusCreated, account)
}

// Update modifies an existing account.
func (h *AccountsHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req domain.UpdateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var encryptedCreds []byte
	if req.Credentials != nil {
		credsJSON, marshalErr := json.Marshal(req.Credentials)
		if marshalErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid credentials format"})
			return
		}
		var encErr error
		encryptedCreds, encErr = crypto.Encrypt(credsJSON, h.encryptionKey)
		if encErr != nil {
			h.log.Error("Failed to encrypt credentials", infralogger.Error(encErr))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encrypt credentials"})
			return
		}
	}

	if err := h.repo.UpdateAccount(
		c.Request.Context(), id,
		req.Name, req.Platform, req.Project, req.Enabled,
		encryptedCreds, req.TokenExpiry,
	); err != nil {
		h.log.Error("Failed to update account", infralogger.Error(err), infralogger.String("account_id", id))
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}

	account, err := h.repo.GetAccountByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"id": id, "status": "updated"})
		return
	}

	c.JSON(http.StatusOK, account)
}

// Delete removes an account by ID.
func (h *AccountsHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.repo.DeleteAccount(c.Request.Context(), id); err != nil {
		h.log.Error("Failed to delete account", infralogger.Error(err), infralogger.String("account_id", id))
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}

	c.Status(http.StatusNoContent)
}
```

**Step 4: Wire routes in router.go**

Update `setupRoutes` in `router.go` to add the `accountsHandler` field and routes:

```go
// Router wires API routes to the infrastructure HTTP server.
type Router struct {
	handler         *Handler
	accountsHandler *AccountsHandler
	repo            *database.Repository
	cfg             *config.Config
}

// NewRouter creates a new API router.
func NewRouter(
	repo *database.Repository, orch *orchestrator.Orchestrator, cfg *config.Config, log logger.Logger,
) *Router {
	return &Router{
		handler:         NewHandler(repo, orch, log),
		accountsHandler: NewAccountsHandler(repo, cfg.Encryption.Key, log),
		repo:            repo,
		cfg:             cfg,
	}
}
```

In `setupRoutes`:

```go
func (r *Router) setupRoutes(router *gin.Engine) {
	v1 := infragin.ProtectedGroup(router, "/api/v1", r.cfg.Auth.JWTSecret)

	v1.POST("/publish", r.handler.Publish)
	v1.GET("/content", r.handler.ListContent)
	v1.GET("/status/:id", r.handler.Status)
	v1.POST("/retry/:id", r.handler.Retry)

	accounts := v1.Group("/accounts")
	accounts.GET("", r.accountsHandler.List)
	accounts.GET("/:id", r.accountsHandler.Get)
	accounts.POST("", r.accountsHandler.Create)
	accounts.PUT("/:id", r.accountsHandler.Update)
	accounts.DELETE("/:id", r.accountsHandler.Delete)
}
```

**Step 5: Run tests**

Run: `cd social-publisher && go test ./internal/api/ -v`
Expected: All PASS

**Step 6: Run linter**

Run: `cd social-publisher && golangci-lint run`
Expected: No issues

**Step 7: Commit**

```bash
git add social-publisher/internal/api/
git commit -m "feat(social-publisher): add accounts CRUD endpoints"
```

---

### Task 8: Implement Retry Endpoint

**Files:**
- Modify: `social-publisher/internal/database/repository.go` (add GetDeliveryByID, ResetDeliveryForRetry)
- Modify: `social-publisher/internal/api/handler.go` (implement Retry)

**Step 1: Write the test**

Add to `social-publisher/internal/api/handler_test.go`:

```go
func TestRetryEndpoint_EmptyID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := api.NewHandler(nil, nil, infralogger.NewNop())

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/api/v1/retry/", nil)
	assert.NoError(t, err)

	r := gin.New()
	r.POST("/api/v1/retry/:id", handler.Retry)
	// Route won't match empty id, so this tests 404 behavior
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
```

**Step 2: Add repository methods**

Add to `repository.go`:

```go
// GetDeliveryByID returns a single delivery by its ID.
func (r *Repository) GetDeliveryByID(ctx context.Context, id string) (*domain.Delivery, error) {
	var delivery domain.Delivery
	err := r.db.GetContext(ctx, &delivery, `SELECT * FROM deliveries WHERE id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("getting delivery: %w", err)
	}
	return &delivery, nil
}

// ResetDeliveryForRetry resets a failed delivery to retrying with next_retry_at = NOW().
func (r *Repository) ResetDeliveryForRetry(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE deliveries SET status = 'retrying', next_retry_at = NOW(), error = NULL
		WHERE id = $1 AND status = 'failed'`, id)
	return err
}
```

**Step 3: Implement the Retry handler**

Replace the Retry method in `handler.go`:

```go
// Retry re-queues a failed delivery for another attempt.
func (h *Handler) Retry(c *gin.Context) {
	deliveryID := c.Param("id")
	if deliveryID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "delivery_id is required"})
		return
	}

	delivery, err := h.repo.GetDeliveryByID(c.Request.Context(), deliveryID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "delivery not found"})
		return
	}

	if delivery.Status != domain.StatusFailed {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "only failed deliveries can be retried",
		})
		return
	}

	if retryErr := h.repo.ResetDeliveryForRetry(c.Request.Context(), deliveryID); retryErr != nil {
		h.log.Error("Failed to reset delivery for retry",
			infralogger.Error(retryErr),
			infralogger.String("delivery_id", deliveryID),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to queue retry"})
		return
	}

	updated, err := h.repo.GetDeliveryByID(c.Request.Context(), deliveryID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"delivery_id": deliveryID, "status": "retrying"})
		return
	}

	c.JSON(http.StatusOK, updated)
}
```

**Step 4: Run tests + linter**

Run: `cd social-publisher && go test ./internal/... -v && golangci-lint run`
Expected: All PASS, no lint issues

**Step 5: Commit**

```bash
git add social-publisher/internal/api/handler.go social-publisher/internal/api/handler_test.go social-publisher/internal/database/repository.go
git commit -m "feat(social-publisher): implement retry endpoint for failed deliveries"
```

---

### Task 9: Wire X Adapter in main.go

**Files:**
- Modify: `social-publisher/main.go:137-139` (register X adapter)

**Step 1: Implement**

In `main.go`, replace the empty adapters map:

```go
	xClient := x.NewClient("") // Empty bearer token — loaded from accounts at publish time
	adapters := map[string]domain.PlatformAdapter{
		"x": x.NewAdapter(xClient),
	}
```

Add import:

```go
	xadapter "github.com/jonesrussell/north-cloud/social-publisher/internal/adapters/x"
```

**Note:** The X client is initialized with an empty bearer token. In a later iteration, the orchestrator will look up account credentials from the database at publish time. For now, this wires the adapter so publishes don't fail with "unknown platform: x" — they'll fail with "X client not configured" or auth errors, which is more accurate and retryable.

**Step 2: Run tests + linter**

Run: `cd social-publisher && go test ./... -v && golangci-lint run`
Expected: All PASS, no lint issues

**Step 3: Commit**

```bash
git add social-publisher/main.go
git commit -m "feat(social-publisher): wire X adapter into service startup"
```

---

### Task 10: Final Verification

**Step 1: Run full test suite**

Run: `cd social-publisher && go test ./... -v -count=1`
Expected: All PASS

**Step 2: Run linter with clean cache**

Run: `cd social-publisher && golangci-lint cache clean && golangci-lint run`
Expected: No issues

**Step 3: Verify build**

Run: `cd social-publisher && go build -o /dev/null .`
Expected: SUCCESS

---

## Summary of Files Changed

| File | Action | Task |
|------|--------|------|
| `migrations/002_add_content_list_indexes.up.sql` | Create | 1 |
| `migrations/002_add_content_list_indexes.down.sql` | Create | 1 |
| `internal/api/router.go` | Modify | 2, 4, 7 |
| `internal/api/handler.go` | Modify | 3, 4, 8 |
| `internal/api/handler_test.go` | Modify | 2, 3, 4, 8 |
| `internal/domain/list.go` | Create | 4 |
| `internal/crypto/crypto.go` | Create | 5 |
| `internal/crypto/crypto_test.go` | Create | 5 |
| `internal/domain/account.go` | Create | 6 |
| `internal/config/config.go` | Modify | 6 |
| `internal/database/repository.go` | Modify | 4, 6, 8 |
| `internal/api/accounts_handler.go` | Create | 7 |
| `internal/api/accounts_handler_test.go` | Create | 7 |
| `main.go` | Modify | 9 |

## New API Endpoints

| Method | Route | Purpose |
|--------|-------|---------|
| GET | `/api/v1/content` | List content with pagination + filters |
| GET | `/api/v1/accounts` | List all accounts |
| GET | `/api/v1/accounts/:id` | Get single account |
| POST | `/api/v1/accounts` | Create account |
| PUT | `/api/v1/accounts/:id` | Update account |
| DELETE | `/api/v1/accounts/:id` | Delete account |

## Modified API Endpoints

| Method | Route | Change |
|--------|-------|--------|
| POST | `/api/v1/publish` | Now parses `scheduled_at`, requires JWT |
| GET | `/api/v1/status/:id` | Now requires JWT |
| POST | `/api/v1/retry/:id` | Now functional (resets failed → retrying), requires JWT |
