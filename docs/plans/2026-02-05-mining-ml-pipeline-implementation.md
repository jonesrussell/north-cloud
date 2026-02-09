# Mining-ML Pipeline Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Get the mining-ML classification pipeline running end-to-end: crawl mining sites → classify with hybrid rules+ML → publish to Redis `articles:mining` → Orewire consumes.

**Architecture:** Add mining-ml sidecar to Docker, wire mining classifier into the classifier service pipeline, add Layer 5 mining routing to the publisher that reads the nested `mining` ES field, and configure production Redis connectivity for Orewire.

**Tech Stack:** Go 1.25+, Python 3.11 (FastAPI), Docker Compose, Elasticsearch, Redis Pub/Sub, PostgreSQL

---

### Task 1: Add mining-ml to docker-compose.base.yml

**Files:**
- Modify: `docker-compose.base.yml:311` (after crime-ml service block)

**Step 1: Add mining-ml service definition**

Add after the `crime-ml` service block (after line 311):

```yaml
  # ------------------------------------------------------------
  # Mining ML Classifier
  # ------------------------------------------------------------

  mining-ml:
    <<: *service-defaults
    build:
      context: ./mining-ml
      dockerfile: Dockerfile
    image: docker.io/jonesrussell/mining-ml:latest
    deploy:
      resources:
        limits:
          cpus: "0.5"
          memory: 512M
    environment:
      MODEL_PATH: /app/models
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8077/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 15s
```

**Step 2: Verify base compose syntax**

Run: `docker compose -f docker-compose.base.yml config --quiet`
Expected: No errors

**Step 3: Commit**

```bash
git add docker-compose.base.yml
git commit -m "feat: add mining-ml service to docker-compose.base.yml"
```

---

### Task 2: Add mining-ml to docker-compose.dev.yml

**Files:**
- Modify: `docker-compose.dev.yml:444` (after crime-ml dev block)

**Step 1: Add mining-ml dev overrides**

Add after the crime-ml dev block (after line 444):

```yaml
  # ------------------------------------------------------------
  # Mining ML Classifier (Dev)
  # ------------------------------------------------------------
  mining-ml:
    ports:
      - "${MINING_ML_PORT:-8077}:8077"
```

**Step 2: Add mining env vars to classifier dev block**

In the classifier service environment block (after line 216, add after `PPROF_PORT: 6060`):

```yaml
      MINING_ENABLED: "${MINING_ENABLED:-true}"
      MINING_ML_SERVICE_URL: "http://mining-ml:8077"
```

**Step 3: Verify dev compose syntax**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml config --quiet`
Expected: No errors

**Step 4: Commit**

```bash
git add docker-compose.dev.yml
git commit -m "feat: add mining-ml dev config and classifier mining env vars"
```

---

### Task 3: Add mining-ml to docker-compose.prod.yml

**Files:**
- Modify: `docker-compose.prod.yml:179` (classifier depends_on block)
- Modify: `docker-compose.prod.yml:197` (classifier environment block)

**Step 1: Add mining-ml to classifier depends_on**

After line 180 (`crime-ml: condition: service_healthy`), add:

```yaml
      mining-ml:
        condition: service_healthy
```

**Step 2: Add mining env vars to classifier**

After line 196 (`CRIME_ML_SERVICE_URL: http://crime-ml:8076`), add:

```yaml
      MINING_ENABLED: "true"
      MINING_ML_SERVICE_URL: http://mining-ml:8077
```

**Step 3: Add mining-ml prod service block**

Add after the crime-ml prod block (find it or add at end of services):

```yaml
  # ------------------------------------------------------------
  # Mining ML Classifier (Prod)
  # ------------------------------------------------------------
  mining-ml:
    <<: *prod-defaults
    image: docker.io/jonesrussell/mining-ml:latest
```

**Step 4: Verify prod compose syntax**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.prod.yml config --quiet`
Expected: No errors

**Step 5: Commit**

```bash
git add docker-compose.prod.yml
git commit -m "feat: add mining-ml to production docker-compose"
```

---

### Task 4: Wire MiningClassifier into classifier service

**Files:**
- Modify: `classifier/internal/classifier/classifier.go:33-58`
- Modify: `classifier/internal/bootstrap/classifier.go:155-174`

**Step 1: Add MiningClassifier to Config struct**

In `classifier/internal/classifier/classifier.go`, add to Config struct (line 39, after CrimeClassifier):

```go
	MiningClassifier *MiningClassifier // Optional: hybrid mining classifier
```

**Step 2: Add mining field to Classifier struct**

In the same file, add to Classifier struct (line 27, after crime):

```go
	mining *MiningClassifier
```

**Step 3: Wire mining in NewClassifier**

In the same file, add to the return struct in NewClassifier (line 54, after crime):

```go
		mining: config.MiningClassifier,
```

**Step 4: Add mining classification step in Classify method**

In the same file, after the crime classification block (after line 106, before line 108 "// 6. Location Classification"):

```go
	// 6. Mining Classification (if enabled)
	var miningResult *domain.MiningResult
	if c.mining != nil {
		minResult, minErr := c.mining.Classify(ctx, raw)
		if minErr != nil {
			c.logger.Warn("Mining classification failed",
				infralogger.String("content_id", raw.ID),
				infralogger.Error(minErr))
		} else if minResult != nil {
			miningResult = minResult
		}
	}
```

Renumber "6. Location Classification" to "7. Location Classification".

**Step 5: Add mining result to ClassificationResult**

In the same file, add to the result struct (line 156, after Crime):

```go
		Mining:   miningResult,
```

**Step 6: Run tests to verify nothing breaks**

Run: `cd classifier && go test ./internal/classifier/... -v`
Expected: All existing tests pass

**Step 7: Commit**

```bash
git add classifier/internal/classifier/classifier.go
git commit -m "feat(classifier): wire MiningClassifier into classification pipeline"
```

---

### Task 5: Add createMiningClassifier to bootstrap

**Files:**
- Modify: `classifier/internal/bootstrap/classifier.go:135-174`

**Step 1: Add import for miningmlclient**

In `classifier/internal/bootstrap/classifier.go`, add to imports (after line 13):

```go
	"github.com/jonesrussell/north-cloud/classifier/internal/miningmlclient"
```

**Step 2: Add MiningClassifier to createClassifierConfig**

In `createClassifierConfig` function (line 155, after CrimeClassifier):

```go
		MiningClassifier: createMiningClassifier(cfg, logger),
```

**Step 3: Add createMiningClassifier function**

After the `createCrimeClassifier` function (after line 174):

```go
// createMiningClassifier creates a Mining classifier if enabled in config.
func createMiningClassifier(cfg *config.Config, logger infralogger.Logger) *classifier.MiningClassifier {
	if !cfg.Classification.Mining.Enabled {
		return nil
	}

	var mlClient classifier.MiningMLClassifier
	if cfg.Classification.Mining.MLServiceURL != "" {
		mlClient = miningmlclient.NewClient(cfg.Classification.Mining.MLServiceURL)
	}

	logger.Info("Mining classifier enabled",
		infralogger.String("ml_service_url", cfg.Classification.Mining.MLServiceURL))

	return classifier.NewMiningClassifier(mlClient, logger, true)
}
```

**Step 4: Run tests**

Run: `cd classifier && go test ./... -v -count=1`
Expected: All tests pass

**Step 5: Run linter**

Run: `cd classifier && golangci-lint run`
Expected: No errors

**Step 6: Commit**

```bash
git add classifier/internal/bootstrap/classifier.go
git commit -m "feat(classifier): add mining classifier bootstrap wiring"
```

---

### Task 6: Add mining mapping to classifier ES mappings

**Files:**
- Modify: `classifier/internal/elasticsearch/mappings/classified_content.go:20-84,148-224`

**Step 1: Add MiningProperties struct**

After `LocationFieldProperties` struct (after line 117):

```go
// MiningProperties defines the nested properties for mining classification.
type MiningProperties struct {
	Type       string                `json:"type,omitempty"`
	Properties MiningFieldProperties `json:"properties,omitempty"`
}

// MiningFieldProperties defines individual fields within mining classification.
type MiningFieldProperties struct {
	Relevance       Field `json:"relevance"`
	MiningStage     Field `json:"mining_stage"`
	Commodities     Field `json:"commodities"`
	Location        Field `json:"location"`
	FinalConfidence Field `json:"final_confidence"`
	ReviewRequired  Field `json:"review_required"`
	ModelVersion    Field `json:"model_version"`
}
```

**Step 2: Add Mining field to ClassifiedContentProperties**

After `Location` field (line 83):

```go
	// Mining classification (hybrid rule + ML)
	Mining MiningProperties `json:"mining,omitempty"`
```

**Step 3: Add createMiningProperties function**

After `createLocationProperties` (after line 198):

```go
// createMiningProperties creates nested properties for mining classification.
func createMiningProperties() MiningProperties {
	return MiningProperties{
		Type: "object",
		Properties: MiningFieldProperties{
			Relevance:       Field{Type: "keyword"},
			MiningStage:     Field{Type: "keyword"},
			Commodities:     Field{Type: "keyword"},
			Location:        Field{Type: "keyword"},
			FinalConfidence: Field{Type: "float"},
			ReviewRequired:  Field{Type: "boolean"},
			ModelVersion:    Field{Type: "keyword"},
		},
	}
}
```

**Step 4: Wire into createClassificationProperties**

In `createClassificationProperties()` (line 165, after Location):

```go
		Mining:   createMiningProperties(),
```

**Step 5: Wire into mergeProperties**

In `mergeProperties()` (line 222, after Location):

```go
		Mining: classified.Mining,
```

**Step 6: Run tests**

Run: `cd classifier && go test ./internal/elasticsearch/... -v`
Expected: All tests pass

**Step 7: Commit**

```bash
git add classifier/internal/elasticsearch/mappings/classified_content.go
git commit -m "feat(classifier): add mining mapping to classified content ES schema"
```

---

### Task 7: Add mining fields to publisher Article struct

**Files:**
- Modify: `publisher/internal/router/service.go:196-246,377-421`

**Step 1: Add MiningData struct**

Before the Article struct (before line 197):

```go
// MiningData holds mining classification fields from Elasticsearch.
type MiningData struct {
	Relevance       string   `json:"relevance"`
	MiningStage     string   `json:"mining_stage"`
	Commodities     []string `json:"commodities"`
	Location        string   `json:"location"`
	FinalConfidence float64  `json:"final_confidence"`
	ReviewRequired  bool     `json:"review_required"`
	ModelVersion    string   `json:"model_version,omitempty"`
}
```

**Step 2: Add Mining field to Article struct**

After the Location detection fields (after line 228):

```go
	// Mining classification (hybrid rule + ML)
	Mining *MiningData `json:"mining,omitempty"`
```

**Step 3: Add mining to publishToChannel payload**

In `publishToChannel`, after the location detection fields (after line 420):

```go
		// Mining classification
		"mining": article.Mining,
```

**Step 4: Run tests**

Run: `cd publisher && go test ./internal/router/... -v`
Expected: All existing tests pass

**Step 5: Commit**

```bash
git add publisher/internal/router/service.go
git commit -m "feat(publisher): add mining fields to Article struct and publish payload"
```

---

### Task 8: Create publisher mining channel generator

**Files:**
- Create: `publisher/internal/router/mining.go`
- Create: `publisher/internal/router/mining_test.go`

**Step 1: Write the test file**

Create `publisher/internal/router/mining_test.go`:

```go
//nolint:testpackage // Testing internal router requires same package access
package router

import (
	"testing"
)

func TestGenerateMiningChannels_CoreMining(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:    "test-mining-1",
		Title: "Gold drill results show high grade intercepts",
		Mining: &MiningData{
			Relevance:   "core_mining",
			MiningStage: "exploration",
			Commodities: []string{"gold"},
			Location:    "local_canada",
		},
	}

	channels := GenerateMiningChannels(article)

	if len(channels) != 1 || channels[0] != "articles:mining" {
		t.Errorf("expected [articles:mining], got %v", channels)
	}
}

func TestGenerateMiningChannels_PeripheralMining(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:    "test-mining-2",
		Title: "Mining industry overview",
		Mining: &MiningData{
			Relevance: "peripheral_mining",
		},
	}

	channels := GenerateMiningChannels(article)

	if len(channels) != 1 || channels[0] != "articles:mining" {
		t.Errorf("expected [articles:mining], got %v", channels)
	}
}

func TestGenerateMiningChannels_NotMining(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:    "test-mining-3",
		Title: "Weather forecast",
		Mining: &MiningData{
			Relevance: "not_mining",
		},
	}

	channels := GenerateMiningChannels(article)

	if len(channels) != 0 {
		t.Errorf("expected no channels for not_mining, got %v", channels)
	}
}

func TestGenerateMiningChannels_NilMining(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:    "test-mining-4",
		Title: "Regular news article",
	}

	channels := GenerateMiningChannels(article)

	if len(channels) != 0 {
		t.Errorf("expected no channels for nil mining, got %v", channels)
	}
}

func TestGenerateMiningChannels_EmptyRelevance(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:     "test-mining-5",
		Title:  "Article with empty mining",
		Mining: &MiningData{},
	}

	channels := GenerateMiningChannels(article)

	if len(channels) != 0 {
		t.Errorf("expected no channels for empty relevance, got %v", channels)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd publisher && go test ./internal/router/... -v -run TestGenerateMining`
Expected: FAIL - `GenerateMiningChannels` not defined

**Step 3: Create mining.go**

Create `publisher/internal/router/mining.go`:

```go
// publisher/internal/router/mining.go
package router

// Mining relevance constants.
const (
	MiningRelevanceNotMining   = "not_mining"
	MiningRelevancePeripheral  = "peripheral_mining"
	MiningRelevanceCoreMining  = "core_mining"
)

// GenerateMiningChannels returns the Redis channels for articles with mining classification.
// All core_mining and peripheral_mining articles publish to a single articles:mining channel.
func GenerateMiningChannels(article *Article) []string {
	if article.Mining == nil {
		return nil
	}

	if article.Mining.Relevance == MiningRelevanceNotMining || article.Mining.Relevance == "" {
		return nil
	}

	return []string{"articles:mining"}
}
```

**Step 4: Run tests to verify they pass**

Run: `cd publisher && go test ./internal/router/... -v -run TestGenerateMining`
Expected: All 5 tests PASS

**Step 5: Run linter**

Run: `cd publisher && golangci-lint run`
Expected: No errors

**Step 6: Commit**

```bash
git add publisher/internal/router/mining.go publisher/internal/router/mining_test.go
git commit -m "feat(publisher): add Layer 5 mining channel generator"
```

---

### Task 9: Wire mining channels into routeArticle

**Files:**
- Modify: `publisher/internal/router/service.go:167-194`

**Step 1: Add Layer 5 mining routing**

In `routeArticle()`, after Layer 4 location channels (after line 193):

```go
	// Layer 5: Mining classification channels
	miningChannels := GenerateMiningChannels(article)
	for _, channel := range miningChannels {
		s.publishToChannel(ctx, article, channel, nil)
	}
```

**Step 2: Run all publisher tests**

Run: `cd publisher && go test ./... -v`
Expected: All tests pass

**Step 3: Run linter**

Run: `cd publisher && golangci-lint run`
Expected: No errors

**Step 4: Commit**

```bash
git add publisher/internal/router/service.go
git commit -m "feat(publisher): wire Layer 5 mining channels into routing loop"
```

---

### Task 10: Add mining topics to publisher metadata

**Files:**
- Modify: `publisher/internal/api/metadata_handler.go:22-39`

**Step 1: Add mining topics to knownTopics**

After the crime sub-categories block (after line 28), add:

```go
	// Mining topics
	"mining",
	"core_mining",
	"peripheral_mining",
```

**Step 2: Run linter**

Run: `cd publisher && golangci-lint run`
Expected: No errors

**Step 3: Commit**

```bash
git add publisher/internal/api/metadata_handler.go
git commit -m "feat(publisher): add mining topics to known topics metadata"
```

---

### Task 11: Update publisher CLAUDE.md with Layer 5

**Files:**
- Modify: `publisher/CLAUDE.md`

**Step 1: Update the Channel Model section**

Add after Layer 3 description:

```
- **Layer 4 (location channels)**: Automatic channels for geographic routing of crime content (`crime:local:{city}`, `crime:province:{code}`, `crime:canada`, `crime:international`).
- **Layer 5 (mining classification channels)**: Automatic channel based on classifier's hybrid rule + ML mining classification. Routes to `articles:mining` for all `core_mining` and `peripheral_mining` articles. Mining metadata (relevance, stage, commodities, location) included in payload for downstream filtering.
```

**Step 2: Add mining channel pattern to Redis section**

```
**Layer 5 Mining Channel**:
- `articles:mining` - All mining-related articles (core + peripheral)
```

**Step 3: Add mining fields to message format**

```
**Mining Classification Fields** (from classifier's hybrid rule + ML):
| Field | Type | Description |
|-------|------|-------------|
| `mining.relevance` | string | `core_mining`, `peripheral_mining`, or `not_mining` |
| `mining.mining_stage` | string | `exploration`, `development`, `production`, `unspecified` |
| `mining.commodities` | []string | Multi-label: `gold`, `copper`, `lithium`, `nickel`, `uranium`, `iron_ore`, `rare_earths`, `other` |
| `mining.location` | string | `local_canada`, `national_canada`, `international`, `not_specified` |
| `mining.final_confidence` | float | 0.0-1.0 confidence score |
| `mining.review_required` | bool | True if rules and ML disagreed |
```

**Step 4: Commit**

```bash
git add publisher/CLAUDE.md
git commit -m "docs(publisher): add Layer 5 mining channels to CLAUDE.md"
```

---

### Task 12: Build and test locally

**Step 1: Start mining-ml service**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d mining-ml`

**Step 2: Verify mining-ml health**

Run: `curl -s http://localhost:8077/health | jq .`
Expected: `{"status": "ok", "models_loaded": true, ...}`

**Step 3: Rebuild classifier with mining enabled**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d --build classifier`

**Step 4: Check classifier logs for mining enabled**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs classifier | grep -i mining`
Expected: "Mining classifier enabled" log line

**Step 5: Rebuild publisher**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d --build publisher`

**Step 6: Commit all remaining changes**

```bash
git add -A
git commit -m "feat: mining-ml pipeline local integration verified"
```

---

### Task 13: Production deployment preparation

> **Note:** Tasks 13-15 involve production changes. Confirm with user before proceeding.

**Step 1: SSH to production and backup databases**

```bash
ssh jones@northcloud.biz
cd /opt/north-cloud  # or wherever deployed
task db:backup:source-manager
task db:backup:crawler
task db:backup:publisher
```

**Step 2: Pull latest code on production**

```bash
git pull origin main
```

**Step 3: Build and deploy mining-ml + updated services**

```bash
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml up -d --build mining-ml classifier publisher
```

**Step 4: Verify mining-ml health on production**

```bash
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml logs mining-ml | tail -20
```

---

### Task 14: Configure Orewire Redis connection

**Step 1: SSH as deployer and update .env**

```bash
ssh deployer@orewire.ca
cd ~/orewire-laravel/current
```

Edit `.env` and add/update:
```
NORTHCLOUD_REDIS_HOST=127.0.0.1
NORTHCLOUD_REDIS_PORT=6379
```

**Step 2: Enable and start the mining consumer systemd service**

```bash
systemctl --user enable orewire-mining-consumer
systemctl --user start orewire-mining-consumer
```

**Step 3: Verify consumer is running**

```bash
systemctl --user status orewire-mining-consumer
journalctl --user -u orewire-mining-consumer -f
```

Expected: Service running, waiting for messages on `articles:mining`

---

### Task 15: Import mining sites from Excel

> **Prerequisite:** User provides the Excel spreadsheet with mining sites.

**Step 1: Confirm databases are backed up** (from Task 13)

**Step 2: Upload Excel via source-manager**

Use the dashboard or API:
```bash
curl -X POST http://localhost:8050/api/v1/sources/import-excel \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@mining-sites.xlsx"
```

**Step 3: Verify sources created**

```bash
curl -s http://localhost:8050/api/v1/sources \
  -H "Authorization: Bearer $TOKEN" | jq '.[] | .name'
```

**Step 4: Create crawl jobs for mining sources**

For each source, schedule crawl jobs via the crawler API or MCP tools.

**Step 5: Monitor pipeline**

- Watch crawler logs: `docker logs north-cloud-crawler -f`
- Watch classifier logs: `docker logs north-cloud-classifier -f | grep mining`
- Watch publisher logs: `docker logs north-cloud-publisher -f | grep articles:mining`
- Watch Orewire consumer: `journalctl --user -u orewire-mining-consumer -f`

---

## Summary

| Task | Service | Type | Est. Lines |
|------|---------|------|------------|
| 1-3 | Infrastructure | Docker config | ~30 |
| 4-5 | Classifier | Go (wiring) | ~25 |
| 6 | Classifier | Go (ES mapping) | ~35 |
| 7 | Publisher | Go (struct) | ~15 |
| 8 | Publisher | Go (new file + tests) | ~90 |
| 9-10 | Publisher | Go (wiring) | ~10 |
| 11 | Publisher | Docs | ~20 |
| 12 | All | Integration test | Commands |
| 13-15 | Production | Deployment | Commands |

**Total new/modified code:** ~225 lines across 3 services

**Dependencies:**
- Tasks 1-3 (Docker) are independent, do first
- Tasks 4-6 (Classifier) depend on Task 1 (mining-ml must exist in compose)
- Tasks 7-11 (Publisher) are independent of classifier tasks
- Task 12 depends on all code tasks (1-11)
- Tasks 13-15 depend on Task 12
