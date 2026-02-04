# StreetCode Runbook: Crime-Only Content & Soft Delete

**Audience**: Ops / deployers  
**StreetCode**: deployer@streetcode.net, app path `streetcode-laravel/current`  
**North Cloud prod**: jones@northcloud.biz, path `/opt/north-cloud`

This runbook covers:

1. **Soft-deleting non-crime articles** on StreetCode (Laravel) so only crime-related content is shown.
2. **Ensuring StreetCode ingests only from crime Redis channels** so no new non-crime articles are added.

---

## 1. Redis channels: crime-only vs mixed

The North Cloud publisher sends articles to several channel layers:

| Layer | Channels | Content |
|-------|----------|--------|
| **Layer 1 (topic)** | `articles:{topic}` e.g. `articles:news`, `articles:politics`, `articles:violent_crime` | All articles with that topic (mixed crime + non-crime) |
| **Layer 3 (crime)** | `crime:homepage`, `crime:category:*`, `crime:courts`, `crime:context` | **Only crime-classified articles** (`crime_relevance` ≠ `not_crime`) |

**If StreetCode subscribes to `articles:news` or `articles:politics`**, it receives worker-safety, Trump, Airbnb, and other non-crime pieces. To be **crime-only**, StreetCode must **only** subscribe to the **Layer 3 crime channels** (and optionally location channels), and **must not** subscribe to `articles:news`, `articles:politics`, `articles:technology`, etc.

### Channels to subscribe to for crime-only (StreetCode)

Subscribe to **all** of these so homepage, category pages, and courts/context content are populated:

**Core crime (Layer 3):**

- `crime:homepage` — homepage-eligible core street crime
- `crime:category:violent-crime`
- `crime:category:property-crime`
- `crime:category:drug-crime`
- `crime:category:gang-violence`
- `crime:category:organized-crime`
- `crime:category:court-news`
- `crime:category:crime`
- `crime:courts` — peripheral_crime + criminal_justice
- `crime:context` — peripheral_crime + crime_context

**Optional (location):**

- `crime:canada`
- `crime:international`
- `crime:local:{city}` (e.g. `crime:local:toronto`)
- `crime:province:{code}` (e.g. `crime:province:on`)

**Do not subscribe to** (they carry non-crime content):

- `articles:news`
- `articles:politics`
- `articles:technology`
- Any other `articles:*` topic channel unless you explicitly want mixed content.

---

## 2. Verify / fix StreetCode Redis subscription

**In the StreetCode Laravel repo** (and on server after deploy):

- The `articles:subscribe` command now subscribes to **all crime-only channels** by default (see `config/database.php` → `database.articles.crime_channels`). No code change needed if you use the default.
- To override: `php artisan articles:subscribe --channel=crime:homepage` (single) or `--channels=crime:homepage,crime:courts` (comma-separated).
- **Do not** pass `articles:news` or `articles:politics`; those deliver mixed content.

**On server** after deploy:

```bash
cd streetcode-laravel/current
# Default: subscribes to all crime_channels from config
php artisan articles:subscribe
```

Restart the systemd service (e.g. `articles-subscribe.service`) after deploy so it uses the updated command.

---

## 3. Soft-delete non-crime articles on StreetCode (Laravel)

The StreetCode Laravel app already uses `SoftDeletes` on the Article model and has a dedicated command. This hides existing non-crime articles (ingested from `articles:news`, `articles:crime`, or legacy rows with no channel) without deleting rows.

### 3.1 One-time: soft-delete existing non-crime articles

Uses `metadata->publisher->channel`: only articles where channel **starts with** `crime:` are kept; all others (e.g. `articles:news`, `articles:crime`, or null) are soft-deleted.

**On StreetCode server** (after deploy):

```bash
cd streetcode-laravel/current
# Preview how many would be soft-deleted
php artisan articles:soft-delete-non-crime --dry-run
# Apply
php artisan articles:soft-delete-non-crime
```

No migration or model change needed; the command is in the repo and works with MySQL, SQLite, and PostgreSQL.

---

## 4. Verify on North Cloud (prod)

**On North Cloud prod** (jones@northcloud.biz, `/opt/north-cloud`):

- Confirm publisher is running and publishing to crime channels:

```bash
cd /opt/north-cloud
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml ps publisher
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml logs publisher --tail=100
```

- Optionally check Redis for active crime channels:

```bash
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml exec redis redis-cli PUBSUB CHANNELS "crime:*"
```

You should see channels like `crime:homepage`, `crime:category:violent-crime`, etc. when the publisher is running and there is traffic.

---

## 5. Summary checklist

- [ ] **StreetCode**: Deploy latest code so `articles:subscribe` uses crime-only channels (default from config).
- [ ] **StreetCode**: Restart the consumer (e.g. `articles-subscribe.service`) after deploy.
- [ ] **StreetCode**: Run `php artisan articles:soft-delete-non-crime --dry-run` then `php artisan articles:soft-delete-non-crime` to hide existing non-crime articles.
- [ ] **North Cloud prod**: Publisher running; logs show no errors; `PUBSUB CHANNELS "crime:*"` shows crime channels when traffic exists.

After this, StreetCode will show only crime-related content: existing non-crime rows are hidden (soft-deleted), and new ingest is limited to crime channels.
