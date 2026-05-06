# Redis Channel Contract — Community Alert Pipeline

**Mission**: `community-alert-pipeline-01KQZC7A`
**Phase**: Plan
**Status**: Authoritative consumer contract

## Channel

```
community_alerts:lifecycle
```

A single Redis pub/sub channel for all `community_alert` lifecycle events. Single channel covers all `category` values; consumers filter on the `category`/`severity`/`scope` convenience fields embedded in the payload.

## Naming rationale

- Prefix `community_alerts` matches the Elasticsearch index family. One concept name across both layers reduces operator cognitive load.
- Suffix `:lifecycle` distinguishes lifecycle events from any future channel partners (e.g., `community_alerts:operator-notifications` if needed). The colon delimiter aligns with existing NC convention (`indigenous:category:{slug}`).

## Payload

Each message body is a UTF-8 JSON document conforming to `lifecycle-event.schema.json` in this directory.

```json
{
  "event_type": "created",
  "event_at": "2026-05-06T19:32:14Z",
  "alert_id": "safersites:20260505fentanyl",
  "category": "harm_reduction",
  "severity": "critical",
  "scope": ["canada:manitoba", "canada:manitoba:winnipeg", "treaty:1"],
  "payload": { /* full CommunityAlert envelope */ }
}
```

## Event types

| `event_type` | Emitted when | Notes |
|---|---|---|
| `created` | Alert seen for the first time. Also emitted for items ingested by the first-deploy backfill. | `payload.lifecycle_state = "active"`. |
| `updated` | Existing Alert's content_hash changes (severity revised, composition refined, expiry extended). | `payload.revision_history` carries the latest entry. |
| `rescinded` | Alert disappeared from upstream feed before its `expires_at`, OR operator manually rescinded. | `payload.lifecycle_state = "rescinded"`, `payload.rescinded_at` set. |

## Delivery semantics

- **Best-effort.** Redis pub/sub does not persist messages and does not retry to disconnected subscribers.
- **No ordering guarantee.** Subscribers MUST handle out-of-order delivery (rare in practice). The `payload` is self-describing — the latest state at the moment of publish.
- **No durability.** Subscribers that disconnect MUST re-read the canonical state from Elasticsearch on reconnect. The `community_alerts` ES index is the authoritative store.
- **No back-pressure.** Subscribers that cannot keep up will drop messages on the Redis side. Re-read ES.

## Subscription pattern

Standard Redis `SUBSCRIBE community_alerts:lifecycle` from any subscriber. Pattern subscriptions (`PSUBSCRIBE community_alerts:*`) are not required for v1; reserved for future subdivision.

## Consumer recommendations

1. **Page-load query**: query Elasticsearch directly for `lifecycle_state == "active" AND expires_at > now()`. Live event channel is for incremental updates only.
2. **Filtering**: use `category` and `scope` from the event envelope to filter at the consumer edge. Avoid re-reading ES for events the consumer does not care about.
3. **Reconciliation**: on connect, re-read the active set from ES. Treat live events as deltas after the connection established.
4. **Rescission**: on `rescinded` event, immediately remove from the consumer's local active list. ES `lifecycle_state` is updated synchronously; a follow-up ES read confirms.

## Operator considerations

- Channel exists only when at least one publisher has emitted; subscribers can connect at any time but receive only future messages.
- Disable alert-crawler temporarily by setting `ALERT_CRAWLER_ENABLED=false` in the host `.env` (planned in Phase C) — Redis remains available for any future publisher.
- Channel is fully inside the existing `north-cloud-network` Docker network; no Redis ACL changes are required.

## Out of scope for v1

- Replay or backfill via Redis Streams (`XADD`/`XRANGE`). If durability beyond ES is needed, that is a separate mission.
- Per-tenant or per-community channels. v1 is single-channel.
- Encryption at the channel level. Network-level isolation (Docker network + Redis password from `.env`) is the v1 boundary.

## Channel name conflict check

Cross-checked against existing NC publisher channel naming conventions on 2026-05-06. The colon-prefixed `community_alerts:lifecycle` does not collide with `indigenous:category:*`, `indigenous:region:*`, or `streetcode:articles` conventions used elsewhere. Re-verify in Phase C.6 before merge.
