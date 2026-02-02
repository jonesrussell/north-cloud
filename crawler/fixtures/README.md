# HTTP Response Fixtures

This directory contains version-controlled HTTP response fixtures for deterministic crawler testing.

## Structure

```
fixtures/
  example-com/
    GET_abc123def456.json    # Cache entry for a specific request
    GET_789xyz012abc.json    # Another request to the same domain
  another-site-org/
    POST_def456abc789.json   # POST request fixture
```

## How It Works

1. **Fixtures are read-only** - The proxy never writes to this directory
2. **Fixtures take priority** - When a request matches a fixture, it's used before checking user cache
3. **Version controlled** - Fixtures are committed to git for reproducible tests

## Adding Fixtures

### Option 1: Record Mode

1. Start proxy in record mode: `task proxy:mode:record`
2. Run your crawler against the proxy
3. Copy responses from `~/.northcloud/http-cache/{domain}/` to this directory
4. Commit the fixtures

### Option 2: Manual Creation

Create a JSON file with this structure:

```json
{
  "url": "https://example.com/page",
  "method": "GET",
  "status_code": 200,
  "headers": {
    "Content-Type": "text/html; charset=utf-8"
  },
  "body": "<html>...</html>",
  "timestamp": "2026-02-01T12:00:00Z"
}
```

## File Naming

Cache keys are generated as: `{METHOD}_{hash}` where hash is SHA-256 of normalized URL (first 12 chars).

Use the proxy's logging to see what cache key is being looked up.

## Best Practices

1. **Keep fixtures minimal** - Only include what's needed for tests
2. **Use meaningful domains** - Organize by source domain
3. **Document purpose** - Add comments explaining what each fixture tests
4. **Clean HTML** - Strip unnecessary scripts/styles to reduce size
