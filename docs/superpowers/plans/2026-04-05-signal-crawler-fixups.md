# Signal Crawler Fixups — Handoff Prompt

Copy everything below the line into a fresh Claude Code session in `/home/jones/dev/north-cloud`.

---

## Context

The signal-crawler is a Go oneshot service in north-cloud that scans 7 sources for lead signals (HN stories, OTF funding, RemoteOK, WeWorkRemotely, HN Who's Hiring, GC Jobs, WorkBC). It was shipped in PR #608, deployed via Docker, and runs daily at 06:00 UTC via systemd timer on razor-crest (147.182.150.145).

**What's working (confirmed via dry-run on 2026-04-05):**
- RemoteOK: 97 postings fetched, 4 matched scoring keywords
- HN Who's Hiring: 170 comments parsed, 3 matched
- WeWorkRemotely: 30 postings fetched (0 matched, but parser works — just no infra-intent listings that day)
- Dedup SQLite DB, Docker networking, systemd timer all functional

**What needs fixing (4 issues):**

### 1. GC Jobs renderer timeout (#613)
The GC Jobs site is a JS-rendered Java app. The playwright-renderer's 30s timeout is exceeded. The render client at `signal-crawler/internal/render/render.go` currently hardcodes `wait_for: "networkidle"`. The renderer API supports a `timeout_ms` field in the request body.

**Best approach:** Reverse-engineer the XHR endpoint that `jobSearch.js` calls on page load. The page at `https://emploisfp-psjobs.cfp-psc.gc.ca/psrs-srfp/applicant/page2440` has `<body onload='doOnLoad("")'>` which fires JS that populates results. Finding and calling that JSON/XML API directly eliminates the need for headless rendering entirely. Use the Playwright MCP browser tools to load the page, watch network requests, and find the API endpoint. Then convert `gcjobs.go` from an HTML parser to a JSON/API client.

**Fallback approach:** Add `timeout_ms` to the render request in `render.go` and pass 60000ms for GC Jobs.

**Files:** `signal-crawler/internal/adapter/jobs/gcjobs.go`, `signal-crawler/internal/render/render.go`

### 2. WorkBC returns 0 postings (#614)
WorkBC is a Drupal + React SPA. The renderer runs but the parser finds 0 `<div class="job-posting">` elements because the actual rendered DOM uses different selectors.

**Approach:** Use Playwright MCP browser tools to navigate to `https://www.workbc.ca/find-jobs/browse-jobs?searchTerm=devops`, wait for results to load, then snapshot the DOM to see the actual HTML structure. Update `parseWorkBCHTML()` selectors to match. Also check if there's a JSON API backing the React frontend (check Network tab for XHR calls).

**Files:** `signal-crawler/internal/adapter/jobs/workbc.go`, `signal-crawler/internal/adapter/jobs/workbc_test.go`

### 3. OTF funding 0 results (#615)
URL was updated from `/funded-grants` (404) to `/our-grants/grants-awarded` (loads but returns 0 parsed grants). The parser looks for `div.views-row` with specific `views-field-*` spans. The new page likely uses different HTML structure.

**Approach:** Fetch `https://otf.ca/our-grants/grants-awarded`, inspect HTML for the grant listing elements. Update `parseGrantRows()` in `funding.go` and test fixtures.

**Files:** `signal-crawler/internal/adapter/funding/funding.go`, `signal-crawler/internal/adapter/funding/funding_test.go`

### 4. PIPELINE_API_KEY setup (#616)
The signal-crawler needs an API key to POST leads to NorthOps. Currently only dry-run works.

**Steps:**
1. Generate a secure random key
2. Add to Ansible vault: `cd ~/dev/northcloud-ansible && ansible-vault edit inventory/group_vars/all/vault.yml` — add `vault_nc_signal_crawler_api_key: "the-key"`
3. Set same key in NorthOps production `.env` as `PIPELINE_API_KEY`
4. Run Ansible: `ansible-playbook playbooks/site.yml --tags north-cloud --limit razor-crest`
5. Test with `--source jobs` (non-dry-run) on VPS

## Project conventions

- Read `north-cloud/CLAUDE.md` and `signal-crawler/CLAUDE.md` first
- Go 1.26+, golangci-lint enforced, testify for assertions
- TDD: write failing test, implement, verify pass
- All Go code runs `golangci-lint run --config ../.golangci.yml ./...` with 0 issues
- Commits directly to main for small fixes, PR for larger changes
- Deploy is automatic: push to main → CI builds Docker image → deploy.sh pulls on VPS
- VPS config managed via `~/dev/northcloud-ansible` (never manual SSH for config changes)
- Dry-run on VPS: `ssh jones@147.182.150.145 "cd /home/deployer/north-cloud && sudo docker compose -f docker-compose.base.yml -f docker-compose.prod.yml run --rm signal-crawler --dry-run"`

## Suggested order

1. #616 (API key) — quick, unblocks live operation for the 3 working boards
2. #615 (OTF) — likely just a selector update, similar to the WWR fix we just did
3. #613 (GC Jobs) — reverse-engineer the XHR API, most impactful
4. #614 (WorkBC) — investigate rendered DOM, update selectors
