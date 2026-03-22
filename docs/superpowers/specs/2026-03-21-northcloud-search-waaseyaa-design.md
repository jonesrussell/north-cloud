# North Cloud Search — Waaseyaa App Replacement

**Date:** 2026-03-21
**Repo:** `waaseyaa/northcloud-search`
**Milestone:** V1 — Live Search at northcloud.one

---

## Context

The current search frontend at `northcloud.one` is a Vue 3 + Vite SPA placeholder (port 3003). It has full-text search, faceted filtering, autocomplete, and topic navigation but no tests and no server-side rendering.

We're replacing it with a Waaseyaa PHP app — a fresh `composer create-project` that serves as:

1. **The first real Waaseyaa app** — dogfoods the `create-project` skeleton in alpha
2. **A live demo** for a tweet reply showing how search autocomplete works
3. **A blog post walkthrough** — "How search autocomplete works, and I rebuilt my frontend to prove it"
4. **The public face of North Cloud** — served at `northcloud.one`

### Tweet Hook

[@EOEboh asks](https://x.com/EOEboh): "You type 3 letters into Google and it already knows the full sentence you're about to search. How?"

The app is the live proof. The blog is the explanation.

---

## Architecture

### Routes

| Route | Method | Purpose |
|-------|--------|---------|
| `/` | GET | Homepage — search box, trending topics, top stories |
| `/search` | GET | Results page — query + topic/source facet filters + pagination |
| `/api/suggest` | GET | Autocomplete JSON — proxies to North Cloud search suggest API |

### North Cloud Search API Contract

Reference: `docs/specs/discovery-querying.md`

**Search** (`POST /api/v1/search`):
```json
// Request
{"query": "trudeau", "from": 0, "size": 10, "topics": ["crime"], "sources": ["cbc_ca"]}

// Response
{"hits": [{"title": "...", "url": "...", "snippet": "...", "source_name": "...", "topics": [...], "published_at": "..."}], "total": 1234, "facets": {"topics": {"crime": 42}, "sources": {"cbc_ca": 15}}}
```

**Suggest** (`GET /api/v1/search/suggest?q=trud`):
```json
// Response — array of suggestion strings
["trudeau", "trudeau resignation", "trudeau housing policy"]
```

Search uses POST because the query body can be complex (filters, facets). Suggest is a simple GET with a `q` param.

**Pagination**: offset/limit via `from` and `size` params. Default `size=10`. Page N = `from=(N-1)*size`.

### Data Flow

```
User types "trud"
  → JS fetch('/api/suggest?q=trud', debounced)
  → SuggestController → HTTP GET search:8092/api/v1/search/suggest?q=trud
  → Returns JSON suggestions
  → JS renders dropdown under search box

User submits search
  → GET /search?q=trudeau
  → SearchController → HTTP POST search:8092/api/v1/search
  → Twig renders results with pagination and facet sidebar
```

### Key Decisions

- **Server-side rendered** — Twig templates, no SPA, no build step
- **Only JS is autocomplete** — inline script, debounced fetch to `/api/suggest`, renders dropdown
- **No CSS framework** — minimal hand-written CSS, clean and fast
- **No auth required** — North Cloud search API public endpoints are unauthenticated
- **Config via env** — `NORTHCLOUD_API_URL` — always `http://search:8092` (container-to-container on Docker network). Never the external URL at runtime.
- **HTTP client** — `symfony/http-client`. If not in skeleton, add to composer.json (issue #1 scaffold step verifies this).
- **Error handling** — if search API is down, render "Search is temporarily unavailable" page

### Project Structure (provisional — verify against actual `composer create-project` output)

```
northcloud-search/
├── bin/waaseyaa
├── config/
│   ├── waaseyaa.php
│   └── services.php
├── public/index.php
├── src/
│   ├── Controller/
│   │   ├── HomeController.php
│   │   ├── SearchController.php
│   │   └── SuggestController.php
│   └── Provider/
│       └── AppServiceProvider.php
├── templates/
│   ├── base.html.twig
│   ├── home.html.twig
│   └── search.html.twig
├── Dockerfile
├── .env
└── composer.json
```

---

## Deployment

**Server:** razor-crest (same server as all North Cloud services)

**Port:** 3003 (same as current Vue frontend — drop-in replacement in nginx config)

**Routing change:**
- Current: Caddy → Docker nginx → Vue search-frontend (port 3003)
- New: Caddy → Docker nginx → Waaseyaa app container (port 3003)
- Nginx config: `infrastructure/nginx/conf.d/` — update the upstream for search-frontend to point at the new container

**Container:** PHP 8.4 image. If the Waaseyaa skeleton ships a Dockerfile, use it. If not, create one (file issue on `waaseyaa/waaseyaa`). Must expose port 3003.

**Health check:** `GET /health` returns 200 — required for deploy.sh auto-rollback.

**Network:** Joins `north-cloud_north-cloud-network` to reach `search:8092` directly.

**Smoke test before cutover:** Deploy container, verify `/health` and `/search?q=test` return correct responses before updating nginx route.

**Rollback:** Tag current Vue image before deploy (`docker tag ... search-frontend:pre-waaseyaa`). Revert nginx upstream and `docker compose up -d` to restore.

---

## What This Tests in Waaseyaa Alpha

- `composer create-project` flow end-to-end
- Twig rendering in production
- Routing with controllers
- Service injection (HTTP client)
- Skeleton Dockerfile / containerization
- Real app consuming an external API

Any friction discovered gets filed as issues on `waaseyaa/framework` or `waaseyaa/waaseyaa`.

**Alpha stability mitigation:** Pin to a specific alpha tag in `composer.json` (e.g., `0.1.0-alpha.38`). Test `composer create-project` locally before opening any implementation issues. If the skeleton is broken, fix it first — that becomes the blog's opening story.

---

## GitHub Issues (build order)

On `waaseyaa/northcloud-search`:

1. **Scaffold project** — `composer create-project`, verify skeleton works, confirm HTTP client available, add `.env.example` with `NORTHCLOUD_API_URL`, push initial commit
2. **Add search controller + results template** — `GET /search?q=` proxies to NC search API, Twig renders results with pagination
3. **Add homepage controller + template** — `GET /` with search box, trending topics, top stories
4. **Add suggest endpoint** — `GET /api/suggest?q=` proxies to NC suggest API, returns JSON
5. **Add autocomplete JS** — Inline script on search box, debounced fetch, dropdown rendering
6. **Add topic/source facet filters** — Sidebar or inline filters on search results
7. **Create Dockerfile** — Containerize the app for production
8. **Deploy to northcloud.one** — Docker container, nginx routing, Caddy TLS
9. **Draft blog post** — "How search autocomplete works — and I rebuilt my frontend to prove it"
10. **Post tweet reply** — Link to northcloud.one + blog post

Skeleton/framework bugs filed on `waaseyaa/framework` or `waaseyaa/waaseyaa` as discovered.

---

## Blog Post Outline

**Title:** "How Search Autocomplete Works — And I Rebuilt My Frontend to Prove It"

**Sections mapping to issues:**

1. **The tweet that started it** — @EOEboh's question, why it's a great question
2. **How autocomplete actually works** — Elasticsearch suggest, prefix matching, completion suggester, ranking by frequency
3. **The setup** — `composer create-project waaseyaa/waaseyaa`, what the skeleton gives you
4. **The suggest endpoint** — PHP controller proxying to the search API, JSON response format
5. **The JavaScript** — 15 lines of debounced fetch, dropdown rendering
6. **The search results page** — server-rendered, facets, pagination
7. **The homepage** — trending topics, top stories from the pipeline
8. **Deploying it** — Docker, nginx, Caddy, goes live at northcloud.one
9. **What I learned** — Waaseyaa alpha friction, what worked, what needs polish
10. **Try it yourself** — link to northcloud.one, link to the repo

---

## Deliverables

1. Live app at `northcloud.one` with working autocomplete
2. GitHub repo `waaseyaa/northcloud-search` with all issues closed
3. Blog post published
4. Tweet reply with link to app + blog
