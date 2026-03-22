# North Cloud Remediation Plan — 2026-03-19

## A. Executive Summary

The full-system drift audit surfaced 17 issues. After investigation, **the system is healthier than the audit suggested.** The audit incorrectly assumed docker-compose defaults apply in production — 8 of 11 "disabled" feature flags are actually enabled via prod `.env`. The real problems were:

1. **deploy.sh was blind** — classifier health checks hit the wrong port, and 2 services weren't monitored at all. Auto-rollback for classifier failures was broken.
2. **Observability had gaps** — 6 of 11 services lacked Prometheus scrape targets. 3 services didn't even expose `/metrics`.
3. **Topic classification was noisy** — broad keyword rules with low thresholds caused 5-topic assignment on nearly every article.
4. **4 crawl sources were stale** — domain migrations broke allowed-domain checks.

All P0 and P1 issues are resolved. 6 PRs are open, all CI-green, all mergeable. 4 P2 issues remain open as tracked operational work.

---

## B. Issue Clusters & Prioritized Backlog

### Cluster 1: Deploy & Health Check Integrity (P0, CRITICAL)
- **#458** — Classifier health check wrong port (8070→8071)
- **#459** — rfp-ingestor and click-tracker missing from deploy.sh

**Status**: Fixed. PRs #475, #476. Merge order: #475 first, then #476 (both modify deploy.sh — will have trivial merge conflict).

### Cluster 2: Observability Blindness (P1)
- **#460** — 5 missing Prometheus scrape targets + 3 services lacked `/metrics`
- **#461** — Indigenous ML port documented as 8080, actual 8081

**Status**: Fixed. PRs #477, #478. Independent — merge in any order.

### Cluster 3: Classification Quality (P1)
- **#471** — Topic over-assignment (articles getting 5 marginal topics)

**Status**: Fixed. PR #479. `defaultMaxTopics` 5→3, `minGlobalConfidence` 0.5 floor.

### Cluster 4: Crawl Source Health (P1)
- **#462** — 4 sources in permanent failure loop

**Status**: Fixed operationally. ACHPR domain migration (achpr.org→achpr.au.int), Ojibwe depth reduced (100→5), Alberta URL corrected. Jobs reset.

### Cluster 5: Code Quality (P2)
- **#469** — context.TODO() in social-publisher health check

**Status**: Fixed. PR #480. Replaced with timeout context.

### Cluster 6: Audit False Positives (closed, no action)
- **#463** — 11 feature flags "disabled" → 8 are enabled in prod
- **#464** — AI Observer "disabled" → enabled in prod
- **#465** — TODOs → standard dev markers
- **#467** — Click-tracker → deployed, feature launch decision
- **#468** — Search naming → intentional dual naming
- **#474** — Empty export_test.go → Go convention

### Cluster 7: Deferred Operational Work (open, tracked)
- **#466** — ES source_name inconsistency (data quality)
- **#470** — Duplicate ES indices (index hygiene)
- **#472** — Indigenous ML sidecar disabled (model validation)
- **#473** — Only 2 formal publisher channels (by design)

---

## C. PR Plans for Open Issues

### Merge Sequence (recommended)

```
1. PR #478 (indigenous ML port docs)     — independent, trivial
2. PR #475 (classifier health port)      — deploy.sh change
3. PR #476 (add services to deploy.sh)   — deploy.sh change, resolve conflict with #475
4. PR #477 (Prometheus scrape targets)   — independent, multi-service
5. PR #479 (topic over-assignment)       — classifier behavior change
6. PR #480 (context.TODO fix)            — independent, trivial
```

### PR #475 + #476 Conflict Resolution
Both PRs modify `scripts/deploy.sh`. After merging #475, PR #476 will have a trivial conflict on the classifier port line (already fixed to 8071 in both branches). GitHub may auto-resolve or require a 1-line conflict resolution.

### Remaining Open Issues — PR Plans

**#466 (ES source_name inconsistency)**
- Files: source-manager source records (DB data, not code)
- Plan: Query ES for all `source_name` values, identify URL-style names, trace to source-manager records, update via API
- Validation: `GET /api/v1/search?facets=source_name` — verify all names are human-readable
- Session: ~2 hours, requires prod ES + source-manager API access

**#470 (Duplicate ES indices)**
- Files: None (ES operational)
- Plan: `GET _cat/indices/*_classified_content?v` → identify duplicates → compare doc counts + mappings → delete stale index
- Validation: Verify search results unchanged after cleanup
- Session: ~1 hour, requires prod ES access

**#472 (Indigenous ML sidecar)**
- Files: prod `.env` (add `INDIGENOUS_ENABLED=true`)
- Prerequisite: Validate model accuracy on sample of 100 indigenous articles
- Plan: Enable in staging first, sample classify 100 articles, compare rule-only vs hybrid results
- Session: ~2 hours

**#473 (Publisher channels)**
- No action needed. Auto-created Redis streams work correctly. Create formal channels only when consumers need configuration metadata.

---

## D. Validation & Regression Strategy

### Pre-Merge Validation (already done)
- [x] All 6 PRs pass CI (lint, test, spec-drift, CodeQL)
- [x] All 6 PRs are MERGEABLE (no conflicts)
- [x] `deploy_test.sh` regression test covers all 10 service port mappings

### Post-Merge Validation (deploy to staging)
| Check | Command | Expected |
|-------|---------|----------|
| Classifier health | `curl :8071/health` | 200 OK |
| rfp-ingestor health | `curl :8095/health` | 200 OK |
| click-tracker health | `curl :8093/health` | 200 OK |
| pipeline metrics | `curl :8075/metrics` | Prometheus exposition format |
| click-tracker metrics | `curl :8093/metrics` | Prometheus exposition format |
| rfp-ingestor metrics | `curl :8095/metrics` | Prometheus exposition format |
| Prometheus targets | Prometheus UI → Status → Targets | 10/10 UP |
| Topic assignment | Classify sample article | ≤3 topics, all score ≥0.5 |
| Deploy rollback test | Kill classifier → trigger deploy | Rollback detects failure on :8071 |

### Regression Tests Added
- `scripts/deploy_test.sh` — validates all deploy.sh health check ports against expected values (10 services, 20 checks)

---

## E. Remaining Risks & Mitigation

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Topic threshold too aggressive | Medium | Medium | Monitor topic distribution for 48h. If too few topics, lower `minGlobalConfidence` to 0.45 |
| Ojibwe crawl still times out at depth=5 | Low | Low | Monitor next crawl run (2026-03-20). If still failing, try depth=3 or increase crawl timeout |
| ACHPR new domain also redirects | Low | Low | Monitor next scheduled run (2026-03-22) |
| deploy.sh PRs conflict on merge | High | None | Trivial 1-line resolution, both branches have same fix |
| nc-http-proxy still lacks metrics | Certain | Low | Not gin-based, needs separate promhttp handler. Low priority — proxy is a dev tool, not critical path |
| Historical articles retain 5+ topics | Certain | Low | Only newly classified articles get stricter thresholds. Consider reclassification batch job if needed |

---

## F. 48-Hour Post-Deploy Monitoring Plan

### Hour 0-1: Smoke Test
- Health check all 11 services
- Verify Prometheus shows 10/10 targets UP
- Verify Grafana dashboards populate for new targets
- Run `scripts/deploy_test.sh` on production server

### Hour 1-6: Classification Monitoring
- Sample 10 newly classified articles — verify ≤3 topics each
- Check topic distribution: no single topic should exceed 30% of articles
- Monitor classifier logs for "Topic matched" debug entries

### Hour 6-24: Crawl Job Monitoring
- Verify Alberta procurement crawl succeeds (~hour 6)
- Verify Ojibwe dictionary crawl succeeds (~hour 18)
- Check rfp-ingestor health — confirm new RFPs indexed

### Hour 24-48: Steady State
- Check Prometheus alert manager — no new alerts
- Verify deploy.sh auto-rollback works (optional: test with intentional failure in staging)
- Review Grafana for any anomalies in request latency or error rates

---

## G. "If I Were the Founder" Recommendations

### 1. Fix the Audit Process, Not Just the Findings
The audit generated 17 issues. 6 were false positives (35%). The root cause: the audit checked compose defaults, not production `.env`. Future audits should SSH into prod and check actual state. Automate this as a `task audit:prod` that checks running containers, env vars, and health endpoints.

### 2. deploy.sh Is a Liability
The deploy script has hand-maintained service lists and port mappings in 3 separate places. Every new service requires touching deploy.sh in multiple locations. Replace the hardcoded lists with dynamic discovery: `docker compose config --services` for the service list, and derive health ports from compose labels or a central manifest.

### 3. Topic Classification Needs a Feedback Loop
The keyword rules in PostgreSQL have no quality signal. There's no way to know if `breaking_news` matching "update" is causing noise until someone samples search results. Add a weekly topic quality check: sample 50 random articles, compute precision per topic, alert when any topic drops below 70% precision.

### 4. The 4 Open Issues Are Low-Leverage
Of the 4 remaining issues, only #472 (indigenous ML) has user-facing impact. The others (#466, #470, #473) are hygiene. Don't schedule them — address them opportunistically when you're already in that area of the system.

### 5. Merge the 6 PRs Now
All CI green. All mergeable. No reason to wait. The deploy.sh fixes (#475, #476) are the highest-leverage changes — they restore rollback safety for the classifier and add monitoring for 2 previously invisible services.
