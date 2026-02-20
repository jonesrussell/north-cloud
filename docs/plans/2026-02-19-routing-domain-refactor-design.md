# Routing Domain Refactor + Coforge Layer 8 Design

**Date:** 2026-02-19
**Services affected:** publisher

---

## Goals

1. Introduce a unified `RoutingDomain` interface for all publisher routing layers.
2. Migrate all existing layers (1–7) into the new architecture.
3. Add Coforge routing as Layer 8.
4. Maintain full functional parity — no behavioral changes to existing channels.
5. Produce reviewer-friendly, minimal-diff changes within `package router`.

---

## Context

The publisher routes classified articles to Redis Pub/Sub channels via 7 sequential layers
hard-coded in `routeArticle()`. Each layer calls a package-level free function
(`GenerateCrimeChannels`, `GenerateMiningChannels`, etc.) that returns `[]string`.

Problems:
- No abstraction — adding a new domain requires editing `service.go` directly.
- No routing decision log — impossible to debug which domains matched.
- No guardrail on excessive channel fanout.
- Coforge classifier output exists but has no routing layer.
- Tests call free functions directly — no uniform test pattern.

---

## Target Architecture

### Interface (`router/domain.go`)

```go
type ChannelRoute struct {
    Channel   string
    ChannelID *uuid.UUID // nil for auto-generated channels; set only by DBChannelDomain
}

type RoutingDomain interface {
    Name() string
    Routes(a *Article) []ChannelRoute
}
```

`Applies()` is folded into `Routes()`. Returning nil or empty means "domain does not apply
to this article." The routing decision log checks `len(routes)` to distinguish match vs no-match.
This keeps the interface minimal — one method to implement, one call site per domain.

A package-level helper converts channel name strings to `[]ChannelRoute` for domains that
produce only auto-generated channels (no DB IDs):

```go
func channelRoutes(names ...string) []ChannelRoute { ... }
```

---

## File Map

All changes stay within `publisher/internal/router` (`package router`). No sub-packages.

| File | Action | Summary |
|------|--------|---------|
| `domain.go` | NEW | `RoutingDomain`, `ChannelRoute`, `channelRoutes()` helper |
| `article.go` | NEW | `Article` struct + all data structs (extracted from `service.go`) |
| `domain_topic.go` | NEW | `TopicDomain` — Layer 1, owns `layer1SkipTopics` |
| `domain_dbchannel.go` | NEW | `DBChannelDomain` — Layer 2, takes `[]models.Channel` |
| `domain_coforge.go` | NEW | `CoforgeDomain` — Layer 8 |
| `crime.go` | MODIFIED | Replace `GenerateCrimeChannels` with `CrimeDomain` struct |
| `mining.go` | MODIFIED | Replace `GenerateMiningChannels` with `MiningDomain` struct |
| `entertainment.go` | MODIFIED | Replace `GenerateEntertainmentChannels` with `EntertainmentDomain` |
| `location.go` | MODIFIED | Replace `GenerateLocationChannels` with `LocationDomain` |
| `anishinaabe.go` | MODIFIED | Replace `GenerateAnishinaabeChannels` with `AnishinaabeeDomain` |
| `service.go` | MODIFIED | Replace 7 explicit layer calls with domain slice loop |
| `*_test.go` | UPDATED | `Generate*Channels(a)` → `NewXDomain().Routes(a)` |
| `domain_coforge_test.go` | NEW | Table-driven tests for CoforgeDomain |
| `domain_topic_test.go` | NEW | Replaces Layer 1 coverage from `service_test.go` |
| `domain_dbchannel_test.go` | NEW | Replaces Layer 2 coverage from `integration_test.go` |
| `MIGRATION.md` | NEW | Reviewer notes: what changed, what stayed the same |

---

## Domain Slice Orchestration

Built fresh each poll cycle inside `routeArticle`. `DBChannelDomain` receives the
channels loaded from Postgres at the top of `pollAndRoute` — this preserves the existing
refresh-per-poll behaviour.

```go
func (s *Service) routeArticle(ctx context.Context, article *Article, channels []models.Channel) []string {
    domains := []RoutingDomain{
        NewTopicDomain(),           // Layer 1
        NewDBChannelDomain(channels), // Layer 2
        NewCrimeDomain(),           // Layer 3
        NewLocationDomain(),        // Layer 4
        NewMiningDomain(),          // Layer 5
        NewEntertainmentDomain(),   // Layer 6
        NewAnishinaabeeDomain(),    // Layer 7
        NewCoforgeDomain(),         // Layer 8
    }

    var published []string
    for _, domain := range domains {
        routes := domain.Routes(article)
        s.logger.Debug("routing decision",
            infralogger.String("domain", domain.Name()),
            infralogger.String("article_id", article.ID),
            infralogger.Int("channels", len(routes)),
        )
        published = append(published, s.publishRoutes(ctx, article, routes)...)
    }

    const maxChannelsPerArticle = 30
    if len(published) > maxChannelsPerArticle {
        s.logger.Warn("article produced excessive channel count",
            infralogger.String("article_id", article.ID),
            infralogger.Int("count", len(published)),
        )
    }

    s.emitPublishedEvent(ctx, article, published)
    return published
}
```

`publishRoutes` replaces the two existing helpers (`publishToChannels` + `publishToChannel`)
and accepts `[]ChannelRoute` directly, passing `route.ChannelID` to `publishToChannel`.

---

## Per-Domain Specification

### Layer 1 — TopicDomain (`domain_topic.go`)

Migrates `GenerateLayer1Channels` and `layer1SkipTopics`.

Skip list (expanded):
```go
var layer1SkipTopics = map[string]bool{
    "mining":      true,
    "anishinaabe": true,
    "coforge":     true, // Layer 8 handles coforge routing
}
```

Output: `articles:{topic}` for each non-skipped topic.

### Layer 2 — DBChannelDomain (`domain_dbchannel.go`)

```go
type DBChannelDomain struct {
    channels []models.Channel
}
```

Migrates the inline loop from `routeArticle`. For each channel whose `Rules.Matches()`
passes, returns a `ChannelRoute{Channel: ch.RedisChannel, ChannelID: &ch.ID}`.
This is the only domain that produces non-nil `ChannelID` values.

### Layer 3 — CrimeDomain (`crime.go`)

Migrates `GenerateCrimeChannels`. Exact channel names preserved:
- `crime:homepage`
- `crime:category:{type}`
- `crime:courts`
- `crime:context`

### Layer 4 — LocationDomain (`location.go`)

Migrates `GenerateLocationChannels` and `activeTopicPrefixes`. Active prefixes
remain crime + entertainment only. Coforge is NOT added to location routing (out of scope).

### Layer 5 — MiningDomain (`mining.go`)

Migrates `GenerateMiningChannels`. All channel names preserved.

### Layer 6 — EntertainmentDomain (`entertainment.go`)

Migrates `GenerateEntertainmentChannels`. All channel names preserved.

### Layer 7 — AnishinaabeeDomain (`anishinaabe.go`)

Migrates `GenerateAnishinaabeChannels`. All channel names preserved.

### Layer 8 — CoforgeDomain (`domain_coforge.go`)

New domain. No catch-all channel. Coforge is a product-specific namespace,
not a public topic domain.

Routing rules:
```
if relevance == "not_relevant" || relevance == "" → return nil

→ coforge:core             (relevance == "core_coforge")
→ coforge:peripheral       (relevance == "peripheral")
→ coforge:audience:{audience}
→ coforge:topic:{slug}     for each topic (lowercased, underscores→hyphens)
→ coforge:industry:{slug}  for each industry (lowercased, underscores→hyphens)
```

Redis message format: unchanged. `article.Coforge` nested object already
present in `Article` struct (to be added in `article.go`).

---

## Article Struct (`article.go`)

Extracted verbatim from `service.go`. No field changes. Adds `CoforgeData` struct
and `Coforge *CoforgeData` field to `Article`, parallel to existing `Mining`, `Crime`,
`Entertainment`, and `Anishinaabe` fields.

```go
type CoforgeData struct {
    Relevance           string   `json:"relevance"`
    RelevanceConfidence float64  `json:"relevance_confidence"`
    Audience            string   `json:"audience"`
    AudienceConfidence  float64  `json:"audience_confidence"`
    Topics              []string `json:"topics"`
    Industries          []string `json:"industries"`
    FinalConfidence     float64  `json:"final_confidence"`
    ReviewRequired      bool     `json:"review_required"`
    ModelVersion        string   `json:"model_version,omitempty"`
}
```

`publishToChannel` payload: add `"coforge": article.Coforge` alongside existing
`"mining"`, `"anishinaabe"`, `"entertainment"` fields.

---

## Testing

Every domain gets a `_test.go` with table-driven tests covering:
- No-match cases (nil / empty / not-relevant relevance)
- Single and multi-channel output cases
- Edge cases specific to each domain

Helper in test files:
```go
func channelNames(routes []router.ChannelRoute) []string {
    names := make([]string, len(routes))
    for i, r := range routes {
        names[i] = r.Channel
    }
    return names
}
```

Existing tests updated: `service_test.go`, `integration_test.go`, `crime_test.go`,
`mining_test.go`, `entertainment_test.go`, `location_test.go`, `anishinaabe_test.go`.

---

## Redis Message Format

**No changes.** All existing channel names are preserved exactly.
New coforge channels are additive. The `publisher` envelope fields are unchanged.

---

## Commit Sequence

Ordered for maximum reviewer clarity. Each commit compiles and passes tests.

1. `add RoutingDomain interface and ChannelRoute type`
2. `extract Article struct to article.go`
3. `migrate CrimeDomain to RoutingDomain`
4. `migrate MiningDomain to RoutingDomain`
5. `migrate EntertainmentDomain to RoutingDomain`
6. `migrate LocationDomain to RoutingDomain`
7. `migrate AnishinaabeeDomain to RoutingDomain`
8. `add TopicDomain (Layer 1)`
9. `add DBChannelDomain (Layer 2)`
10. `add CoforgeDomain (Layer 8)`
11. `refactor service.go to use domain slice`
12. `update all tests to table-driven domain pattern`
13. `add MIGRATION.md reviewer notes`

---

## Non-Goals

- No changes to the Redis message payload format.
- No changes to deduplication semantics.
- No changes to polling or discovery logic.
- No location routing for Coforge (future extension if needed).
- No sub-packages — everything stays in `package router`.
