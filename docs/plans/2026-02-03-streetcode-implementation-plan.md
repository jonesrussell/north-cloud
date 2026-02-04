# StreetCode Classifier Implementation Plan

**Date**: 2026-02-03
**Status**: Ready for Implementation
**Scope**: Integrate hybrid classifier into existing Go services

---

## Overview

This plan integrates the StreetCode hybrid classifier into the existing North Cloud infrastructure:

- **Classifier service** → Enhanced with hybrid rules + ML
- **Publisher service** → Extended with StreetCode routing
- **New ML microservice** → Python service for ML inference

---

## Architecture Decision: ML as Microservice

**Recommendation: Microservice (Option 2)**

| Approach | Pros | Cons |
|----------|------|------|
| Embedded (ONNX in Go) | Low latency, single deployment | Complex Go bindings, harder to update model |
| **Microservice (Python)** | Easy model updates, standard sklearn, debuggable | Network latency (~10ms), extra service |
| Sidecar | Close to classifier, easy updates | K8s-specific, overkill for docker-compose |

**Why microservice wins:**
1. Model updates are a `docker pull` + restart, no Go recompile
2. Python sklearn is the reference implementation
3. Network latency is acceptable (10-20ms vs 100ms classification budget)
4. Easier debugging and monitoring
5. Can run multiple instances if needed

---

## Service Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         NORTH CLOUD INFRASTRUCTURE                      │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ┌─────────────┐      ┌─────────────────┐      ┌─────────────────┐     │
│  │   Crawler   │──────│  Elasticsearch  │──────│  Classifier     │     │
│  │   (8060)    │      │  raw_content    │      │  (8071)         │     │
│  └─────────────┘      └─────────────────┘      │                 │     │
│                                                │  ┌───────────┐  │     │
│                                                │  │ Rules     │  │     │
│                                                │  │ (topic.go)│  │     │
│                                                │  └─────┬─────┘  │     │
│                                                │        │        │     │
│                                                │  ┌─────▼─────┐  │     │
│  ┌─────────────┐                               │  │ Hybrid    │  │     │
│  │ streetcode- │◄──────────HTTP────────────────│  │ Merger    │  │     │
│  │ ml (8076)   │                               │  └─────┬─────┘  │     │
│  │             │                               │        │        │     │
│  │ Python/     │                               └────────┼────────┘     │
│  │ sklearn     │                                        │              │
│  └─────────────┘                                        ▼              │
│                                                ┌─────────────────┐     │
│                                                │  Elasticsearch  │     │
│                                                │  classified_    │     │
│                                                │  content        │     │
│                                                └────────┬────────┘     │
│                                                         │              │
│                                                         ▼              │
│                                                ┌─────────────────┐     │
│                                                │   Publisher     │     │
│                                                │   (8070)        │     │
│                                                │                 │     │
│                                                │  ┌───────────┐  │     │
│                                                │  │StreetCode │  │     │
│                                                │  │ Router    │  │     │
│                                                │  └─────┬─────┘  │     │
│                                                └────────┼────────┘     │
│                                                         │              │
│                                                         ▼              │
│                                                ┌─────────────────┐     │
│                                                │     Redis       │     │
│                                                │  streetcode:    │     │
│                                                │  homepage       │     │
│                                                │  streetcode:    │     │
│                                                │  category:*     │     │
│                                                └─────────────────┘     │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Phase 1: ML Microservice (Week 1)

### 1.1 Create `streetcode-ml` Service

**Directory structure:**
```
streetcode-ml/
├── Dockerfile
├── requirements.txt
├── config.yml
├── main.py              # FastAPI server
├── models/
│   ├── relevance.joblib
│   ├── crime_type.joblib
│   └── location.joblib
├── classifier/
│   ├── __init__.py
│   ├── preprocessor.py  # Text cleaning
│   ├── relevance.py     # 3-class model
│   ├── crime_type.py    # Multi-label model
│   └── location.py      # 4-class model
└── tests/
    └── test_classifier.py
```

**API Endpoints:**
```yaml
POST /classify
  Request:
    title: string
    body: string (optional, first 500 chars used)
  Response:
    relevance: "core_street_crime" | "peripheral_crime" | "not_crime"
    relevance_confidence: float
    crime_types: [string]
    crime_type_scores: {string: float}
    location: string
    location_confidence: float
    processing_time_ms: int

GET /health
  Response:
    status: "healthy"
    model_version: string
    loaded_at: timestamp
```

**main.py:**
```python
from fastapi import FastAPI
from pydantic import BaseModel
import joblib

app = FastAPI()

# Load models on startup
relevance_model = joblib.load("models/relevance.joblib")
crime_type_model = joblib.load("models/crime_type.joblib")
location_model = joblib.load("models/location.joblib")

class ClassifyRequest(BaseModel):
    title: str
    body: str = ""

class ClassifyResponse(BaseModel):
    relevance: str
    relevance_confidence: float
    crime_types: list[str]
    crime_type_scores: dict[str, float]
    location: str
    location_confidence: float
    processing_time_ms: int

@app.post("/classify", response_model=ClassifyResponse)
def classify(req: ClassifyRequest):
    # Implementation from streetcode_ml_training.py
    ...

@app.get("/health")
def health():
    return {"status": "healthy", "model_version": "1.0.0"}
```

**Dockerfile:**
```dockerfile
FROM python:3.11-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
EXPOSE 8076
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8076"]
```

**docker-compose.base.yml addition:**
```yaml
  streetcode-ml:
    build: ./streetcode-ml
    container_name: north-cloud-streetcode-ml
    ports:
      - "8076:8076"
    environment:
      - MODEL_PATH=/app/models
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8076/health"]
      interval: 30s
      timeout: 10s
      retries: 3
    networks:
      - north-cloud
```

### 1.2 Export Models from Training

```python
# Add to streetcode_ml_training.py
import joblib

def export_models(relevance_results, crime_type_results, location_results):
    # Export best relevance model
    best = relevance_results['Logistic Regression']
    joblib.dump({
        'model': best['model'],
        'vectorizer': best['vectorizer'],
    }, 'models/relevance.joblib')

    # Export crime type model
    joblib.dump({
        'model': crime_type_results['model'],
        'vectorizer': crime_type_results['vectorizer'],
        'mlb': crime_type_results['mlb'],
    }, 'models/crime_type.joblib')

    # Export location model
    joblib.dump({
        'model': location_results['model'],
        'vectorizer': location_results['vectorizer'],
    }, 'models/location.joblib')
```

---

## Phase 2: Classifier Service Enhancement (Week 2)

### 2.1 Add ML Client

**New file: `classifier/internal/mlclient/client.go`**
```go
package mlclient

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type Client struct {
    baseURL    string
    httpClient *http.Client
}

type ClassifyRequest struct {
    Title string `json:"title"`
    Body  string `json:"body"`
}

type ClassifyResponse struct {
    Relevance           string             `json:"relevance"`
    RelevanceConfidence float64            `json:"relevance_confidence"`
    CrimeTypes          []string           `json:"crime_types"`
    CrimeTypeScores     map[string]float64 `json:"crime_type_scores"`
    Location            string             `json:"location"`
    LocationConfidence  float64            `json:"location_confidence"`
    ProcessingTimeMs    int64              `json:"processing_time_ms"`
}

func NewClient(baseURL string) *Client {
    return &Client{
        baseURL: baseURL,
        httpClient: &http.Client{
            Timeout: 5 * time.Second,
        },
    }
}

func (c *Client) Classify(ctx context.Context, title, body string) (*ClassifyResponse, error) {
    reqBody, err := json.Marshal(ClassifyRequest{Title: title, Body: body})
    if err != nil {
        return nil, fmt.Errorf("marshal request: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/classify", bytes.NewReader(reqBody))
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("http request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("ml service returned %d", resp.StatusCode)
    }

    var result ClassifyResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("decode response: %w", err)
    }

    return &result, nil
}

func (c *Client) Health(ctx context.Context) error {
    req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/health", nil)
    if err != nil {
        return err
    }
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("unhealthy: %d", resp.StatusCode)
    }
    return nil
}
```

### 2.2 Add StreetCode Hybrid Classifier

**New file: `classifier/internal/classifier/streetcode.go`**
```go
package classifier

import (
    "context"
    "regexp"
    "strings"

    "github.com/jonesrussell/north-cloud/classifier/internal/domain"
    "github.com/jonesrussell/north-cloud/classifier/internal/mlclient"
    infralogger "github.com/north-cloud/infrastructure/logger"
)

// StreetCode classification thresholds
const (
    HomepageMinConfidence  = 0.75
    RuleHighConfidence     = 0.85
    MLOverrideThreshold    = 0.90
)

// StreetCodeClassifier implements hybrid rule + ML classification
type StreetCodeClassifier struct {
    mlClient *mlclient.Client
    logger   infralogger.Logger
    enabled  bool
}

// StreetCodeResult holds the hybrid classification result
type StreetCodeResult struct {
    Relevance          string             `json:"street_crime_relevance"`
    CrimeTypes         []string           `json:"crime_types"`
    LocationSpecificity string            `json:"location_specificity"`
    FinalConfidence    float64            `json:"final_confidence"`
    HomepageEligible   bool               `json:"homepage_eligible"`
    CategoryPages      []string           `json:"category_pages"`
    ReviewRequired     bool               `json:"review_required"`
    RuleRelevance      string             `json:"rule_relevance"`
    RuleConfidence     float64            `json:"rule_confidence"`
    MLRelevance        string             `json:"ml_relevance"`
    MLConfidence       float64            `json:"ml_confidence"`
}

func NewStreetCodeClassifier(mlClient *mlclient.Client, logger infralogger.Logger, enabled bool) *StreetCodeClassifier {
    return &StreetCodeClassifier{
        mlClient: mlClient,
        logger:   logger,
        enabled:  enabled,
    }
}

func (s *StreetCodeClassifier) Classify(ctx context.Context, raw *domain.RawContent) (*StreetCodeResult, error) {
    if !s.enabled {
        return nil, nil
    }

    // Layer 1 & 2: Rule-based classification
    ruleResult := s.classifyByRules(raw.Title, raw.RawText)

    // Layer 3: ML classification (if ML service available)
    var mlResult *mlclient.ClassifyResponse
    if s.mlClient != nil {
        var err error
        mlResult, err = s.mlClient.Classify(ctx, raw.Title, raw.RawText[:min(len(raw.RawText), 500)])
        if err != nil {
            s.logger.Warn("ML classification failed, using rules only",
                infralogger.String("content_id", raw.ID),
                infralogger.Error(err))
        }
    }

    // Decision layer: merge results
    return s.mergeResults(ruleResult, mlResult), nil
}

// Rule patterns (from streetcode_classifier_rules.py)
var (
    excludePatterns = []*regexp.Regexp{
        regexp.MustCompile(`(?i)^(Register|Sign up|Login|Subscribe)`),
        regexp.MustCompile(`(?i)^(Listings? By|Directory|Careers|Jobs)`),
        regexp.MustCompile(`(?i)(Part.Time|Full.Time|Hiring|Position)`),
        regexp.MustCompile(`(?i)^Local (Sports|Events|Weather)$`),
    }

    violentCrimePatterns = []struct {
        pattern    *regexp.Regexp
        confidence float64
    }{
        {regexp.MustCompile(`(?i)(murder|homicide|manslaughter)`), 0.95},
        {regexp.MustCompile(`(?i)(shooting|shootout|shot dead|gunfire)`), 0.90},
        {regexp.MustCompile(`(?i)(stab|stabbing|stabbed)`), 0.90},
        {regexp.MustCompile(`(?i)(assault|assaulted).*(charged|arrest|police)`), 0.85},
        {regexp.MustCompile(`(?i)(sexual assault|rape|sex assault)`), 0.90},
        {regexp.MustCompile(`(?i)(found dead|human remains)`), 0.80},
    }

    propertyCrimePatterns = []struct {
        pattern    *regexp.Regexp
        confidence float64
    }{
        {regexp.MustCompile(`(?i)(theft|stolen|shoplifting).*(police|arrest)`), 0.85},
        {regexp.MustCompile(`(?i)(burglary|break.in)`), 0.85},
        {regexp.MustCompile(`(?i)arson`), 0.80},
        {regexp.MustCompile(`(?i)\$[\d,]+.*(stolen|theft)`), 0.85},
    }

    drugCrimePatterns = []struct {
        pattern    *regexp.Regexp
        confidence float64
    }{
        {regexp.MustCompile(`(?i)(drug bust|drug raid|drug seizure)`), 0.90},
        {regexp.MustCompile(`(?i)(fentanyl|cocaine|heroin).*(seiz|arrest|trafficking)`), 0.90},
    }

    internationalPatterns = []*regexp.Regexp{
        regexp.MustCompile(`(?i)(Minneapolis|U\.S\.|American|Mexico|European|Israel)`),
    }
)

type ruleResult struct {
    relevance  string
    confidence float64
    crimeTypes []string
}

func (s *StreetCodeClassifier) classifyByRules(title, body string) *ruleResult {
    // Check exclusions first
    for _, p := range excludePatterns {
        if p.MatchString(title) {
            return &ruleResult{relevance: "not_crime", confidence: 0.95}
        }
    }

    result := &ruleResult{
        relevance:  "not_crime",
        confidence: 0.5,
        crimeTypes: []string{},
    }

    // Check violent crime patterns
    for _, p := range violentCrimePatterns {
        if p.pattern.MatchString(title) {
            result.relevance = "core_street_crime"
            result.confidence = max(result.confidence, p.confidence)
            if !contains(result.crimeTypes, "violent_crime") {
                result.crimeTypes = append(result.crimeTypes, "violent_crime")
            }
        }
    }

    // Check property crime patterns
    for _, p := range propertyCrimePatterns {
        if p.pattern.MatchString(title) {
            result.relevance = "core_street_crime"
            result.confidence = max(result.confidence, p.confidence)
            if !contains(result.crimeTypes, "property_crime") {
                result.crimeTypes = append(result.crimeTypes, "property_crime")
            }
        }
    }

    // Check drug crime patterns
    for _, p := range drugCrimePatterns {
        if p.pattern.MatchString(title) {
            result.relevance = "core_street_crime"
            result.confidence = max(result.confidence, p.confidence)
            if !contains(result.crimeTypes, "drug_crime") {
                result.crimeTypes = append(result.crimeTypes, "drug_crime")
            }
        }
    }

    // Check international (downgrade to peripheral)
    for _, p := range internationalPatterns {
        if p.MatchString(title) && result.relevance == "core_street_crime" {
            result.relevance = "peripheral_crime"
            result.confidence *= 0.7
        }
    }

    // Add criminal_justice if has crime types and mentions arrest/charged
    if len(result.crimeTypes) > 0 {
        if regexp.MustCompile(`(?i)(charged|arrest|sentenced|trial)`).MatchString(title) {
            result.crimeTypes = append(result.crimeTypes, "criminal_justice")
        }
    }

    return result
}

func (s *StreetCodeClassifier) mergeResults(rule *ruleResult, ml *mlclient.ClassifyResponse) *StreetCodeResult {
    result := &StreetCodeResult{
        RuleRelevance:  rule.relevance,
        RuleConfidence: rule.confidence,
    }

    if ml != nil {
        result.MLRelevance = ml.Relevance
        result.MLConfidence = ml.RelevanceConfidence
    }

    // Decision logic (from hybrid architecture)
    if rule.relevance == "core_street_crime" {
        if ml != nil && ml.Relevance == "core_street_crime" {
            // Both agree: high confidence
            result.Relevance = "core_street_crime"
            result.FinalConfidence = (rule.confidence + ml.RelevanceConfidence) / 2
            result.HomepageEligible = result.FinalConfidence >= HomepageMinConfidence
        } else if ml != nil && ml.Relevance == "not_crime" {
            // Rule says core, ML says not_crime: flag for review
            result.Relevance = "core_street_crime"
            result.FinalConfidence = rule.confidence * 0.7
            result.HomepageEligible = rule.confidence >= RuleHighConfidence
            result.ReviewRequired = true
        } else {
            // Rule says core, ML uncertain or unavailable
            result.Relevance = "core_street_crime"
            result.FinalConfidence = rule.confidence
            result.HomepageEligible = rule.confidence >= RuleHighConfidence
        }
    } else if ml != nil && ml.Relevance == "core_street_crime" && ml.RelevanceConfidence >= MLOverrideThreshold {
        // ML says core with high confidence, rule missed it
        result.Relevance = "peripheral_crime"
        result.FinalConfidence = ml.RelevanceConfidence * 0.8
        result.ReviewRequired = true
    } else {
        result.Relevance = rule.relevance
        result.FinalConfidence = rule.confidence
    }

    // Merge crime types
    result.CrimeTypes = rule.crimeTypes
    if ml != nil {
        for _, ct := range ml.CrimeTypes {
            if !contains(result.CrimeTypes, ct) {
                result.CrimeTypes = append(result.CrimeTypes, ct)
            }
        }
    }

    // Map to category pages
    result.CategoryPages = mapToCategoryPages(result.CrimeTypes)

    // Location from ML (rules don't do location yet)
    if ml != nil {
        result.LocationSpecificity = ml.Location
    }

    return result
}

func mapToCategoryPages(crimeTypes []string) []string {
    mapping := map[string][]string{
        "violent_crime":    {"violent-crime", "crime"},
        "property_crime":   {"property-crime", "crime"},
        "drug_crime":       {"drug-crime", "crime"},
        "gang_violence":    {"gang-violence", "crime"},
        "organized_crime":  {"organized-crime", "crime"},
        "criminal_justice": {"court-news"},
        "other_crime":      {"crime"},
    }

    pages := make(map[string]bool)
    for _, ct := range crimeTypes {
        for _, page := range mapping[ct] {
            pages[page] = true
        }
    }

    result := make([]string, 0, len(pages))
    for page := range pages {
        result = append(result, page)
    }
    return result
}

func contains(slice []string, item string) bool {
    for _, s := range slice {
        if s == item {
            return true
        }
    }
    return false
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}

func max(a, b float64) float64 {
    if a > b {
        return a
    }
    return b
}
```

### 2.3 Integrate into Main Classifier

**Update: `classifier/internal/classifier/classifier.go`**

Add StreetCode classifier to the main Classifier struct:

```go
type Classifier struct {
    contentType      *ContentTypeClassifier
    quality          *QualityScorer
    topic            *TopicClassifier
    sourceReputation *SourceReputationScorer
    streetcode       *StreetCodeClassifier  // NEW
    logger           infralogger.Logger
    version          string
}

// In Classify method, add after topic classification:
// 5. StreetCode Classification (if enabled)
var streetcodeResult *StreetCodeResult
if c.streetcode != nil {
    streetcodeResult, err = c.streetcode.Classify(ctx, raw)
    if err != nil {
        c.logger.Warn("StreetCode classification failed",
            infralogger.String("content_id", raw.ID),
            infralogger.Error(err))
    }
}

// Add to ClassificationResult
result.StreetCode = streetcodeResult
```

### 2.4 Update ClassifiedContent Model

**Update: `classifier/internal/domain/classification.go`**

```go
type ClassificationResult struct {
    // ... existing fields ...

    // StreetCode hybrid classification
    StreetCode *StreetCodeResult `json:"streetcode,omitempty"`
}

type StreetCodeResult struct {
    Relevance          string   `json:"street_crime_relevance"`
    CrimeTypes         []string `json:"crime_types"`
    LocationSpecificity string  `json:"location_specificity"`
    FinalConfidence    float64  `json:"final_confidence"`
    HomepageEligible   bool     `json:"homepage_eligible"`
    CategoryPages      []string `json:"category_pages"`
    ReviewRequired     bool     `json:"review_required"`
}
```

---

## Phase 3: Publisher Integration (Week 3)

### 3.1 Add StreetCode Router

**New file: `publisher/internal/router/streetcode.go`**

```go
package router

import (
    "context"
    "encoding/json"
    "fmt"

    infralogger "github.com/north-cloud/infrastructure/logger"
    "github.com/redis/go-redis/v9"
)

// StreetCodeRouter routes articles to StreetCode channels
type StreetCodeRouter struct {
    redis  *redis.Client
    logger infralogger.Logger
}

// StreetCodeArticle represents a classified article with StreetCode fields
type StreetCodeArticle struct {
    ID                  string   `json:"id"`
    Title               string   `json:"title"`
    Body                string   `json:"body"`
    URL                 string   `json:"url"`
    QualityScore        int      `json:"quality_score"`
    StreetCrimeRelevance string  `json:"street_crime_relevance"`
    CrimeTypes          []string `json:"crime_types"`
    LocationSpecificity string   `json:"location_specificity"`
    Confidence          float64  `json:"confidence"`
    HomepageEligible    bool     `json:"homepage_eligible"`
    CategoryPages       []string `json:"category_pages"`
    PublishedAt         string   `json:"published_at"`
}

func NewStreetCodeRouter(redis *redis.Client, logger infralogger.Logger) *StreetCodeRouter {
    return &StreetCodeRouter{
        redis:  redis,
        logger: logger,
    }
}

func (r *StreetCodeRouter) Route(ctx context.Context, article *StreetCodeArticle) error {
    // Skip non-crime articles
    if article.StreetCrimeRelevance == "not_crime" {
        return nil
    }

    // Publish to homepage channel if eligible
    if article.HomepageEligible {
        if err := r.publishToChannel(ctx, "streetcode:homepage", article); err != nil {
            r.logger.Error("Failed to publish to homepage",
                infralogger.String("article_id", article.ID),
                infralogger.Error(err))
        } else {
            r.logger.Info("Published to homepage",
                infralogger.String("article_id", article.ID),
                infralogger.String("title", article.Title[:min(len(article.Title), 50)]))
        }
    }

    // Publish to category channels
    for _, category := range article.CategoryPages {
        channel := fmt.Sprintf("streetcode:category:%s", category)
        if err := r.publishToChannel(ctx, channel, article); err != nil {
            r.logger.Warn("Failed to publish to category",
                infralogger.String("category", category),
                infralogger.Error(err))
        }
    }

    return nil
}

func (r *StreetCodeRouter) publishToChannel(ctx context.Context, channel string, article *StreetCodeArticle) error {
    data, err := json.Marshal(article)
    if err != nil {
        return fmt.Errorf("marshal article: %w", err)
    }

    return r.redis.Publish(ctx, channel, data).Err()
}
```

### 3.2 Add Review Queue

For articles where rules and ML disagree, write to a review queue:

```go
// In router/streetcode.go

func (r *StreetCodeRouter) RouteWithReview(ctx context.Context, article *StreetCodeArticle, reviewRequired bool) error {
    // Route as normal
    if err := r.Route(ctx, article); err != nil {
        return err
    }

    // Add to review queue if flagged
    if reviewRequired {
        if err := r.addToReviewQueue(ctx, article); err != nil {
            r.logger.Warn("Failed to add to review queue",
                infralogger.String("article_id", article.ID),
                infralogger.Error(err))
        }
    }

    return nil
}

func (r *StreetCodeRouter) addToReviewQueue(ctx context.Context, article *StreetCodeArticle) error {
    data, err := json.Marshal(article)
    if err != nil {
        return err
    }

    // Use Redis list for review queue
    return r.redis.LPush(ctx, "streetcode:review_queue", data).Err()
}
```

---

## Phase 4: Configuration & Deployment (Week 4)

### 4.1 Configuration

**classifier/config.yml addition:**
```yaml
streetcode:
  enabled: true
  ml_service_url: "http://streetcode-ml:8076"
  homepage_min_confidence: 0.75
  rule_high_confidence: 0.85
  ml_override_threshold: 0.90
```

**Environment variables:**
```bash
STREETCODE_ENABLED=true
STREETCODE_ML_SERVICE_URL=http://streetcode-ml:8076
STREETCODE_HOMEPAGE_MIN_CONFIDENCE=0.75
```

### 4.2 Database Migrations

**classifier/migrations/004_streetcode_corrections.up.sql:**
```sql
CREATE TABLE IF NOT EXISTS streetcode_corrections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    article_id TEXT NOT NULL,
    original_relevance TEXT NOT NULL,
    corrected_relevance TEXT NOT NULL,
    original_crime_types TEXT[] NOT NULL DEFAULT '{}',
    corrected_crime_types TEXT[] DEFAULT '{}',
    corrected_by TEXT NOT NULL,
    corrected_at TIMESTAMP DEFAULT NOW(),
    notes TEXT,
    UNIQUE(article_id)
);

CREATE INDEX idx_streetcode_corrections_article ON streetcode_corrections(article_id);
CREATE INDEX idx_streetcode_corrections_date ON streetcode_corrections(corrected_at);
```

### 4.3 Deployment Sequence

```bash
# 1. Deploy ML service (can run independently)
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d streetcode-ml

# 2. Verify ML service health
curl http://localhost:8076/health

# 3. Run classifier migration
cd classifier && go run cmd/migrate/main.go up

# 4. Deploy updated classifier (shadow mode first)
STREETCODE_ENABLED=true docker compose up -d classifier

# 5. Monitor logs for errors
docker compose logs -f classifier | grep -i streetcode

# 6. Verify classification output
curl http://localhost:8071/api/v1/classify -X POST \
  -H "Content-Type: application/json" \
  -d '{"title": "Man charged with murder after stabbing", "raw_text": "..."}'

# 7. Deploy updated publisher
docker compose up -d publisher

# 8. Verify Redis channels
redis-cli SUBSCRIBE streetcode:homepage
```

---

## Testing Strategy

### Unit Tests

```go
// classifier/internal/classifier/streetcode_test.go
func TestStreetCodeClassifier_ViolentCrime(t *testing.T) {
    tests := []struct {
        title    string
        expected string
    }{
        {"Man charged with murder after stabbing", "core_street_crime"},
        {"Police arrest suspect in downtown shooting", "core_street_crime"},
        {"City council debates police budget", "not_crime"},
        {"European leaders discuss trade", "not_crime"},
    }

    sc := NewStreetCodeClassifier(nil, testLogger, true)
    for _, tt := range tests {
        result := sc.classifyByRules(tt.title, "")
        if result.relevance != tt.expected {
            t.Errorf("%s: got %s, want %s", tt.title, result.relevance, tt.expected)
        }
    }
}
```

### Integration Tests

```bash
# Test ML service
curl -X POST http://localhost:8076/classify \
  -H "Content-Type: application/json" \
  -d '{"title": "Man charged with murder", "body": "Police arrested..."}'

# Test end-to-end
# 1. Insert test article into raw_content
# 2. Wait for classifier to process
# 3. Check classified_content for streetcode fields
# 4. Check Redis for published message
```

---

## Monitoring Checklist

| Metric | Target | Alert |
|--------|--------|-------|
| ML service latency p99 | < 50ms | > 100ms |
| Classification rate | > 10/min | < 1/min |
| Homepage articles/day | 10-30 | < 5 |
| Review queue size | < 20 | > 50 |
| Rule-only classifications | < 20% | > 50% |

---

## Rollback Plan

If issues arise:

```bash
# Disable StreetCode classification
STREETCODE_ENABLED=false docker compose up -d classifier

# Articles continue to be classified by existing topic system
# Publisher falls back to standard routing
```

---

## Summary

| Phase | Deliverable | Effort |
|-------|-------------|--------|
| 1 | ML microservice | 3-4 days |
| 2 | Classifier integration | 3-4 days |
| 3 | Publisher integration | 2-3 days |
| 4 | Config & deployment | 2-3 days |

**Total: ~2 weeks** with testing and iteration.

---

## Next Steps

1. [ ] Create `streetcode-ml` service directory
2. [ ] Export trained models to joblib
3. [ ] Build and test ML service locally
4. [ ] Implement `classifier/internal/mlclient`
5. [ ] Implement `classifier/internal/classifier/streetcode.go`
6. [ ] Add integration tests
7. [ ] Deploy to staging
8. [ ] Shadow mode testing
9. [ ] Production deployment
