---
description: View logs for a specific service
variables:
  - name: SERVICE
    description: Service name to view logs for
    default: crawler
---

# View Service Logs

Tails logs for a specific North Cloud service running in Docker.

## Usage

This command will:
1. Navigate to the project root
2. Connect to the service's log stream
3. Tail logs in real-time
4. Press Ctrl+C to stop

## Command

```bash
cd /home/jones/dev/north-cloud && \
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f $SERVICE
```

## Example

```bash
# View crawler logs
SERVICE=crawler
```

## Options

**Show last N lines before tailing:**
```bash
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs --tail=100 -f $SERVICE
```

**View logs without tailing:**
```bash
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs --tail=50 $SERVICE
```

**View logs for multiple services:**
```bash
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f crawler source-manager
```

## Log Format

Development mode logs include:
- Timestamp
- Service name
- Log level (DEBUG, INFO, WARN, ERROR)
- Message
- Structured fields (JSON in production)

## Debugging Tips

- Look for ERROR level logs for failures
- WARN logs indicate potential issues
- DEBUG logs show detailed operation traces
- Check health endpoint if service not logging

## Related Commands

- Use `check-health.md` to verify service status
- Use `restart-service.md` if service is failing
