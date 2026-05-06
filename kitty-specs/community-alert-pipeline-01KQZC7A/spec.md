# Community Alert Pipeline

**Mission ID**: `01KQZC7A7SJJZ6EKHZ9JW3AZJG` (mid8: `01KQZC7A`)
**Mission Slug**: `community-alert-pipeline-01KQZC7A`
**Mission Type**: software-dev
**Target Branch**: `main`
**Created**: 2026-05-06

---

## 1. Overview

### 1.1 Purpose

North Cloud distributes time-sensitive community safety alerts (drug supply hazards, water advisories, evacuation orders, missing persons, and similar life-critical signals) from authoritative regional sources to community-facing applications, beginning with Minoo's community pages.

The first source is Manitoba Harm Reduction Network (safersites.ca), which publishes drug supply alerts identifying contamination risks (e.g., carfentanil, medetomidine) hours after laboratory confirmation. These alerts inform harm-reduction practices in real time. The issuing organization has publicly stated that their primary social-media distribution channel suppresses these alerts, making a deliberate, durable, sovereignty-aware distribution path a community safety priority.

### 1.2 Why this belongs in North Cloud

North Cloud is the content intelligence layer that ingests external signals, normalizes them, and routes them to consuming applications. Community safety alerts share that pattern but differ from existing pipelines (lead generation, content discovery) in three ways:

1. **Latency**: hours-fresh, not days-fresh.
2. **Failure cost**: a missed alert is a safety regression, not an annoyance.
3. **Scope semantics**: routing must be sovereignty-aware (Treaty territories, urban Indigenous communities, First Nations, Métis settlements) rather than naive geography.

A separate pipeline isolates blast radius (a misbehaving alert poll loop must not stall the lead pipeline) and lets cadence, reliability, and observability targets be tuned independently.

---

## 2. User Scenarios & Testing

### 2.1 Primary Actors

- **Community Members**: visit Minoo community pages and rely on alerts to make safety decisions.
- **Issuing Organizations**: publish alerts upstream (first instance: Manitoba Harm Reduction Network).
- **Maintainer**: operates the alert pipeline, configures sources, monitors freshness.
- **Downstream Consumers**: applications consuming the alert stream (Minoo today; future: SMS, email, mobile push).

### 2.2 Acceptance Scenarios

**AS-01: Drug supply alert from Manitoba reaches a Treaty 1 community page**

```
Given Manitoba Harm Reduction Network publishes a drug supply alert tagged for Winnipeg
When alert-crawler's next poll cycle completes (within 60 minutes)
Then the alert is persisted to the durable alert store within that cycle
And the alert is published to the live event channel
And a Minoo community page configured to surface alerts for Treaty 1 displays the alert
    with severity and expiry visible to the community member
And the alert appears for any subsequent page load (not only live subscribers active at publish time)
```

**AS-02: Corrected alert supersedes earlier version**

```
Given an alert was previously published with severity="high" and partial chemical composition
When the upstream source updates the same alert with severity="critical" and refined composition
Then alert-crawler updates the existing alert document on its next poll
And a revision_history entry records the change
And the live event channel emits an "updated" event for the alert
And consumers see only the latest version on their next read
```

**AS-03: Rescinded alert disappears within one poll cycle**

```
Given an active alert with expires_at three days in the future
When the upstream source removes that alert before its natural expiry
Then alert-crawler marks the alert as rescinded on its next poll cycle
And the live event channel emits a "rescinded" event
And consumers querying the alert store no longer see the alert as active
```

**AS-04: Subscriber reconnects after downtime and recovers active alerts**

```
Given Minoo was disconnected from the live event channel for two hours
And three alerts were published during that window
When Minoo's community pages render after reconnection
Then Minoo retrieves all currently-active alerts from the durable store
And no alerts are missed solely because of the subscriber outage
```

**AS-05: Source unreachable for an extended period**

```
Given safersites.ca has been unreachable for the last six poll cycles
When a community member views a Minoo community page
Then the page surfaces all currently-active alerts already in the store
And operational observability records source-unreachable events
And an operator can determine source freshness from observability output
```

[NEEDS CLARIFICATION: should community pages display a visible staleness indicator to the community member when the source has been unreachable for longer than a defined threshold?]

**AS-06: Scope vocabulary lookup**

```
Given a Minoo community page is configured for community "Sagkeeng First Nation" (Treaty 1)
When alert-crawler ingests an alert with scope tokens that include "treaty1" or "manitoba"
Then the page resolves the scope hierarchy via the controlled vocabulary
And displays the alert without the page configurator having to enumerate every parent token
```

### 2.3 Edge Cases

- **Edge-01**: Same alert content fetched on every poll. Outcome: idempotent, no document churn, no spurious update events.
- **Edge-02**: Upstream source returns malformed or incomplete data. Outcome: alert-crawler skips the malformed record, records the parse failure to observability, continues processing siblings.
- **Edge-03**: Alert with `expires_at` already in the past at fetch time. Outcome: recorded historically but never published as active; never emitted to the live event channel.
- **Edge-04**: Scope token not present in indigenous-taxonomy. Outcome: alert is recorded with the unknown token, observability records the missing-vocabulary event, alert is still published; consumers may fail to route until the vocabulary is extended.
- **Edge-05**: Two upstream sources publishing overlapping alerts (future state). Outcome: out of scope for this mission; deduplication is explicitly deferred.
- **Edge-06**: Alert-crawler service restart mid-poll. Outcome: idempotent semantics (Edge-01) ensure no double-publish; in-flight events may be re-emitted on restart, and consumers must tolerate.
- **Edge-07**: Massive alert burst (e.g., evacuation event affecting many regions). Outcome: publication rate bounded by poll cycle; live event channel must handle the entire batch atomically per cycle.

---

## 3. Functional Requirements

| ID | Requirement | Status |
|---|---|---|
| FR-001 | The system SHALL ingest alerts from configured external sources on a fixed cadence per source, configurable in the range of 30 to 60 minutes. | Active |
| FR-002 | The system SHALL normalize each ingested alert into a single envelope shape with top-level fields including `id`, `category`, `severity`, `scope`, `issued_at`, `expires_at`, `title`, `summary`, `hazard`, `guidance`, `sources`, and `revision_history`. | Active |
| FR-003 | The envelope SHALL support a discriminated `category` field with `harm_reduction` as the first implemented category and additional categories (water, evacuation, missing_person, etc.) reserved for future sources without schema changes. | Active |
| FR-004 | The system SHALL persist every active alert to a durable, queryable, indexed store such that consumers can retrieve the set of currently-active alerts on demand, independently of any subscription state. | Active |
| FR-005 | The system SHALL emit lifecycle events (`created`, `updated`, `rescinded`) to a dedicated live event channel each time an alert transitions state. | Active |
| FR-006 | An alert SHALL be addressable by a stable identifier across re-fetches; re-fetching unchanged content SHALL be idempotent and SHALL NOT emit lifecycle events. | Active |
| FR-007 | When an alert's content changes upstream, the system SHALL update the existing alert document in place, append an entry to `revision_history`, and emit an `updated` event. | Active |
| FR-008 | When an alert is no longer present in the upstream source before its `expires_at`, the system SHALL mark it as rescinded within one poll cycle and emit a `rescinded` event. | Active |
| FR-009 | The `scope` field SHALL be a list of tokens drawn from the controlled vocabulary maintained in the `jonesrussell/indigenous-taxonomy` package; tokens SHALL participate in a defined hierarchy (e.g., a treaty token transitively includes its constituent communities). | Active |
| FR-010 | Consumers SHALL be able to determine "does this alert apply to my community?" by resolving a single community identifier against the scope hierarchy, without enumerating parent tokens. | Active |
| FR-011 | Alerts with `expires_at` in the past SHALL be excluded from the "currently active" query result, regardless of how recently they were published. | Active |
| FR-012 | The system SHALL bypass the existing classifier and publisher routing layers; alerts SHALL flow directly from alert-crawler to the durable store and live event channel. | Active |
| FR-013 | The system SHALL operate independently of the existing signal-crawler service such that a failure in alert-crawler SHALL NOT degrade the lead-generation pipeline, and vice versa. | Active |
| FR-014 | A maintainer SHALL be able to add a new alert source without code changes if the source conforms to a supported acquisition pattern (feed or structured page); adding a new acquisition pattern is a code change. | Active |
| FR-015 | The system SHALL record a `revision_history` entry on each document for each state change, including timestamp and a human-readable change summary. | Active |

---

## 4. Non-Functional Requirements

| ID | Requirement | Threshold | Status |
|---|---|---|---|
| NFR-001 | End-to-end latency from upstream publication to consumer visibility | 95% of alerts visible to consumers within one poll cycle of upstream availability (≤60 minutes); 99% within two poll cycles (≤120 minutes), measured monthly | Active |
| NFR-002 | Durable store availability for read | 99.5% of "currently-active" queries SHALL succeed within 2 seconds, measured monthly | Active |
| NFR-003 | Live event channel delivery for connected subscribers | 99% of lifecycle events SHALL be delivered to actively-connected subscribers within 5 seconds of state change | Active |
| NFR-004 | Subscriber recovery after disconnection | A subscriber that reconnects SHALL be able to retrieve all currently-active alerts from the durable store in a single operation, with no per-alert recovery required | Active |
| NFR-005 | Source-unreachable handling | Alert-crawler SHALL retry transient failures with exponential backoff and SHALL NOT block other source polls; six consecutive failures on a source SHALL surface an operator-actionable signal | Active |
| NFR-006 | Idempotent ingestion | Re-fetching identical content over 100 consecutive poll cycles SHALL produce zero spurious lifecycle events | Active |
| NFR-007 | Test coverage | Alert-crawler service SHALL meet or exceed the 80% coverage target set by the project charter | Active |
| NFR-008 | Operational observability | Each poll cycle SHALL emit structured metrics including source identifier, fetch duration, alerts processed, alerts created/updated/rescinded counts, and parse failure counts | Active |
| NFR-009 | Blast-radius isolation | A complete failure of alert-crawler (process crash, source DDoS, parse exception loop) SHALL NOT consume resources reserved for the existing crawler/classifier/publisher pipeline | Active |

---

## 5. Constraints

| ID | Constraint | Status |
|---|---|---|
| C-001 | Alert-crawler is a new service in the north-cloud monorepo and SHALL respect existing service boundaries: services may import only from `infrastructure/` and not from each other. | Active |
| C-002 | Alert-crawler SHALL NOT introduce new languages, frameworks, or scaffold-level dependencies. Go 1.26+, the existing `infrastructure/logger` package, the existing config patterns, Redis, Elasticsearch, and Postgres are the only permitted runtime building blocks. | Active |
| C-003 | The `scope` controlled vocabulary SHALL live in `jonesrussell/indigenous-taxonomy`. This mission MAY extend that package; this mission SHALL NOT define or maintain a parallel registry inside north-cloud. | Active |
| C-004 | The durable alert store SHALL be Elasticsearch, naming-aligned with existing classifier indices (`*_classified_content` family) where appropriate, and the index mapping SHALL be managed via `index-manager`. | Active |
| C-005 | The live event channel SHALL be Redis pub/sub, with channel naming consistent with existing north-cloud conventions (channel naming follows from indigenous-taxonomy slugs where applicable). | Active |
| C-006 | Alert-crawler SHALL bypass classifier rules and publisher routing entirely; no new classifier rules or publisher channels are required for this mission. | Active |
| C-007 | Alert-crawler SHALL be operable as a oneshot process driven by an external schedule (systemd timer via the `northcloud-ansible` repo), consistent with `signal-crawler`'s deployment topology. | Active |
| C-008 | Alert-crawler SHALL respect existing quality gates: `task lint:force` clean, `task test` clean, `task drift:check` clean, `task ports:check` clean, `task layers:check` clean, lefthook pre-commit and pre-push pass without `--no-verify`. | Active |
| C-009 | This mission's specifications, plans, and implementations SHALL document material decisions per DIRECTIVE_003 and SHALL remain faithful to the approved spec per DIRECTIVE_010. | Active |
| C-010 | Production deployment SHALL follow existing patterns: Docker Compose orchestrated by `deploy.sh`, container naming `north-cloud-alert-crawler`, migrations with unique numeric prefixes, health-check skip list updated for the oneshot pattern. | Active |
| C-011 | Charter exception: this mission introduces a net-new service to a repo declared "frozen except for backlog." The maintainer has authorized the exception; rationale is documented in §8 (Architectural Decisions). | Active |

---

## 6. Success Criteria

| ID | Criterion | Measurement |
|---|---|---|
| SC-001 | A drug supply alert published by Manitoba Harm Reduction Network is visible on a Treaty 1-scoped Minoo community page within one upstream-to-consumer cycle | Time from upstream `issued_at` to first observable consumer render: ≤60 minutes for 95% of alerts, ≤120 minutes for 99% |
| SC-002 | A community member loading a Minoo community page sees all currently-active alerts that apply to their community, regardless of when they connected to the live event channel | A consumer that has never connected to the live event channel can retrieve a complete, current view via the durable store query alone |
| SC-003 | A corrected upstream alert reaches consumers without requiring consumer-side reconciliation logic | After an upstream correction, a consumer reading the durable store sees the latest state on the next read, without merging versions |
| SC-004 | A rescinded upstream alert disappears from consumers within one poll cycle | Time from upstream removal to alert no longer appearing in `currently-active` queries: ≤60 minutes |
| SC-005 | A failure isolated to alert-crawler does not degrade lead-generation throughput | During a synthetic alert-crawler outage, lead-pipeline cycle time and throughput remain within their existing baselines |
| SC-006 | Adding a new harm-reduction source (post-mission) requires no schema changes | A second harm-reduction source can be onboarded without modifying the envelope or the durable index mapping |
| SC-007 | Sovereignty-aware routing | A community page configured only for "Sagkeeng First Nation" correctly receives alerts scoped to "treaty1" or "manitoba" without manual scope enumeration |

---

## 7. Key Entities

### 7.1 Alert (community_alert envelope)

A single normalized community safety alert. Discriminated by `category`. Has a stable identifier across re-fetches. Carries severity, scope, issuance and expiry timestamps, hazard-specific structured data, recommended guidance for community members, attribution back to the issuing source(s), and a revision history.

### 7.2 Source

An upstream alert publisher. Currently a single instance: Manitoba Harm Reduction Network (safersites.ca). Has a configured polling cadence, an acquisition pattern (feed or structured page), and a freshness signal observable by operators.

### 7.3 Scope Token

A token in the controlled vocabulary maintained by `jonesrussell/indigenous-taxonomy`. Participates in a hierarchy (treaty territory → province/region → community/city). Used to express "which communities does this alert apply to?" and to resolve "does alert X apply to community Y?".

### 7.4 Lifecycle Event

A named event type emitted on the live event channel: `created`, `updated`, `rescinded`. Carries the alert identifier and sufficient payload for live subscribers to act without an additional fetch (though they MAY choose to re-read the durable store).

### 7.5 Consumer Subscription

A downstream application's subscription to the live event channel and its access pattern to the durable store. The first consumer is Minoo. The contract is the envelope schema and the channel naming convention; consumers MAY use either channel, both, or only the durable store query.

---

## 8. Architectural Decisions

The following material decisions are recorded here per DIRECTIVE_003. Detailed ADRs MAY be authored during the plan phase.

- **AD-001: Separate service from signal-crawler.** Cadence (hours vs days), failure cost (life-critical vs annoying), and blast radius (must not stall lead pipeline) are fundamentally different. A separate service isolates each concern.
- **AD-002: Generic `community_alert` envelope.** The first source is harm reduction, but the envelope is designed to accommodate water, evacuation, missing-person, and similar future categories without schema migration. Schema generality serves long-term maintainability and consumer simplicity.
- **AD-003: Elasticsearch + Redis pub/sub.** ES is the canonical durable store; Redis is the live event channel. ES gives consumers a deterministic "currently active" query independent of subscription state. Redis gives live subscribers low-latency push. This is the only shape that satisfies safety, latency, durability, and consumer-simplicity simultaneously.
- **AD-004: Bypass classifier and publisher routing.** Alerts are pre-classified at the source; routing-by-hazard is structural, not semantic. Mirrors the existing `rfp-ingestor` bypass pattern.
- **AD-005: Hierarchical scope vocabulary in `indigenous-taxonomy`.** Sovereignty-aware routing requires a controlled vocabulary, not geographic inference. Reusing and extending the existing taxonomy package keeps the vocabulary in one place rather than fragmenting across services.
- **AD-006: Stable ID, mutable document, explicit rescission, with `revision_history`.** Single source of truth per alert with full audit trail. Rescinded alerts disappear within one poll cycle (not at natural expiry) for safety reasons.
- **AD-007: Charter exception.** The repo charter declares the codebase "frozen except for backlog." This mission introduces a net-new service. The maintainer authorized the exception because the issuing organization has stated their primary distribution channel suppresses these alerts, making this a strategic addition to fill an active distribution gap. The mission still flows through specify → plan → tasks → implement → review.

---

## 9. Risks

| ID | Risk | Mitigation |
|---|---|---|
| RK-001 | Upstream source (safersites.ca) silently changes layout/format | Acquisition layer is isolated per source; parse failures emit observability signals (NFR-008); a malformed-page alarm fires; manual operator intervention to update the parser |
| RK-002 | Source unreachable for an extended period creates a "silent gap" | Six-failure threshold (NFR-005) fires an operator-actionable signal; durable store still serves last-known-active alerts (FR-004); staleness signaling to community members is flagged in AS-05 as an open clarification |
| RK-003 | Indigenous-taxonomy extension introduces breaking changes for other consumers of the package | Extension SHALL be additive only; new tokens and hierarchical relationships SHALL NOT remove or rename existing tokens; package versioning enforced |
| RK-004 | Alert-crawler publishes a corrupted alert that propagates to consumers | Strict envelope validation at publish time; malformed records are skipped (Edge-02), not published; revision_history (FR-015) lets operators see what the document looked like at each state |
| RK-005 | Misclassified scope (e.g., wrong treaty token) routes alert to wrong communities | Scope vocabulary is controlled, not free text (FR-009); operator-facing observability records each scope token used; downstream consumers are responsible for resolving the hierarchy correctly (FR-010) |
| RK-006 | Charter "freeze" exception sets precedent for unbounded scope creep | Mission documents the exception explicitly (AD-007); future net-new services require fresh maintainer authorization; this mission is sized to a single new service plus one taxonomy extension |
| RK-007 | Live event channel outage causes lost real-time updates | Subscribers fall back to durable store query (NFR-004); NFR-001 latency targets are stated against publication, not delivery to a specific subscriber |

---

## 10. Assumptions

- Consumers (Minoo today) already have read access to north-cloud's Elasticsearch and Redis surfaces, or will gain such access via the existing `northcloud-laravel` package.
- The first source (safersites.ca) publishes alerts at a cadence that is well-served by 30 to 60 minute polling. Sources requiring sub-30-minute freshness are out of scope.
- Indigenous-taxonomy package extension does not introduce breaking changes for existing consumers of that package.
- The community member viewing a Minoo page is the only end-user actor in scope; no end-user authentication or per-user preferences are introduced by this mission.
- Alert content (chemical composition, location names, service hours) is treated as not personally identifiable and does not require redaction in this mission.
- The maintainer (solo) authorizes the charter exception (AD-007); no external stakeholder review is required.

---

## 11. Out of Scope

- Categories beyond `harm_reduction` (envelope ready, sources not implemented).
- Minoo UI work beyond the consumer subscription contract (ribbon design, push notifications, mobile delivery: deferred to Minoo missions).
- Geofencing, alert deduplication across sources, severity scoring, multi-source correlation.
- Internationalization or translation of alert content beyond what the source provides.
- An admin UI for source registration (sources may be configured via the existing source-manager APIs or via service-local config; UX work is deferred).
- Push notifications, SMS, or email delivery (separate consumer integrations, separate missions).
- Authoritative quoting/citation tooling for alert provenance beyond the `sources` field.

---

## 12. Open Research Items (resolved before plan phase)

These items are explicitly deferred from spec to the research phase. They affect plan-level decisions but not spec-level requirements.

- **R-001: Feed availability on safersites.ca.** Determine whether the source exposes RSS/Atom/JSON before assuming HTML scraping. Outcome shapes the acquisition strategy in plan.
- **R-002: signal-crawler operational surface.** Read signal-crawler internals (config shape, source registration, oneshot vs daemon, Redis publish path, systemd timer cadence, Ansible role expectations). Outcome shapes alert-crawler's surface for ops consistency.
- **R-003: Confirmation of classifier/publisher bypass pattern.** Verify rfp-ingestor's bypass approach is the reference implementation; surface any divergences.
- **R-004: Indigenous-taxonomy extension scope.** Identify the specific taxonomy gaps (treaty territories, urban Indigenous designations, "all Manitoba", etc.) needed to express scopes for the first source.

---

## 13. References

- Spec template directives: DIRECTIVE_003 (Decision Documentation), DIRECTIVE_010 (Specification Fidelity).
- Existing related specs: `docs/specs/lead-pipeline.md` (signal-crawler/lead pipeline), `docs/specs/rfp-ingestor.md` (classifier/publisher bypass pattern reference).
- Charter: `.kittify/charter/charter.md`.
- External package to be extended: `github.com/jonesrussell/indigenous-taxonomy`.
- First source: Manitoba Harm Reduction Network, https://www.safersites.ca/.
