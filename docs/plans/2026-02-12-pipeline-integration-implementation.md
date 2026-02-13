# Pipeline Service Integration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Wire the existing `infrastructure/pipeline.Client` into crawler, classifier, and publisher so they emit `indexed`, `classified`, and `published` events to the pipeline observability service.

**Architecture:** Each service gets a `pipeline.Client` injected via constructor. The client is fire-and-forget with a 2s timeout and circuit breaker. When `PIPELINE_URL` is empty (default), the client no-ops. Events are emitted inline after each stage completes.

**Tech Stack:** Go, `infrastructure/pipeline` client library, Docker Compose env vars

**Design doc:** `docs/plans/2026-02-12-pipeline-integration-design.md`

---

### Task 1: Crawler — Add pipeline client to RawContentService

**Files:**
- Modify: `crawler/internal/content/rawcontent/service.go`
- Modify: `crawler/internal/crawler/constructor.go`
- Modify: `crawler/internal/crawler/constructor.go` (CrawlerParams)

**Step 1: Add pipeline client field and constructor parameter to RawContentService**

In `crawler/internal/content/rawcontent/service.go`, add `pipeline *pipeline.Client` to the struct and constructor:

```go
// Add import
"github.com/north-cloud/infrastructure/pipeline"

// Add field to RawContentService struct (after rawIndexer)
pipeline *pipeline.Client

// Update NewRawContentService signature
func NewRawContentService(
	log infralogger.Logger,
	storage types.Interface,
	sourcesManager sources.Interface,
	pipelineClient *pipeline.Client,
) *RawContentService {
	rawIndexer := storagepkg.NewRawContentIndexer(storage, log)
	return &RawContentService{
		logger:     log,
		storage:    storage,
		sources:    sourcesManager,
		rawIndexer: rawIndexer,
		pipeline:   pipelineClient,
	}
}
```

**Step 2: Add pipeline emission to Process()**

In `crawler/internal/content/rawcontent/service.go`, add emission after the successful `IndexRawContent()` call (after line 94, before the debug log on line 96):

```go
// Emit pipeline event (fire-and-forget)
if s.pipeline != nil {
	indexName := sourceName + "_raw_content"
	pipelineErr := s.pipeline.Emit(ctx, pipeline.Event{
		ArticleURL: sourceURL,
		SourceName: sourceName,
		Stage:      "indexed",
		OccurredAt: time.Now(),
		Metadata: map[string]any{
			"title":       rawData.Title,
			"word_count":  rawContent.WordCount,
			"index_name":  indexName,
			"document_id": rawContent.ID,
		},
	})
	if pipelineErr != nil {
		s.logger.Warn("Failed to emit pipeline event",
			infralogger.Error(pipelineErr),
			infralogger.String("url", sourceURL),
			infralogger.String("stage", "indexed"),
		)
	}
}
```

**Step 3: Update constructor.go to pass pipeline client**

In `crawler/internal/crawler/constructor.go`:

Add `PipelineClient *pipeline.Client` field to `CrawlerParams` struct (after `DB`):

```go
PipelineClient *pipeline.Client // Pipeline observability client (optional)
```

Add import: `"github.com/north-cloud/infrastructure/pipeline"`

Update the `NewCrawlerWithParams` call to `NewRawContentService` (line 56-60):

```go
rawContentService := rawcontent.NewRawContentService(
	p.Logger,
	p.Storage,
	p.Sources,
	p.PipelineClient,
)
```

**Step 4: Run lint and tests**

Run: `cd /home/fsd42/dev/north-cloud/crawler && golangci-lint run`
Expected: 0 errors (pipeline client is nil-safe)

Run: `cd /home/fsd42/dev/north-cloud/crawler && go test ./...`
Expected: All tests pass (existing tests don't use pipeline client, `nil` is handled)

**Step 5: Commit**

```bash
git add crawler/internal/content/rawcontent/service.go crawler/internal/crawler/constructor.go
git commit -m "feat(crawler): emit 'indexed' pipeline event after raw content indexing"
```

---

### Task 2: Crawler — Wire pipeline client in bootstrap

**Files:**
- Modify: `crawler/internal/bootstrap/services.go`
- Modify: `crawler/internal/config/config.go` (Interface + Config struct)

**Step 1: Add PipelineURL to crawler config**

In `crawler/internal/config/config.go`, add to the `Interface`:

```go
// GetPipelineURL returns the pipeline service URL (empty = disabled).
GetPipelineURL() string
```

Find the `Config` struct implementation file. Add a `PipelineURL` field with env tag. Then implement `GetPipelineURL()`.

> **Note:** The crawler config uses an `Interface` pattern. Find the concrete struct (likely in `config.go` or a sub-package) and add:
> ```go
> Pipeline struct {
> 	URL string `env:"PIPELINE_URL" yaml:"url"`
> } `yaml:"pipeline"`
> ```
> And the getter:
> ```go
> func (c *Config) GetPipelineURL() string { return c.Pipeline.URL }
> ```

**Step 2: Create pipeline client in bootstrap and pass to crawler**

In `crawler/internal/bootstrap/services.go`, update `createCrawler()` (line 243-265):

Add import: `"github.com/north-cloud/infrastructure/pipeline"`

Before the `crawler.NewCrawlerWithParams()` call, create the client:

```go
pipelineClient := pipeline.NewClient(deps.Config.GetPipelineURL(), "crawler")
```

Add `PipelineClient: pipelineClient` to `CrawlerParams` in the `createCrawler()` function:

```go
crawlerResult, err := crawler.NewCrawlerWithParams(crawler.CrawlerParams{
	Logger:         deps.Logger,
	Bus:            bus,
	IndexManager:   storage.IndexManager,
	Sources:        sourceManager,
	Config:         crawlerCfg,
	Storage:        storage.Storage,
	FullConfig:     deps.Config,
	DB:             db,
	PipelineClient: pipelineClient,
})
```

**Step 3: Build and test**

Run: `cd /home/fsd42/dev/north-cloud/crawler && go build ./...`
Expected: Clean build

Run: `cd /home/fsd42/dev/north-cloud/crawler && golangci-lint run`
Expected: 0 errors

Run: `cd /home/fsd42/dev/north-cloud/crawler && go test ./...`
Expected: All pass

**Step 4: Commit**

```bash
git add crawler/internal/bootstrap/services.go crawler/internal/config/
git commit -m "feat(crawler): wire pipeline client through bootstrap config"
```

---

### Task 3: Classifier — Add pipeline client to Poller

**Files:**
- Modify: `classifier/internal/processor/poller.go`

**Step 1: Add pipeline client field and constructor parameter**

In `classifier/internal/processor/poller.go`:

Add import: `"github.com/north-cloud/infrastructure/pipeline"`

Add field to `Poller` struct (after `logger`):

```go
pipeline *pipeline.Client
```

Update `NewPoller` signature and body:

```go
func NewPoller(
	esClient ElasticsearchClient,
	dbClient DatabaseClient,
	batchProcessor *BatchProcessor,
	logger infralogger.Logger,
	config PollerConfig,
	pipelineClient *pipeline.Client,
) *Poller {
```

Add `pipeline: pipelineClient,` to the return struct.

**Step 2: Add pipeline emission to indexResults()**

In `classifier/internal/processor/poller.go`, in `indexResults()`, after the status update loop (after line 235, before the success log on line 237), emit batch events:

```go
// Emit pipeline events for classified content (fire-and-forget)
if p.pipeline != nil && len(classifiedContents) > 0 {
	events := make([]pipeline.Event, 0, len(classifiedContents))
	for _, content := range classifiedContents {
		events = append(events, pipeline.Event{
			ArticleURL: content.URL,
			SourceName: content.SourceName,
			Stage:      "classified",
			OccurredAt: time.Now(),
			Metadata: map[string]any{
				"quality_score": content.QualityScore,
				"topics":        content.Topics,
				"content_type":  content.ContentType,
			},
		})
	}
	if emitErr := p.pipeline.EmitBatch(ctx, events); emitErr != nil {
		p.logger.Warn("Failed to emit pipeline events",
			infralogger.Error(emitErr),
			infralogger.Int("count", len(events)),
			infralogger.String("stage", "classified"),
		)
	}
}
```

**Step 3: Update existing tests that call NewPoller**

Three files create `NewPoller`:
- `classifier/internal/processor/integration_test.go:430`
- `classifier/internal/processor/poller_test.go` (if it calls NewPoller)
- `classifier/cmd/processor/processor.go:280` and `:361`

Add `nil` as the last argument to each `NewPoller()` call in test files:

```go
// Before:
poller := NewPoller(esClient, dbClient, batchProcessor, logger, pollerConfig)
// After:
poller := NewPoller(esClient, dbClient, batchProcessor, logger, pollerConfig, nil)
```

**Step 4: Run lint and tests**

Run: `cd /home/fsd42/dev/north-cloud/classifier && golangci-lint run`
Expected: 0 errors

Run: `cd /home/fsd42/dev/north-cloud/classifier && go test ./...`
Expected: All pass

**Step 5: Commit**

```bash
git add classifier/internal/processor/poller.go classifier/internal/processor/integration_test.go classifier/internal/processor/poller_test.go
git commit -m "feat(classifier): emit 'classified' pipeline events after bulk indexing"
```

---

### Task 4: Classifier — Wire pipeline client in bootstrap

**Files:**
- Modify: `classifier/cmd/processor/processor.go`
- Modify: `classifier/internal/config/config.go`

**Step 1: Add PipelineURL to classifier config**

In `classifier/internal/config/config.go`, add to `ServiceConfig` struct:

```go
PipelineURL string `env:"PIPELINE_URL" yaml:"pipeline_url"`
```

**Step 2: Wire pipeline client into ProcessorConfig and both Start functions**

In `classifier/cmd/processor/processor.go`:

Add import: `"github.com/north-cloud/infrastructure/pipeline"`

Add to `ProcessorConfig` struct:

```go
PipelineURL string
```

Update `LoadConfig()` to include it in the return:

```go
return &ProcessorConfig{
	// ... existing fields ...
	PipelineURL: cfg.Service.PipelineURL,
}, cfg
```

In `Start()` function, create the client and pass to `NewPoller` (around line 280-286):

```go
pipelineClient := pipeline.NewClient(cfg.PipelineURL, "classifier")
poller := processor.NewPoller(
	esStorage,
	dbAdapter,
	batchProcessor,
	procLogger,
	pollerConfig,
	pipelineClient,
)
```

Apply the same change in `StartWithStop()` (around line 361-367).

**Step 3: Build and test**

Run: `cd /home/fsd42/dev/north-cloud/classifier && go build ./...`
Expected: Clean build

Run: `cd /home/fsd42/dev/north-cloud/classifier && golangci-lint run`
Expected: 0 errors

Run: `cd /home/fsd42/dev/north-cloud/classifier && go test ./...`
Expected: All pass

**Step 4: Commit**

```bash
git add classifier/cmd/processor/processor.go classifier/internal/config/config.go
git commit -m "feat(classifier): wire pipeline client through processor config"
```

---

### Task 5: Publisher — Add pipeline client to router Service

**Files:**
- Modify: `publisher/internal/router/service.go`

**Step 1: Add pipeline client field and constructor parameter**

In `publisher/internal/router/service.go`:

Add import: `"github.com/north-cloud/infrastructure/pipeline"`

Add field to `Service` struct (after `lastSort`):

```go
pipeline *pipeline.Client
```

Update `NewService` signature:

```go
func NewService(
	repo *database.Repository,
	disc *discovery.Service,
	esClient *elasticsearch.Client,
	redisClient *redis.Client,
	cfg Config,
	logger infralogger.Logger,
	pipelineClient *pipeline.Client,
) *Service {
```

Add `pipeline: pipelineClient,` to the return struct.

**Step 2: Modify routeArticle() to collect channels and emit**

In `publisher/internal/router/service.go`, in `routeArticle()` (line 175-216):

Add a `publishedChannels` slice at the top of the function to collect all channels the article was published to. After all 6 layers, emit a single `published` event.

```go
func (s *Service) routeArticle(ctx context.Context, article *Article, channels []models.Channel) {
	var publishedChannels []string

	// Layer 1: Automatic topic channels (skip topics with dedicated layers)
	for _, topic := range article.Topics {
		if layer1SkipTopics[topic] {
			continue
		}
		channel := fmt.Sprintf("articles:%s", topic)
		if s.publishToChannel(ctx, article, channel, nil) {
			publishedChannels = append(publishedChannels, channel)
		}
	}

	// Layer 2: Custom channels
	for i := range channels {
		ch := &channels[i]
		if ch.Rules.Matches(article.QualityScore, article.ContentType, article.Topics) {
			if s.publishToChannel(ctx, article, ch.RedisChannel, &ch.ID) {
				publishedChannels = append(publishedChannels, ch.RedisChannel)
			}
		}
	}

	// Layer 3: Crime classification channels
	crimeChannels := GenerateCrimeChannels(article)
	for _, channel := range crimeChannels {
		if s.publishToChannel(ctx, article, channel, nil) {
			publishedChannels = append(publishedChannels, channel)
		}
	}

	// Layer 4: Location-based channels
	locationChannels := GenerateLocationChannels(article)
	for _, channel := range locationChannels {
		if s.publishToChannel(ctx, article, channel, nil) {
			publishedChannels = append(publishedChannels, channel)
		}
	}

	// Layer 5: Mining classification channels
	miningChannels := GenerateMiningChannels(article)
	for _, channel := range miningChannels {
		if s.publishToChannel(ctx, article, channel, nil) {
			publishedChannels = append(publishedChannels, channel)
		}
	}

	// Layer 6: Entertainment classification channels
	entertainmentChannels := GenerateEntertainmentChannels(article)
	for _, channel := range entertainmentChannels {
		if s.publishToChannel(ctx, article, channel, nil) {
			publishedChannels = append(publishedChannels, channel)
		}
	}

	// Emit pipeline event (one event per article, all channels in metadata)
	if s.pipeline != nil && len(publishedChannels) > 0 {
		pipelineErr := s.pipeline.Emit(ctx, pipeline.Event{
			ArticleURL: article.URL,
			SourceName: article.Source,
			Stage:      "published",
			OccurredAt: time.Now(),
			Metadata: map[string]any{
				"channels":      publishedChannels,
				"quality_score": article.QualityScore,
				"topics":        article.Topics,
			},
		})
		if pipelineErr != nil {
			s.logger.Warn("Failed to emit pipeline event",
				infralogger.Error(pipelineErr),
				infralogger.String("article_id", article.ID),
				infralogger.String("stage", "published"),
			)
		}
	}
}
```

**Important:** `publishToChannel()` currently returns nothing (`void`). Change its signature to return `bool` (true if published, false if skipped/error):

```go
func (s *Service) publishToChannel(ctx context.Context, article *Article, channelName string, channelID *uuid.UUID) bool {
```

- Return `false` on dedup check error, already-published, marshal error, publish error
- Return `true` after successful publish + history record

**Step 3: Run lint and tests**

Run: `cd /home/fsd42/dev/north-cloud/publisher && golangci-lint run`
Expected: 0 errors

Run: `cd /home/fsd42/dev/north-cloud/publisher && go test ./...`
Expected: All pass

**Step 4: Commit**

```bash
git add publisher/internal/router/service.go
git commit -m "feat(publisher): emit 'published' pipeline event with channel list"
```

---

### Task 6: Publisher — Wire pipeline client in bootstrap

**Files:**
- Modify: `publisher/router_config.go`
- Modify: `publisher/cmd_router.go`

**Step 1: Add PipelineURL to RouterConfig**

In `publisher/router_config.go`, add field to `RouterConfig` struct:

```go
PipelineURL string
```

In `LoadRouterConfig()`, load the value. The publisher config uses `infraconfig.ApplyEnvOverrides`. Add to the return struct:

```go
PipelineURL: os.Getenv("PIPELINE_URL"),
```

> **Note:** The publisher's `LoadRouterConfig()` already uses `infraconfig.GetConfigPath` and `infraconfig.ApplyEnvOverrides`. If the main config struct has a `Pipeline` section, use that. Otherwise, for `RouterConfig` which is a plain struct without env tags, reading from the main config's field is preferred. Check `publisher/internal/config/config.go` for the best approach.

Actually, the cleanest pattern matching the existing code is to add `PipelineURL` to the main `config.Config` with an `env:"PIPELINE_URL"` tag and read it in `LoadRouterConfig()`. If the main config doesn't have it, fall back to `cfg.Pipeline.URL` or add a field.

**Step 2: Create pipeline client and pass to router.NewService**

In `publisher/cmd_router.go`, in `runRouterWithStop()`:

Add import: `"github.com/north-cloud/infrastructure/pipeline"`

After loading config (line 58), create the client:

```go
pipelineClient := pipeline.NewClient(cfg.PipelineURL, "publisher")
```

Update the `router.NewService()` call (line 87):

```go
routerService := router.NewService(repo, discoveryService, esClient, redisClient, routerConfig, appLogger, pipelineClient)
```

**Step 3: Build and test**

Run: `cd /home/fsd42/dev/north-cloud/publisher && go build ./...`
Expected: Clean build

Run: `cd /home/fsd42/dev/north-cloud/publisher && golangci-lint run`
Expected: 0 errors

Run: `cd /home/fsd42/dev/north-cloud/publisher && go test ./...`
Expected: All pass

**Step 4: Commit**

```bash
git add publisher/router_config.go publisher/cmd_router.go
git commit -m "feat(publisher): wire pipeline client through router config"
```

---

### Task 7: Docker Compose — Add PIPELINE_URL env var

**Files:**
- Modify: `docker-compose.dev.yml`
- Modify: `docker-compose.prod.yml`

**Step 1: Add PIPELINE_URL to dev compose**

In `docker-compose.dev.yml`, add to the `environment` block of each service:

**crawler** (after line 84, in the environment section):
```yaml
      PIPELINE_URL: "${PIPELINE_URL:-http://pipeline:8075}"
```

**classifier** (after line 217, in the environment section):
```yaml
      PIPELINE_URL: "${PIPELINE_URL:-http://pipeline:8075}"
```

**publisher** (after line 534, in the environment section):
```yaml
      PIPELINE_URL: "${PIPELINE_URL:-http://pipeline:8075}"
```

**Step 2: Add PIPELINE_URL to prod compose**

In `docker-compose.prod.yml`, add to the `environment` block of each service:

**crawler** (after line 67):
```yaml
      PIPELINE_URL: http://pipeline:8075
```

**classifier** (after line 204):
```yaml
      PIPELINE_URL: http://pipeline:8075
```

**publisher** (after line 148):
```yaml
      PIPELINE_URL: http://pipeline:8075
```

**Step 3: Commit**

```bash
git add docker-compose.dev.yml docker-compose.prod.yml
git commit -m "feat(compose): add PIPELINE_URL env var to crawler, classifier, publisher"
```

---

### Task 8: Verify — Build all three services

**Step 1: Build all three services**

Run: `cd /home/fsd42/dev/north-cloud && task build:crawler && task build:classifier && task build:publisher`

Or manually:
```bash
cd /home/fsd42/dev/north-cloud/crawler && go build ./...
cd /home/fsd42/dev/north-cloud/classifier && go build ./...
cd /home/fsd42/dev/north-cloud/publisher && go build ./...
```

Expected: All 3 build cleanly

**Step 2: Lint all three services**

Run: `cd /home/fsd42/dev/north-cloud && task lint:crawler && task lint:classifier && task lint:publisher`
Expected: 0 lint errors across all 3

**Step 3: Test all three services**

Run: `cd /home/fsd42/dev/north-cloud && task test:crawler && task test:classifier && task test:publisher`
Expected: All tests pass

**Step 4: Final commit (if any fixes needed)**

```bash
git add -A
git commit -m "fix: address lint/test issues from pipeline integration"
```
