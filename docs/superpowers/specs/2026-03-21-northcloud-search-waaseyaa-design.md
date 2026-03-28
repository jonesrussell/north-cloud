# North Cloud Search — Waaseyaa App at northcloud.one

**Date:** 2026-03-21 (revised 2026-03-28)
**Repo:** `waaseyaa/northcloud-search`
**Milestone:** V1 — Live Search at northcloud.one

---

## Context

`northcloud.one` currently returns "Not Found" — Caddy has no block for the domain. The old Vue 3 + Vite search-frontend is being retired.

We're replacing it with a Waaseyaa PHP app that:

1. **Subscribes to North Cloud's Redis pub/sub** — ingests all content types (articles, recipes, jobs, RFPs) as they flow through the pipeline
2. **Indexes into SQLite FTS5** — using Waaseyaa's search provider (already implemented, merged PR #533)
3. **Serves a search portal** — faceted full-text search across the entire pipeline
4. **Dogfoods Waaseyaa alpha** — first real app using the framework in production

### Why FTS5 Instead of Elasticsearch Proxy

The original design proxied to north-cloud's search service. The new approach is self-contained:
- No dependency on north-cloud's search service at runtime
- Indexes all 4 content types (not just what ES search exposes)
- Proves Waaseyaa's FTS5 search provider in production
- Simpler deployment — SQLite file, no ES cluster required

---

## Architecture

### Content Ingestion

```
North Cloud Publisher → Redis pub/sub → Subscriber Command → FTS5 Indexer → SQLite
```

**Redis subscriber** — a Symfony console command (`app:subscribe`) that:
- Subscribes to `content:*` (wildcard catches all topic channels)
- Deserializes JSON messages (see `publisher/docs/REDIS_MESSAGE_FORMAT.md`)
- Deduplicates by content `id` (same content publishes to multiple channels)
- Maps fields to a searchable document and indexes into FTS5
- Runs as a systemd service (long-running process)

**Indexed fields:**

| FTS5 Field | Source | Searchable |
|------------|--------|------------|
| `title` | `title` | yes |
| `body` | `body` / `raw_text` | yes |
| `content_type` | `content_type` | facet |
| `topics` | `topics` (JSON array → space-separated) | facet |
| `source_name` | derived from `source` URL domain | facet |
| `url` | `canonical_url` | stored |
| `og_image` | `og_image` | stored |
| `published_at` | `published_date` | sort/filter |
| `quality_score` | `quality_score` | filter |
| `metadata` | domain-specific objects as JSON | stored |

**Content types routed:** `article`, `recipe`, `job`, `rfp` (matches publisher's content_type filter).

### Routes

| Route | Method | Purpose |
|-------|--------|---------|
| `/` | GET | Homepage — search box, recent content across all types, content type counts |
| `/search` | GET | Results page — query + content_type/topic facet filters + pagination |
| `/content/{id}` | GET | Content detail — title, body, metadata, source link |
| `/api/suggest` | GET | Autocomplete JSON — FTS5 prefix query |
| `/health` | GET | Health check (200 OK) |

### Data Flow

```
User types "trud"
  → JS fetch('/api/suggest?q=trud', debounced)
  → SuggestController → FTS5 prefix search
  → Returns JSON suggestions
  → JS renders dropdown under search box

User submits search
  → GET /search?q=trudeau&type=article&topic=crime
  → SearchController → Fts5SearchProvider::search(SearchRequest)
  → Twig renders results with facet sidebar and pagination

Homepage
  → GET /
  → HomeController → recent content query + facet counts
  → Twig renders search box, type stats, latest content cards
```

### Key Decisions

- **Server-side rendered** — Twig templates, no SPA, no build step
- **Only JS is autocomplete** — inline script, debounced fetch to `/api/suggest`, renders dropdown
- **No CSS framework** — minimal hand-written CSS, clean and fast
- **No auth** — public search portal
- **Self-contained search** — FTS5 in SQLite, no external search service dependency
- **Config via env** — `REDIS_URL` for subscriber, `DATABASE_PATH` for SQLite
- **Error handling** — if FTS5 index is empty, show "No content indexed yet" state

### Project Structure

```
northcloud-search/
├── bin/waaseyaa
├── config/
│   ├── waaseyaa.php
│   └── services.php
├── public/index.php
├── src/
│   ├── Command/
│   │   └── SubscribeCommand.php    # Redis subscriber (app:subscribe)
│   ├── Controller/
│   │   ├── HomeController.php
│   │   ├── SearchController.php
│   │   ├── ContentController.php
│   │   └── SuggestController.php
│   ├── Indexer/
│   │   └── ContentIndexer.php      # Maps Redis messages → FTS5 documents
│   └── Provider/
│       └── AppServiceProvider.php
├── templates/
│   ├── base.html.twig
│   ├── home.html.twig
│   ├── search.html.twig
│   └── content.html.twig
├── storage/
│   └── search.sqlite               # FTS5 database (gitignored)
├── Dockerfile
├── .env
└── composer.json
```

---

## Deployment

**Server:** razor-crest (same server as all North Cloud services)

**Port:** 3003 (same as current Vue frontend — drop-in replacement in nginx config)

**Two containers:**
1. **Web** — PHP-FPM serving the Waaseyaa app (port 3003)
2. **Subscriber** — same image, runs `bin/waaseyaa app:subscribe` (long-running)

Both share a volume for `storage/search.sqlite`.

**Caddy change:** Add `northcloud.one` block to `northcloud-ansible/roles/north-cloud/templates/Caddyfile.j2` → proxy to nginx:8443 (existing internal nginx already routes to search-frontend:3003).

**Nginx:** Update upstream from Vue container to Waaseyaa container (same port, drop-in).

**Network:** Joins `north-cloud_north-cloud-network` to reach Redis.

**Health check:** `GET /health` returns 200 — required for deploy.sh auto-rollback.

**Rollback:** Tag current Vue image before deploy. Revert nginx upstream and `docker compose up -d` to restore.

---

## What This Tests in Waaseyaa Alpha

- `composer create-project` flow end-to-end
- Twig rendering in production
- Routing with controllers
- FTS5 search provider under real load
- Console commands (Redis subscriber)
- Containerization with shared SQLite volume

Any friction discovered gets filed as issues on `waaseyaa/framework` or `waaseyaa/waaseyaa`.

**Alpha stability mitigation:** Pin to a specific alpha tag in `composer.json`. Test `composer create-project` locally before implementation. If the skeleton is broken, fix it first.

---

## GitHub Issues (build order)

On `waaseyaa/northcloud-search`:

1. **Scaffold project** — `composer create-project`, verify skeleton works, add search package dependency, push initial commit
2. **Add Redis subscriber command** — `app:subscribe` consumes `content:*`, deduplicates, indexes to FTS5
3. **Add search controller + results template** — `GET /search?q=` queries FTS5, Twig renders results with pagination
4. **Add homepage controller + template** — `GET /` with search box, content type counts, recent items
5. **Add content detail page** — `GET /content/{id}` shows full content with metadata
6. **Add suggest endpoint + autocomplete JS** — `GET /api/suggest?q=` FTS5 prefix query, inline JS dropdown
7. **Add facet filters** — content_type and topic facets on search results sidebar
8. **Create Dockerfile** — Web + subscriber containers, shared SQLite volume
9. **Deploy to northcloud.one** — Caddy block, nginx routing, systemd subscriber
10. **Draft blog post** — "How search autocomplete works — and I rebuilt my frontend to prove it"

Skeleton/framework bugs filed on `waaseyaa/framework` or `waaseyaa/waaseyaa` as discovered.

---

## Blog Post Outline

**Title:** "How Search Autocomplete Works — And I Rebuilt My Frontend to Prove It"

1. **The tweet that started it** — @EOEboh's question, why it's a great question
2. **How autocomplete actually works** — prefix matching, ranking, FTS5 tokenization
3. **The setup** — `composer create-project waaseyaa/waaseyaa`, what the skeleton gives you
4. **Ingesting content** — Redis subscriber, mapping pipeline messages to search documents
5. **The suggest endpoint** — FTS5 prefix query, JSON response
6. **The JavaScript** — 15 lines of debounced fetch, dropdown rendering
7. **The search results page** — server-rendered, facets, pagination
8. **The homepage** — content type stats, latest content from the pipeline
9. **Deploying it** — Docker, Caddy, goes live at northcloud.one
10. **What I learned** — Waaseyaa alpha friction, FTS5 performance, what needs polish
11. **Try it yourself** — link to northcloud.one, link to the repo

---

## Deliverables

1. Live search portal at `northcloud.one` indexing all pipeline content
2. GitHub repo `waaseyaa/northcloud-search` with all issues closed
3. Blog post published
4. Tweet reply with link to app + blog
