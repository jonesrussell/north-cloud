---
work_package_id: WP09
title: RSS Parser and Degraded Fallback
dependencies:
- WP05
- WP06
requirement_refs:
- FR-002
- FR-006
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T036
- T037
- T038
- T039
phase: B
agent: "claude:sonnet:implementer:implementer"
shell_pid: "244954"
history:
- at: '2026-05-06T20:51:29Z'
  event: created
  by: spec-kitty.tasks
authoritative_surface: alert-crawler/internal/adapter/rss/
execution_mode: code_change
mission_id: 01KQZC7A7SJJZ6EKHZ9JW3AZJG
mission_slug: community-alert-pipeline-01KQZC7A
owned_files:
- alert-crawler/internal/adapter/rss/parser.go
- alert-crawler/internal/adapter/rss/parser_test.go
- alert-crawler/internal/adapter/rss/extractor.go
- alert-crawler/internal/adapter/rss/extractor_test.go
- alert-crawler/internal/adapter/rss/testdata/**
priority: P1
tags: []
---

# WP09 — RSS Parser and Degraded Fallback

## Objective

Parse RSS XML and extract structured fields from `<description>` HTML (substance composition, location, lab source, confirmation date). Defensive: any item we cannot parse cleanly is recorded with `parse_quality: degraded` and the raw description preserved — never auto-rescinded (TC-010).

## Context

- Spec §3 FR-002, FR-006, §2.3 Edge-02
- Plan §Component Design (Acquisition), §TC-010 (parser-degraded handling)
- Research R-001 (RSS structure: title, description HTML, pubDate RFC-822, link as canonical URL, no `<guid>`)

## Branch Strategy

Standard. Parallel-safe with WP08; both target `internal/adapter/rss/` but different files.

## Subtasks

### T036 — Create `internal/adapter/rss/parser.go`

**Purpose**: RSS XML parser using stdlib `encoding/xml`.

**Steps**:
1. Create `alert-crawler/internal/adapter/rss/parser.go`:
   ```go
   package rss

   import (
       "encoding/xml"
       "fmt"
       "time"
   )

   type Feed struct {
       XMLName xml.Name `xml:"rss"`
       Channel Channel  `xml:"channel"`
   }

   type Channel struct {
       Title string `xml:"title"`
       Items []Item `xml:"item"`
   }

   type Item struct {
       Title       string `xml:"title"`
       Description string `xml:"description"`
       Link        string `xml:"link"`
       PubDate     string `xml:"pubDate"`
       Author      string `xml:"author"`
   }

   func ParseFeed(body []byte) (*Feed, error) {
       var f Feed
       if err := xml.Unmarshal(body, &f); err != nil {
           return nil, fmt.Errorf("parse feed: %w", err)
       }
       return &f, nil
   }

   // ParsePubDate parses RFC-822 with timezone (NationBuilder format).
   func ParsePubDate(s string) (time.Time, error) {
       layouts := []string{time.RFC1123Z, time.RFC1123, time.RFC822Z, time.RFC822}
       for _, layout := range layouts {
           if t, err := time.Parse(layout, s); err == nil {
               return t.UTC(), nil
           }
       }
       return time.Time{}, fmt.Errorf("parse pubDate %q: unrecognized format", s)
   }
   ```
2. Add a `DeriveID` function that builds the stable ID from a `Link`: extract path, slug-canonicalize, prepend `source_id:`. E.g., `safersites:20260505fentanyl` from `http://www.safersites.ca/20260505fentanyl`.

**Files**:
- `alert-crawler/internal/adapter/rss/parser.go` (new, ~120 lines).

**Validation**:
- Parses real safersites.ca RSS body (golden fixture).
- `DeriveID` returns deterministic, lowercase, slug-only IDs.

### T037 — `<description>` HTML extractor

**Purpose**: Extract structured fields from the HTML body inside the RSS `<description>` element.

**Steps**:
1. Create `alert-crawler/internal/adapter/rss/extractor.go`.
2. The NationBuilder `<description>` is HTML with predictable section markers. From research findings, expected sections:
   - Date / location (e.g., "Drug Alert: Winnipeg - Tue. May 5, 2026")
   - Substance sold as (e.g., "Yellow chunk sold as fentanyl")
   - Composition list (e.g., "Medetomidine (2.31%)", "Caffeine (10.8%)", "Mannitol (52.9%)", ...)
   - Reactions / notes
   - Lab source (e.g., "Health Canada Drug Analysis Service")
3. Implement extractors:
   - `ExtractTitle(item Item) string` — primarily from RSS `<title>`, fallback to first sentence of description.
   - `ExtractLocation(description string) string` — regex/string match for "Drug Alert: <location>".
   - `ExtractSubstances(description string) []string` — list of substance names from the body.
   - `ExtractComposition(description string) []domain.Substance` — pairs of name + percentage from the lab-results block.
   - `ExtractLabSource(description string) string` — "Health Canada..." or similar.
   - `ExtractGuidance(description string) []string` — bulleted recommendations ("Use with a friend", "Have naloxone").
4. Use `golang.org/x/net/html` or stdlib regex with care; do NOT pull in a heavyweight HTML parser unless necessary.

**Files**:
- `alert-crawler/internal/adapter/rss/extractor.go` (new, ~250 lines).

**Validation**:
- Each extractor has unit tests with golden inputs.
- Extracts sensible values for the safersites.ca sample.

### T038 — Defensive parsing with `parse_quality` flag

**Purpose**: When extraction fails for a given item, record `parse_quality: degraded` (not `failed`) and preserve the raw description for operator inspection. **Never** auto-rescind (TC-010).

**Steps**:
1. Create a `ParseItem(item Item, src domain.AlertSource) (domain.Alert, error)` function that combines parser + extractor.
2. Logic:
   - Required fields present (link, pubDate, title): proceed; otherwise return `parse_quality: failed` and an error.
   - Optional fields parse cleanly (substances, composition, location): `parse_quality: clean`.
   - Optional fields partial (e.g., substances list extracted but composition could not be parsed): `parse_quality: degraded`. Preserve raw description in `Alert.Summary` if Summary couldn't be derived.
3. The runner (WP15) treats `parse_quality: failed` items as "skip with metric"; `degraded` items are persisted as alerts with the flag visible to operators.
4. Auto-rescission: only happens when an alert is **absent from the next poll's feed** (handled in catalogue diff). Parse failures NEVER auto-rescind.

**Files**:
- `alert-crawler/internal/adapter/rss/parser.go` (extend with ParseItem function, +~40 lines).

**Validation**:
- Synthetic input with all fields → `parse_quality: clean`.
- Synthetic input with missing composition → `parse_quality: degraded`.
- Synthetic input with missing link → returns error (caller skips).

### T039 — Unit tests with golden fixture

**Purpose**: Verify the parser+extractor against a representative real-world fixture anonymized for the test suite.

**Steps**:
1. Create `alert-crawler/internal/adapter/rss/testdata/safersites_sample.rss` with a sanitized version of a real safersites.ca RSS response (10–15 items; mix of typical and edge-case content).
2. Create `alert-crawler/internal/adapter/rss/parser_test.go` and `extractor_test.go` with cases:
   - **TestParseFeed_GoldenFixture**: parse the fixture, assert 10–15 items returned with expected titles.
   - **TestDeriveID_Stability**: same Link → same ID; different Link → different ID.
   - **TestParsePubDate_Variants**: cover at least 3 RFC-822 variants.
   - **TestExtractComposition_HappyPath**: clean lab results extracted.
   - **TestExtractComposition_Missing**: input without composition section returns empty + degraded flag in caller.
   - **TestParseItem_AllParseQualities**: assert clean / degraded / failed flags drive correctly off input.
3. Use `t.Helper()`. Coverage ≥80%.

**Files**:
- `alert-crawler/internal/adapter/rss/testdata/safersites_sample.rss` (new fixture, ~5KB).
- `alert-crawler/internal/adapter/rss/parser_test.go` (new, ~180 lines).
- `alert-crawler/internal/adapter/rss/extractor_test.go` (new, ~250 lines).

**Validation**:
- `task test:alert-crawler` passes for the rss package.
- Coverage ≥80%.

## Definition of Done

- Parser handles real safersites.ca RSS body.
- Extractor produces structured fields for harm-reduction alerts.
- Defensive parsing flags degraded items without auto-rescinding (TC-010).
- Tests pass against golden fixture.
- Coverage ≥80%.

## Risks

- **RR-002**: NationBuilder template changes break the extractor. Mitigation: `parse_quality: degraded` flag preserves data; metric `alert_crawler.parse.failure_total` (added by WP15 observability) signals to operators that the parser needs an update.
- **HTML quirks**: NationBuilder may emit slight HTML variations across alert types. Tests should include at least two distinct alert formats.
- **Time zone handling**: pubDate carries timezone; extractor must normalize to UTC.

## Reviewer Guidance

- Verify `parse_quality: degraded` is set when extraction is partial.
- Verify NO auto-rescission on parse failure (the runner WP15 must rely solely on feed-delta detection for rescission).
- Verify the golden fixture is realistic (anonymized, but structurally faithful to real responses).
- Verify `DeriveID` is deterministic.

## Implementation Command

```bash
spec-kitty agent action implement WP09 --agent <name>
```

Depends on WP05, WP06. Parallel-safe with WP08, WP10, WP11, WP12, WP13, WP14.

## Activity Log

- 2026-05-06T22:39:45Z – claude:sonnet:implementer:implementer – shell_pid=244954 – Started implementation via action command
