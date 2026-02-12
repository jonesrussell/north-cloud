# Pipeline and Intelligence Dashboard Testing Checklist

## Context

North Cloud's pipeline is **crawl → classify → publish** (Crawler → ES raw_content → Classifier → ES classified_content → Publisher → Redis). The **Intelligence dashboard** ([docs/plans/2026-02-11-intelligence-dashboard-redesign.md](plans/2026-02-11-intelligence-dashboard-redesign.md)) answers "Is the pipeline healthy?" via problem detection rules, KPIs, and source health. Existing test assets:

- **Contract tests** in [tests/contracts/](../tests/contracts/): classifier/publisher assert fields exist in [index-manager/pkg/contracts/](../index-manager/pkg/contracts/) mappings (no runtime pipeline).
- **Planned Layer 2** pipeline integration suite in [docs/plans/2026-02-05-testing-standardization-design.md](plans/2026-02-05-testing-standardization-design.md): full stack, fixture crawl, poll ES/Redis, verify one article through.

The checklist below maps your five pitfall categories to North Cloud and then defines the crawl→classify→publish happy-path test contract.

---

## 1. Checklist: Over-reliance on happy paths


| Pitfall                               | North Cloud application                                                                     | Checklist item                                                                                                                                                                      |
| ------------------------------------- | ------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Green happy-path as "proof" of health | Pipeline integration (when built) and dashboard "all metrics healthy" are smoke checks only | **Do not** use the single pipeline integration test or "empty problem list" as the only CI gate. Keep PR gate = lint + unit + contract tests; pipeline run on merge/main or manual. |
| Skipping negative/edge tests          | Bad schemas, late data, retries are where failures show up                                  | **Add** (over time): contract tests for "missing required field" behavior at publisher; classifier tests for malformed raw docs; crawler tests for 4xx/5xx and retries.             |
| Happy-path as only gate               | Avoid one green e2e = ship                                                                  | **Do** run contract tests on every PR ([tests/contracts/](../tests/contracts/)). **Do not** block releases solely on the single pipeline integration run.                              |


**Concrete for Intelligence dashboard:** The "happy path test: all metrics healthy, expect empty problem list" in [docs/plans/2026-02-11-intelligence-dashboard-redesign.md](plans/2026-02-11-intelligence-dashboard-redesign.md) (line 77) should be documented as: "Smoke check that the rules don't fire when inputs are nominal; it does **not** prove the pipeline is healthy." Add one test per rule (condition → expected problem) and edge cases at thresholds; do not rely on this suite to validate data correctness or volume.

---

## 2. Checklist: Unrealistic test data and volume


| Pitfall                                      | North Cloud application                                                                 | Checklist item                                                                                                                                                                                                                            |
| -------------------------------------------- | --------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Tiny, clean datasets                         | Fixtures in [crawler/fixtures/](../crawler/fixtures/) and nc-http-proxy replay are minimal | **Accept** that the pipeline integration test uses small, deterministic fixtures for "one article flows through." **Do not** assert performance, skew, or scale. Document that load/volume/backlog tests are out of scope for this suite. |
| No late arrivals, spikes, missing partitions | Real failures often come from timing and distribution                                   | **Do not** have the happy-path test assert latency, throughput, or backlog depth. **Do** (separately, if needed) add tests or runbooks for: classifier backlog growth, crawler job failures, publisher cursor lag.                        |


**Concrete:** The design's fixture strategy (news article, listing page, crime article) is appropriate for "contract + connectivity" only. Add a short note in the test design or README: "This run does not exercise volume, late data, or spike behavior."

---

## 3. Checklist: Schema and contract evolution


| Pitfall                                      | North Cloud application                                            | Checklist item                                                                                                                                                                                                                                                                                                                        |
| -------------------------------------------- | ------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Validating "it runs" but not schema/contract | Pipeline integration will verify "message arrives"                 | **Do** assert in the pipeline integration test that the Redis message and ES documents contain **contract-relevant fields** (e.g. required fields from [publisher/docs/REDIS_MESSAGE_FORMAT.md](../publisher/docs/REDIS_MESSAGE_FORMAT.md) and [index-manager/pkg/contracts/](../index-manager/pkg/contracts/)), not just "non-empty JSON." |
| Schema drift between envs or over time       | Classifier/publisher share [index-manager](../index-manager) mappings | **Do** keep contract tests in PR gate so that new producer/consumer code cannot merge if it drops or renames required fields. **Do** document that index-manager mapping changes require contract test updates and a pipeline integration run.                                                                                        |


**Concrete:** In the Layer 2 test sequence (design doc lines 168–173), "Verify … Redis message has expected fields and values" and "All fields match contract expectations" should be spelled out: e.g. assert presence (and optionally types) of `title`, `body`, `content_type`, `quality_score`, `topics`, `crime` (or equivalent), and `publisher.published_at`, `publisher.channel`. Do **not** assert every optional field or full schema equality; that belongs in contract tests and docs.

---

## 4. Checklist: End-to-end and monitoring integration


| Pitfall                    | North Cloud application                                                 | Checklist item                                                                                                                                                                                                                                   |
| -------------------------- | ----------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Only per-stage happy paths | Single pipeline integration run covers crawl→classify→publish           | **Do** verify in one run: raw_content has doc with `classification_status` updated, classified_content has the expected document, and at least one Redis message is received on the expected channel with the expected article identifier.       |
| No runtime monitoring      | Dashboard problems and pipeline service events are the monitoring layer | **Do not** expect the happy-path test to replace monitoring. **Do** document that production health is judged by: Intelligence dashboard (problems, KPIs, source health), pipeline service funnel/events, and alerts on freshness/volume/errors. |


**Concrete:** The pipeline integration test should be described as: "End-to-end **smoke** check: one article flows from fixture URL to Redis. It does not validate volume, completeness, or SLAs." Add a line to the design or runbook: "Production health = dashboard + pipeline events + alerts; this test is not a substitute."

---

## 5. Checklist: Test maintenance and brittleness


| Pitfall                                | North Cloud application                       | Checklist item                                                                                                                                                                                                                                                                    |
| -------------------------------------- | --------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Happy-path drift from reality          | Pipelines and contracts change                | **Do** keep the pipeline integration test focused: one fixture, one source, one channel, one route. When adding stages (e.g. pipeline service) or new routing layers, extend the test **minimally** (e.g. one more assertion), and avoid turning it into a full regression suite. |
| Mixing many concerns into one scenario | Security, rare edge cases in one "happy path" | **Do not** add auth bypass tests, chaos, or rare edge cases into the same test that asserts "one article published." Keep that test **single-purpose**: pipeline connectivity and contract at the Redis/ES boundaries.                                                            |


**Concrete:** In the test sequence, avoid asserting exact quality score, exact topic set, or exact crime subtype unless necessary for contract (e.g. "article is on channel X"). Prefer "message contains required fields and linked article id" over "message matches this exact JSON."

---

## Crawl → Classify → Publish happy-path test: what to assert and what not to

### Should assert (minimal smoke + contract)

- **Crawler:** After the run, the `{source}_raw_content` index exists and contains at least one document with `classification_status` = `"classified"` (or the final status after classifier run).
- **Classifier:** The `{source}_classified_content` index exists and contains at least one document with the same logical article (e.g. same id or url), with required contract fields present (e.g. `content_type`, `quality_score`, `topics`; and nested `crime` or `location` if the fixture is crime/news and routing depends on it).
- **Publisher:** A message is received on the expected Redis channel (e.g. topic or crime channel) within a bounded timeout; the message body includes required payload fields: `id`, `title`, `body` or `raw_text`, `content_type`, `quality_score`, `topics`, and `publisher.channel`, `publisher.published_at`.
- **End-to-end:** The article id (or url) that appears in raw_content and classified_content is the same as the one in the Redis message.

Optionally: assert that a listing-page fixture does **not** produce a publish (content_type ≠ article), to confirm publisher filtering in the same run.

### Should not assert

- **Exact values:** Specific quality score, exact topic list, or exact crime subtype (unless a fixture is designed for that and the test is explicitly a "contract for this fixture").
- **Volume or performance:** Count of documents beyond "at least one," latency, throughput, or backlog.
- **Full schema:** Every optional field or full JSON equality; that belongs in contract tests and REDIS_MESSAGE_FORMAT.
- **Security or auth:** Auth is tested elsewhere; the pipeline test can use a test token or bypass only for this harness.
- **Resilience:** No retries, no chaos, no "Redis down" or "ES slow" in this test; those are separate tests or runbooks.

---

## Summary


| Area             | Action                                                                                                                            |
| ---------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| Overconfidence   | Treat pipeline integration and "empty problems" as smoke only; keep PR gate = unit + contract; add negative/edge tests over time. |
| Data/volume      | Keep fixtures small; document "no volume/performance" scope.                                                                      |
| Schema/contract  | In pipeline test, assert required fields on ES docs and Redis message; keep contract tests in PR.                                 |
| E2E + monitoring | One e2e smoke run; document that production health = dashboard + pipeline events + alerts.                                        |
| Brittleness      | One article, one purpose; no exact JSON, no mixing in resilience/security.                                                        |
