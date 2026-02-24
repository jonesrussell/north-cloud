# Air removal — baseline and measurement

This doc describes the protocol for measuring resource usage before and after removing Air from the north-cloud dev setup. We use a single service (**auth**) with no dependencies so the full stack is not required.

## Protocol

### 1. Ensure 0 running

```bash
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml down
```

### 2. Baseline (with Air)

Only run this **before** the Air-removal changes (on a branch that still has Air):

```bash
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d auth
```

Wait for health (e.g. `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml ps` shows auth healthy, or `curl -s http://localhost:8040/health`).

Capture stats:

```bash
docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}"
```

Record the output below (or in a one-off file). Then:

```bash
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml down
```

### 3. After removing Air

Rebuild and run:

```bash
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml build auth
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d auth
```

Same wait and same `docker stats --no-stream` capture. Compare with baseline.

### 4. Optional: JSON output

For scripted comparison:

```bash
docker stats --no-stream --format "{{json .}}"
```

## Results (fill in after runs)

| Label              | CPU % | Mem usage | Mem % | Notes |
|--------------------|-------|-----------|-------|--------|
| baseline-with-air  |       |           |       |        |
| after-no-air       |       |           |       |        |

Delta: ___

## Checklist

- [ ] Branch created
- [ ] Baseline: 0 → up auth (with Air) → capture `docker stats` → down
- [ ] Air removed from compose, Dockerfiles, and root Taskfile (retained in service Taskfiles for local `task dev`)
- [ ] Rebuild auth image, up auth (no Air), capture `docker stats` again
- [ ] Compare and record delta above
