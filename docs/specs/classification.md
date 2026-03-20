# Classification Specification

> Last verified: 2026-03-20 (fix ML sidecar model path resolution via Path.resolve())

Covers the classifier service, hybrid rule+ML classification pipeline, ML sidecar integration, and content enrichment.

## File Map

| File | Purpose |
|------|---------|
| `classifier/cmd/httpd/main.go` | HTTP API entry point |
| `classifier/cmd/processor/processor.go` | Batch processor entry point |
| `classifier/internal/classifier/classifier.go` | Main orchestrator: Classify() method |
| `classifier/internal/classifier/content_type.go` | Step 1: content type + subtype detection |
| `classifier/internal/classifier/quality.go` | Step 2: quality scoring (0-100) |
| `classifier/internal/classifier/topic.go` | Step 3: topic detection |
| `classifier/internal/classifier/rule_engine.go` | Aho-Corasick keyword matching engine |
| `classifier/internal/classifier/source_reputation.go` | Step 4: source reputation scoring |
| `classifier/internal/classifier/crime.go` | Crime hybrid classifier (rules + ML) |
| `classifier/internal/classifier/crime_rules.go` | Crime keyword patterns and exclusions |
| `classifier/internal/classifier/mining.go` | Mining hybrid classifier + drill extraction wiring |
| `classifier/internal/classifier/mining_rules.go` | Mining keyword patterns (incl. drillKeywordMatched flag) |
| `classifier/internal/classifier/drill_extractor.go` | Regex-based drill results extraction |
| `classifier/internal/classifier/drill_normalizer.go` | Commodity/unit normalization and dedup |
| `classifier/internal/classifier/drill_llm.go` | Two-stage orchestrator (regex → LLM fallback) |
| `classifier/internal/drillmlclient/client.go` | Anthropic Claude API client for drill extraction |
| `classifier/internal/classifier/ml_helper.go` | Shared CallMLWithBodyLimit[T]() helper |
| `classifier/internal/mlclient/client.go` | Base ML client interface |
| `classifier/internal/mltransport/transport.go` | Shared HTTP transport (DoClassify, DoHealth) |
| `classifier/internal/miningmlclient/client.go` | Mining ML sidecar client |
| `classifier/internal/coforgemlclient/client.go` | Coforge ML sidecar client |
| `classifier/internal/entertainmentmlclient/client.go` | Entertainment ML sidecar client |
| `classifier/internal/indigenousmlclient/client.go` | Indigenous ML sidecar client |
| `classifier/internal/mlhealth/health.go` | ML sidecar health checks |
| `classifier/internal/processor/poller.go` | ES polling loop for pending content |
| `classifier/internal/processor/batch.go` | Worker pool batch processor |
| `classifier/internal/domain/classification.go` | ClassificationResult, ClassifiedContent |
| `classifier/internal/domain/raw_content.go` | RawContent input model |
| `classifier/internal/elasticsearch/mappings/classified_content.go` | ES mapping builders |
| `classifier/internal/bootstrap/classifier.go` | Service initialization |
| `classifier/internal/testhelpers/mocks.go` | Mock source reputation DB |
| `classifier/migrations/` | PostgreSQL schema (12 migrations) |

## Interface Signatures

### Classifier (`internal/classifier/classifier.go`)
```go
func (c *Classifier) Classify(ctx context.Context, raw *domain.RawContent) (*domain.ClassificationResult, error)
func (c *Classifier) ResolveSidecars(contentType, subtype string) []string
```

### ML Client (shared pattern — concrete structs, not an interface)
```go
// Each ML client (mlclient, miningmlclient, coforgemlclient, etc.) is a concrete
// Client struct following this pattern. There is no shared MLClassifier interface.

// Shared HTTP transport (mltransport/transport.go)
func DoClassify(ctx context.Context, baseURL string, req *ClassifyRequest, respPtr any) (latencyMs int64, responseSizeBytes int, err error)
func DoHealth(ctx context.Context, baseURL string) (reachable bool, latencyMs int64, modelVersion string, err error)

// Body truncation helper (classifier/ml_helper.go)
func CallMLWithBodyLimit[T any](ctx context.Context, title, body string, maxChars int, call func(context.Context, string, string) (*T, error)) (*T, error)
```

### Poller (`internal/processor/poller.go`)
```go
func (p *Poller) Start(ctx context.Context) error  // Background polling loop
func (p *Poller) Stop()
```

## Data Flow

### 4-Step Base Pipeline
```
1. ContentType detection:
   - Checks: crawler metadata → URL exclusion patterns → OG metadata → content patterns → heuristics
   - Returns: contentType (article|page|video|image|job|recipe|event|obituary) + subtype + confidence + method
   - Thread-safe stats tracking (sync.Mutex + map): GetStats() returns per-content-type hit counts

2. Quality scoring (0-100, 4 factors × 25 pts):
   - Word count: <100→10, 100-200→15, 200-300→20, 300+→25
   - Metadata completeness: title, published_date, author, description
   - Content richness: paragraphs, headings, formatting
   - Readability: sentence length variety

3. Topic detection (Aho-Corasick, O(n+m)):
   - Rules loaded from PostgreSQL at startup (cached, no live reload)
   - Priority-descending evaluation, max 5 topics per document
   - Returns: topic names + scores + matched keywords
   - Thread-safe stats tracking (sync.Mutex + map): GetTopicStats() returns per-topic hit counts

4. Source reputation:
   - Lookup by source_name, create with default 50 if missing
   - Update after classification based on quality score
   - Spam threshold: quality < 30 → penalize reputation
```

### Hybrid Classification (optional, per content type/subtype)
```
For each enabled sidecar (crime, mining, coforge, entertainment, indigenous):
  1. Run keyword rules (fast, deterministic)
  2. Call ML sidecar POST /classify with truncated body (500 chars)
  3. Merge via decision matrix:
     - Both agree → high confidence result
     - Rule says yes, ML says no → rule wins, review_required=true
     - Rule says no, ML says yes (>0.9) → ML wins as peripheral, review_required=true
     - ML unreachable → fall back to rules only
  4. ML failures are non-blocking (log warning, continue)
```

### Sidecar Routing Table
```
ResolveSidecars(contentType, subtype) determines which optional classifiers run:
- "article" (no subtype) → all enabled classifiers
- "article:blotter" → crime only
- "article:event" → location only
- "article:report" → none
- "page", "video", "image", "job" → none
```

## Storage / Schema

### ClassificationResult
```go
type ClassificationResult struct {
    ContentID        string
    ContentType      string             // "article", "page", "video", "image", "job", "recipe", "event", "obituary"
    ContentSubtype   string             // "press_release", "blog_post", "event", "blotter", etc.
    TypeConfidence   float64
    TypeMethod       string             // "detected_content_type", "url_exclusion", "og_metadata", etc.
    QualityScore     int                // 0-100
    QualityFactors   map[string]any     // Breakdown of quality score
    Topics           []string
    TopicScores      map[string]float64
    SourceReputation int
    SourceCategory   string             // "news", "blog", "government", "unknown"
    ClassifierVersion    string
    ClassificationMethod string         // "rule_based", "ml_model", "hybrid"
    ModelVersion     string
    Confidence       float64
    ProcessingTimeMs int64
    ClassifiedAt     time.Time
    Crime            *CrimeResult       // nil when disabled
    Mining           *MiningResult      // nil when disabled
    Coforge          *CoforgeResult
    Entertainment    *EntertainmentResult
    Indigenous       *IndigenousResult
    Location         *LocationResult
    Recipe           *RecipeResult      // nil unless content_type=recipe
    Job              *JobResult         // nil unless content_type=job
}
```

### CrimeResult
```go
type CrimeResult struct {
    Relevance string           `json:"street_crime_relevance"` // NOTE: unique JSON tag
    // Values: "core_street_crime", "peripheral_crime", "not_crime"
    SubLabel            string   `json:"sub_label,omitempty"` // "criminal_justice", "crime_context"
    CrimeTypes          []string
    LocationSpecificity string
    FinalConfidence     float64
    HomepageEligible    bool
    CategoryPages       []string
    ReviewRequired      bool
    DecisionPath        string   // "both_agree", "rule_override", "rules_only", "ml_override", "ml_upgrade", "default"
    // Observability fields (all optional, omitempty):
    MLConfidenceRaw  float64
    RuleTriggered    string
    ProcessingTimeMs int64
}
```

### MiningResult
```go
type MiningResult struct {
    Relevance        string        // "core_mining", "peripheral_mining", "not_mining"
    MiningStage      string        // "exploration", "development", "production", "unspecified"
    Commodities      []string      // "gold", "copper", "lithium", etc.
    Location         string
    FinalConfidence  float64
    ReviewRequired   bool
    ModelVersion     string
    DrillResults     []DrillResult // Extracted drill intercepts (omitempty)
    ExtractionMethod string        // "regex", "llm", "hybrid", "" (omitempty)
    // Observability fields (all optional, omitempty):
    DecisionPath     string
    MLConfidenceRaw  float64
    RuleTriggered    string
    ProcessingTimeMs int64
}

type DrillResult struct {
    HoleID     string  // e.g. "DDH-24-001"
    Commodity  string  // normalized: "gold", "copper", etc.
    InterceptM float64 // intercept length in meters
    Grade      float64 // grade value
    Unit       string  // "g/t", "%", "ppm", "oz/t"
}
```

### PostgreSQL Tables
- **classification_rules**: id, rule_name, rule_type, topic_name, keywords (TEXT[]), min_confidence, enabled, priority
- **source_reputation**: id, source_name, source_url, category, reputation_score, total_articles, average_quality_score, spam_count
- **classification_history**: content_id, source_name, content_type, quality_score, topics, classified_at (audit trail)
- **dead_letter_queue**: content_id, raw_content (JSONB), error_message, classifier_version

### ML Sidecar Ports
| Sidecar | Port | Env Flag | Env URL |
|---------|------|----------|---------|
| crime-ml | 8076 | CRIME_ENABLED | CRIME_ML_SERVICE_URL |
| mining-ml | 8077 | MINING_ENABLED | MINING_ML_SERVICE_URL |
| coforge-ml | 8078 | COFORGE_ENABLED | COFORGE_ML_SERVICE_URL |
| entertainment-ml | 8079 | ENTERTAINMENT_ENABLED | ENTERTAINMENT_ML_SERVICE_URL |
| indigenous-ml | 8081 | INDIGENOUS_ENABLED | INDIGENOUS_ML_SERVICE_URL |

## Configuration

- `CLASSIFIER_PORT` (default: 8071)
- `CLASSIFIER_CONCURRENCY` (default: 10) — batch processor workers
- `CLASSIFIER_POLL_INTERVAL` (default: 30s)
- `{DOMAIN}_ENABLED` — enable/disable each hybrid classifier
- `{DOMAIN}_ML_SERVICE_URL` — ML sidecar endpoint
- `DRILL_EXTRACTION_ENABLED` — enable drill results extraction on mining articles
- `DRILL_LLM_FALLBACK` — enable Claude Haiku fallback when regex extraction is partial/none
- `ANTHROPIC_API_KEY` — required when LLM fallback is enabled
- `ANTHROPIC_MODEL` (default: `claude-haiku-4-5`) — model for drill extraction

`INDIGENOUS_ENABLED` defaults to `false` in the compose files. This is intentional: the sidecar is wired and supported, but should stay feature-flagged off until its model has been validated for the target environment.

## Drill Extraction Pipeline

When `DRILL_EXTRACTION_ENABLED=true` and mining classification is enabled, `MiningClassifier.Classify()` runs drill extraction on articles classified as `core_mining` or `peripheral_mining`.

**Two-stage pipeline** (`drill_llm.go: orchestrateDrillExtraction`):
1. **Regex pass** (`drill_extractor.go`): Pattern-matches drill intercepts from full body text. Returns `(results, confidence)` where confidence is `complete`, `partial`, or `none`.
2. **LLM fallback** (optional, `drillmlclient`): When regex confidence is `partial`/`none` and `DRILL_LLM_FALLBACK=true`, sends body to Claude Haiku for structured extraction. Results are merged (hybrid) or used alone (llm).

**Normalization** (`drill_normalizer.go`): All results pass through normalization — commodity symbols to slugs (`Au` → `gold`), unit variants (`gpt` → `g/t`), hole ID uppercasing, and deduplication.

**Trigger condition**: Extraction runs when `relevance != not_mining`. The `drillKeywordMatched` flag from `mining_rules.go` (pattern: `drill\s+results?|assay\s+results?|intercept\s+\d`) controls LLM escalation for `confidence=none` cases.

**ES mapping**: `drill_results` is a nested object under `mining` in `classified_content` indices. Fields: `hole_id` (keyword), `commodity` (keyword), `intercept_m` (float), `grade` (float), `unit` (keyword). `extraction_method` is a keyword field.

**Bootstrap wiring**: `bootstrap/classifier.go` instantiates `drillmlclient.Client` when both mining and drill extraction are enabled, and attaches it to `MiningClassifier` via `WithDrillExtraction()`.

## Edge Cases

- **Missing Body/Source aliases**: ClassifiedContent must set Body=RawText and Source=URL or publisher silently skips.
- **Rules cached at startup**: Changes to classification_rules table require service restart.
- **Nil optional classifiers**: When disabled, field is nil in result and omitted from ES document. Downstream queries return empty.
- **Mining keywords narrow by design**: Ambiguous terms excluded; ML handles nuance. Don't add broad keywords.
- **Crime authority indicators**: Patterns require presence of authority terms (police, rcmp, court, etc.) alongside crime terms for high confidence.
- **Spam still classified**: quality < 30 flags spam but document is still written to classified_content index.
