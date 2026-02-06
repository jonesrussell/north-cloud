# Crime Pipeline Precision Fix

**Date**: 2026-02-05
**Goal**: Fix the crime content pipeline so only genuine street-crime events reach Streetcode.net
**Scope**: Publisher field mapping, crime classifier rules, Streetcode subscriber

---

## Problem Statement

Streetcode.net is displaying non-crime articles: travel pieces ("A new lifeline for anyone travelling through BC"), home improvement ("7 best house renovation contractors"), gaming ("PUGB online tournament"), political opinion, and international politics. The site should show only real-world street crime events (incidents, arrests, investigations, sentencing).

### Root Causes

1. **Field mapping mismatch (CRITICAL)**: The classifier stores crime data nested under `crime.street_crime_relevance` in ES, but the publisher's `Article` struct expects flat `crime_relevance`. Go's `json.Unmarshal` silently leaves `CrimeRelevance` as `""`, so `GenerateCrimeChannels()` returns empty. **Layer 3 crime channels have never fired.** Confirmed by publish_history: zero rows for any `crime:*` channel.

2. **Topic classifier too loose**: The Layer 2 `streetcode:crime_feed` channel filters by topics (`violent_crime`, `criminal_justice`), but the topic classifier assigns these to opinion pieces and political articles that merely mention crime vocabulary.

3. **No defense in depth at Streetcode**: `ARTICLES_MIN_QUALITY_SCORE=0`, no crime-relevance validation on ingest, all topics tagged as `crime_category`.

---

## Design

### Phase 1: Fix Publisher Field Mapping

**Service**: `publisher`
**Files**: `publisher/internal/router/service.go`

Add nested structs to the publisher's `Article` that match the classifier's ES schema, then extract flat fields after unmarshaling.

**Current Article struct** (broken):
```go
CrimeRelevance      string   `json:"crime_relevance"`       // never matches nested crime.street_crime_relevance
CrimeSubLabel       string   `json:"crime_sub_label"`       // never matches crime.sub_label
HomepageEligible    bool     `json:"homepage_eligible"`      // never matches crime.homepage_eligible
CategoryPages       []string `json:"category_pages"`         // never matches crime.category_pages
LocationCity        string   `json:"location_city"`          // never matches location.city
LocationProvince    string   `json:"location_province"`      // never matches location.province
LocationCountry     string   `json:"location_country"`       // never matches location.country
```

**Fix**: Add nested structs and a post-unmarshal extraction method:
```go
// Nested structs matching classifier's ES schema
type CrimeData struct {
    Relevance       string   `json:"street_crime_relevance"`
    SubLabel        string   `json:"sub_label,omitempty"`
    CrimeTypes      []string `json:"crime_types"`
    Specificity     string   `json:"location_specificity"`
    Confidence      float64  `json:"final_confidence"`
    Homepage        bool     `json:"homepage_eligible"`
    Categories      []string `json:"category_pages"`
    ReviewRequired  bool     `json:"review_required"`
}

type LocationData struct {
    City        string  `json:"city,omitempty"`
    Province    string  `json:"province,omitempty"`
    Country     string  `json:"country"`
    Specificity string  `json:"specificity"`
    Confidence  float64 `json:"confidence"`
}

// Add to Article struct
Crime    *CrimeData    `json:"crime,omitempty"`
Location *LocationData `json:"location,omitempty"`
```

After unmarshaling each article from ES, call `extractNestedFields()` to populate the existing flat fields used by `GenerateCrimeChannels()` and `GenerateLocationChannels()`.

**Verification**: After deploying, `crime:homepage` and `crime:category:*` channels should start appearing in `publish_history`.

### Phase 2: Tighten Crime Classifier Rules

**Service**: `classifier`
**Files**: `classifier/internal/classifier/crime_rules.go`

#### A. Strengthen Exclusion Patterns

Add exclusions for content types that should never be `core_street_crime`:

```
# Opinion/Editorial
(?i)^(opinion|editorial|commentary|letters?|column|op-ed)\s*:
(?i)\b(my view|in our view|i think|we believe)\b

# Lifestyle/Non-crime
(?i)\b(renovation|contractor|tournament|recipe|travel guide|lifeline)\b
(?i)\b(best .+ in the .+ area)\b

# International politics (without crime event)
(?i)\b(imperialisme|impérialisme|geopolitical|diplomatic)\b
```

#### B. Tighten core_street_crime Requirements

Require BOTH an **action word** AND an **authority indicator** for `core_street_crime`:

**Action words**: arrested, charged, shot, stabbed, killed, murdered, robbed, assaulted, seized, busted, raided, found dead, attempted murder, sexual assault, homicide, drug bust

**Authority indicators**: police, RCMP, OPP, SQ, court, judge, investigation, suspect, accused, officer, constable, detective, prosecution

Articles with crime vocabulary but no action+authority pairing get `peripheral_crime` at most.

#### C. Sentencing/Verdicts as core_street_crime

Court outcomes (sentenced, convicted, found guilty, pleaded guilty, prison term) with an authority indicator qualify as `core_street_crime` and route to `crime:category:court-news`.

### Phase 3: Streetcode Subscriber Changes

**Repo**: `streetcode-laravel`

#### A. Update Channel List

Remove `crime:courts` and `crime:context` (peripheral crime catch-alls). Keep `crime:category:court-news` (actual court outcomes).

```php
// config/database.php
'crime_channels' => [
    'crime:homepage',
    'crime:category:violent-crime',
    'crime:category:property-crime',
    'crime:category:drug-crime',
    'crime:category:gang-violence',
    'crime:category:organized-crime',
    'crime:category:court-news',
    'crime:category:crime',
],
```

#### B. Set Quality Threshold

```env
ARTICLES_MIN_QUALITY_SCORE=50
```

#### C. Add Crime-Relevance Validation on Ingest

In `ProcessIncomingArticle.php`, add a check before storing:

```php
// Defense-in-depth: verify crime relevance
$crimeRelevance = $data['crime_relevance'] ?? '';
if ($crimeRelevance !== 'core_street_crime') {
    Log::info('Skipping non-core-crime article', [
        'crime_relevance' => $crimeRelevance,
        'title' => $data['title'] ?? 'unknown',
    ]);
    return;
}
```

#### D. Fix Topic Tag Assignment

Only assign actual crime sub-types as `crime_category` tags. Filter out generic topics:

```php
$crimeTopics = ['violent_crime', 'property_crime', 'drug_crime', 'organized_crime', 'criminal_justice', 'gang_violence'];
$filteredTopics = array_intersect($data['topics'] ?? [], $crimeTopics);
// Only create crime_category tags for actual crime topics
```

### Phase 4: Cleanup and Rollout

#### Deployment Order

1. **Publisher** (Phase 1) - Fix field mapping. Layer 3 starts working.
2. **Classifier** (Phase 2) - Tighten rules. New articles get stricter classification.
3. **Streetcode** (Phase 3) - Update channels, quality threshold, validation.
4. **Cleanup** - Run `php artisan articles:soft-delete-non-crime` to remove existing junk.

#### No Reclassification

Existing ES documents stay as-is. New rules apply going forward. Streetcode cleanup handles consumer-side remediation.

---

## Files to Modify

### North Cloud (publisher)
| File | Change |
|------|--------|
| `publisher/internal/router/service.go` | Add `CrimeData`, `LocationData` nested structs to `Article`; add `extractNestedFields()` |
| `publisher/internal/router/crime.go` | No changes needed (already uses flat fields) |
| `publisher/internal/router/crime_test.go` | Update tests with nested field extraction |

### North Cloud (classifier)
| File | Change |
|------|--------|
| `classifier/internal/classifier/crime_rules.go` | Add exclusion patterns, tighten core requirements |
| `classifier/internal/classifier/crime_rules_test.go` | Add test cases for new exclusions and tighter rules |

### Streetcode Laravel
| File | Change |
|------|--------|
| `config/database.php` | Remove `crime:courts`, `crime:context` from channels |
| `.env` / `.env.example` | Set `ARTICLES_MIN_QUALITY_SCORE=50` |
| `app/Jobs/ProcessIncomingArticle.php` | Add crime-relevance validation, fix topic tag assignment |

---

## Success Criteria

- [ ] `publish_history` shows entries for `crime:homepage` and `crime:category:*` channels
- [ ] Streetcode homepage shows only real crime events (incidents, arrests, investigations, sentencing)
- [ ] No travel, lifestyle, opinion, or political articles on Streetcode
- [ ] Quality score threshold prevents spam/low-quality content
- [ ] Defense-in-depth: even if upstream mis-classifies, Streetcode rejects non-crime articles
