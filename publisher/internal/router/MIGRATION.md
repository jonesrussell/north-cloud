# Router Package Refactor — Reviewer Reference

This document explains what changed in `publisher/internal/router/` during the routing
domain refactor (Tasks 1–12). It is written for code reviewers: what to look at, what
to verify, and what was deliberately left unchanged.

---

## Purpose

The pre-refactor code had all routing logic written as exported free functions
(`GenerateLayer1Channels`, `GenerateCrimeChannels`, etc.) called sequentially in
`routeArticle`. Adding a new routing domain required touching `service.go` directly.

The refactor introduces a `RoutingDomain` interface so that:
- Each routing layer lives in its own file and owns its own tests.
- `routeArticle` is a uniform loop over `[]RoutingDomain` — adding a new domain is a
  one-line addition to the slice.
- All exported free functions that were internal-use-only are removed (they were only
  called from `routeArticle`).

Routing logic itself is **not changed**. This is a structural refactor only.

---

## What Changed

### New Files

| File | What It Contains |
|------|-----------------|
| `domain.go` | `RoutingDomain` interface, `ChannelRoute` struct, `channelRoutesFromSlice` helper |
| `article.go` | All article/data types moved from `service.go`; adds `CoforgeData` type and `Coforge *CoforgeData` field on `Article` |
| `domain_topic.go` | `TopicDomain` (Layer 1); `layer1SkipTopics` map moved here from `service.go`; `"coforge"` added to skip list |
| `domain_dbchannel.go` | `DBChannelDomain` (Layer 2); wraps `[]models.Channel` rule matching |
| `domain_coforge.go` | `CoforgeDomain` (Layer 8); new domain for Coforge ML classification |
| `testhelpers_test.go` | `routeChannelNames` test helper (package `router`, internal tests) |
| `domain_topic_test.go` | Tests for `TopicDomain` |
| `domain_dbchannel_test.go` | Tests for `DBChannelDomain` |
| `domain_coforge_test.go` | Tests for `CoforgeDomain` |

### Modified Files

**`service.go`**
- Removed all `Article`/data type declarations (moved to `article.go`).
- `routeArticle` replaced 7 explicit layer calls with a `[]RoutingDomain` loop (8 domains).
- `publishRoutes(ctx, article, []ChannelRoute) []string` added; replaces `publishToChannels`.
- `publishToChannels` removed.
- `GenerateLayer1Channels` exported free function removed.
- `layer1SkipTopics` var removed (now in `domain_topic.go`).
- `"coforge": article.Coforge` added to Redis payload in `publishToChannel`.
- Per-domain debug log added inside the routing loop.
- `maxChannelsPerArticle = 30` guardrail added (warn-only, not an error).

**`crime.go`** — `CrimeDomain` struct added; old `GenerateCrimeChannels` logic inlined
into `Routes()`; exported free function removed.

**`location.go`** — `LocationDomain` struct added; `GenerateLocationChannels` logic
inlined into `Routes()`; exported free function removed.

**`mining.go`** — `MiningDomain` struct added; `GenerateMiningChannels` logic inlined
into `Routes()`; exported free function removed.

**`entertainment.go`** — `EntertainmentDomain` struct added;
`GenerateEntertainmentChannels` logic inlined into `Routes()`; exported free function
removed.

**`anishinaabe.go`** — `AnishinaabeeDomain` struct added (note double-e, matches
existing naming convention); `GenerateAnishinaabeChannels` logic inlined into `Routes()`;
exported free function removed.

**Test files** (`crime_test.go`, `mining_test.go`, `entertainment_test.go`,
`location_test.go`, `anishinaabe_test.go`, `service_test.go`, `integration_test.go`) —
updated to call the domain `Routes()` API instead of the removed free functions.

### Removed Items (by design)

- `GenerateLayer1Channels` (was only called from `routeArticle`)
- `GenerateCrimeChannels`
- `GenerateLocationChannels`
- `GenerateMiningChannels`
- `GenerateEntertainmentChannels`
- `GenerateAnishinaabeChannels`
- `publishToChannels` (replaced by `publishRoutes`)
- `layer1SkipTopics` global var in `service.go` (moved to `domain_topic.go`)

---

## What Did NOT Change (Functional Parity)

- **Routing rules**: Every channel generation rule in every domain is identical to the
  pre-refactor free functions. No conditions were added, removed, or reordered.
- **Layer ordering**: Topic (1) → DBChannel (2) → Crime (3) → Location (4) → Mining (5)
  → Entertainment (6) → Anishinaabe (7) → Coforge (8).
- **Deduplication**: `publishToChannel` still calls `repo.CheckArticlePublished` before
  each publish; per-channel dedup behaviour is unchanged.
- **`publishToChannel` method**: Unchanged except for the `"coforge"` payload field
  (which was missing before; adding it is backwards-compatible — consumers that do not
  read it are unaffected).
- **`pollAndRoute`, `fetchArticles`, `buildESQuery`, `emitPublishedEvent`**: Unchanged.
- **Redis message format**: Backwards-compatible. Only `"coforge"` is new.

---

## How to Verify

```bash
# From the repo root (worktree)
cd /home/fsd42/dev/north-cloud/.claude/worktrees/routing-domain-refactor

# Run all router tests
cd publisher && GOWORK=off go test ./internal/router/... -v

# Run linter
cd publisher && GOWORK=off golangci-lint run ./internal/router/...

# Confirm no exported Generate* functions remain
grep -r "func Generate" publisher/internal/router/

# Confirm RoutingDomain interface is satisfied by all domains
grep -r "func.*Routes\(a \*Article\)" publisher/internal/router/

# Confirm layer ordering in routeArticle
grep -A 12 "domains := \[\]RoutingDomain" publisher/internal/router/service.go
```

Expected results:
- All tests pass.
- No linter errors.
- `grep -r "func Generate"` returns no output.
- Eight `Routes` implementations found (one per domain file).
- Layer slice in `routeArticle` lists domains in order: Topic, DBChannel, Crime,
  Location, Mining, Entertainment, Anishinaabe, Coforge.

---

## Key Design Decisions

**`ChannelRoute` instead of plain `string`**: DBChannelDomain must carry a `*uuid.UUID`
back to `publishToChannel` (for the `channel_id` payload field). Wrapping both in
`ChannelRoute` lets all domains return the same type; non-DB domains simply leave
`ChannelID` as nil.

**`channelRoutesFromSlice` helper**: Keeps the string-returning internal logic in each
domain unchanged while satisfying the `[]ChannelRoute` return type of `Routes()`.

**`AnishinaabeeDomain` (double-e)**: Intentional — matches the existing directory and
type naming already in the codebase.

**`layer1SkipTopics` moved to `domain_topic.go`**: The map is only read by
`TopicDomain.Routes()`; keeping it in the same file makes the skip logic self-contained
and easier to review.

**`maxChannelsPerArticle = 30` guardrail**: A warn-only safety net. An article published
to more than 30 channels is almost certainly a misconfiguration. It does not block
publishing — it logs a warning so operators can investigate.
