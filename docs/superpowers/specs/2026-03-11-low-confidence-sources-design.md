# Low-Confidence Sources Investigation — Design

**Date:** 2026-03-11
**Issue:** #311 — classifier: investigate chronically low-confidence sources
**Milestone:** M3: Observability Hardening

---

## Problem

The AI Observer has consistently flagged several sources with low classification confidence (below the 0.6 borderline threshold). These sources fall into four categories:

1. **Regional news with extraction issues** — Battlefords News-Optimist (0.53-0.54), Western Standard (0.53-0.56)
2. **Wrong content type** — We Work Remotely (job board, not news)
3. **Out-of-scope genres** — CNET, 9to5Mac, Consequence of Sound (tech/entertainment)
4. **Indigenous relevance** — Waatea News (Māori news, keep and improve)

## Decisions

### Source Disposition

| Source | Action | Reason |
|--------|--------|--------|
| Battlefords News-Optimist | Investigate + fix extraction | Regional news, keep |
| Western Standard | Investigate + fix extraction | Regional news, keep |
| Waatea News | Keep, improve classification | Indigenous relevance, priority for Oceania expansion |
| We Work Remotely | Disable | `wrong_content_type_job_board` |
| CNET | Disable | `out_of_scope_tech_entertainment` |
| 9to5Mac | Disable | `out_of_scope_tech_entertainment` |
| Consequence of Sound | Disable | `out_of_scope_tech_entertainment` |

### Disable Strategy

- **Deactivate, don't delete** — set `enabled = false` with metadata explaining why
- **No ES purge** — filter disabled sources at query time; defer hard deletion until storage pressure
- **Auditable** — `disable_reason` field preserves the rationale permanently

---

## Design

### 1. Source Disable with Reason (source-manager)

Add `disabled_at` and `disable_reason` fields to the source model, following the existing `feed_disabled_at`/`feed_disable_reason` pattern.

**Migration 015:**
- `disabled_at` — nullable timestamp
- `disable_reason` — nullable string

**Model changes:**
- Add `DisabledAt *time.Time` and `DisableReason *string` to `Source` struct
- Add `IsDisabled() bool` helper method for expressive conditionals

**No crawler changes needed** — the crawler already filters on the `enabled` field.

**Disable flow:** Set `enabled = false`, `disabled_at = NOW()`, `disable_reason = "<reason>"`.

### 2. Diagnostic CLI Tool (tools/source-diagnose)

A standalone read-only CLI for investigating extraction quality of any source.

**Capabilities:**
1. Query ES (`*_classified_content`) for recent documents from a given source
2. Report per-document: title, word count, confidence, quality score, content_type, published date
3. Compute aggregates: avg confidence, borderline rate (% below 0.6), avg word count
4. Optionally fetch live page and compare word count (extraction loss detection)

**Structure:**
```
tools/source-diagnose/
├── main.go          # CLI entry point (flags)
├── es.go            # ES query helpers
├── report.go        # Output formatting (table + JSON modes)
└── compare.go       # Live page fetch + comparison (--compare-live)
```

**Usage:**
```bash
# Basic: show ES doc stats for a source
go run tools/source-diagnose/main.go --source "Battlefords News-Optimist"

# With live comparison
go run tools/source-diagnose/main.go --source "Western Standard" --compare-live

# JSON output for scripting
go run tools/source-diagnose/main.go --source "Waatea News" --format json

# Custom sample size
go run tools/source-diagnose/main.go --source "CNET" --limit 20
```

**Dependencies:** ES client from `infrastructure/`, HTTP client + goquery for `--compare-live`. No database, no daemon, no source-manager API integration.

### 3. Investigation Plan

**For kept sources** (Battlefords, Western Standard, Waatea News):
1. Run diagnostic tool to identify root cause (extraction loss, missing metadata, selector issues)
2. Fix CSS selectors in source-manager if extraction is the problem
3. Re-run diagnostic to confirm improvement

**For disabled sources** (CNET, 9to5Mac, Consequence of Sound, We Work Remotely):
1. Run diagnostic tool once for baseline documentation
2. Disable with appropriate reason metadata

---

## Success Criteria

From issue #311:
- [ ] Each flagged source investigated with root cause identified
- [ ] Sources with extraction issues have crawler/selector fixes
- [ ] Inappropriate sources disabled with reason metadata
- [ ] Average borderline rate drops below 40% for remaining sources

## Not In Scope

- ES content purge for disabled sources (defer)
- Classifier retraining or threshold changes
- Automated scheduled diagnostics
- MCP tool integration for diagnostics
