Checking how the project uses Elasticsearch indexes so we can give an informed answer.


[5 tools called]


Checking shard configuration per index type:


[2 tools called]


Here’s a concise summary of how your current design behaves and what to do at scale.

---

## Is “one raw + one classified index per site” OK?

**For hundreds to a few thousand sites:** yes, it’s a reasonable and common pattern.

**For tens of thousands and especially 100k sites:** no. The limit you hit is not “number of indexes” in the abstract, it’s **shards and cluster state**.

In your codebase:

- Each index is created with a configurable number of shards (default **1** per index) in `index-manager` (e.g. `getShards`, `RawContent.Shards`, `ClassifiedContent.Shards`).
- Classifier uses `*_raw_content`, publisher/search use `*_classified_content`, so you already query across many indexes via wildcards.

Rough scale:

| Sites | Indexes | Shards (1 per index, no replicas) | Fits in “1000 shards/node”? |
|-------|---------|------------------------------------|-----------------------------|
| 1k    | 2k      | 2k                                 | Yes (2–3 nodes)             |
| 10k   | 20k     | 20k                                | Needs ~20+ data nodes      |
| 100k  | 200k    | 200k                               | Needs ~200+ data nodes     |

Elasticsearch’s practical limit is **shards per node** (default 1000 per data node). So 200k indexes with 1 shard each would require on the order of 200 data nodes just for shard count, and cluster state (metadata for 200k indices) would be heavy. So the per-site index design does **not** scale to 100k sites as-is.

---

## What’s better at scale?

Two main approaches:

### 1. **Shared indexes with a `source` field (recommended at large scale)**

- **One** `raw_content` index and **one** `classified_content` index (or a small set if you use something like ILM/rollover).
- Every document has a **keyword** field (e.g. `source_name` or `source_id`) identifying the site.
- Crawler/classifier/publisher all use the same mapping; they filter or route by `source_name` when they need “per site” behavior.
- **Pros:** Fixed, small number of shards (e.g. 5–20 total); cluster state stays small; scaling is about doc volume and shard size, not number of sites.
- **Cons:** No physical isolation per site (dropping one site = delete by query on `source_name`); one mapping for all sources.

Your pipeline already has `source_name` (and similar) in the docs; the change would be to stop creating new indexes per source and to target these shared indexes instead.

### 2. **Bounded set of partitioned indexes (hybrid)**

- Keep the “many indexes” idea but cap the count: e.g. 100 “buckets” so you have at most 100 raw + 100 classified indexes.
- Index name = `raw_content_0` … `raw_content_99` (and same for classified), with bucket = `hash(source_id) % 100` (or similar).
- Each document still has `source_name` (or `source_id`) for filtering.
- **Pros:** Only 200 indexes and 200 shards (with 1 shard each), so well within “1000 shards per node”; some logical separation between buckets.
- **Cons:** More complex routing and discovery (you need to know which bucket(s) to query for a given source or for “all sources”).

---

## Practical recommendation

- **Up to ~1k–2k sites:** Your current “one raw + one classified index per site” is fine; just watch total shard count and nodes (stay under ~1000 shards per data node).
- **Beyond that (10k, 100k sites):** Move to **shared index(es) with a `source_name` (or `source_id`) field** so index and shard count no longer grow with the number of sites. That’s the standard, scalable pattern for multi-tenant or multi-source content in Elasticsearch.
