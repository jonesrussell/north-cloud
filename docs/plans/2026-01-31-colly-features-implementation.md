# Colly Features Implementation Plan (Refined)

## Scope

Implement all Colly improvements from the review, with refinements: default `UseRandomUserAgent` true, sensible `MaxURLLength` and `MaxRequests` defaults, wire `RespectRobotsTxt` and add a mechanism to show robots.txt details for a crawl, and add basic test coverage to prevent regressions.

## 1. Config additions and defaults

**File:** [crawler/internal/config/crawler/config.go](crawler/internal/config/crawler/config.go)

- Add fields (env/yaml and defaults):
  - `UseRandomUserAgent bool` — **default `true`**
  - `UseReferer bool` — default `true`
  - `MaxURLLength int` — **default `2048`** (0 = no filter; extensions.URLLengthFilter when > 0)
  - `MaxRequests uint32` — **default `50000`** (0 = no limit; safety cap per crawl)
  - `DetectCharset bool` — default `false`
  - `TraceHTTP bool` — default `false`
  - `HTTPRetryMax int` — default `2`
  - `HTTPRetryDelay time.Duration` — default `2 * time.Second`
- Ensure `MaxBodySize` is used: config already has `DefaultMaxBodySize`; add `MaxBodySize int` field if not present (default 10MB) for collector and OnResponseHeaders.
- In `New()`, set the new defaults. Validation: `MaxURLLength >= 0`, `MaxRequests` can be 0 (unlimited) or positive.

## 2. Robots.txt: respect config and show details

**Current state:** Config has `RespectRobotsTxt` (default `true`), but [crawler/internal/crawler/collector.go](crawler/internal/crawler/collector.go) always uses `colly.IgnoreRobotsTxt()`.

- **Wire RespectRobotsTxt:** In `setupCollector`, add `colly.IgnoreRobotsTxt()` only when `!c.cfg.RespectRobotsTxt`. When `RespectRobotsTxt` is true, do not add IgnoreRobotsTxt (Colly then respects robots.txt by default).
- **Robots.txt details mechanism:**
  - Extend [crawler/internal/crawler/crawl_context.go](crawler/internal/crawler/crawl_context.go) `CrawlContext` with optional fields: `RobotsTxtURL string`, `RobotsTxtStatus int`, `RobotsTxtPreview string` (e.g. first 500 chars of body or "allowed/blocked" summary). These are for logging and future API exposure.
  - When `RespectRobotsTxt` is true, at crawl start (e.g. at end of `validateAndSetup` or in a small helper called from there), derive robots.txt URL from source URL (e.g. `scheme + host + "/robots.txt"`), fetch it once (HTTP GET via same transport/timeout as crawler, or Colly’s collector.Visit in a fire-and-forget way that doesn’t block the main visit). On response: set `CrawlContext.RobotsTxtURL`, `RobotsTxtStatus`, `RobotsTxtPreview` (truncated body or summary), and log at debug level (e.g. "robots.txt fetched", status, preview). If fetch fails, log at debug and leave fields zero/empty. This gives a clear place to later expose "last crawl’s robots.txt details" via API if needed.

## 3. Collector setup: context, timeout, body size, options

**File:** [crawler/internal/crawler/collector.go](crawler/internal/crawler/collector.go)

- `setupCollector(ctx context.Context, source *configtypes.Source)`.
- Build opts: add `colly.StdlibContext(ctx)`, `colly.MaxBodySize(c.cfg.MaxBodySize)`. Add `colly.IgnoreRobotsTxt()` only when `!c.cfg.RespectRobotsTxt`.
- When `c.cfg.UseRandomUserAgent` is true, do **not** set `colly.UserAgent(c.cfg.UserAgent)`; apply `extensions.RandomUserAgent(c.collector)` after creation. When false, set `colly.UserAgent(c.cfg.UserAgent)` as today.
- After `NewCollector(opts...))`: `SetRequestTimeout(c.cfg.RequestTimeout)`; if `DetectCharset`: ensure it’s in opts or applied; if `TraceHTTP`: add to opts; if `MaxRequests > 0`: add to opts. Apply extensions: Referer when `UseReferer`, URLLengthFilter when `MaxURLLength > 0` (default 2048 so filter is on).

**File:** [crawler/internal/crawler/start.go](crawler/internal/crawler/start.go)

- Call `c.setupCollector(ctx, source)` and, after setup, trigger the robots.txt fetch when `RespectRobotsTxt` is true (populate CrawlContext and log).

## 4. URLFilters from source Rules

Unchanged from original plan: in `setupCollector`, compile `source.Rules` by Action ("allow" → URLFilters, "disallow" → DisallowedURLFilters), add to opts. Skip invalid regexes with log.

## 5. OnError HTTP retry with Request.Retry()

Unchanged: retry count in `r.Ctx`, transient-error detection, `HTTPRetryMax` and `HTTPRetryDelay`, call `r.Request.Retry()` from OnError.

## 6. OnResponseHeaders early abort

Unchanged: abort on non-HTML Content-Type or Content-Length > MaxBodySize.

## 7. Extractor ChildAttr / ChildText

Unchanged: simplify extractMeta, extractAttr, extractTitle using Colly’s ChildAttr/ChildText.

## 8. Optional TraceHTTP logging

When `TraceHTTP` and `Debug`, log trace in OnResponse/OnScraped.

## 9. Test coverage (new)

Add tests to guard against regressions:

- **Config:** Unit test that `crawler.New()` (or equivalent) returns config with expected defaults for new fields: `UseRandomUserAgent == true`, `MaxURLLength == 2048`, `MaxRequests == 50000`, `UseReferer == true`, `RespectRobotsTxt == true`, etc. Test that validation rejects invalid values (e.g. `MaxURLLength < 0` if validated).
- **Collector setup:** Unit test that when `setupCollector(ctx, source)` is called with a mock or real config: (1) when `RespectRobotsTxt` is false, collector opts or behavior implies IgnoreRobotsTxt; when true, no IgnoreRobotsTxt. (2) When `UseRandomUserAgent` is true, UserAgent option is not set and RandomUserAgent is applied (test via collector behavior or by inspecting that extensions were called if using a wrapper). (3) When `MaxURLLength > 0`, URLLengthFilter is applied. (4) When source has Rules, URLFilters or DisallowedURLFilters are present. Can use a test helper that builds a minimal Crawler with test config and source and asserts on collector options or first request behavior. Prefer table-driven tests for true/false and 0 vs non-zero.
- **handleCrawlError retry:** Unit test that on transient error (e.g. timeout or 5xx), when retry count < HTTPRetryMax, handler calls Retry and does not IncrementError; when retries exhausted or non-retryable error, IncrementError and no Retry. Mock or capture Retry/IncrementError.
- **OnResponseHeaders:** Unit test that when Content-Type is not HTML (or Content-Length > MaxBodySize), the callback calls Abort; when HTML and within size, does not abort. Use a fake Response with appropriate headers.
- **Extractor:** Unit test that extractMeta returns content from meta[property='x'] and meta[name='x'] using ChildAttr (e.g. minimal HTML fragment and assert output). Same for extractAttr and extractTitle with ChildText.

Place tests in:

- [crawler/internal/config/crawler/config_test.go](crawler/internal/config/crawler/config_test.go) (or create) for config defaults/validation.
- [crawler/internal/crawler/collector_test.go](crawler/internal/crawler/collector_test.go) (new) for setupCollector, handleCrawlError, OnResponseHeaders behavior.
- [crawler/internal/content/rawcontent/extractor_test.go](crawler/internal/content/rawcontent/extractor_test.go) (new or extend) for extractMeta, extractAttr, extractTitle.

Use `t.Helper()` in test helpers; keep tests focused and avoid unnecessary network or full crawler runs.

## 10. Implementation order

1. Config: new fields and defaults (UseRandomUserAgent true, MaxURLLength 2048, MaxRequests 50000), MaxBodySize usage.
2. CrawlContext: add RobotsTxtURL, RobotsTxtStatus, RobotsTxtPreview; robots.txt fetch helper and call from validateAndSetup when RespectRobotsTxt.
3. Collector: wire RespectRobotsTxt (IgnoreRobotsTxt only when !RespectRobotsTxt); setupCollector(ctx, source) with StdlibContext, MaxBodySize, SetRequestTimeout; extensions (RandomUserAgent when true, Referer, URLLengthFilter); URLFilters from Rules; start.go call site.
4. OnError retry logic; OnResponseHeaders; optional TraceHTTP log.
5. Extractor: ChildAttr/ChildText simplifications.
6. Tests: config, collector (setup + handleCrawlError + OnResponseHeaders), extractor.

## Files to touch (summary)

| File | Changes |
|------|--------|
| [crawler/internal/config/crawler/config.go](crawler/internal/config/crawler/config.go) | New fields; defaults UseRandomUserAgent true, MaxURLLength 2048, MaxRequests 50000; MaxBodySize used |
| [crawler/internal/crawler/crawl_context.go](crawler/internal/crawler/crawl_context.go) | RobotsTxtURL, RobotsTxtStatus, RobotsTxtPreview |
| [crawler/internal/crawler/collector.go](crawler/internal/crawler/collector.go) | RespectRobotsTxt wiring, setupCollector(ctx, source), opts, extensions, URLFilters, retry, OnResponseHeaders |
| [crawler/internal/crawler/start.go](crawler/internal/crawler/start.go) | setupCollector(ctx, source); trigger robots.txt fetch when RespectRobotsTxt |
| [crawler/internal/content/rawcontent/extractor.go](crawler/internal/content/rawcontent/extractor.go) | extractMeta, extractAttr, extractTitle use ChildAttr/ChildText |
| [crawler/internal/config/crawler/config_test.go](crawler/internal/config/crawler/config_test.go) | New or extend: defaults and validation for new fields |
| [crawler/internal/crawler/collector_test.go](crawler/internal/crawler/collector_test.go) | New: setupCollector (RespectRobotsTxt, extensions, URLFilters), handleCrawlError retry, OnResponseHeaders |
| [crawler/internal/content/rawcontent/extractor_test.go](crawler/internal/content/rawcontent/extractor_test.go) | New or extend: extractMeta, extractAttr, extractTitle |

## Refinements vs original

- **UseRandomUserAgent** default `true`.
- **MaxURLLength** default `2048` (filter on by default).
- **MaxRequests** default `50000` (safety cap).
- **Robots.txt:** Use `RespectRobotsTxt` in collector (only IgnoreRobotsTxt when false); fetch robots.txt at crawl start when respecting, store URL/status/preview in CrawlContext and log at debug for visibility and future API.
- **Tests:** Config defaults/validation, collector setup and retry/OnResponseHeaders, extractor helpers.
