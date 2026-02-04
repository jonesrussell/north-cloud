# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with the classifier service.

## Quick Reference

```bash
# Development
task dev              # Start with hot reload
task test             # Run tests
task lint             # Run linter
task migrate:up       # Run migrations

# API (port 8071)
curl http://localhost:8071/health
curl http://localhost:8071/api/v1/classify -X POST -d '{"title":"...", "raw_text":"..."}'
```

## Architecture

```
internal/
├── api/           # HTTP handlers (Gin)
├── classifier/    # Core classification logic
│   ├── classifier.go      # Orchestrator
│   ├── content_type.go    # Article vs page detection
│   ├── quality.go         # Quality scoring (0-100)
│   ├── topic.go           # Rule-based topic detection
│   └── source_reputation.go  # Source trust scoring
├── processor/     # Background polling & batch processing
│   ├── poller.go          # Polls raw_content for pending items
│   ├── batch.go           # Batch classification
│   └── ratelimiter.go     # Rate limiting
├── database/      # PostgreSQL repositories (rules, reputation)
├── storage/       # Elasticsearch read/write
├── domain/        # RawContent, ClassifiedContent, Rule models
└── elasticsearch/mappings/  # ES index mappings
```

## Classification Pipeline

**4-Step Classification**:
1. **Content Type** (`content_type.go`) - Determines: `article`, `page`, `listing`
2. **Quality Score** (`quality.go`) - 0-100 based on length, structure, readability
3. **Topic Detection** (`topic.go`) - Rule-based matching from DB rules
4. **Source Reputation** (`source_reputation.go`) - Historical source quality

**Pipeline Flow**:
```
Elasticsearch: {source}_raw_content (classification_status=pending)
    ↓
Poller fetches batch → Classifier processes → BuildClassifiedContent()
    ↓
Elasticsearch: {source}_classified_content
```

## Quality Score Factors

| Factor | Weight | Description |
|--------|--------|-------------|
| Word count | High | 300+ words = good, <100 = poor |
| Title quality | Medium | Length, keyword presence |
| Structure | Medium | Has paragraphs, headings |
| Readability | Low | Sentence variety |

**Thresholds**:
- `quality_score >= 70`: High quality
- `quality_score 40-69`: Medium quality
- `quality_score < 40`: Low quality/spam

## Topic Classification Rules

Stored in PostgreSQL `classification_rules` table:

```sql
-- Example crime sub-category rules
INSERT INTO classification_rules (topic, keywords, priority) VALUES
('violent_crime', '["murder", "assault", "shooting"]', 100),
('property_crime', '["theft", "burglary", "vandalism"]', 100),
('drug_crime', '["drug", "narcotic", "trafficking"]', 100);
```

**5 Crime Sub-Categories**:
- `violent_crime` - murder, assault, shooting, homicide
- `property_crime` - theft, burglary, robbery, vandalism
- `drug_crime` - drugs, narcotics, trafficking
- `organized_crime` - gang, cartel, money laundering
- `criminal_justice` - court, sentencing, trial

## Crime Hybrid Classification

**Enabled via**: `CRIME_ENABLED=true` and `CRIME_ML_SERVICE_URL=http://crime-ml:8076`

**Architecture**: Rules (precision) + ML (recall) with decision matrix

**5-Step Classification** (when enabled):
1. Content Type
2. Quality Score
3. Topic Detection
4. Source Reputation
5. **Crime Classification** (hybrid rule + ML)

**Relevance Classes**:
- `core_street_crime` - Homepage eligible (murders, shootings, assaults with arrest)
- `peripheral_crime` - Category pages only (impaired driving, international, policy)
- `not_crime` - Excluded

**Decision Matrix**:
| Rules | ML | Result | Confidence |
|-------|-----|--------|------------|
| core | core | core | High (avg) |
| core | not_crime | core + review | Medium |
| core | - | core | Rule conf |
| - | core (>0.9) | peripheral + review | ML conf * 0.8 |
| other | other | Rule result | Rule conf |

**Crime Types** (multi-label):
- `violent_crime`, `property_crime`, `drug_crime`
- `gang_violence`, `organized_crime`, `criminal_justice`, `other_crime`

**Location Classes**:
- `local_canada` - Local Canadian news
- `national_canada` - National Canadian news
- `international` - Foreign news (downgraded)
- `not_specified` - Location unknown

**Output Fields** (in ClassifiedContent):
```json
{
  "crime": {
    "street_crime_relevance": "core_street_crime",
    "crime_types": ["violent_crime"],
    "location_specificity": "local_canada",
    "final_confidence": 0.92,
    "homepage_eligible": true,
    "category_pages": ["violent-crime", "crime"],
    "review_required": false
  }
}
```

## Common Gotchas

1. **Must populate `Body` and `Source` aliases**: Publisher expects these fields:
   ```go
   Body:   raw.RawText,  // Alias for RawText
   Source: raw.URL,      // Alias for URL
   ```

2. **Spam threshold is 30**: Items with `quality_score < 30` are flagged as spam.

3. **Poller interval is configurable**: Default 30 seconds, set via `CLASSIFIER_POLL_INTERVAL`.

4. **Rules are cached**: Changes to `classification_rules` table require service restart or API call to reload.

5. **Source reputation updates on every classification**: Quality scores feed back into source reputation.

## API Endpoints

**Health**: `GET /health`

**Classification**:
- `POST /api/v1/classify` - Classify single article
- `POST /api/v1/classify/batch` - Classify multiple articles

**Rules**:
- `GET /api/v1/rules` - List classification rules
- `POST /api/v1/rules` - Create rule
- `PUT /api/v1/rules/:id` - Update rule
- `DELETE /api/v1/rules/:id` - Delete rule

## Configuration

```yaml
classifier:
  poll_interval: 30s       # CLASSIFIER_POLL_INTERVAL
  batch_size: 50           # CLASSIFIER_BATCH_SIZE
  min_quality_score: 0     # CLASSIFIER_MIN_QUALITY_SCORE
```

## Database Schema

**classification_rules**:
- `id`, `topic`, `keywords` (JSON array), `priority`, `enabled`

**source_reputation**:
- `source_name`, `reputation_score`, `total_articles`, `spam_count`

**classification_history**:
- `content_id`, `quality_score`, `topics`, `classified_at`

## Testing

```bash
# Run all tests
task test

# Run with coverage
task test:cover

# Test specific package
go test ./internal/classifier/...
```
