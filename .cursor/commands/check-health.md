---
description: Check if a service is healthy
variables:
  - name: PORT
    description: Service port (8040=auth, 8050=source-manager, 8060=crawler, 8070=classifier, 8080=publisher, 8090=index-manager/search)
    default: "8060"
---

# Check Service Health

Checks the health status of a North Cloud service by querying its /health endpoint.

## Usage

This command will:
1. Send a GET request to the service's health endpoint
2. Pretty-print the JSON response with jq
3. Show service status and any health metrics

## Service Ports

- `8040` - Auth service
- `8050` - Source Manager
- `8060` - Crawler
- `8070` - Classifier
- `8080` - Publisher API
- `8090` - Index Manager / Search

## Command

```bash
curl -s http://localhost:$PORT/health | jq
```

## Example

```bash
# Check crawler health
PORT=8060
```

## Example Response

```json
{
  "status": "healthy",
  "timestamp": "2026-01-06T10:00:00Z",
  "version": "1.0.0"
}
```

## Troubleshooting

If the command fails:
- Service might not be running (use `start-service.md`)
- Port might be wrong
- Service might still be starting up (wait a few seconds)
