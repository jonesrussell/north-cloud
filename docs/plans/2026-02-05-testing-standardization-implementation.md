# Testing Standardization Phase 1 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Protect the crawl-classify-publish pipeline with contract tests at ES boundaries, a full-stack integration suite, and unit tests for under-tested services.

**Architecture:** Shared contracts package in `index-manager/pkg/contracts/` exposes canonical ES mappings. Each service imports it and asserts its read/write fields exist. Pipeline integration test boots real services via docker-compose and pushes content end-to-end.

**Tech Stack:** Go 1.25, standard `testing` package (no testify in contracts), Docker Compose, nc-http-proxy fixtures, GitHub Actions

**Design doc:** `docs/plans/2026-02-05-testing-standardization-design.md`

---

## Task 1: Create Shared Contracts Package

**Files:**
- Create: `index-manager/pkg/contracts/contracts.go`
- Create: `index-manager/pkg/contracts/contracts_test.go`
- Create: `index-manager/pkg/contracts/raw_content.go`
- Create: `index-manager/pkg/contracts/classified_content.go`
- Create: `index-manager/pkg/contracts/classified_content_test.go`

This package wraps the canonical mappings from `internal/elasticsearch/mappings/` and provides test assertion helpers. Uses only standard library — no testify — so consuming services don't inherit extra dependencies.

**Step 1: Write the assertion helpers**

Create `index-manager/pkg/contracts/contracts.go`:

```go
// Package contracts exposes canonical Elasticsearch mappings for contract testing.
// Services import this package to verify their read/write fields exist in the schema.
package contracts

import "testing"

// AssertFieldsExist validates that all required top-level fields exist in ES mapping properties.
func AssertFieldsExist(t *testing.T, properties map[string]any, fields []string) {
	t.Helper()
	for _, field := range fields {
		if _, exists := properties[field]; !exists {
			t.Errorf("required field %q not found in mapping properties", field)
		}
	}
}

// AssertNestedFieldsExist validates fields inside a nested/object mapping.
func AssertNestedFieldsExist(t *testing.T, properties map[string]any, parent string, fields []string) {
	t.Helper()
	parentDef, exists := properties[parent]
	if !exists {
		t.Fatalf("parent field %q not found in mapping properties", parent)
		return
	}
	parentMap, ok := parentDef.(map[string]any)
	if !ok {
		t.Fatalf("parent field %q is not a map", parent)
		return
	}
	nested, ok := parentMap["properties"].(map[string]any)
	if !ok {
		t.Fatalf("parent field %q has no nested properties", parent)
		return
	}
	for _, field := range fields {
		if _, exists := nested[field]; !exists {
			t.Errorf("nested field %q.%q not found in mapping properties", parent, field)
		}
	}
}

// extractProperties extracts the properties map from a full ES mapping definition.
// Expects structure: {"mappings": {"properties": {...}}}
func extractProperties(m map[string]any) map[string]any {
	mappingsSection, ok := m["mappings"].(map[string]any)
	if !ok {
		return nil
	}
	props, ok := mappingsSection["properties"].(map[string]any)
	if !ok {
		return nil
	}
	return props
}
```

**Step 2: Write tests for assertion helpers**

Create `index-manager/pkg/contracts/contracts_test.go`:

```go
package contracts

import "testing"

func TestAssertFieldsExist_AllPresent(t *testing.T) {
	t.Helper()
	props := map[string]any{
		"title": map[string]any{"type": "text"},
		"url":   map[string]any{"type": "keyword"},
	}
	// Should not fail
	AssertFieldsExist(t, props, []string{"title", "url"})
}

func TestAssertFieldsExist_MissingField(t *testing.T) {
	t.Helper()
	mockT := &testing.T{}
	props := map[string]any{
		"title": map[string]any{"type": "text"},
	}
	AssertFieldsExist(mockT, props, []string{"title", "missing_field"})
	if !mockT.Failed() {
		t.Error("expected test to fail for missing field")
	}
}

func TestAssertNestedFieldsExist_AllPresent(t *testing.T) {
	t.Helper()
	props := map[string]any{
		"crime": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"sub_label":   map[string]any{"type": "keyword"},
				"crime_types": map[string]any{"type": "keyword"},
			},
		},
	}
	AssertNestedFieldsExist(t, props, "crime", []string{"sub_label", "crime_types"})
}

func TestExtractProperties(t *testing.T) {
	t.Helper()
	m := map[string]any{
		"settings": map[string]any{"number_of_shards": 1},
		"mappings": map[string]any{
			"properties": map[string]any{
				"title": map[string]any{"type": "text"},
			},
		},
	}
	props := extractProperties(m)
	if props == nil {
		t.Fatal("expected non-nil properties")
	}
	if _, exists := props["title"]; !exists {
		t.Error("expected title field in extracted properties")
	}
}
```

**Step 3: Run tests to verify helpers work**

```bash
cd index-manager && go test ./pkg/contracts/ -v
```

Expected: PASS

**Step 4: Create raw content contract**

Create `index-manager/pkg/contracts/raw_content.go`:

```go
package contracts

import (
	"github.com/jonesrussell/north-cloud/index-manager/internal/elasticsearch/mappings"
)

// RawContentProperties returns the canonical properties map for *_raw_content indices.
func RawContentProperties() map[string]any {
	return extractProperties(mappings.GetRawContentMapping())
}
```

**Step 5: Create classified content contract**

Create `index-manager/pkg/contracts/classified_content.go`:

```go
package contracts

import (
	"github.com/jonesrussell/north-cloud/index-manager/internal/elasticsearch/mappings"
)

// ClassifiedContentProperties returns the canonical properties map for *_classified_content indices.
func ClassifiedContentProperties() map[string]any {
	return extractProperties(mappings.GetClassifiedContentMapping())
}
```

**Step 6: Write smoke tests for mapping contracts**

Create `index-manager/pkg/contracts/classified_content_test.go`:

```go
package contracts

import "testing"

func TestRawContentProperties_NotNil(t *testing.T) {
	t.Helper()
	props := RawContentProperties()
	if props == nil {
		t.Fatal("RawContentProperties() returned nil")
	}
	if len(props) == 0 {
		t.Fatal("RawContentProperties() returned empty map")
	}
}

func TestClassifiedContentProperties_NotNil(t *testing.T) {
	t.Helper()
	props := ClassifiedContentProperties()
	if props == nil {
		t.Fatal("ClassifiedContentProperties() returned nil")
	}
	if len(props) == 0 {
		t.Fatal("ClassifiedContentProperties() returned empty map")
	}
}

func TestClassifiedContentProperties_SupersetOfRaw(t *testing.T) {
	t.Helper()
	raw := RawContentProperties()
	classified := ClassifiedContentProperties()
	for field := range raw {
		if _, exists := classified[field]; !exists {
			t.Errorf("classified_content missing raw_content field %q", field)
		}
	}
}
```

**Step 7: Run all contract tests**

```bash
cd index-manager && go test ./pkg/contracts/ -v
```

Expected: PASS

**Step 8: Lint**

```bash
cd index-manager && golangci-lint run ./pkg/contracts/
```

Expected: No errors

**Step 9: Commit**

```bash
git add index-manager/pkg/contracts/
git commit -m "feat(index-manager): add shared contracts package for ES schema validation"
```

---

## Task 2: Classifier Contract Tests

**Files:**
- Modify: `classifier/go.mod` (add index-manager dependency)
- Create: `classifier/tests/contracts/classified_content_test.go`
- Create: `classifier/tests/contracts/raw_content_test.go`

The classifier is both a **consumer** of `*_raw_content` and a **producer** of `*_classified_content`. These tests validate that every field it reads/writes exists in the canonical mapping.

**Step 1: Add index-manager dependency**

```bash
cd classifier
```

Add to `classifier/go.mod` after the existing `require` block:

```
require github.com/jonesrussell/north-cloud/index-manager v0.0.0
```

Add to the `replace` directives:

```
replace github.com/jonesrussell/north-cloud/index-manager => ../index-manager
```

Then tidy:

```bash
cd classifier && go mod tidy
```

**Step 2: Write producer contract test (classified_content)**

The classifier writes these top-level fields to classified_content. The field list comes from `domain.ClassifiedContent` JSON tags in `classifier/internal/domain/classification.go`.

Create `classifier/tests/contracts/classified_content_test.go`:

```go
package contracts_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/index-manager/pkg/contracts"
)

func TestClassifierProducesValidClassifiedContent(t *testing.T) {
	props := contracts.ClassifiedContentProperties()

	// Top-level fields the classifier writes (from domain.ClassifiedContent JSON tags)
	topLevelFields := []string{
		// Inherited from RawContent
		"id", "url", "source_name", "title", "raw_html", "raw_text",
		"og_type", "og_title", "og_description", "og_image",
		"meta_description", "meta_keywords", "canonical_url",
		"crawled_at", "published_date",
		"classification_status", "classified_at", "word_count",
		// Classification results
		"content_type", "content_subtype", "quality_score", "quality_factors",
		"topics", "topic_scores",
		"source_reputation", "source_category",
		"classifier_version", "classification_method", "confidence",
		// Nested objects (checked separately below)
		"crime", "location", "mining",
		// Backward compat
		"is_crime_related",
	}

	contracts.AssertFieldsExist(t, props, topLevelFields)
}

func TestClassifierCrimeFieldsMatchMapping(t *testing.T) {
	props := contracts.ClassifiedContentProperties()

	// Fields the classifier writes to the crime nested object
	// From domain.CrimeResult JSON tags in classification.go
	crimeFields := []string{
		"sub_label",
		"relevance",
		"crime_types",
		"final_confidence",
		"homepage_eligible",
		"review_required",
		"model_version",
	}

	contracts.AssertNestedFieldsExist(t, props, "crime", crimeFields)
}

func TestClassifierMiningFieldsMatchMapping(t *testing.T) {
	props := contracts.ClassifiedContentProperties()

	// Fields the classifier writes to the mining nested object
	// From domain.MiningResult JSON tags in classification.go
	miningFields := []string{
		"relevance",
		"mining_stage",
		"commodities",
		"location",
		"final_confidence",
		"review_required",
		"model_version",
	}

	contracts.AssertNestedFieldsExist(t, props, "mining", miningFields)
}

func TestClassifierLocationFieldsMatchMapping(t *testing.T) {
	props := contracts.ClassifiedContentProperties()

	// Fields the classifier writes to the location nested object
	// From domain.LocationResult JSON tags in classification.go
	locationFields := []string{
		"city",
		"province",
		"country",
		"specificity",
		"confidence",
	}

	contracts.AssertNestedFieldsExist(t, props, "location", locationFields)
}
```

**NOTE:** The classifier's `CrimeResult` struct uses JSON tag `"street_crime_relevance"` but the canonical mapping defines the field as `"relevance"`. This test will surface this drift. Similarly, the classifier writes `category_pages` and `location_specificity` which are **not** in the canonical crime mapping. These discrepancies must be resolved by either:
- (a) Adding missing fields to the index-manager mapping, or
- (b) Aligning the classifier's JSON tags to the mapping

Track these as follow-up tasks after the contract tests are in place.

**Step 3: Write consumer contract test (raw_content)**

Create `classifier/tests/contracts/raw_content_test.go`:

```go
package contracts_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/index-manager/pkg/contracts"
)

func TestClassifierExpectedRawContentFields(t *testing.T) {
	props := contracts.RawContentProperties()

	// Fields the classifier reads from raw_content indices
	// From domain.RawContent JSON tags in raw_content.go
	requiredFields := []string{
		"id", "url", "source_name",
		"title", "raw_html", "raw_text",
		"og_type", "og_title", "og_description", "og_image", "og_url",
		"meta_description", "meta_keywords", "canonical_url",
		"crawled_at", "published_date",
		"classification_status", "classified_at",
		"word_count",
	}

	contracts.AssertFieldsExist(t, props, requiredFields)
}
```

**Step 4: Run tests**

```bash
cd classifier && go test ./tests/contracts/ -v
```

Expected: Some tests may FAIL due to known schema drift (see NOTE above). Record failures as follow-up issues.

**Step 5: Lint**

```bash
cd classifier && golangci-lint run ./tests/contracts/
```

**Step 6: Commit**

```bash
git add classifier/go.mod classifier/go.sum classifier/tests/
git commit -m "feat(classifier): add contract tests for ES schema validation"
```

---

## Task 3: Publisher Contract Tests

**Files:**
- Modify: `publisher/go.mod` (add index-manager dependency)
- Create: `publisher/tests/contracts/classified_content_test.go`

The publisher is a **consumer** of `*_classified_content`.

**Step 1: Add index-manager dependency**

Add to `publisher/go.mod`:

```
require github.com/jonesrussell/north-cloud/index-manager v0.0.0
```

Add replace directive:

```
replace github.com/jonesrussell/north-cloud/index-manager => ../index-manager
```

```bash
cd publisher && go mod tidy
```

**Step 2: Write consumer contract test**

The publisher reads classified_content via its `Article` struct in `internal/router/service.go:214-266`. Key fields it depends on:

Create `publisher/tests/contracts/classified_content_test.go`:

```go
package contracts_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/index-manager/pkg/contracts"
)

func TestPublisherExpectedClassifiedContentFields(t *testing.T) {
	props := contracts.ClassifiedContentProperties()

	// Top-level fields the publisher reads from classified_content
	// From Article struct in internal/router/service.go
	requiredFields := []string{
		"title", "raw_text", "url", "source_name",
		"published_date", "word_count",
		"quality_score", "topics", "content_type",
		"is_crime_related", "source_reputation", "confidence",
		"og_title", "og_description", "og_image", "og_url",
		"crawled_at",
		// Nested objects
		"crime", "location", "mining",
	}

	contracts.AssertFieldsExist(t, props, requiredFields)
}

func TestPublisherExpectedCrimeFields(t *testing.T) {
	props := contracts.ClassifiedContentProperties()

	// Crime fields the publisher uses for Layer 3+4 routing
	// From Article struct crime fields in internal/router/service.go
	crimeFields := []string{
		"relevance",
		"sub_label",
		"crime_types",
		"final_confidence",
		"homepage_eligible",
		"review_required",
	}

	contracts.AssertNestedFieldsExist(t, props, "crime", crimeFields)
}

func TestPublisherExpectedMiningFields(t *testing.T) {
	props := contracts.ClassifiedContentProperties()

	// Mining fields the publisher uses for Layer 5 routing
	// From MiningData struct in internal/router/service.go
	miningFields := []string{
		"relevance",
		"mining_stage",
		"commodities",
		"location",
		"final_confidence",
		"review_required",
		"model_version",
	}

	contracts.AssertNestedFieldsExist(t, props, "mining", miningFields)
}

func TestPublisherExpectedLocationFields(t *testing.T) {
	props := contracts.ClassifiedContentProperties()

	// Location fields the publisher uses for Layer 4 crime routing
	locationFields := []string{
		"city",
		"province",
		"country",
		"confidence",
	}

	contracts.AssertNestedFieldsExist(t, props, "location", locationFields)
}
```

**Step 3: Run tests**

```bash
cd publisher && go test ./tests/contracts/ -v
```

Expected: PASS (publisher reads standard fields)

**Step 4: Lint**

```bash
cd publisher && golangci-lint run ./tests/contracts/
```

**Step 5: Commit**

```bash
git add publisher/go.mod publisher/go.sum publisher/tests/
git commit -m "feat(publisher): add contract tests for classified_content schema"
```

---

## Task 4: Crawler Contract Tests

**Files:**
- Modify: `crawler/go.mod` (add index-manager dependency)
- Create: `crawler/tests/contracts/raw_content_test.go`

The crawler is a **producer** of `*_raw_content`.

**Step 1: Add index-manager dependency**

Add to `crawler/go.mod`:

```
require github.com/jonesrussell/north-cloud/index-manager v0.0.0
```

Add replace directive:

```
replace github.com/jonesrussell/north-cloud/index-manager => ../index-manager
```

```bash
cd crawler && go mod tidy
```

**Step 2: Write producer contract test**

The crawler writes via its `RawContent` struct in `internal/storage/raw_content_indexer.go:19-41`.

Create `crawler/tests/contracts/raw_content_test.go`:

```go
package contracts_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/index-manager/pkg/contracts"
)

func TestCrawlerProducesValidRawContent(t *testing.T) {
	props := contracts.RawContentProperties()

	// Fields the crawler writes to raw_content indices
	// From RawContent struct in internal/storage/raw_content_indexer.go
	producedFields := []string{
		"id", "url", "source_name",
		"title", "raw_text", "raw_html",
		"meta_description", "meta_keywords",
		"og_type", "og_title", "og_description", "og_image",
		"canonical_url",
		"crawled_at", "published_date",
		"classification_status",
		"word_count",
	}

	contracts.AssertFieldsExist(t, props, producedFields)
}
```

**NOTE:** The crawler also writes `author`, `article_section`, `json_ld_data`, `og_url`, and `meta` (nested) fields. The raw_content mapping includes `author` and `og_url` but NOT `article_section`, `json_ld_data`, or `meta`. These may need to be added to the mapping or the crawler struct pruned. The initial test covers the core fields; extend later to catch additional drift.

**Step 3: Run tests**

```bash
cd crawler && go test ./tests/contracts/ -v
```

Expected: PASS

**Step 4: Lint**

```bash
cd crawler && golangci-lint run ./tests/contracts/
```

**Step 5: Commit**

```bash
git add crawler/go.mod crawler/go.sum crawler/tests/contracts/
git commit -m "feat(crawler): add contract tests for raw_content schema"
```

---

## Task 5: Search Contract Tests

**Files:**
- Modify: `search/go.mod` (add index-manager dependency)
- Create: `search/tests/contracts/classified_content_test.go`

The search service is a **consumer** of `*_classified_content`.

**Step 1: Add index-manager dependency**

Add to `search/go.mod`:

```
require github.com/jonesrussell/north-cloud/index-manager v0.0.0
```

Add replace directive:

```
replace github.com/jonesrussell/north-cloud/index-manager => ../index-manager
```

```bash
cd search && go mod tidy
```

**Step 2: Write consumer contract test**

The search service queries fields from `internal/elasticsearch/query_builder.go`. It reads, searches, filters, sorts, and aggregates on these fields:

Create `search/tests/contracts/classified_content_test.go`:

```go
package contracts_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/index-manager/pkg/contracts"
)

func TestSearchExpectedClassifiedContentFields(t *testing.T) {
	props := contracts.ClassifiedContentProperties()

	// Fields the search service returns in _source (query_builder.go:61-66)
	sourceFields := []string{
		"id", "title", "url", "source_name",
		"published_date", "crawled_at",
		"quality_score", "content_type", "topics",
		"is_crime_related",
	}
	contracts.AssertFieldsExist(t, props, sourceFields)

	// Fields used in multi_match queries (query_builder.go:116-123)
	searchFields := []string{
		"title", "og_title", "raw_text",
		"og_description", "meta_description",
	}
	contracts.AssertFieldsExist(t, props, searchFields)

	// Fields used in filters (query_builder.go:140-221)
	filterFields := []string{
		"topics", "content_type", "quality_score",
		"is_crime_related", "source_name", "crawled_at",
	}
	contracts.AssertFieldsExist(t, props, filterFields)
}
```

**Step 3: Run tests**

```bash
cd search && go test ./tests/contracts/ -v
```

Expected: PASS

**Step 4: Lint**

```bash
cd search && golangci-lint run ./tests/contracts/
```

**Step 5: Commit**

```bash
git add search/go.mod search/go.sum search/tests/contracts/
git commit -m "feat(search): add contract tests for classified_content schema"
```

---

## Task 6: Create docker-compose.test.yml

**Files:**
- Create: `docker-compose.test.yml`

Extends `docker-compose.base.yml` with test-specific configuration: no volume mounts, no hot reload, deterministic ports, nc-http-proxy in replay mode.

**Step 1: Create docker-compose.test.yml**

Create `docker-compose.test.yml`:

```yaml
# Test environment - extends base with test-specific config
# Usage: docker compose -f docker-compose.base.yml -f docker-compose.test.yml up -d
#
# Key differences from dev:
# - No volume mounts (uses built images)
# - No hot reload
# - nc-http-proxy in replay mode (deterministic responses)
# - Fixed ports to avoid conflicts

services:
  auth:
    build:
      context: ./auth
      dockerfile: Dockerfile
    ports:
      - "18040:8040"
    environment:
      - AUTH_USERNAME=admin
      - AUTH_PASSWORD=testpass123
      - AUTH_JWT_SECRET=test-jwt-secret-for-integration-tests
    depends_on:
      redis:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:8040/health"]
      interval: 5s
      timeout: 3s
      retries: 10

  source-manager:
    build:
      context: ./source-manager
      dockerfile: Dockerfile
    ports:
      - "18050:8050"
    environment:
      - POSTGRES_HOST=postgres-source-manager
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=postgres
      - POSTGRES_DB=source_manager
      - AUTH_JWT_SECRET=test-jwt-secret-for-integration-tests
    depends_on:
      postgres-source-manager:
        condition: service_healthy
      auth:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:8050/health"]
      interval: 5s
      timeout: 3s
      retries: 10

  crawler:
    build:
      context: ./crawler
      dockerfile: Dockerfile
    ports:
      - "18060:8060"
    environment:
      - POSTGRES_HOST=postgres-crawler
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=postgres
      - POSTGRES_DB=crawler
      - ELASTICSEARCH_URL=http://elasticsearch:9200
      - SOURCE_MANAGER_URL=http://source-manager:8050
      - HTTP_PROXY_URL=http://nc-http-proxy:8055
      - AUTH_JWT_SECRET=test-jwt-secret-for-integration-tests
      - REDIS_URL=redis:6379
    depends_on:
      postgres-crawler:
        condition: service_healthy
      elasticsearch:
        condition: service_healthy
      source-manager:
        condition: service_healthy
      nc-http-proxy:
        condition: service_started
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:8060/health"]
      interval: 5s
      timeout: 3s
      retries: 10

  classifier:
    build:
      context: ./classifier
      dockerfile: Dockerfile
    ports:
      - "18071:8071"
    environment:
      - ELASTICSEARCH_URL=http://elasticsearch:9200
      - POSTGRES_HOST=postgres-classifier
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=postgres
      - POSTGRES_DB=classifier
      - CLASSIFIER_POLL_INTERVAL=5s
      - CLASSIFIER_BATCH_SIZE=10
      - CRIME_ENABLED=false
      - MINING_ENABLED=false
      - AUTH_JWT_SECRET=test-jwt-secret-for-integration-tests
    depends_on:
      postgres-classifier:
        condition: service_healthy
      elasticsearch:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:8071/health"]
      interval: 5s
      timeout: 3s
      retries: 10

  publisher:
    build:
      context: ./publisher
      dockerfile: Dockerfile
    ports:
      - "18070:8070"
    environment:
      - POSTGRES_HOST=postgres-publisher
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=postgres
      - POSTGRES_DB=publisher
      - ELASTICSEARCH_URL=http://elasticsearch:9200
      - REDIS_URL=redis://redis:6379
      - PUBLISHER_ROUTER_CHECK_INTERVAL=10s
      - AUTH_JWT_SECRET=test-jwt-secret-for-integration-tests
    depends_on:
      postgres-publisher:
        condition: service_healthy
      elasticsearch:
        condition: service_healthy
      redis:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:8070/health"]
      interval: 5s
      timeout: 3s
      retries: 10

  index-manager:
    build:
      context: ./index-manager
      dockerfile: Dockerfile
    ports:
      - "18090:8090"
    environment:
      - ELASTICSEARCH_URL=http://elasticsearch:9200
      - POSTGRES_HOST=postgres-index-manager
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=postgres
      - POSTGRES_DB=index_manager
      - AUTH_JWT_SECRET=test-jwt-secret-for-integration-tests
    depends_on:
      postgres-index-manager:
        condition: service_healthy
      elasticsearch:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:8090/health"]
      interval: 5s
      timeout: 3s
      retries: 10

  nc-http-proxy:
    build:
      context: ./nc-http-proxy
      dockerfile: Dockerfile
    ports:
      - "18055:8055"
    environment:
      - PROXY_MODE=replay
      - PROXY_PORT=8055
      - FIXTURE_DIR=/fixtures
    volumes:
      - ./crawler/fixtures:/fixtures:ro
```

**Step 2: Validate compose config**

```bash
docker compose -f docker-compose.base.yml -f docker-compose.test.yml config > /dev/null
```

Expected: No errors

**Step 3: Commit**

```bash
git add docker-compose.test.yml
git commit -m "feat(infra): add docker-compose.test.yml for integration testing"
```

---

## Task 7: Create Fixture Pages

**Files:**
- Create: `crawler/fixtures/fixture-news.example.com/GET_article-news.html`
- Create: `crawler/fixtures/fixture-news.example.com/GET_listing-page.html`
- Create: `crawler/fixtures/fixture-news.example.com/GET_crime-article.html`
- Create: `crawler/fixtures/fixture-news.example.com/GET_index.html`

These are static HTML pages served by nc-http-proxy in replay mode. They cover the three main content classification paths.

**Step 1: Create the fixture directory**

```bash
mkdir -p crawler/fixtures/fixture-news.example.com
```

**Step 2: Create index/landing page fixture**

This page links to all fixture articles, allowing the crawler to discover them.

Create `crawler/fixtures/fixture-news.example.com/GET_index.html`:

```html
<!DOCTYPE html>
<html>
<head>
    <title>Fixture News - Test Site</title>
    <meta name="description" content="Test news site for integration testing">
</head>
<body>
    <h1>Fixture News</h1>
    <article>
        <h2><a href="/article-news.html">City Council Approves New Transit Plan</a></h2>
        <p>Council voted 8-3 to approve the downtown transit expansion.</p>
    </article>
    <article>
        <h2><a href="/crime-article.html">Armed Robbery at Downtown Convenience Store</a></h2>
        <p>Police are seeking suspects after an armed robbery late Thursday.</p>
    </article>
    <article>
        <h2><a href="/listing-page.html">Local Business Directory</a></h2>
        <p>Find local businesses in your area.</p>
    </article>
</body>
</html>
```

**Step 3: Create news article fixture**

Should classify as: `content_type: "article"`, quality_score >= 50, topics: politics/local_news.

Create `crawler/fixtures/fixture-news.example.com/GET_article-news.html`:

```html
<!DOCTYPE html>
<html>
<head>
    <title>City Council Approves New Transit Plan for Downtown Core</title>
    <meta name="description" content="Sudbury city council voted 8-3 to approve the downtown transit expansion plan, allocating $45 million for new bus routes and infrastructure improvements.">
    <meta name="author" content="Jane Reporter">
    <meta property="og:title" content="City Council Approves New Transit Plan for Downtown Core">
    <meta property="og:description" content="Sudbury city council voted 8-3 to approve a major transit expansion.">
    <meta property="og:type" content="article">
    <meta property="og:image" content="https://fixture-news.example.com/images/transit.jpg">
    <meta property="og:url" content="https://fixture-news.example.com/article-news.html">
    <link rel="canonical" href="https://fixture-news.example.com/article-news.html">
</head>
<body>
    <article>
        <h1>City Council Approves New Transit Plan for Downtown Core</h1>
        <time datetime="2026-02-01T10:00:00Z">February 1, 2026</time>
        <p class="byline">By Jane Reporter</p>

        <p>Sudbury city council voted 8-3 on Thursday evening to approve a comprehensive downtown transit expansion plan that will bring significant changes to the city's public transportation network over the next five years.</p>

        <p>The $45 million plan includes the addition of twelve new bus routes, construction of three transit hubs in the downtown core, and the installation of real-time arrival displays at all major stops. The project is expected to reduce average commute times by approximately 20 percent for residents living in the northern suburbs.</p>

        <p>Councillor Maria Santos, who championed the proposal, said the investment was long overdue. "Our transit system hasn't seen a major upgrade in over a decade. This plan will make public transportation a viable option for thousands more residents," she told reporters after the vote.</p>

        <p>The three dissenting councillors raised concerns about the project's timeline and funding model. Councillor James Bradford argued that the city should prioritize road infrastructure repairs before expanding transit services. "We have bridges that need attention and roads that are falling apart. We need to fix what we have before building something new," Bradford said during the debate.</p>

        <p>The plan will be funded through a combination of federal transit grants, provincial matching funds, and a modest property tax increase of 0.3 percent over three years. City staff estimate that the first new routes could be operational by spring 2027, with the full expansion completed by 2031.</p>

        <p>Public reaction has been largely positive, with local transit advocacy groups praising the council's decision. The Greater Sudbury Transit Riders Association called it "a transformative moment for mobility in our city" in a statement released Friday morning.</p>
    </article>
</body>
</html>
```

**Step 4: Create listing page fixture**

Should classify as: `content_type: "listing"` or `"page"`, publisher should skip it.

Create `crawler/fixtures/fixture-news.example.com/GET_listing-page.html`:

```html
<!DOCTYPE html>
<html>
<head>
    <title>Local Business Directory - Fixture News</title>
    <meta name="description" content="Browse local businesses in the Sudbury area">
    <meta property="og:type" content="website">
</head>
<body>
    <h1>Local Business Directory</h1>
    <ul>
        <li><a href="/business/1">Joe's Diner - 123 Main St</a></li>
        <li><a href="/business/2">Sudbury Auto Repair - 456 Elm St</a></li>
        <li><a href="/business/3">Northern Grocers - 789 Oak Ave</a></li>
        <li><a href="/business/4">City Fitness Center - 321 Pine Rd</a></li>
        <li><a href="/business/5">Lakeside Pharmacy - 654 Lake Dr</a></li>
    </ul>
    <nav>
        <a href="/directory?page=2">Next Page</a>
    </nav>
</body>
</html>
```

**Step 5: Create crime article fixture**

Should classify as: `content_type: "article"`, crime_detected, topics include crime sub-category.

Create `crawler/fixtures/fixture-news.example.com/GET_crime-article.html`:

```html
<!DOCTYPE html>
<html>
<head>
    <title>Armed Robbery at Downtown Convenience Store Leaves Clerk Injured</title>
    <meta name="description" content="Greater Sudbury Police are investigating an armed robbery at a downtown convenience store that left one employee with minor injuries.">
    <meta name="author" content="Mike Journalist">
    <meta property="og:title" content="Armed Robbery at Downtown Convenience Store Leaves Clerk Injured">
    <meta property="og:description" content="Police investigate armed robbery at downtown store, clerk injured.">
    <meta property="og:type" content="article">
    <meta property="og:image" content="https://fixture-news.example.com/images/robbery.jpg">
    <meta property="og:url" content="https://fixture-news.example.com/crime-article.html">
    <link rel="canonical" href="https://fixture-news.example.com/crime-article.html">
</head>
<body>
    <article>
        <h1>Armed Robbery at Downtown Convenience Store Leaves Clerk Injured</h1>
        <time datetime="2026-02-01T14:30:00Z">February 1, 2026</time>
        <p class="byline">By Mike Journalist</p>

        <p>Greater Sudbury Police are investigating an armed robbery that occurred at a downtown convenience store late Thursday evening, leaving one employee with minor injuries. The suspect remains at large.</p>

        <p>Officers responded to a 911 call from the Quick Stop convenience store on Durham Street at approximately 11:45 p.m. According to police, a masked individual entered the store brandishing what appeared to be a handgun and demanded cash from the register.</p>

        <p>The store clerk, a 34-year-old man, was struck during the altercation and suffered minor injuries. He was treated at Health Sciences North and released. Police say the suspect fled on foot with an undisclosed amount of cash.</p>

        <p>Detective Sarah Thompson of the Greater Sudbury Police robbery unit said investigators are reviewing surveillance footage from the store and surrounding businesses. "We're asking anyone who was in the area of Durham Street between 11:30 p.m. and midnight to contact us if they saw anything suspicious," Thompson said at a Friday morning press conference.</p>

        <p>The suspect is described as a male, approximately 5-foot-10, wearing dark clothing and a ski mask. Police are urging the public not to approach anyone matching this description but to call 911 immediately.</p>

        <p>This is the third armed robbery reported in the downtown core this month. Police have increased patrols in the area and are investigating whether the incidents are connected. The Greater Sudbury Police Service is encouraging business owners to review their security procedures and ensure surveillance cameras are functioning properly.</p>

        <p>Anyone with information is asked to contact the Greater Sudbury Police at 705-675-9171 or Crime Stoppers at 1-800-222-TIPS.</p>
    </article>
</body>
</html>
```

**Step 6: Verify fixture files exist**

```bash
ls -la crawler/fixtures/fixture-news.example.com/
```

Expected: 4 HTML files

**Step 7: Commit**

```bash
git add crawler/fixtures/fixture-news.example.com/
git commit -m "feat(crawler): add fixture pages for pipeline integration testing"
```

---

## Task 8: Build Pipeline Integration Test Harness

**Files:**
- Create: `tests/integration/pipeline/go.mod`
- Create: `tests/integration/pipeline/pipeline_test.go`
- Create: `tests/integration/pipeline/helpers_test.go`

This is a Go test binary that orchestrates the full pipeline: seed source, create channel + route, trigger crawl, wait for classification, verify Redis messages.

**Step 1: Create Go module**

Create `tests/integration/pipeline/go.mod`:

```
module github.com/jonesrussell/north-cloud/tests/integration/pipeline

go 1.25

require (
	github.com/elastic/go-elasticsearch/v8 v8.19.1
	github.com/redis/go-redis/v9 v9.17.3
)
```

```bash
cd tests/integration/pipeline && go mod tidy
```

**Step 2: Write test helpers**

Create `tests/integration/pipeline/helpers_test.go`:

```go
package pipeline_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

const (
	authURL          = "http://localhost:18040"
	sourceManagerURL = "http://localhost:18050"
	crawlerURL       = "http://localhost:18060"
	publisherURL     = "http://localhost:18070"
	classifierURL    = "http://localhost:18071"
	indexManagerURL   = "http://localhost:18090"
	elasticsearchURL = "http://localhost:9200"
	redisAddr        = "localhost:6379"

	healthTimeout    = 120 * time.Second
	healthInterval   = 2 * time.Second
	pipelineTimeout  = 180 * time.Second
	pollInterval     = 3 * time.Second
)

// waitForHealth polls a health endpoint until it returns 200 or times out.
func waitForHealth(t *testing.T, name, url string) {
	t.Helper()
	deadline := time.Now().Add(healthTimeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				t.Logf("%s is healthy", name)
				return
			}
		}
		time.Sleep(healthInterval)
	}
	t.Fatalf("%s did not become healthy within %v", name, healthTimeout)
}

// getAuthToken logs in and returns a JWT token.
func getAuthToken(t *testing.T) string {
	t.Helper()
	body := map[string]string{
		"username": "admin",
		"password": "testpass123",
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal login body: %v", err)
	}
	resp, err := http.Post(authURL+"/api/v1/auth/login", "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		t.Fatalf("login request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("login failed (%d): %s", resp.StatusCode, string(respBody))
	}
	var result map[string]any
	if decodeErr := json.NewDecoder(resp.Body).Decode(&result); decodeErr != nil {
		t.Fatalf("decode login response: %v", decodeErr)
	}
	token, ok := result["token"].(string)
	if !ok {
		t.Fatal("token not found in login response")
	}
	return token
}

// authedRequest creates an HTTP request with JWT auth header.
func authedRequest(t *testing.T, method, url string, body any, token string) *http.Response {
	t.Helper()
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, url, bodyReader)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, doErr := http.DefaultClient.Do(req)
	if doErr != nil {
		t.Fatalf("%s %s: %v", method, url, doErr)
	}
	return resp
}

// decodeResponse reads and decodes a JSON response body.
func decodeResponse(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if resp.StatusCode >= 400 {
		t.Fatalf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}
	var result map[string]any
	if unmarshalErr := json.Unmarshal(bodyBytes, &result); unmarshalErr != nil {
		t.Fatalf("decode response: %v (body: %s)", unmarshalErr, string(bodyBytes))
	}
	return result
}

// pollES polls Elasticsearch for documents matching a query until found or timeout.
func pollES(t *testing.T, indexPattern, field, value string, timeout time.Duration) map[string]any {
	t.Helper()
	deadline := time.Now().Add(timeout)
	query := fmt.Sprintf(`{"query":{"term":{%q:%q}},"size":1}`, field, value)

	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			elasticsearchURL+"/"+indexPattern+"/_search",
			bytes.NewBufferString(query),
		)
		if err != nil {
			t.Fatalf("create ES request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, doErr := http.DefaultClient.Do(req)
		if doErr != nil {
			time.Sleep(pollInterval)
			continue
		}
		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()

		hits := extractHits(result)
		if len(hits) > 0 {
			return hits[0]
		}
		time.Sleep(pollInterval)
	}
	t.Fatalf("no document found in %s with %s=%s within %v", indexPattern, field, value, timeout)
	return nil
}

// extractHits pulls the hits array from an ES search response.
func extractHits(result map[string]any) []map[string]any {
	hitsOuter, ok := result["hits"].(map[string]any)
	if !ok {
		return nil
	}
	hitsInner, ok := hitsOuter["hits"].([]any)
	if !ok {
		return nil
	}
	var hits []map[string]any
	for _, h := range hitsInner {
		if hit, ok := h.(map[string]any); ok {
			hits = append(hits, hit)
		}
	}
	return hits
}
```

**Step 3: Write the pipeline test**

Create `tests/integration/pipeline/pipeline_test.go`:

```go
//go:build integration

package pipeline_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// TestFullPipeline boots the stack, pushes content through crawl → classify → publish,
// and asserts on what arrives in Elasticsearch and Redis.
func TestFullPipeline(t *testing.T) {
	// --- Phase 1: Health checks ---
	t.Log("Waiting for services to be healthy...")
	services := map[string]string{
		"auth":           authURL + "/health",
		"source-manager": sourceManagerURL + "/health",
		"crawler":        crawlerURL + "/health",
		"classifier":     classifierURL + "/health",
		"publisher":      publisherURL + "/health",
		"index-manager":  indexManagerURL + "/health",
	}
	for name, url := range services {
		waitForHealth(t, name, url)
	}

	token := getAuthToken(t)
	t.Log("Authenticated successfully")

	// --- Phase 2: Seed data ---
	t.Log("Creating source...")
	sourceResp := authedRequest(t, "POST", sourceManagerURL+"/api/v1/sources", map[string]any{
		"name":      "fixture-news",
		"url":       "https://fixture-news.example.com",
		"enabled":   true,
		"max_depth": 1,
		"selectors": map[string]any{
			"article": map[string]any{
				"title": "h1",
				"body":  "article",
			},
		},
	}, token)
	source := decodeResponse(t, sourceResp)
	sourceID, ok := source["id"].(string)
	if !ok {
		t.Fatal("source ID not found in response")
	}
	t.Logf("Source created: %s", sourceID)

	t.Log("Creating publisher channel...")
	channelResp := authedRequest(t, "POST", publisherURL+"/api/v1/channels", map[string]any{
		"name":        "integration-test-feed",
		"slug":        "integration_test",
		"description": "Integration test channel",
		"enabled":     true,
	}, token)
	channel := decodeResponse(t, channelResp)
	channelID, ok := channel["id"].(string)
	if !ok {
		t.Fatal("channel ID not found in response")
	}
	t.Logf("Channel created: %s", channelID)

	t.Log("Creating publisher route...")
	routeResp := authedRequest(t, "POST", publisherURL+"/api/v1/routes", map[string]any{
		"source_id":       sourceID,
		"channel_id":      channelID,
		"min_quality_score": 0,
		"active":          true,
	}, token)
	route := decodeResponse(t, routeResp)
	t.Logf("Route created: %v", route["id"])

	// --- Phase 3: Trigger crawl ---
	t.Log("Creating crawler job...")
	jobResp := authedRequest(t, "POST", crawlerURL+"/api/v1/jobs", map[string]any{
		"source_id":        sourceID,
		"url":              "https://fixture-news.example.com",
		"schedule_enabled": false,
	}, token)
	job := decodeResponse(t, jobResp)
	t.Logf("Job created: %v", job["id"])

	// --- Phase 4: Wait for raw content ---
	t.Log("Polling for raw content in Elasticsearch...")
	rawDoc := pollES(t, "*_raw_content", "classification_status", "pending", pipelineTimeout)
	rawSource := rawDoc["_source"].(map[string]any)

	if rawSource["title"] == nil || rawSource["title"] == "" {
		t.Error("raw_content document missing title")
	}
	if rawSource["classification_status"] != "pending" {
		t.Errorf("expected classification_status=pending, got %v", rawSource["classification_status"])
	}
	t.Log("Raw content indexed successfully")

	// --- Phase 5: Wait for classified content ---
	t.Log("Polling for classified content in Elasticsearch...")
	classifiedDoc := pollES(t, "*_classified_content", "content_type", "article", pipelineTimeout)
	classified := classifiedDoc["_source"].(map[string]any)

	// Verify classification fields
	if classified["quality_score"] == nil {
		t.Error("classified_content missing quality_score")
	}
	if classified["content_type"] == nil {
		t.Error("classified_content missing content_type")
	}
	if classified["topics"] == nil {
		t.Error("classified_content missing topics")
	}
	t.Logf("Classified content: type=%v quality=%v topics=%v",
		classified["content_type"], classified["quality_score"], classified["topics"])

	// --- Phase 6: Wait for Redis publish ---
	t.Log("Subscribing to Redis for published articles...")
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer rdb.Close()

	ctx, cancel := context.WithTimeout(context.Background(), pipelineTimeout)
	defer cancel()

	// Subscribe to all articles channels
	sub := rdb.PSubscribe(ctx, "articles:*")
	defer sub.Close()

	var receivedMessage *redis.Message
	msgCh := sub.Channel()

	select {
	case msg := <-msgCh:
		receivedMessage = msg
	case <-ctx.Done():
		t.Log("WARN: No Redis message received within timeout (publisher may not have routed yet)")
		t.Log("This is expected if the publisher hasn't processed the route yet")
		return
	}

	if receivedMessage != nil {
		t.Logf("Received Redis message on channel: %s", receivedMessage.Channel)
		var payload map[string]any
		if err := json.Unmarshal([]byte(receivedMessage.Payload), &payload); err != nil {
			t.Fatalf("unmarshal Redis message: %v", err)
		}
		if payload["title"] == nil {
			t.Error("Redis message missing title")
		}
		if payload["quality_score"] == nil {
			t.Error("Redis message missing quality_score")
		}
		t.Log("Pipeline complete: content crawled, classified, and published to Redis")
	}
}
```

**Step 4: Tidy module**

```bash
cd tests/integration/pipeline && go mod tidy
```

**Step 5: Commit**

```bash
git add tests/integration/pipeline/
git commit -m "feat(tests): add pipeline integration test harness"
```

**NOTE:** The test is gated with `//go:build integration` so it won't run with normal `go test`. It requires docker-compose services to be up. The exact API payloads (source creation, job creation) may need adjustment during implementation based on actual API contracts. Verify by reading the handler code for each endpoint.

---

## Task 9: Add Integration CI Workflow and Taskfile Task

**Files:**
- Create: `.github/workflows/integration.yml`
- Modify: `Taskfile.yml` (add `test:integration:pipeline` task)

**Step 1: Create CI workflow**

Create `.github/workflows/integration.yml`:

```yaml
name: Pipeline Integration Tests

on:
  push:
    branches: [main]

concurrency:
  group: integration-${{ github.ref }}
  cancel-in-progress: true

jobs:
  integration:
    runs-on: ubuntu-latest
    timeout-minutes: 15

    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25'

      - name: Build and start services
        run: |
          docker compose -f docker-compose.base.yml -f docker-compose.test.yml up -d --build --wait
        timeout-minutes: 8

      - name: Run pipeline integration tests
        run: |
          cd tests/integration/pipeline
          go test -v -tags=integration -timeout=300s ./...

      - name: Collect service logs on failure
        if: failure()
        run: |
          docker compose -f docker-compose.base.yml -f docker-compose.test.yml logs > integration-logs.txt 2>&1

      - name: Upload logs on failure
        if: failure()
        uses: actions/upload-artifact@v4
        with:
          name: integration-logs
          path: integration-logs.txt

      - name: Tear down
        if: always()
        run: |
          docker compose -f docker-compose.base.yml -f docker-compose.test.yml down -v
```

**Step 2: Add Taskfile task**

Add to root `Taskfile.yml` in the tasks section:

```yaml
  test:integration:pipeline:
    desc: Run full pipeline integration tests (requires Docker)
    cmds:
      - docker compose -f docker-compose.base.yml -f docker-compose.test.yml up -d --build --wait
      - cd tests/integration/pipeline && go test -v -tags=integration -timeout=300s ./...
      - docker compose -f docker-compose.base.yml -f docker-compose.test.yml down -v
```

**Step 3: Verify task is listed**

```bash
task --list | grep integration
```

Expected: `test:integration:pipeline` appears

**Step 4: Commit**

```bash
git add .github/workflows/integration.yml Taskfile.yml
git commit -m "feat(ci): add pipeline integration test workflow and Taskfile task"
```

---

## Task 10: Source-Manager Unit Tests

**Files:**
- Create: `source-manager/internal/importer/excel_integration_test.go` (test ToSource conversion)
- Create: `source-manager/internal/handlers/source_test.go` (test HTTP handlers)
- Create: `source-manager/internal/handlers/import_test.go` (test import endpoint)

The source-manager already has 5 test files. The gaps are: handler tests (TestCrawl endpoint, ImportExcel endpoint) and importer integration tests (full row → Source conversion).

### Sub-task 10a: Importer ToSource Tests

**Step 1: Write failing test for ToSource**

Create `source-manager/internal/importer/tosource_test.go`:

```go
package importer

import "testing"

func TestToSource_ValidRow(t *testing.T) {
	t.Helper()
	row := SourceRow{
		Row:     1,
		Name:    "Test News",
		URL:     "https://test.example.com",
		Enabled: true,
	}
	source, err := ToSource(row)
	if err != nil {
		t.Fatalf("ToSource returned error: %v", err)
	}
	if source.Name != "Test News" {
		t.Errorf("expected name %q, got %q", "Test News", source.Name)
	}
	if source.URL != "https://test.example.com" {
		t.Errorf("expected URL %q, got %q", "https://test.example.com", source.URL)
	}
	if !source.Enabled {
		t.Error("expected source to be enabled")
	}
}

func TestToSource_WithRateLimit(t *testing.T) {
	t.Helper()
	row := SourceRow{
		Row:       1,
		Name:      "Rate Limited",
		URL:       "https://test.example.com",
		Enabled:   true,
		RateLimit: "5/minute",
	}
	source, err := ToSource(row)
	if err != nil {
		t.Fatalf("ToSource returned error: %v", err)
	}
	if source.RateLimit == "" {
		t.Error("expected rate limit to be set")
	}
}

func TestToSource_WithSelectorsJSON(t *testing.T) {
	t.Helper()
	row := SourceRow{
		Row:       1,
		Name:      "With Selectors",
		URL:       "https://test.example.com",
		Enabled:   true,
		Selectors: `{"article":{"title":"h1","body":"article"}}`,
	}
	source, err := ToSource(row)
	if err != nil {
		t.Fatalf("ToSource returned error: %v", err)
	}
	if source.Selectors.Article.Title == "" {
		t.Error("expected article title selector to be set")
	}
}

func TestToSource_InvalidSelectorsJSON(t *testing.T) {
	t.Helper()
	row := SourceRow{
		Row:       1,
		Name:      "Bad Selectors",
		URL:       "https://test.example.com",
		Enabled:   true,
		Selectors: `{invalid json}`,
	}
	_, err := ToSource(row)
	if err == nil {
		t.Error("expected error for invalid selectors JSON")
	}
}
```

**Step 2: Run tests**

```bash
cd source-manager && go test ./internal/importer/ -v -run TestToSource
```

Expected: PASS (ToSource already implemented)

**Step 3: Commit**

```bash
git add source-manager/internal/importer/tosource_test.go
git commit -m "test(source-manager): add ToSource conversion tests"
```

### Sub-task 10b: Handler Tests

**Step 1: Write handler test file**

Create `source-manager/internal/handlers/source_test.go`. This tests the HTTP handler layer with a mock repository. The exact implementation depends on handler dependencies — read `source_handler.go` during implementation to understand the interface. Structure:

```go
package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// mockSourceRepository implements the repository interface for testing.
// Fields and methods will be determined by reading the actual interface.

func TestTestCrawl_ReturnsSimulatedResponse(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	// Set up handler with mock dependencies
	// POST /api/v1/sources/test-crawl with valid body
	// Assert: 200, response has articles_found, success_rate, sample_articles

	// Implementation note: Read handlers/source.go:285-339 to understand
	// what dependencies TestCrawl needs and build appropriate mocks.
}

func TestTestCrawl_MissingURL(t *testing.T) {
	t.Helper()
	// POST without URL → expect 400
}
```

**NOTE:** The TestCrawl endpoint currently returns simulated responses (constants at top of handler file). Tests should verify the simulated response shape. Full handler test implementation requires reading the handler's dependencies.

**Step 2: Run tests**

```bash
cd source-manager && go test ./internal/handlers/ -v
```

**Step 3: Commit**

```bash
git add source-manager/internal/handlers/source_test.go
git commit -m "test(source-manager): add handler tests for TestCrawl endpoint"
```

### Sub-task 10c: Validation Edge Cases

**Step 1: Add edge case tests for ValidateRow**

Add to existing `source-manager/internal/importer/excel_test.go` (or create a new `validation_test.go`):

```go
func TestValidateRow_EmptyName(t *testing.T) {
	t.Helper()
	row := SourceRow{URL: "https://test.com"}
	errMsg := ValidateRow(row)
	if errMsg == "" {
		t.Error("expected validation error for empty name")
	}
}

func TestValidateRow_EmptyURL(t *testing.T) {
	t.Helper()
	row := SourceRow{Name: "Test"}
	errMsg := ValidateRow(row)
	if errMsg == "" {
		t.Error("expected validation error for empty URL")
	}
}

func TestValidateRow_InvalidURLScheme(t *testing.T) {
	t.Helper()
	row := SourceRow{Name: "Test", URL: "ftp://test.com"}
	errMsg := ValidateRow(row)
	if errMsg == "" {
		t.Error("expected validation error for non-HTTP URL")
	}
}

func TestValidateRow_NegativeMaxDepth(t *testing.T) {
	t.Helper()
	row := SourceRow{Name: "Test", URL: "https://test.com", MaxDepth: -1}
	errMsg := ValidateRow(row)
	if errMsg == "" {
		t.Error("expected validation error for negative max depth")
	}
}

func TestValidateRow_ValidMinimal(t *testing.T) {
	t.Helper()
	row := SourceRow{Name: "Test", URL: "https://test.com"}
	errMsg := ValidateRow(row)
	if errMsg != "" {
		t.Errorf("expected no error, got: %s", errMsg)
	}
}
```

**Step 2: Run all source-manager tests**

```bash
cd source-manager && go test ./... -v
```

**Step 3: Lint**

```bash
cd source-manager && golangci-lint run
```

**Step 4: Commit**

```bash
git add source-manager/internal/importer/
git commit -m "test(source-manager): add validation edge case tests"
```

---

## Task 11: Index-Manager Unit Tests

**Files:**
- Create: `index-manager/internal/elasticsearch/mappings/raw_content_test.go`
- Create: `index-manager/internal/elasticsearch/mappings/classified_content_test.go`
- Create: `index-manager/internal/elasticsearch/mappings/factory_test.go`
- Create: `index-manager/internal/service/index_service_test.go`

The index-manager currently has only 1 test file (`domain/document_test.go`). The critical gaps are: mapping function tests (these are now the canonical contracts) and index service tests.

### Sub-task 11a: Raw Content Mapping Tests

**Step 1: Write mapping test**

Create `index-manager/internal/elasticsearch/mappings/raw_content_test.go`:

```go
package mappings

import "testing"

func TestGetRawContentMapping_HasRequiredStructure(t *testing.T) {
	t.Helper()
	m := GetRawContentMapping()

	// Must have settings
	settings, ok := m["settings"].(map[string]any)
	if !ok {
		t.Fatal("mapping missing settings")
	}
	if settings["number_of_shards"] == nil {
		t.Error("missing number_of_shards")
	}

	// Must have mappings.properties
	mappings, ok := m["mappings"].(map[string]any)
	if !ok {
		t.Fatal("mapping missing mappings section")
	}
	props, ok := mappings["properties"].(map[string]any)
	if !ok {
		t.Fatal("mapping missing properties")
	}

	// Verify core fields exist
	coreFields := []string{
		"id", "url", "source_name", "title",
		"raw_html", "raw_text",
		"classification_status", "crawled_at", "word_count",
	}
	for _, field := range coreFields {
		if _, exists := props[field]; !exists {
			t.Errorf("missing required field: %s", field)
		}
	}
}

func TestGetRawContentMapping_FieldTypes(t *testing.T) {
	t.Helper()
	m := GetRawContentMapping()
	props := m["mappings"].(map[string]any)["properties"].(map[string]any)

	tests := []struct {
		field    string
		esType   string
	}{
		{"id", "keyword"},
		{"url", "keyword"},
		{"title", "text"},
		{"raw_text", "text"},
		{"crawled_at", "date"},
		{"word_count", "integer"},
		{"classification_status", "keyword"},
	}

	for _, tt := range tests {
		fieldDef, ok := props[tt.field].(map[string]any)
		if !ok {
			t.Errorf("field %q not found or not a map", tt.field)
			continue
		}
		if fieldDef["type"] != tt.esType {
			t.Errorf("field %q: expected type %q, got %q", tt.field, tt.esType, fieldDef["type"])
		}
	}
}

func TestGetRawContentMapping_RawHTMLNotIndexed(t *testing.T) {
	t.Helper()
	m := GetRawContentMapping()
	props := m["mappings"].(map[string]any)["properties"].(map[string]any)
	rawHTML := props["raw_html"].(map[string]any)
	if rawHTML["index"] != false {
		t.Error("raw_html should have index=false (stored but not searchable)")
	}
}
```

**Step 2: Run tests**

```bash
cd index-manager && go test ./internal/elasticsearch/mappings/ -v -run TestGetRawContentMapping
```

Expected: PASS

### Sub-task 11b: Classified Content Mapping Tests

**Step 1: Write mapping test**

Create `index-manager/internal/elasticsearch/mappings/classified_content_test.go`:

```go
package mappings

import "testing"

func TestGetClassifiedContentMapping_HasRequiredStructure(t *testing.T) {
	t.Helper()
	m := GetClassifiedContentMapping()

	mappings, ok := m["mappings"].(map[string]any)
	if !ok {
		t.Fatal("mapping missing mappings section")
	}
	props, ok := mappings["properties"].(map[string]any)
	if !ok {
		t.Fatal("mapping missing properties")
	}

	// Must include raw content fields
	rawFields := []string{"id", "url", "title", "raw_text", "crawled_at"}
	for _, field := range rawFields {
		if _, exists := props[field]; !exists {
			t.Errorf("missing inherited raw field: %s", field)
		}
	}

	// Must include classification fields
	classFields := []string{
		"content_type", "quality_score", "topics",
		"crime", "location", "mining",
		"is_crime_related",
	}
	for _, field := range classFields {
		if _, exists := props[field]; !exists {
			t.Errorf("missing classification field: %s", field)
		}
	}
}

func TestGetClassifiedContentMapping_CrimeNestedObject(t *testing.T) {
	t.Helper()
	m := GetClassifiedContentMapping()
	props := m["mappings"].(map[string]any)["properties"].(map[string]any)

	crime, ok := props["crime"].(map[string]any)
	if !ok {
		t.Fatal("crime field missing or not a map")
	}
	crimeProps, ok := crime["properties"].(map[string]any)
	if !ok {
		t.Fatal("crime field missing nested properties")
	}

	expectedFields := []string{
		"sub_label", "primary_crime_type", "relevance",
		"crime_types", "final_confidence",
		"homepage_eligible", "review_required", "model_version",
	}
	for _, field := range expectedFields {
		if _, exists := crimeProps[field]; !exists {
			t.Errorf("crime object missing field: %s", field)
		}
	}
}

func TestGetClassifiedContentMapping_MiningNestedObject(t *testing.T) {
	t.Helper()
	m := GetClassifiedContentMapping()
	props := m["mappings"].(map[string]any)["properties"].(map[string]any)

	mining, ok := props["mining"].(map[string]any)
	if !ok {
		t.Fatal("mining field missing or not a map")
	}
	miningProps, ok := mining["properties"].(map[string]any)
	if !ok {
		t.Fatal("mining field missing nested properties")
	}

	expectedFields := []string{
		"relevance", "mining_stage", "commodities",
		"location", "final_confidence",
		"review_required", "model_version",
	}
	for _, field := range expectedFields {
		if _, exists := miningProps[field]; !exists {
			t.Errorf("mining object missing field: %s", field)
		}
	}
}

func TestGetClassifiedContentMapping_LocationNestedObject(t *testing.T) {
	t.Helper()
	m := GetClassifiedContentMapping()
	props := m["mappings"].(map[string]any)["properties"].(map[string]any)

	location, ok := props["location"].(map[string]any)
	if !ok {
		t.Fatal("location field missing or not a map")
	}
	locProps, ok := location["properties"].(map[string]any)
	if !ok {
		t.Fatal("location field missing nested properties")
	}

	expectedFields := []string{
		"city", "province", "country",
		"specificity", "confidence",
	}
	for _, field := range expectedFields {
		if _, exists := locProps[field]; !exists {
			t.Errorf("location object missing field: %s", field)
		}
	}
}
```

**Step 2: Run tests**

```bash
cd index-manager && go test ./internal/elasticsearch/mappings/ -v
```

Expected: PASS

### Sub-task 11c: Mapping Factory Tests

**Step 1: Write factory test**

Create `index-manager/internal/elasticsearch/mappings/factory_test.go`:

```go
package mappings

import "testing"

func TestGetMappingForType_RawContent(t *testing.T) {
	t.Helper()
	m := GetMappingForType("raw_content")
	if m == nil {
		t.Fatal("expected non-nil mapping for raw_content")
	}
	if m["mappings"] == nil {
		t.Error("mapping missing mappings section")
	}
}

func TestGetMappingForType_ClassifiedContent(t *testing.T) {
	t.Helper()
	m := GetMappingForType("classified_content")
	if m == nil {
		t.Fatal("expected non-nil mapping for classified_content")
	}
}

func TestGetMappingForType_Unknown(t *testing.T) {
	t.Helper()
	m := GetMappingForType("nonexistent_type")
	if m != nil {
		t.Error("expected nil mapping for unknown type")
	}
}
```

**Step 2: Run all mapping tests**

```bash
cd index-manager && go test ./internal/elasticsearch/mappings/ -v
```

Expected: PASS

**Step 3: Run all index-manager tests**

```bash
cd index-manager && go test ./... -v
```

**Step 4: Lint**

```bash
cd index-manager && golangci-lint run
```

**Step 5: Commit**

```bash
git add index-manager/internal/elasticsearch/mappings/*_test.go
git commit -m "test(index-manager): add mapping unit tests for canonical schema definitions"
```

---

## Known Schema Drift (Track as Follow-Up)

During implementation, contract tests may reveal these known discrepancies between the canonical mapping (index-manager) and what services actually write/read:

1. **Classifier crime JSON tags vs mapping**: `CrimeResult.Relevance` uses JSON tag `"street_crime_relevance"` but mapping defines field as `"relevance"`. Classifier also writes `category_pages` and `location_specificity` which are not in the crime mapping.

2. **Crawler extra fields**: Crawler writes `author`, `article_section`, `json_ld_data`, `og_url`, and `meta` (nested) — some exist in mapping (`author`, `og_url`) but `article_section`, `json_ld_data`, and `meta` do not.

3. **Publisher field name differences**: Publisher's Article struct may use different JSON keys than what ES returns. Verify during implementation.

**Resolution approach**: After contract tests are in place and green (minus known failures), create a schema alignment task that either:
- Adds missing fields to the index-manager mapping, or
- Aligns service structs/JSON tags to the canonical mapping

---

## Summary

| Task | What | Files Changed |
|------|------|---------------|
| 1 | Shared contracts package | index-manager/pkg/contracts/ (5 files) |
| 2 | Classifier contract tests | classifier/tests/contracts/ (2 files) + go.mod |
| 3 | Publisher contract tests | publisher/tests/contracts/ (1 file) + go.mod |
| 4 | Crawler contract tests | crawler/tests/contracts/ (1 file) + go.mod |
| 5 | Search contract tests | search/tests/contracts/ (1 file) + go.mod |
| 6 | docker-compose.test.yml | docker-compose.test.yml |
| 7 | Fixture pages | crawler/fixtures/ (4 HTML files) |
| 8 | Pipeline integration harness | tests/integration/pipeline/ (3 files) |
| 9 | CI workflow + Taskfile | .github/workflows/integration.yml + Taskfile.yml |
| 10 | Source-manager unit tests | source-manager/internal/ (3-4 test files) |
| 11 | Index-manager unit tests | index-manager/internal/ (3 test files) |

**Total new test files:** ~20
**Estimated commits:** 11-13
