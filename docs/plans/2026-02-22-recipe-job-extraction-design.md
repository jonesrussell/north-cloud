# Recipe & Job Structured Extraction Design

**Date:** 2026-02-22
**Status:** Approved
**Services affected:** classifier, publisher, search, index-manager
**Services unchanged:** crawler, source-manager, pipeline, auth, dashboard

---

## Overview

Add structured extraction support for two new content types — cooking recipes and career job postings — to the North Cloud content pipeline. Content is detected via Schema.org structured data (primary) and keyword rules (fallback), with structured fields extracted and stored in Elasticsearch for search and routing.

---

## 1. Detection (Classifier — Content Type Step)

### Schema.org JSON-LD Detection (Primary, High Precision)

Parse `raw_html` for `<script type="application/ld+json">` blocks. Handle JSON-LD arrays (common pattern: `[{@type: BreadcrumbList}, {@type: Recipe}]`) by filtering for the target `@type`.

- `@type: "Recipe"` → set `content_type = "recipe"`, confidence 1.0
- `@type: "JobPosting"` → set `content_type = "job"`, confidence 1.0

**Placement:** Early-exit in content type classifier, after `detected_content_type` from crawler meta, before OG/heuristic checks. When Schema.org fires, short-circuit all subsequent OG and heuristic checks to prevent conflicting signals (e.g., a recipe page with `og:type=article`).

### Keyword Rules (Fallback, Broader Recall)

Add rules to `classification_rules` PostgreSQL table via migration:

- **Recipe keywords:** "recipe", "ingredients", "prep time", "cook time", "servings", "tablespoon", "preheat"
- **Job keywords:** "apply now", "job posting", "salary", "qualifications", "employment type", "full-time", "part-time"

These produce `recipe` or `jobs` topics via the existing TrieRuleEngine. The topic signal triggers extraction in step 5 even when Schema.org is absent.

**Key distinction:** Schema.org sets the content *type* directly. Keyword rules add a *topic* that signals "attempt extraction."

---

## 2. Structured Extraction (Classifier — New Step 5)

### Pipeline Position

```
1. Content Type Detection  ← Schema.org detection added here
2. Quality Scoring
3. Topic Classification    ← keyword rules add recipe/jobs topic
4. Source Reputation
5. Structured Extraction   ← NEW STEP
6. Optional Classifiers    ← crime, mining, etc. (unchanged)
```

Runs after topics so it can use both signals: content type from step 1 (Schema.org) OR topic from step 3 (keyword rules).

### Tier 1 — Schema.org Field Mapping (High Fidelity)

When Schema.org was detected in step 1, parse the full JSON-LD block and map fields directly.

**Recipe fields** (from Schema.org `Recipe`):

| Schema.org Field | Extracted Field | Type |
|---|---|---|
| `name` | `recipe.name` | string |
| `recipeIngredient[]` | `recipe.ingredients[]` | string array |
| `recipeInstructions[]` | `recipe.instructions` | string (joined) |
| `prepTime` (ISO 8601) | `recipe.prep_time_minutes` | int |
| `cookTime` (ISO 8601) | `recipe.cook_time_minutes` | int |
| `totalTime` (ISO 8601) | `recipe.total_time_minutes` | int |
| `recipeYield` | `recipe.servings` | string |
| `recipeCategory` | `recipe.category` | string |
| `recipeCuisine` | `recipe.cuisine` | string |
| `nutrition.calories` | `recipe.calories` | string |
| `image` | `recipe.image_url` | string |
| `aggregateRating.ratingValue` | `recipe.rating` | float |
| `aggregateRating.ratingCount` | `recipe.rating_count` | int |

**Job fields** (from Schema.org `JobPosting`):

| Schema.org Field | Extracted Field | Type |
|---|---|---|
| `title` | `job.title` | string |
| `hiringOrganization.name` | `job.company` | string |
| `jobLocation.address` | `job.location` | string (normalized) |
| `baseSalary` (min) | `job.salary_min` | float |
| `baseSalary` (max) | `job.salary_max` | float |
| `baseSalary` (currency) | `job.salary_currency` | string |
| `employmentType` | `job.employment_type` | string |
| `datePosted` | `job.posted_date` | date |
| `validThrough` | `job.expires_date` | date |
| `description` | `job.description` | string |
| `industry` | `job.industry` | string |
| `qualifications` | `job.qualifications` | string |
| `jobBenefits` | `job.benefits` | string |

### Tier 2 — Heuristic Extraction (Lower Fidelity)

When no Schema.org is present but keyword rules flagged `recipe` or `jobs` topic, attempt best-effort extraction from `raw_text`:

**Recipes:**
- Section headers: "Ingredients:", "Instructions:", "Directions:", "Method:"
- Bullet patterns: `-`, `*`, numbered steps (`1.`, `2.`)
- Time patterns: "Prep Time: 30 min", "Cook Time: 1 hour"

**Jobs:**
- Section headers: "Responsibilities", "Requirements", "Qualifications", "About the company", "How to apply"
- Field patterns: "Company:", "Location:", "Salary:", "Apply by:"

### Fidelity Tracking

Both extractors set `extraction_method`:
- `"schema_org"` — Tier 1, high fidelity
- `"heuristic"` — Tier 2, lower fidelity

Consumers can filter or weight results based on fidelity.

### Error Handling

- Malformed Schema.org JSON → log warning, fall through to Tier 2
- Tier 2 finds nothing useful → still set content type, nested object has only `extraction_method` and whatever fields were found
- Extraction never blocks the pipeline — errors log warnings, article classifies normally

### Implementation Pattern

- `RecipeExtractor` and `JobExtractor` structs
- Method: `Extract(ctx, raw *domain.RawContent, contentType string, topics []string) (*domain.RecipeResult, error)`
- Returns `(nil, nil)` when not applicable — field omitted from output
- Gated by content type (`recipe`/`job`) or topic match (`recipe`/`jobs`)

---

## 3. Elasticsearch Mappings

### Recipe Nested Object

```
recipe: object
├─ extraction_method: keyword        ("schema_org" | "heuristic")
├─ name: text                        (searchable)
├─ ingredients: text                 (array, searchable)
├─ instructions: text                (searchable)
├─ prep_time_minutes: integer        (nullable)
├─ cook_time_minutes: integer        (nullable)
├─ total_time_minutes: integer       (nullable)
├─ servings: keyword
├─ category: keyword                 (lowercase normalizer)
├─ cuisine: keyword                  (lowercase normalizer)
├─ calories: keyword
├─ image_url: keyword                (index: false, stored only)
├─ rating: float                     (nullable)
└─ rating_count: integer             (nullable)
```

### Job Nested Object

```
job: object
├─ extraction_method: keyword        ("schema_org" | "heuristic")
├─ title: text                       (searchable)
├─ company: keyword                  (lowercase normalizer)
├─ location: keyword                 (lowercase normalizer)
├─ salary_min: float                 (nullable)
├─ salary_max: float                 (nullable)
├─ salary_currency: keyword
├─ employment_type: keyword
├─ posted_date: date                 (nullable)
├─ expires_date: date                (nullable)
├─ description: text                 (searchable)
├─ industry: keyword
├─ qualifications: text              (searchable)
└─ benefits: text                    (searchable)
```

### Implementation

- `classifier/internal/elasticsearch/mappings/classified_content.go` — add `getRecipeMapping()` and `getJobMapping()` helpers
- `classifier/internal/domain/classification.go` — add `RecipeResult` and `JobResult` structs
- Index-manager gets matching explicit mappings to prevent mapping drift
- Both objects nullable at top level — omitted when not applicable

### Optional Enhancements (Not Blocking)

- `copy_to` for search convenience fields (`recipe_search`, `job_search`)
- Lowercase normalizer on facet keyword fields to prevent aggregation fragmentation

---

## 4. Publisher Routing

### Layer 9 — Recipe Routing

```
File: publisher/internal/router/domain_recipe.go
Skip if: no recipe object in article

Channels:
├─ articles:recipes              (catch-all)
├─ recipes:category:{slug}       (per category)
└─ recipes:cuisine:{slug}        (per cuisine)
```

### Layer 10 — Job Routing

```
File: publisher/internal/router/domain_job.go
Skip if: no job object in article

Channels:
├─ articles:jobs                 (catch-all)
├─ jobs:type:{slug}              (per employment type)
└─ jobs:industry:{slug}          (per industry)
```

### Publisher Changes

- Append recipe and job domains to `domains` slice in `routeArticle()`
- Add `recipe` and `jobs` to `layer1SkipTopics` map in `domain_topic.go`
- Expand publisher fetch filter from `content_type=article` to `content_type IN (article, recipe, job)`
- Add `Recipe *RecipeFields` and `Job *JobFields` to publisher `Article` struct
- Add `extractRecipeFields()` and `extractJobFields()` to `extractNestedFields()`

### Content Type Decision

Recipes and jobs get their own content types (`recipe`, `job`) rather than being subtypes of `article`. This is cleaner semantically and enables precise filtering.

---

## 5. Search Service

### New Filters

```go
// Recipe filters
RecipeCuisine    []string  // e.g. ["italian", "thai"]
RecipeCategory   []string  // e.g. ["dessert", "main-course"]
MaxPrepTime      *int      // prep_time_minutes <= N
MaxCookTime      *int      // cook_time_minutes <= N
MaxTotalTime     *int      // total_time_minutes <= N

// Job filters
JobEmploymentType []string // e.g. ["full_time", "contract"]
JobIndustry       []string
SalaryMin         *float64 // salary_min >= N
SalaryMax         *float64 // salary_max <= N
JobLocation       []string
```

### New Facets

```go
RecipeCuisines    []FacetBucket
RecipeCategories  []FacetBucket
JobTypes          []FacetBucket
JobIndustries     []FacetBucket
JobLocations      []FacetBucket
```

### Query Builder Changes

- Term filters for keyword fields (`recipe.cuisine`, `job.employment_type`, etc.)
- Range filters for numeric fields (`recipe.prep_time_minutes`, `job.salary_min`)
- Aggregations for new facets on keyword fields
- Content-type-aware field boosting: `recipe.name^2`, `recipe.ingredients^1.5` for recipe searches; `job.title^3`, `job.company^2` for job searches (only when content type filter is active)

### MCP Tool

No changes needed — `search_articles` already accepts `content_type` filter. Users search with `content_type: "recipe"` or `content_type: "job"`.

---

## Deployment Considerations

- **Consumer apps** subscribing to new Redis channels (`articles:recipes`, `articles:jobs`, etc.) need channel configuration updates
- **Post-deploy validation:** check `PUBSUB CHANNELS` includes expected channels, verify publisher stats show recipe/job routing
- **Migration order:** classifier mappings first, then publisher routing, then search filters
