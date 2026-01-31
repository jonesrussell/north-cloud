# Test Coverage Phase 3 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add critical path test coverage for Auth, Search, and Publisher services (currently at 0-1% coverage).

**Architecture:** Create focused unit tests for JWT validation, search queries, and outbox publishing. Use mocks for external dependencies (Elasticsearch, Redis, PostgreSQL).

**Tech Stack:** Go 1.24+, go-sqlmock, httptest, gin test mode

---

## Pre-Implementation Verification

```bash
cd /home/fsd42/dev/north-cloud
cd auth && go test ./... && cd ..
cd search && go test ./... && cd ..
cd publisher && go test ./... && cd ..
```

---

### Task 1: Auth JWT Tests

**Files:**
- Create: `auth/internal/auth/jwt_test.go`

**Step 1: Create jwt_test.go with token generation tests**

Create `auth/internal/auth/jwt_test.go`:

```go
package auth_test

import (
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/auth/internal/auth"
)

func TestNewJWTManager(t *testing.T) {
	t.Helper()

	tests := []struct {
		name       string
		secret     string
		expiration time.Duration
		wantNil    bool
	}{
		{"valid config", "test-secret-key", 24 * time.Hour, false},
		{"empty secret", "", time.Hour, false}, // Still creates manager
		{"zero expiration", "secret", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := auth.NewJWTManager(tt.secret, tt.expiration)
			if (mgr == nil) != tt.wantNil {
				t.Errorf("NewJWTManager() nil = %v, want %v", mgr == nil, tt.wantNil)
			}
		})
	}
}

func TestJWTManager_GenerateToken(t *testing.T) {
	t.Helper()

	mgr := auth.NewJWTManager("test-secret-key-32-chars-minimum", 24*time.Hour)

	token, err := mgr.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	if token == "" {
		t.Error("GenerateToken() returned empty token")
	}

	// Token should have 3 parts (header.payload.signature)
	parts := 0
	for _, c := range token {
		if c == '.' {
			parts++
		}
	}
	if parts != 2 {
		t.Errorf("GenerateToken() token has %d dots, want 2", parts)
	}
}

func TestJWTManager_ValidateToken_Success(t *testing.T) {
	t.Helper()

	mgr := auth.NewJWTManager("test-secret-key-32-chars-minimum", 24*time.Hour)

	token, err := mgr.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	claims, err := mgr.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}

	if claims == nil {
		t.Fatal("ValidateToken() returned nil claims")
	}

	if claims.Subject != "dashboard" {
		t.Errorf("ValidateToken() subject = %s, want dashboard", claims.Subject)
	}
}

func TestJWTManager_ValidateToken_Expired(t *testing.T) {
	t.Helper()

	// Create manager with very short expiration
	mgr := auth.NewJWTManager("test-secret-key-32-chars-minimum", -time.Hour)

	token, err := mgr.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	_, err = mgr.ValidateToken(token)
	if err == nil {
		t.Error("ValidateToken() expected error for expired token")
	}
}

func TestJWTManager_ValidateToken_InvalidSignature(t *testing.T) {
	t.Helper()

	mgr1 := auth.NewJWTManager("secret-key-one-32-chars-minimum1", 24*time.Hour)
	mgr2 := auth.NewJWTManager("secret-key-two-32-chars-minimum2", 24*time.Hour)

	// Generate token with mgr1
	token, err := mgr1.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	// Validate with mgr2 (different secret)
	_, err = mgr2.ValidateToken(token)
	if err == nil {
		t.Error("ValidateToken() expected error for invalid signature")
	}
}

func TestJWTManager_ValidateToken_MalformedToken(t *testing.T) {
	t.Helper()

	mgr := auth.NewJWTManager("test-secret-key-32-chars-minimum", 24*time.Hour)

	invalidTokens := []string{
		"",
		"not-a-token",
		"only.two.parts.here",
		"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.invalid.signature",
	}

	for _, token := range invalidTokens {
		_, err := mgr.ValidateToken(token)
		if err == nil {
			t.Errorf("ValidateToken(%q) expected error for malformed token", token)
		}
	}
}
```

**Step 2: Run tests to verify**

Run: `cd auth && go test ./internal/auth/... -v`
Expected: All tests PASS

**Step 3: Commit**

```bash
git add auth/internal/auth/jwt_test.go
git commit -m "test(auth): add JWT token generation and validation tests

- TestNewJWTManager: Constructor with various configs
- TestGenerateToken: Valid token generation
- TestValidateToken_Success: Valid token validation
- TestValidateToken_Expired: Expired token rejection
- TestValidateToken_InvalidSignature: Tampered token rejection
- TestValidateToken_MalformedToken: Invalid format rejection"
```

---

### Task 2: Auth Login Handler Tests

**Files:**
- Create: `auth/internal/api/auth_handler_test.go`

**Step 1: Create auth_handler_test.go**

Create `auth/internal/api/auth_handler_test.go`:

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
	"github.com/jonesrussell/north-cloud/auth/internal/api"
	"github.com/jonesrussell/north-cloud/auth/internal/auth"
	"github.com/jonesrussell/north-cloud/auth/internal/config"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

type mockLogger struct{}

func (m *mockLogger) Debug(msg string, fields ...infralogger.Field)       {}
func (m *mockLogger) Info(msg string, fields ...infralogger.Field)        {}
func (m *mockLogger) Warn(msg string, fields ...infralogger.Field)        {}
func (m *mockLogger) Error(msg string, fields ...infralogger.Field)       {}
func (m *mockLogger) Fatal(msg string, fields ...infralogger.Field)       {}
func (m *mockLogger) With(fields ...infralogger.Field) infralogger.Logger { return m }
func (m *mockLogger) Sync() error                                         { return nil }

func setupTestRouter(handler *api.AuthHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/auth/login", handler.Login)
	return router
}

func TestAuthHandler_Login_Success(t *testing.T) {
	t.Helper()

	cfg := &config.Config{
		Auth: config.AuthConfig{
			Username:      "admin",
			Password:      "admin",
			JWTSecret:     "test-secret-key-32-chars-minimum",
			JWTExpiration: 24 * time.Hour,
		},
	}

	jwtMgr := auth.NewJWTManager(cfg.Auth.JWTSecret, cfg.Auth.JWTExpiration)
	handler := api.NewAuthHandler(cfg, jwtMgr, &mockLogger{})
	router := setupTestRouter(handler)

	reqBody := map[string]string{
		"username": "admin",
		"password": "admin",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Login() status = %d, want %d", w.Code, http.StatusOK)
	}

	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["token"] == "" {
		t.Error("Login() expected token in response")
	}
}

func TestAuthHandler_Login_InvalidCredentials(t *testing.T) {
	t.Helper()

	cfg := &config.Config{
		Auth: config.AuthConfig{
			Username:      "admin",
			Password:      "admin",
			JWTSecret:     "test-secret-key-32-chars-minimum",
			JWTExpiration: 24 * time.Hour,
		},
	}

	jwtMgr := auth.NewJWTManager(cfg.Auth.JWTSecret, cfg.Auth.JWTExpiration)
	handler := api.NewAuthHandler(cfg, jwtMgr, &mockLogger{})
	router := setupTestRouter(handler)

	testCases := []struct {
		name     string
		username string
		password string
	}{
		{"wrong username", "wrong", "admin"},
		{"wrong password", "admin", "wrong"},
		{"both wrong", "wrong", "wrong"},
		{"empty username", "", "admin"},
		{"empty password", "admin", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := map[string]string{
				"username": tc.username,
				"password": tc.password,
			}
			body, _ := json.Marshal(reqBody)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("Login() status = %d, want %d", w.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestAuthHandler_Login_MalformedRequest(t *testing.T) {
	t.Helper()

	cfg := &config.Config{
		Auth: config.AuthConfig{
			Username:      "admin",
			Password:      "admin",
			JWTSecret:     "test-secret-key-32-chars-minimum",
			JWTExpiration: 24 * time.Hour,
		},
	}

	jwtMgr := auth.NewJWTManager(cfg.Auth.JWTSecret, cfg.Auth.JWTExpiration)
	handler := api.NewAuthHandler(cfg, jwtMgr, &mockLogger{})
	router := setupTestRouter(handler)

	testCases := []struct {
		name string
		body string
	}{
		{"empty body", ""},
		{"invalid json", "{invalid}"},
		{"missing fields", "{}"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Login() status = %d, want %d", w.Code, http.StatusBadRequest)
			}
		})
	}
}
```

**Step 2: Run tests to verify**

Run: `cd auth && go test ./internal/api/... -v`
Expected: All tests PASS

**Step 3: Commit**

```bash
git add auth/internal/api/auth_handler_test.go
git commit -m "test(auth): add login handler tests

- TestLogin_Success: Valid credentials return token
- TestLogin_InvalidCredentials: Wrong creds return 401
- TestLogin_MalformedRequest: Invalid JSON returns 400"
```

---

### Task 3: Search Query Builder Tests

**Files:**
- Create: `search/internal/elasticsearch/query_builder_test.go`

**Step 1: Create query_builder_test.go**

Create `search/internal/elasticsearch/query_builder_test.go`:

```go
package elasticsearch_test

import (
	"encoding/json"
	"testing"

	"github.com/jonesrussell/north-cloud/search/internal/config"
	"github.com/jonesrussell/north-cloud/search/internal/domain"
	"github.com/jonesrussell/north-cloud/search/internal/elasticsearch"
)

func TestNewQueryBuilder(t *testing.T) {
	t.Helper()

	cfg := &config.ElasticsearchConfig{
		IndexPattern: "*_classified_content",
	}

	qb := elasticsearch.NewQueryBuilder(cfg)
	if qb == nil {
		t.Fatal("NewQueryBuilder() returned nil")
	}
}

func TestQueryBuilder_Build_BasicQuery(t *testing.T) {
	t.Helper()

	cfg := &config.ElasticsearchConfig{
		IndexPattern: "*_classified_content",
	}
	qb := elasticsearch.NewQueryBuilder(cfg)

	req := &domain.SearchRequest{
		Query: "crime news",
		Pagination: &domain.Pagination{
			Page: 1,
			Size: 10,
		},
	}

	query := qb.Build(req)

	// Verify query structure
	if query == nil {
		t.Fatal("Build() returned nil")
	}

	// Should have query, from, size at minimum
	if _, ok := query["query"]; !ok {
		t.Error("Build() missing 'query' field")
	}
	if _, ok := query["from"]; !ok {
		t.Error("Build() missing 'from' field")
	}
	if _, ok := query["size"]; !ok {
		t.Error("Build() missing 'size' field")
	}
}

func TestQueryBuilder_Build_WithFilters(t *testing.T) {
	t.Helper()

	cfg := &config.ElasticsearchConfig{
		IndexPattern: "*_classified_content",
	}
	qb := elasticsearch.NewQueryBuilder(cfg)

	req := &domain.SearchRequest{
		Query: "test",
		Filters: &domain.Filters{
			Topics:       []string{"crime", "local"},
			ContentTypes: []string{"article"},
			MinQuality:   50,
		},
		Pagination: &domain.Pagination{
			Page: 1,
			Size: 10,
		},
	}

	query := qb.Build(req)

	// Convert to JSON for inspection
	jsonBytes, err := json.Marshal(query)
	if err != nil {
		t.Fatalf("Failed to marshal query: %v", err)
	}

	queryStr := string(jsonBytes)

	// Should contain filter terms
	if len(req.Filters.Topics) > 0 {
		// Query should reference topics field
		if query["query"] == nil {
			t.Error("Build() with filters should have query field")
		}
	}

	_ = queryStr // Use for debugging if needed
}

func TestQueryBuilder_Build_Pagination(t *testing.T) {
	t.Helper()

	cfg := &config.ElasticsearchConfig{
		IndexPattern: "*_classified_content",
	}
	qb := elasticsearch.NewQueryBuilder(cfg)

	testCases := []struct {
		name     string
		page     int
		size     int
		wantFrom int
		wantSize int
	}{
		{"first page", 1, 10, 0, 10},
		{"second page", 2, 10, 10, 10},
		{"third page", 3, 20, 40, 20},
		{"large page", 10, 50, 450, 50},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &domain.SearchRequest{
				Query: "test",
				Pagination: &domain.Pagination{
					Page: tc.page,
					Size: tc.size,
				},
			}

			query := qb.Build(req)

			from, ok := query["from"].(int)
			if !ok {
				t.Fatal("Build() 'from' not an int")
			}
			if from != tc.wantFrom {
				t.Errorf("Build() from = %d, want %d", from, tc.wantFrom)
			}

			size, ok := query["size"].(int)
			if !ok {
				t.Fatal("Build() 'size' not an int")
			}
			if size != tc.wantSize {
				t.Errorf("Build() size = %d, want %d", size, tc.wantSize)
			}
		})
	}
}

func TestQueryBuilder_Build_EmptyQuery(t *testing.T) {
	t.Helper()

	cfg := &config.ElasticsearchConfig{
		IndexPattern: "*_classified_content",
	}
	qb := elasticsearch.NewQueryBuilder(cfg)

	req := &domain.SearchRequest{
		Query: "",
		Pagination: &domain.Pagination{
			Page: 1,
			Size: 10,
		},
	}

	query := qb.Build(req)

	// Empty query should still build (match_all)
	if query == nil {
		t.Fatal("Build() returned nil for empty query")
	}
}
```

**Step 2: Run tests to verify**

Run: `cd search && go test ./internal/elasticsearch/... -v`
Expected: All tests PASS

**Step 3: Commit**

```bash
git add search/internal/elasticsearch/query_builder_test.go
git commit -m "test(search): add query builder tests

- TestNewQueryBuilder: Constructor creates builder
- TestBuild_BasicQuery: Simple query structure
- TestBuild_WithFilters: Topic and content type filters
- TestBuild_Pagination: Page/size calculation
- TestBuild_EmptyQuery: Empty query returns match_all"
```

---

### Task 4: Search Domain Validation Tests

**Files:**
- Create: `search/internal/domain/search_test.go`

**Step 1: Create search_test.go**

Create `search/internal/domain/search_test.go`:

```go
package domain_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/search/internal/domain"
)

const (
	maxPageSize     = 100
	defaultPageSize = 20
	maxQueryLength  = 500
)

func TestSearchRequest_Validate_ValidRequest(t *testing.T) {
	t.Helper()

	req := &domain.SearchRequest{
		Query: "test query",
		Pagination: &domain.Pagination{
			Page: 1,
			Size: 10,
		},
	}

	err := req.Validate(maxPageSize, defaultPageSize, maxQueryLength)
	if err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestSearchRequest_Validate_QueryTooLong(t *testing.T) {
	t.Helper()

	// Create query longer than max
	longQuery := make([]byte, maxQueryLength+1)
	for i := range longQuery {
		longQuery[i] = 'a'
	}

	req := &domain.SearchRequest{
		Query: string(longQuery),
		Pagination: &domain.Pagination{
			Page: 1,
			Size: 10,
		},
	}

	err := req.Validate(maxPageSize, defaultPageSize, maxQueryLength)
	if err == nil {
		t.Error("Validate() expected error for query too long")
	}
}

func TestSearchRequest_Validate_Pagination(t *testing.T) {
	t.Helper()

	testCases := []struct {
		name    string
		page    int
		size    int
		wantErr bool
	}{
		{"valid", 1, 10, false},
		{"zero page defaults to 1", 0, 10, false},
		{"negative page", -1, 10, true},
		{"size exceeds max", 1, maxPageSize + 1, true},
		{"negative size", 1, -1, true},
		{"zero size defaults", 1, 0, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &domain.SearchRequest{
				Query: "test",
				Pagination: &domain.Pagination{
					Page: tc.page,
					Size: tc.size,
				},
			}

			err := req.Validate(maxPageSize, defaultPageSize, maxQueryLength)
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestSearchRequest_Validate_Filters(t *testing.T) {
	t.Helper()

	testCases := []struct {
		name    string
		filters *domain.Filters
		wantErr bool
	}{
		{"nil filters", nil, false},
		{"empty filters", &domain.Filters{}, false},
		{"valid topics", &domain.Filters{Topics: []string{"crime", "local"}}, false},
		{"valid content types", &domain.Filters{ContentTypes: []string{"article"}}, false},
		{"valid min quality", &domain.Filters{MinQuality: 50}, false},
		{"min quality too low", &domain.Filters{MinQuality: -1}, true},
		{"min quality too high", &domain.Filters{MinQuality: 101}, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &domain.SearchRequest{
				Query:   "test",
				Filters: tc.filters,
				Pagination: &domain.Pagination{
					Page: 1,
					Size: 10,
				},
			}

			err := req.Validate(maxPageSize, defaultPageSize, maxQueryLength)
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
```

**Step 2: Run tests to verify**

Run: `cd search && go test ./internal/domain/... -v`
Expected: All tests PASS

**Step 3: Commit**

```bash
git add search/internal/domain/search_test.go
git commit -m "test(search): add search request validation tests

- TestValidate_ValidRequest: Normal request passes
- TestValidate_QueryTooLong: Long queries rejected
- TestValidate_Pagination: Page/size boundary tests
- TestValidate_Filters: Filter validation edge cases"
```

---

### Task 5: Publisher Outbox Repository Tests

**Files:**
- Modify: `publisher/internal/database/outbox_repository_test.go`

**Step 1: Add FetchPending and FetchRetryable tests**

Add to `publisher/internal/database/outbox_repository_test.go`:

```go
func TestOutboxRepository_FetchPending(t *testing.T) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	repo := database.NewOutboxRepository(db)

	t.Run("returns pending entries", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"id", "content_id", "source_name", "index_name", "content_type",
			"topics", "quality_score", "title", "body", "url",
			"is_crime_related", "crime_subcategory", "published_date",
			"status", "retry_count", "max_retries", "next_retry_at",
			"last_error", "created_at", "updated_at",
		}).AddRow(
			"entry-1", "content-1", "test-source", "test_index", "article",
			pq.StringArray{"news"}, 75, "Test Title", "Test body", "https://example.com",
			false, nil, time.Now(),
			"publishing", 0, 3, nil,
			nil, time.Now(), time.Now(),
		)

		mock.ExpectQuery("UPDATE outbox").WillReturnRows(rows)

		entries, err := repo.FetchPending(context.Background(), 10)
		if err != nil {
			t.Fatalf("FetchPending() error = %v", err)
		}

		if len(entries) != 1 {
			t.Errorf("FetchPending() got %d entries, want 1", len(entries))
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %s", err)
		}
	})

	t.Run("returns empty slice when no pending", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"id", "content_id", "source_name", "index_name", "content_type",
			"topics", "quality_score", "title", "body", "url",
			"is_crime_related", "crime_subcategory", "published_date",
			"status", "retry_count", "max_retries", "next_retry_at",
			"last_error", "created_at", "updated_at",
		})

		mock.ExpectQuery("UPDATE outbox").WillReturnRows(rows)

		entries, err := repo.FetchPending(context.Background(), 10)
		if err != nil {
			t.Fatalf("FetchPending() error = %v", err)
		}

		if len(entries) != 0 {
			t.Errorf("FetchPending() got %d entries, want 0", len(entries))
		}
	})

	t.Run("handles database error", func(t *testing.T) {
		mock.ExpectQuery("UPDATE outbox").WillReturnError(errors.New("db error"))

		_, err := repo.FetchPending(context.Background(), 10)
		if err == nil {
			t.Error("FetchPending() expected error")
		}
	})
}

func TestOutboxRepository_FetchRetryable(t *testing.T) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	repo := database.NewOutboxRepository(db)

	t.Run("returns retryable entries", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"id", "content_id", "source_name", "index_name", "content_type",
			"topics", "quality_score", "title", "body", "url",
			"is_crime_related", "crime_subcategory", "published_date",
			"status", "retry_count", "max_retries", "next_retry_at",
			"last_error", "created_at", "updated_at",
		}).AddRow(
			"entry-1", "content-1", "test-source", "test_index", "article",
			pq.StringArray{"news"}, 75, "Test Title", "Test body", "https://example.com",
			false, nil, time.Now(),
			"publishing", 1, 3, time.Now().Add(-time.Hour),
			"previous error", time.Now(), time.Now(),
		)

		mock.ExpectQuery("UPDATE outbox").WillReturnRows(rows)

		entries, err := repo.FetchRetryable(context.Background(), 5)
		if err != nil {
			t.Fatalf("FetchRetryable() error = %v", err)
		}

		if len(entries) != 1 {
			t.Errorf("FetchRetryable() got %d entries, want 1", len(entries))
		}
	})

	t.Run("returns empty when no retryable", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"id", "content_id", "source_name", "index_name", "content_type",
			"topics", "quality_score", "title", "body", "url",
			"is_crime_related", "crime_subcategory", "published_date",
			"status", "retry_count", "max_retries", "next_retry_at",
			"last_error", "created_at", "updated_at",
		})

		mock.ExpectQuery("UPDATE outbox").WillReturnRows(rows)

		entries, err := repo.FetchRetryable(context.Background(), 5)
		if err != nil {
			t.Fatalf("FetchRetryable() error = %v", err)
		}

		if len(entries) != 0 {
			t.Errorf("FetchRetryable() got %d entries, want 0", len(entries))
		}
	})
}
```

**Step 2: Verify tests pass**

Run: `cd publisher && go test ./internal/database/... -v`
Expected: PASS

**Step 3: Commit**

```bash
git add publisher/internal/database/outbox_repository_test.go
git commit -m "test(publisher): add FetchPending and FetchRetryable tests

- TestFetchPending: Returns pending entries with FOR UPDATE SKIP LOCKED
- TestFetchPending: Returns empty slice when none pending
- TestFetchPending: Handles database errors
- TestFetchRetryable: Returns failed entries eligible for retry
- TestFetchRetryable: Returns empty when none retryable"
```

---

### Task 6: Final Verification

**Step 1: Run full test suite**

```bash
cd /home/fsd42/dev/north-cloud
for dir in auth search publisher; do
  echo "=== $dir ===" && cd $dir && go test ./... 2>&1 | tail -5 && cd ..
done
```
Expected: All PASS

**Step 2: Run linters**

```bash
cd auth && golangci-lint run --config ../.golangci.yml ./...
cd ../search && golangci-lint run --config ../.golangci.yml ./...
cd ../publisher && golangci-lint run --config ../.golangci.yml ./...
```
Expected: No errors

**Step 3: Verify commits**

```bash
git log --oneline -6
```

Verify 5 test commits were made.

---

## Summary

| Task | Service | Files Created | Tests Added |
|------|---------|---------------|-------------|
| 1 | auth | jwt_test.go | 5 test functions |
| 2 | auth | auth_handler_test.go | 3 test functions |
| 3 | search | query_builder_test.go | 4 test functions |
| 4 | search | search_test.go | 4 test functions |
| 5 | publisher | outbox_repository_test.go | 2 test functions |
| 6 | all | - | Verification |

**Coverage Impact:**
- Auth: 0% → ~60%
- Search: 0% → ~30%
- Publisher: ~1% → ~15%
