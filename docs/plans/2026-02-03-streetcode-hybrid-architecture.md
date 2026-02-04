# StreetCode Hybrid Classifier Architecture

**Date**: 2026-02-03
**Status**: Ready for Implementation
**Purpose**: Production architecture combining rules + ML for street crime classification

---

## Overview

The hybrid classifier uses **three layers** working together:

```
┌─────────────────────────────────────────────────────────────────┐
│                     INCOMING ARTICLE                            │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  LAYER 1: EXCLUSION RULES (Fast, High Precision)                │
│  ─────────────────────────────────────────────────────────────  │
│  • Job postings → EXCLUDE                                       │
│  • Directory pages → EXCLUDE                                    │
│  • Section headers → EXCLUDE                                    │
│  • International politics → EXCLUDE from homepage               │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  LAYER 2: CRIME DETECTION RULES (High Precision Core Detection) │
│  ─────────────────────────────────────────────────────────────  │
│  • Murder/homicide patterns → CORE                              │
│  • Shooting/stabbing → CORE                                     │
│  • Drug seizures → CORE                                         │
│  • Returns: rule_relevance, rule_crime_types, rule_confidence   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  LAYER 3: ML MODEL (Recall Enhancement)                         │
│  ─────────────────────────────────────────────────────────────  │
│  • TF-IDF + Logistic Regression                                 │
│  • Catches edge cases rules miss                                │
│  • Returns: ml_relevance, ml_confidence, ml_crime_types         │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  DECISION LAYER: Agreement Gating                               │
│  ─────────────────────────────────────────────────────────────  │
│  • Both agree CORE + confidence ≥ 0.75 → HOMEPAGE               │
│  • Rule says CORE, ML uncertain → CATEGORY PAGE                 │
│  • ML says CORE, rule says NO → REVIEW QUEUE                    │
│  • Both say NOT_CRIME → EXCLUDE                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## Classifier Output Schema

```json
{
  "article_id": "abc123",
  "title": "Man charged with murder after downtown stabbing",

  "classification": {
    "street_crime_relevance": "core_street_crime",
    "crime_types": ["violent_crime", "criminal_justice"],
    "location_specificity": "local_canada",
    "final_confidence": 0.92
  },

  "rule_layer": {
    "relevance": "core_street_crime",
    "crime_types": ["violent_crime", "criminal_justice"],
    "confidence": 0.95,
    "matched_patterns": ["murder", "charged", "stabbing"]
  },

  "ml_layer": {
    "relevance": "core_street_crime",
    "confidence": 0.89,
    "crime_type_scores": {
      "violent_crime": 0.94,
      "property_crime": 0.12,
      "drug_crime": 0.08,
      "criminal_justice": 0.78
    }
  },

  "routing": {
    "homepage_eligible": true,
    "category_pages": ["violent-crime", "court-news"],
    "review_required": false,
    "exclusion_reason": null
  },

  "metadata": {
    "classified_at": "2026-02-03T14:30:00Z",
    "classifier_version": "1.0.0",
    "processing_time_ms": 45
  }
}
```

---

## Decision Matrix

### Homepage Eligibility

| Rule Says | ML Says | ML Confidence | Decision |
|-----------|---------|---------------|----------|
| CORE | CORE | ≥ 0.75 | ✅ HOMEPAGE |
| CORE | CORE | 0.50-0.74 | ✅ HOMEPAGE (rule override) |
| CORE | PERIPHERAL | any | ⚠️ CATEGORY ONLY |
| CORE | NOT_CRIME | any | ⚠️ REVIEW QUEUE |
| PERIPHERAL | CORE | ≥ 0.85 | ⚠️ REVIEW QUEUE |
| PERIPHERAL | PERIPHERAL | any | ⚠️ CATEGORY ONLY |
| PERIPHERAL | NOT_CRIME | any | ❌ EXCLUDE |
| NOT_CRIME | CORE | ≥ 0.90 | ⚠️ REVIEW QUEUE |
| NOT_CRIME | NOT_CRIME | any | ❌ EXCLUDE |

### Key Principles

1. **Rules have veto power** — If rules say EXCLUDE, article is excluded
2. **ML boosts recall** — If ML finds crime rules missed, flag for review
3. **Confidence gates homepage** — Only high-confidence articles appear on homepage
4. **Disagreement → conservative** — When in doubt, demote to category page

---

## Confidence Thresholds

| Threshold | Value | Purpose |
|-----------|-------|---------|
| `HOMEPAGE_MIN_CONFIDENCE` | 0.75 | Minimum for homepage display |
| `RULE_HIGH_CONFIDENCE` | 0.85 | Rule alone sufficient for homepage |
| `ML_OVERRIDE_THRESHOLD` | 0.90 | ML can override rule exclusion (sends to review) |
| `EXCLUDE_THRESHOLD` | 0.30 | Below this, definitively not crime |

---

## Integration with Go Classifier Service

### Option 1: Embedded ML (Recommended for latency)

```go
// classifier/internal/classifier/hybrid_classifier.go

type HybridClassifier struct {
    ruleClassifier  *RuleClassifier
    mlClassifier    *MLClassifier  // ONNX runtime or native Go
    logger          infralogger.Logger
}

type ClassificationResult struct {
    ArticleID          string              `json:"article_id"`
    Relevance          string              `json:"street_crime_relevance"`
    CrimeTypes         []string            `json:"crime_types"`
    LocationSpecificity string             `json:"location_specificity"`
    FinalConfidence    float64             `json:"final_confidence"`
    HomepageEligible   bool                `json:"homepage_eligible"`
    CategoryPages      []string            `json:"category_pages"`
    ReviewRequired     bool                `json:"review_required"`
    RuleResult         *RuleLayerResult    `json:"rule_layer"`
    MLResult           *MLLayerResult      `json:"ml_layer"`
}

func (h *HybridClassifier) Classify(ctx context.Context, article *Article) (*ClassificationResult, error) {
    // Layer 1 & 2: Rules
    ruleResult := h.ruleClassifier.Classify(article)

    // Layer 3: ML
    mlResult := h.mlClassifier.Classify(article)

    // Decision layer
    return h.mergeResults(ruleResult, mlResult)
}

func (h *HybridClassifier) mergeResults(rule *RuleLayerResult, ml *MLLayerResult) *ClassificationResult {
    result := &ClassificationResult{
        RuleResult: rule,
        MLResult:   ml,
    }

    // Agreement check
    if rule.Relevance == "core_street_crime" && ml.Relevance == "core_street_crime" {
        result.Relevance = "core_street_crime"
        result.FinalConfidence = (rule.Confidence + ml.Confidence) / 2
        result.HomepageEligible = result.FinalConfidence >= HomepageMinConfidence
    } else if rule.Relevance == "core_street_crime" {
        // Rule says core, ML disagrees
        result.Relevance = "core_street_crime"
        result.FinalConfidence = rule.Confidence
        result.HomepageEligible = rule.Confidence >= RuleHighConfidence
        if ml.Relevance == "not_crime" {
            result.ReviewRequired = true
        }
    } else if ml.Relevance == "core_street_crime" && ml.Confidence >= MLOverrideThreshold {
        // ML says core with high confidence, rule missed it
        result.Relevance = "peripheral_crime" // Conservative
        result.ReviewRequired = true
    } else {
        result.Relevance = rule.Relevance
        result.FinalConfidence = rule.Confidence
    }

    // Merge crime types (union of both)
    result.CrimeTypes = h.mergeCrimeTypes(rule.CrimeTypes, ml.CrimeTypes)

    // Map to category pages
    result.CategoryPages = h.mapToCategoryPages(result.CrimeTypes)

    return result
}
```

### Option 2: ML as Microservice (Recommended for flexibility)

```yaml
# docker-compose.yml addition
services:
  streetcode-ml:
    build: ./streetcode-ml
    ports:
      - "8075:8075"
    environment:
      - MODEL_PATH=/models/streetcode_classifier.joblib
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8075/health"]
```

```go
// classifier/internal/classifier/ml_client.go

type MLClient struct {
    baseURL    string
    httpClient *http.Client
}

func (c *MLClient) Classify(ctx context.Context, title, body string) (*MLLayerResult, error) {
    req := MLRequest{Title: title, Body: body}
    resp, err := c.httpClient.Post(c.baseURL+"/classify", "application/json", req)
    // ...
}
```

---

## Category Page Routing

```go
var categoryMapping = map[string][]string{
    "violent_crime":    {"violent-crime", "crime"},
    "property_crime":   {"property-crime", "crime"},
    "drug_crime":       {"drug-crime", "crime"},
    "gang_violence":    {"gang-violence", "violent-crime", "crime"},
    "organized_crime":  {"organized-crime", "crime"},
    "criminal_justice": {"court-news"},
    "other_crime":      {"crime"},
}

func mapToCategoryPages(crimeTypes []string) []string {
    pages := make(map[string]bool)
    for _, ct := range crimeTypes {
        for _, page := range categoryMapping[ct] {
            pages[page] = true
        }
    }
    return mapKeys(pages)
}
```

---

## Publisher Integration

The publisher service routes articles to Redis channels based on classification:

```go
// publisher/internal/router/streetcode_router.go

func (r *StreetCodeRouter) Route(article *ClassifiedArticle) error {
    if !article.HomepageEligible {
        return nil // Don't publish to homepage channel
    }

    // Publish to homepage channel
    if err := r.redis.Publish("streetcode:homepage", article); err != nil {
        return err
    }

    // Publish to category channels
    for _, category := range article.CategoryPages {
        channel := fmt.Sprintf("streetcode:category:%s", category)
        if err := r.redis.Publish(channel, article); err != nil {
            r.logger.Warn("Failed to publish to category",
                infralogger.String("category", category),
                infralogger.Error(err))
        }
    }

    return nil
}
```

---

## Monitoring & Feedback Loop

### Metrics to Track

| Metric | Target | Alert Threshold |
|--------|--------|-----------------|
| Homepage precision (spot-check) | ≥ 95% | < 90% |
| Core articles per day | 10-30 | < 5 or > 50 |
| Review queue size | < 20/day | > 50/day |
| ML-only detections | 5-15% of core | > 30% |
| Classification latency p99 | < 100ms | > 500ms |

### Feedback Integration

```sql
-- Table for editor corrections
CREATE TABLE classification_corrections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    article_id TEXT NOT NULL,
    original_relevance TEXT NOT NULL,
    corrected_relevance TEXT NOT NULL,
    original_crime_types TEXT[] NOT NULL,
    corrected_crime_types TEXT[],
    corrected_by TEXT NOT NULL,
    corrected_at TIMESTAMP DEFAULT NOW(),
    notes TEXT
);

-- Weekly export for retraining
SELECT
    a.title,
    a.raw_text,
    COALESCE(c.corrected_relevance, a.relevance) as relevance,
    COALESCE(c.corrected_crime_types, a.crime_types) as crime_types
FROM classified_articles a
LEFT JOIN classification_corrections c ON a.id = c.article_id
WHERE a.classified_at > NOW() - INTERVAL '7 days';
```

### Retraining Schedule

| Trigger | Action |
|---------|--------|
| 50+ corrections accumulated | Retrain ML model |
| Weekly | Evaluate model metrics |
| Monthly | Full rule + ML audit |
| Quarterly | Consider transformer upgrade |

---

## Implementation Phases

### Phase 1: Rule Enhancement (Week 1)
- [ ] Update Go classifier with new rule patterns
- [ ] Add exclusion patterns for job listings, directories
- [ ] Deploy to staging, validate against test set

### Phase 2: ML Integration (Week 2)
- [ ] Export ML model to ONNX or joblib
- [ ] Build ML microservice or embed in Go
- [ ] Implement hybrid decision logic
- [ ] Deploy to staging

### Phase 3: Publisher Integration (Week 3)
- [ ] Add StreetCode routing logic to publisher
- [ ] Create Redis channels for homepage + categories
- [ ] Configure confidence thresholds
- [ ] Deploy to production (shadow mode)

### Phase 4: Monitoring & Feedback (Week 4)
- [ ] Set up classification metrics dashboard
- [ ] Build editor correction interface
- [ ] Implement retraining pipeline
- [ ] Enable production traffic

---

## Appendix: Confidence Calibration

The hybrid confidence is computed as:

```python
def compute_final_confidence(rule_conf, ml_conf, rule_relevance, ml_relevance):
    # Perfect agreement: average with bonus
    if rule_relevance == ml_relevance:
        return min(1.0, (rule_conf + ml_conf) / 2 + 0.05)

    # Rule says core, ML says peripheral: trust rule with penalty
    if rule_relevance == "core" and ml_relevance == "peripheral":
        return rule_conf * 0.85

    # Rule says core, ML says not_crime: significant penalty
    if rule_relevance == "core" and ml_relevance == "not_crime":
        return rule_conf * 0.70

    # ML says core, rule says not_crime: use ML but flag
    if ml_relevance == "core" and rule_relevance == "not_crime":
        return ml_conf * 0.80  # Penalized, requires review

    return max(rule_conf, ml_conf) * 0.75
```

---

## Version History

| Date | Version | Change |
|------|---------|--------|
| 2026-02-03 | 1.0.0 | Initial architecture design |
