# Prospect Engine Plan

**Date:** 2026-04-18
**Status:** Approved — implementation proceeds per Appendix B
**Goal:** Evolve North Cloud from a signal engine into a prospect engine that delivers a small, high-quality daily briefing of qualified sales leads for NorthOps, with an accept/reject feedback loop that tunes the ranker over time.

---

## Design Principles (load-bearing)

These principles override any future convenience argument. They are stated first, named, and referenced by section number wherever they apply. Future edits to this spec must not relax them silently.

### P1 — CASL posture: no automated outreach

The prospect engine produces briefings. It never drafts outreach, never sends email on my behalf, and never stores contact data beyond what the source made public at crawl time. When volume grows and the temptation to automate drafting appears, the design must refuse.

Rationale: CASL exposure, brand risk, and — most importantly — the quality of outreach is the load-bearing human task. Automating it destroys the differentiation the engine is meant to support. Any feature that crosses this line is a different product and belongs in a different plan.

### P2 — Briefing-email minimalism

The daily email carries enough to identify a lead (organization name, signal type, deep link to the authenticated Waaseyaa detail page) and nothing more. Contact data, body snippets, and enrichment payloads render only on the authenticated Waaseyaa page.

Rationale: screenshots and forwards of an email must not be a data-leakage surface. Auth boundary is the contact boundary.

### P3 — Procurement gaps extend rfp-ingestor, not new crawlers

Canadian procurement sources that are missing today (MERX, Biddingo, PSPC Indigenous Business Directory) extend the existing `PortalParser` interface in `rfp-ingestor`. No new procurement crawler. No parallel path. The adapter pattern stays consistent and the ES mapping stays unified under `rfp_classified_content`.

Rationale: duplication in this layer creates two bug surfaces, two ToS risks, two mapping drifts. We already chose rfp-ingestor as the procurement landing zone; the decision is not re-litigated per new source.

---

## 1. Current-state summary

### 1.1 Corrections to the initial framing

- **"NorthOps" and "Waaseyaa" are the same service.** `signal-crawler` posts to `${NORTHOPS_URL}/api/signals`; the Lead Intel milestone posts to `${WAASEYAA_URL}/api/signals` where `WAASEYAA_URL=https://northops.ca`. Same target, two names. This is not a fourth pipeline.
- **`ClaudrielLead` is RFP-shaped, not generalized.** Fields: title, description, contact_name/email, url, closing_date, budget, sector. The existing `/api/leads` is an RFP bridge to Claudriel's `NorthCloudLeadFetcher`, not a general lead API. Repurposing it would break Claudriel. Leave it alone.
- **No existing ranker, feedback store, ICP filter, or briefing UI.** The intelligence layer is green field.
- **rfp-ingestor has CanadaBuys + SEAO Quebec only.** MERX, Biddingo, PSPC IBD are genuinely missing.

### 1.2 Signal paths today (and after the plan)

| Path | Purpose | Producer → Consumer | Status after plan |
|---|---|---|---|
| `signal-crawler` → Waaseyaa `/api/signals` | External-source scraping (HN, funding, jobs) | Go binary, cron, SQLite dedup | Kept during transition; superseded by `signal-producer` (Lead Intel #592). MIGRATION.md documents successor direction. |
| classifier `need_signal` | ES raw_content → structured NeedSignalResult | In-pipeline, gated by `NEED_SIGNAL_ENABLED` | Extended — canonical taxonomy owner for classification-derived signals. Adds `sector_alignment` component. |
| `rfp-ingestor` → ES `rfp_classified_content` | Procurement portals, bypasses classifier | Go service, bulk-indexes, Layer 11 routing | Extended — adds MERX / Biddingo / PSPC IBD parsers (per P3). |
| publisher `GET /api/leads` | Claudriel RFP bridge | Postgres `claudriel_leads` table | Kept as-is. Not repurposed. |

### 1.3 Lead Intelligence milestone as umbrella input

Issues #592–#600 describe a clean producer/enrichment architecture this plan absorbs:

- **#592** signal-producer binary (successor to signal-crawler)
- **#594** Waaseyaa HTTP client for signal POST
- **#595** checkpoint persistence
- **#596** enrichment service binary (port 8095)
- **#597** `company_intel`, `tech_stack`, `hiring` enrichers
- **#598** enrichment callback client to Waaseyaa
- **#599, #600** deployment and scheduling

The prospect engine plan retains all of this as first-wave implementation and adds the intelligence layer on top: ICP-matched sourcing, commercial signal taxonomy extension, `sector_alignment` component, `prospect_contact` enricher, briefing page, daily email digest, ranker, feedback loop.

### 1.4 What does not exist today and is in scope

- Ranker or scoring blend
- Feedback store / accept-reject surface
- ICP rule storage, segment classifier, sector-alignment scoring
- Briefing UI, email digest, delivery pipeline
- MERX / Biddingo / PSPC IBD ingestion
- Canadian funding adapter beyond OTF

---

## 2. Ordered cleanup pass

Cleanup runs before any net-new work. Each item is a PR-sized scope.

| # | Issue | Why first | Files touched |
|---|---|---|---|
| 1 | #640 write `docs/specs/lead-pipeline.md` | Umbrella vocabulary. Subsequent work re-litigates terms without it. | New spec file; cross-links from `CLAUDE.md` orchestration table, `signal-crawler/CLAUDE.md`, `classifier/CLAUDE.md`, `publisher/CLAUDE.md`, `rfp-ingestor/CLAUDE.md` |
| 2 | New: `signal-crawler/MIGRATION.md` | Declares Lead Intel `signal-producer` as the successor direction so future edits land in the right place. | `signal-crawler/MIGRATION.md` (new), `signal-crawler/CLAUDE.md` (link at top) |
| 3 | #638 unify threshold between signal-crawler and classifier `need_signal` | Required before #639 so attribution work doesn't mask a broken threshold. **Move threshold up, not down** — match the classifier's 2-keyword / 0.80-confidence gate. Extract shared helper. | `infrastructure/signal/threshold.go` (new), `signal-crawler/internal/scoring/scoring.go`, `classifier/internal/classifier/content_type_need_signal_heuristic.go`, tests both sides |
| 4 | #639 organization attribution (email-domain fallback, sector/province populated, normalization) | Benefits from the unified threshold + shared helper. | `signal-crawler/internal/adapter/jobs/*.go`, `classifier/internal/classifier/need_signal_extractor.go`, `infrastructure/signal/org_normalize.go` (new) |

**Tests and checks added:**

- Shared-threshold unit test parameterized over ~20 sample titles/bodies: same input produces same accept/reject across both paths.
- Attribution test on a dry-run production sample: org-from-email-domain yields populated `organization_name` on ≥80% of signals.
- Drift-check entry for `docs/specs/lead-pipeline.md` in `tools/drift-detector.sh`.
- `.layers` update if `infrastructure/signal/` is added (layer boundary check in CI).

---

## 3. Signal taxonomy extension

### 3.1 New commercial signal types, mapped to offers

| New / modified signal | Offer fit | Detection surface | Keyword category |
|---|---|---|---|
| `funding_announcement` (narrows `funding_win`) | Architecture advisory, platform | classifier `need_signal`; funding adapter | **new** `funding_announcement` — fresh rounds/grants to Canadian orgs |
| `rfp_published` | Platform engagement, differentiated bid | rfp-ingestor (bypasses classifier) | n/a — tagged at ingest |
| `ai_strategy_public` | Architecture advisory | classifier `need_signal` | **new** `ai_strategy`: "AI strategy", "AI roadmap", "AI governance", "responsible AI", "AI council", "AI policy draft" |
| `cto_or_senior_eng_gap` (extends `job_posting`) | Fractional wedge | classifier `need_signal`; jobs adapter | **new** `senior_eng_gap`: "hiring cto", "first engineering hire", "fractional cto", "interim cto", "head of engineering", "vp engineering" |
| `tech_migration` | Platform engagement | existing, unchanged | existing `tech_migration` |
| `ocap_data_sovereignty` | Differentiated Indigenous bid | classifier `need_signal` + Indigenous sidecar | **new** `data_sovereignty`: "OCAP", "First Nations data governance", "Indigenous data sovereignty", "community-controlled data", "consent-based data", "IDS protocols" |

### 3.2 Back-compatibility

- Existing constants (`SignalTypeOutdatedWebsite`, `SignalTypeFundingWin`, `SignalTypeJobPosting`, `SignalTypeNewProgram`, `SignalTypeTechMigration`) stay. No rename.
- `allNeedSignalKeywords()` keeps flattening the map; new categories are additive.
- `funding_win` narrows — broad "org announces grant" cases move to `funding_announcement`. Add a deprecation note in-code; do not remove the constant.

### 3.3 Files

- `classifier/internal/classifier/need_signal_extractor.go` — add `SignalType*` constants and map entries.
- `classifier/internal/classifier/content_type_need_signal_heuristic.go` — no change (shared keyword list).
- `classifier/internal/domain/rfp.go` — `rfp_published` tag applied at rfp-ingestor layer; classifier domain struct gains the type for downstream consumers.
- `classifier/internal/classifier/testdata/` — one sample per new signal type.

### 3.4 `sector_alignment` classifier component

Lightweight in-classifier component (not a sidecar — no ML model). Reads ICP rules from source-manager, runs keyword-and-topic matching, writes `icp.segments[]` onto the classified doc.

This is the **in-pipeline path** for content that flows through the classifier. Signals that arrive via signal-producer (and therefore never produce a classified_content doc) get the equivalent scoring from the `sector_alignment` **enricher** in §5.1 — same rules, same output shape, different invocation point.

```json
{
  "icp": {
    "segments": [
      {"segment": "indigenous_channel", "score": 0.92, "matched_keywords": ["First Nation", "economic development"]},
      {"segment": "northern_ontario_industry", "score": 0.41, "matched_keywords": ["Sudbury"]}
    ],
    "model_version": "v1"
  }
}
```

Gated by `SECTOR_ALIGNMENT_ENABLED`. Scores feed the ranker in §7.

#### Initial seed segments

Seed launches with three segments, matching the NorthOps target-list structure:

- `indigenous_channel` — Indigenous-owned or -adjacent orgs, Indigenous financial institutions, IBA-relevant industry
- `northern_ontario_industry` — mining, forestry, energy, municipalities in the Algoma / Manitoulin / Sudbury / Thunder Bay corridor
- `private_sector_smb` — mid-size Canadian law, accounting, engineering consultancies, bootstrapped SaaS, family-owned businesses

Source of truth at runtime is `source-manager/data/icp-segments.yml`. This list is the day-one seed; add, rename, or retire via the Ownership flow below.

#### Ownership — ICP seed YAML

Seed lives at `source-manager/data/icp-segments.yml`. Editable, but gated:

- **Edits go through PR + CI.** ICP definitions are business-critical; review discipline matters. Mild friction on the tuning loop is the acceptable cost against silent scoring corruption from a malformed edit.
- **Hot-reload preferred.** fsnotify-based reload in source-manager (service is Go and already reads from disk). Fallback: periodic re-read on a short interval. Restart-required reloads are operational friction that discourages iteration.
- **CI schema validator.** JSON Schema at `source-manager/data/icp-segments.schema.json`, enforced in CI, catches typos before they reach prod.
- **`model_version` bumps on logic changes, not seed edits.** Seed tweaks stay at v1 with a `seed_updated_at` timestamp; algorithm changes (new matcher, new field shape) bump to v2. §7 ranker weights will be keyed on `model_version` — don't invalidate learned weights every time a keyword is added.

### 3.5 Validator for `sector_alignment`

Mirrors the #663 earned-promotion pattern. Two metrics, both required before enabling on prod:

- **Coverage (nightly):** % of new classified docs with non-empty `icp.segments[]`. Threshold starts permissive; promotes to a stricter gate after N clean days.
- **Accuracy on held-out set:** measured against a hand-labelled corpus. **Labelling is pre-work for step 3 of Appendix B, not a post-deploy retrofit.** The labelled set is the ground truth — labelling after deploy defeats the validator.

**Labelled set:**

- Lives at `classifier/testdata/icp_labels.yml`, versioned, PR-reviewed.
- 50–100 docs covering the three seed segments (`indigenous_channel`, `northern_ontario_industry`, `private_sector_smb`). Russell is the labeller — domain intuition (especially Indigenous-channel) is the point; don't delegate labelling to automation, that would be circular.
- Must exist before step 3 enables `SECTOR_ALIGNMENT_ENABLED=true` on prod.

---

## 4. Adapter additions

Priority ordered. ICP fit is the sort key, not source volume.

### 4.1 High priority (Indigenous + Northern Ontario coverage gaps)

| # | Source | Landing zone | Adapter file | Notes |
|---|---|---|---|---|
| 1 | **PSPC Indigenous Business Directory** | rfp-ingestor (directory-as-seed) | `rfp-ingestor/internal/parser/pspc_ibd.go` | Open-data. Used to pre-tag organizations as `indigenous_channel` via `sector_alignment`. Low ToS risk. May need a sibling `DirectoryParser` interface — flag in §9. |
| 2 | **MERX** procurement feed | rfp-ingestor | `rfp-ingestor/internal/parser/merx.go` | **Commercial portal — ToS gate.** Preferred path: authenticated RSS/ATOM per category. If scraping is required, do not build. |
| 3 | **Biddingo** procurement feed | rfp-ingestor | `rfp-ingestor/internal/parser/biddingo.go` | Same ToS gate as MERX. Ontario-focused, strong fit for Northern Ontario segment. |
| 4 | **Northern Ontario Business** regional press | signal-crawler / signal-producer | `signal-crawler/internal/adapter/nobiz/nobiz.go` | Static HTML, plain fetch. Low volume, high signal. |
| 5 | **CCAB member news / press releases** | signal-crawler / signal-producer | `signal-crawler/internal/adapter/ccab/ccab.go` | Member directory + news feed; ICP-native for Indigenous channel. CCAB data-use terms gate. |

### 4.2 Lower priority (general Canadian funding / SMB)

| # | Source | Landing zone | Notes |
|---|---|---|---|
| 6 | Canadian Federated Press (funding announcements) | signal-crawler | RSS available. General Canadian business press. |
| 7 | Provincial economic-development news (ON, NWON, QC, MB) | signal-crawler | Per-province RSS, aggregated. |
| 8 | Crunchbase Canadian filter | signal-crawler | Paid API. Defer unless press signals prove insufficient. |

### 4.3 Deprioritized / remove from roadmap

- HN general, WeWorkRemotely, RemoteOK, HN Hiring — ICP-mismatched for the four target buyer segments. Keep running in signal-crawler as examples, **do not promote to signal-producer**. Document in `signal-crawler/MIGRATION.md`.
- GC Jobs / WorkBC — marginal for senior-engineering-gap detection; keep running, low priority.

### 4.4 Interface compliance

- All signal-crawler adapters continue to implement `adapter.Source`. No interface change.
- All rfp-ingestor parsers implement `parser.PortalParser`. PSPC IBD is a directory, not a portal — may need a new `parser.DirectoryParser` interface or a sibling `rfp-ingestor/internal/directory/` package. Decide during implementation.

### 4.5 Rate-limit and ToS risks (pre-implementation review)

- **MERX / Biddingo:** commercial portals. ToS review is a hard gate. Preferred path = authenticated feed; otherwise, skip and document the decision.
- **PSPC IBD:** open Canadian government data, CAN-permissive.
- **CCAB:** member-data terms unknown — gate.
- **Indigenous / community websites:** `docs/policies/respectful-crawl.md` (proposed — written as part of wave 1). Honors `robots.txt`, conservative default rate limits, no re-hosting of community-protected content, consent-before-scrape for band-owned domains, honors an `X-Community-Protocol: no-crawl` header as a North Cloud convention. Applies transitively when `sector_alignment` lights up Indigenous-community sources. Referenced from PSPC IBD and CCAB adapter CLAUDE.md files.

---

## 5. Enrichment sidecars

### 5.1 Services

Follow the Lead Intel #596 / #597 architecture. Each enricher is a handler on the enrichment service binary (port 8095), keyed by `enrichment_type`.

| Enricher | Input | Output | Notes |
|---|---|---|---|
| `company_intel` (#597) | `{organization_name, domain}` | `{hq_city, hq_province, employee_range, last_round, website_tech_stack_hint, years_in_operation}` | Queries ES for prior classified_content matching the org. |
| `tech_stack` (#597) | `{domain}` | `{cms, framework, analytics, hosting, age_of_site_days, observable_debt_score}` | Public site fingerprinting. |
| `hiring` (#597) | `{organization_name, domain}` | `{open_roles_count, senior_roles_count, stack_hint_from_jd}` | Queries ES for matching job-type content. |
| **`sector_alignment`** (new) | `{organization_name, domain, topics, location, indigenous_flags}` | `{segments: [{segment, score, reasoning}]}` | Reads ICP rules from source-manager. Mirrors the classifier component so recently-produced signals without classified_content still get scored. |
| **`prospect_contact`** (new, CASL-bounded per P1) | `{organization_name, domain}` | `{published_contacts: [{name, role, source_url, public_confidence}]}` | **Only contacts publicly listed on the org's own site or a public government directory.** See §5.4 for the out-of-scope list. If no public contact exists, returns empty and the briefing renders "no public contact" visibly. Queries source-manager `communities` / `people` first. |

### 5.2 I/O contract

Matches the #598 `EnrichResult` shape:

```go
type EnrichResult struct {
    EnrichmentType string         `json:"enrichment_type"`
    Confidence     float64        `json:"confidence"`
    Data           map[string]any `json:"data"`
}
```

The enrichment service POSTs results to `${WAASEYAA_URL}/api/signals/{signal_id}/enrichments`.

### 5.3 Boundaries

- Enrichment service reads ES (`${ES_URL}`) for prior matches on `organization_name` / `domain`. Does not call source-manager directly except from `sector_alignment` and `prospect_contact`.
- `prospect_contact` queries source-manager `communities` and `people` first (already authoritative for Indigenous community leadership), falls back to a public org-site scrape gated by `docs/policies/respectful-crawl.md`.

### 5.4 `prospect_contact` — explicitly out of scope

The following sources are **prohibited** by P1. They are not merely deprioritized; reintroducing any of them requires a named amendment to this spec:

- **LinkedIn scraping** — ToS violation and a scaled source of misattributed contacts.
- **Email-pattern synthesis** (e.g., `first.last@domain`) — CASL exposure on false positives.
- **Third-party data-broker lookups** (ZoomInfo, Apollo, RocketReach, etc.) — provenance is unverifiable; contacts may not meet CASL's "public disclosure" bar even when legally purchased.

If a future reviewer proposes any of these under a different name or a new enrichment type, the reviewer must cite the amendment reopening §5.4.

---

## 6. Daily briefing service

### 6.1 Stack choice: Waaseyaa (Symfony)

Justification:

- Signal-producer and enrichment callbacks already target Waaseyaa per #594 / #598 — data lands there natively.
- Waaseyaa is authenticated and in production; no new service surface.
- Feedback buttons (§7) are natural Symfony forms; ranker weight store lives alongside the accept/reject store.
- Waaseyaa's mail abstraction handles the digest.

Rejected alternatives:

- **Dashboard (Vue + Go API):** would require new auth, new schema, new deployment. No win.
- **New Go service:** gratuitous deployment surface with no reuse advantage.
- **Laravel / Drupal:** client-work stacks, not operator stacks.

### 6.2 Data reads

- `prospects` table (new, in Waaseyaa DB) — join of signal + enrichments + `icp.segments` + ranker score.
- Top N per day (default N=5), sorted by ranker score descending, filtered to the last 24 h, deduped by organization within a 14-day window.

### 6.3 Output

**Email (notification tier — minimalist per P2):**

```
Subject: NorthOps briefing — 2026-04-18

1. Acme Dev Corp · funding_announcement
   https://northops.ca/briefing/2026-04-18#p1
2. Northern Services Ltd · rfp_published
   https://northops.ca/briefing/2026-04-18#p2
3. <org> · ai_strategy_public
   https://northops.ca/briefing/2026-04-18#p3
```

No contact data. No body snippets. No enrichment payloads. Org + signal type + deep link only.

**Template guards (enforced in Waaseyaa CI):**

- Lint rule asserts no `{{ prospect.contact* }}` or `{{ prospect.enrichment* }}` substitutions exist in the digest-email template.
- Template test asserts the "no public contact found" case renders visibly on the briefing page (prevents a silent regression where an empty result disappears from the UI).

**Waaseyaa briefing page (substance tier — auth-gated):**

Per prospect:
- Signal summary + source URL
- Published contact (if `prospect_contact` found one)
- Enrichment cards: `company_intel`, `tech_stack`, `hiring`
- `sector_alignment` scores with matched keywords
- Accept / Reject / Snooze buttons
- Free-text note field

### 6.4 Delivery

- Symfony Messenger job runs daily at `BRIEFING_SCHEDULE_CRON` (default `0 6 * * *` America/Toronto).
- Delivery via existing Waaseyaa mail config.
- Single recipient initially (me). Multi-recipient is out of scope.

### 6.5 Slack is out of scope

If adopted later, Slack is a second notification transport. The substance still lives on Waaseyaa. P2 applies to Slack too — no contact data in the Slack message.

---

## 7. Feedback loop (the moat)

### 7.1 Storage schema (Waaseyaa DB)

```sql
CREATE TABLE prospect_feedback (
    id UUID PRIMARY KEY,
    prospect_id UUID NOT NULL,
    decision VARCHAR(20) NOT NULL,           -- 'accept', 'reject', 'snooze'
    reason VARCHAR(60),                       -- 'wrong_segment' | 'stale' | 'no_budget' | 'already_client' | 'not_now' | 'too_large'
    signal_type VARCHAR(40) NOT NULL,         -- denormalized for aggregation
    icp_segment VARCHAR(40) NOT NULL,         -- denormalized
    decided_at TIMESTAMPTZ NOT NULL,
    notes TEXT
);

CREATE TABLE ranker_weights (
    feature VARCHAR(80) PRIMARY KEY,          -- e.g. 'signal_type:funding_announcement'
    weight NUMERIC(8, 4) NOT NULL,
    alpha NUMERIC(10, 4) NOT NULL,            -- Beta-Binomial numerator
    beta NUMERIC(10, 4) NOT NULL,             -- Beta-Binomial denominator
    decisions_informing_weight INT NOT NULL,
    model_version VARCHAR(20) NOT NULL,       -- 'beta_binomial_v1', 'logreg_v1', ...
    updated_at TIMESTAMPTZ NOT NULL
);
```

### 7.2 Weight-adjustment mechanism

**Phase 1 (cold start to ~50 decisions) — Beta-Binomial per feature.**

- Every feature starts at Beta(1, 1) (weight 0.5).
- Each accept on a prospect increments `alpha` for each of its features; each reject increments `beta`.
- Weight at read time = `alpha / (alpha + beta)`.
- Prospect score = product of weights across the prospect's feature set, times enrichment-confidence blend.
- Transparent, auditable, survives cold start, zero ML infra.

**Phase 2 (≥50 decisions) — nightly logistic regression.**

- Features: signal_type, icp_segment, each enrichment confidence, org age, funding recency, job-posting presence.
- Label: accept=1, reject=0 (snooze excluded).
- Trained by a Symfony console command; weights atomically replace Phase 1 weights; `model_version` advances.
- Phase 1 weights remain in the table for rollback.

### 7.3 UX touchpoints

On each prospect card:

- **Accept** (green)
- **Reject** (red) + optional dropdown (wrong_segment / stale / no_budget / already_client / not_now / too_large)
- **Snooze** (grey) — re-surface in 14 days
- **Note** (free text)

One click. No confirmation modal. History view reverses any decision.

### 7.4 Cold-start behavior

- First ~50 decisions: weights hover near 0.5 (Beta-Binomial with low sample). Ranking is driven by base signal strength and enrichment confidence. Expected and intentional.
- Bootstrap seed: `source-manager/data/icp-bootstrap-weights.yml` pre-declares priors (e.g., `signal_type:rfp_published × icp_segment:indigenous_channel → 0.85`). Seeded on first deploy; marked `bootstrap=true` in the DB. DB is authoritative after seed (same pattern as ICP segments, §9.1). `task ranker:reset-to-bootstrap` admin command restores from YAML.

### 7.5 Guardrails against ranker drift

Two symmetric floors / ceilings catch both failure modes:

- **Rejection-rate floor** (prevents "accept everything"): weekly rejection rate < 15% logs a warning; < 5% freezes weight updates until manual review.
- **Acceptance-rate ceiling** (prevents "collapse on yes"): weekly acceptance rate > 80% logs a warning; > 90% freezes weight updates until manual review.

Both guardrails evaluate on a rolling 7-day window. Freeze state is stored in `ranker_weights.model_version` (e.g., `beta_binomial_v1_frozen`); operator lifts the freeze via a Symfony console command after reviewing the weekly decision distribution.

---

## 8. Elasticsearch mapping migration plan

### 8.1 New fields on classified_content

Top-level `icp` object populated by `sector_alignment`:

```yaml
icp:
  properties:
    segments:
      type: nested
      properties:
        segment: {type: keyword}
        score: {type: float}
        matched_keywords: {type: keyword}
    model_version: {type: keyword}
```

### 8.2 Migration plan

1. **Versioned migration file:** `classifier/internal/elasticsearch/mappings/vNN_add_icp.json` following the existing convention.
2. **Rollout order:**
   1. Deploy mapping update via index-manager. Additive — no reindex required.
   2. Deploy classifier with `SECTOR_ALIGNMENT_ENABLED=false`.
   3. Enable on dev; smoke-test ICP tagging on a 100-doc sample.
   4. Enable on prod. New docs populate `icp`; historical docs do not.
3. **Merge gate:** Step 3 of Appendix B (classifier `sector_alignment` component) must not merge until step 2's mapping migration is verified live in prod via `GET classified_content/_mapping`. Verification recorded in the step 3 PR body. Mirrors the #648→#649 pattern from Wave 1.
4. **Backfill:** optional, low priority. A `classifier reclassify --component=sector_alignment --since=YYYY-MM-DD` subcommand would handle it. See §9.

### 8.3 No breaking changes

- No existing field renamed or removed.
- `rfp`, `mining`, `indigenous`, `coforge`, `crime`, `entertainment` objects unchanged.

---

## 9. Risks, unknowns, questions

### 9.1 Risks

| Risk | Impact | Mitigation |
|---|---|---|
| MERX / Biddingo ToS forbid automated access | Procurement coverage stays CanadaBuys + SEAO-only | ToS review as hard gate. CanadaBuys + SEAO already covers federal + QC, which is the bulk of in-ICP procurement. |
| `prospect_contact` produces false "public" contacts | CASL exposure, brand risk (violates P1) | Hard constraint: only contacts appearing on the org's own site or a public government directory. Source URL stored as proof. If unsure, return empty. |
| Ranker drifts toward "accept everything" | Briefing quality degrades | Rejection-rate floor guardrails (§7.5). |
| Waaseyaa is SPOF for briefing + feedback + ranker | Operational risk | Acceptable today — Waaseyaa is already production. Revisit if its reliability tier drops. |
| YAML-to-DB sync confusion | Operators unsure of source of truth | **Documented pattern: YAML seeds on first deploy; DB is authoritative after.** `task icp:reset-from-yaml` admin command restores. Not a two-way merge. |
| Indigenous community data sensitivity | Reputational risk if crawled content is republished | `sector_alignment` tags are metadata for me, not published. Briefing URL is auth-gated. Respectful-crawl policy applies. |
| Email digest widens into a data-leakage surface | Violates P2 | Template is version-controlled; a CI lint asserts no contact-field substitutions in the email template. |

### 9.2 Unknowns (research needed before implementation)

1. MERX / Biddingo: do authenticated RSS / ATOM per-category feeds exist? If not, is scraping under their ToS?
2. PSPC IBD: machine-readable feed, or scrape-only?
3. Waaseyaa's current mail config: does it support the minimalist-email template without new dependencies?
4. Signal-producer (Lead Intel #592) checkpoint schema: nailed down, or room to influence before #595 lands?
5. Does a respectful-crawl policy doc already exist in this repo, or does it need writing?

### 9.3 Decisions (answered)

Decisions made at plan approval (2026-04-18). Recorded here so the spec is self-contained.

1. **Briefing cadence and size** — daily 06:00 America/Toronto, top-5. Tunable downward via config without re-spec if attention dilution appears.
2. **Snooze window** — 14 days.
3. **Reject reasons** — `wrong_segment` / `stale` / `no_budget` / `already_client` / `not_now` / **`too_large`** (engagements materially beyond single-operator ship scope, e.g., Big Four-scale multi-year programs).
4. **Phase-2 ranker trigger** — ≥50 decisions.
5. **Historical backfill of `icp` tags** — **no**. New docs populate forward. The `classifier reclassify --component=sector_alignment` subcommand in §8.2 is documented but not built in wave 1.
6. **`signal-crawler/MIGRATION.md` scope** — successor-direction only. Does not enumerate cleanup issues. Cleanup lives in `docs/specs/lead-pipeline.md` and the issue tracker.
7. **Respectful-crawl policy** — **propose new** `docs/policies/respectful-crawl.md` as part of wave 1. Referenced from §4.5 and §5.3.

---

## Appendix A — Files created or modified

```
docs/
  prospect-engine-plan.md                            # this file
  specs/lead-pipeline.md                             # cleanup #1 (issue #640)
  policies/respectful-crawl.md                       # §4.5 / §5.3 — proposed new policy

signal-crawler/
  MIGRATION.md                                       # cleanup #2

infrastructure/signal/
  threshold.go                                       # cleanup #3 (issue #638)
  org_normalize.go                                   # cleanup #4 (issue #639)

classifier/internal/classifier/
  need_signal_extractor.go                           # §3 (new signal types)
  sector_alignment.go                                # §3.4 (new component)
  elasticsearch/mappings/vNN_add_icp.json            # §8

rfp-ingestor/internal/parser/
  pspc_ibd.go                                        # §4.1 #1
  merx.go                                            # §4.1 #2 (ToS gated)
  biddingo.go                                        # §4.1 #3 (ToS gated)

signal-crawler/internal/adapter/
  nobiz/nobiz.go                                     # §4.1 #4
  ccab/ccab.go                                       # §4.1 #5

source-manager/data/
  icp-segments.yml                                   # ICP seed (YAML source of truth)
  icp-bootstrap-weights.yml                          # §7.4 bootstrap priors

(Waaseyaa repo — not this repo)
  Briefing controllers, prospect_feedback + ranker_weights migrations,
  minimalist email template (+ CI lint), nightly logistic-regression console command.
```

## Appendix B — Work-stream sequence

1. **Cleanup wave** — #640 spec, MIGRATION.md, #638 threshold, #639 attribution.
2. **ES `icp` mapping migration** (§8).
3. **classifier `sector_alignment` component + ICP seed YAML** (§3.4, source-manager seed).
4. **rfp-ingestor PSPC IBD parser** (§4.1 #1) — lowest ToS risk first.
5. **signal-producer** absorbs signal-crawler (#592 / #594 / #595).
6. **Enrichment service** (#596 – #600) plus new `sector_alignment` and `prospect_contact` enrichers (§5).
7. **Waaseyaa briefing page + feedback schema + daily email digest** (§6, §7 phase 1).
8. **Additional adapters** (§4 lower-priority items).
9. **Ranker phase 2 (logistic regression)** once the 50-decision threshold is reached (§7.2).

---
