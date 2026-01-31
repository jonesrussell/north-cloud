# WSL2: Checking Service Logs and Avoiding Crashes

## Current run: no logs available

After `docker compose down`, container logs are removed with the containers. There is no way to retrieve the logs from the run that may have preceded a WSL2 crash. To investigate next time:

1. **Check logs before shutting down** – Use the commands below while containers are still running.
2. **Use observability (optional)** – If you start with `task docker:dev:up:observability`, logs are also sent to Loki and can be queried in Grafana after a crash (if Loki/Grafana were running and their data is on disk).

---

## Services by memory (likely WSL2 OOM order)

Configured limits from `docker-compose.base.yml` + `docker-compose.dev.yml`:

| Service              | Memory limit | Notes                          |
|----------------------|-------------:|--------------------------------|
| **elasticsearch**    | **1G** + 256MB shm | Java heap; often biggest consumer. |
| **postgres-\*** (×5) | **512M** each | ~2.5G total for all Postgres.  |
| **crawler**          | 512M         | Go; depends on ES, MinIO, Redis. |
| **classifier**       | 512M         | Go; depends on ES.            |
| **publisher**        | 512M         | Go.                            |
| **Loki** (observability) | 512M   | Only if observability profile. |
| **Grafana** (observability) | 512M | Only if observability profile. |
| **source-manager**   | 256M         | Go.                            |
| **index-manager**    | 256M         | Go.                            |
| **search-service**   | 256M         | Go.                            |
| **redis**            | 256M         |                                |
| **minio**            | 256M         |                                |
| **Alloy** (observability) | 256M  |                                |
| **dashboard**        | 256M         | Node/Vue.                      |
| **auth**             | 128M         |                                |
| **search-frontend**  | 128M         |                                |

Rough total for **core stack** (no observability): ~5–6G configured limits, plus Docker daemon and WSL2 overhead. WSL2 often has a default 8G cap; hitting it can cause a crash.

---

## Checking logs one service at a time (without bringing up everything)

Do **not** run `task docker:dev:up` (full stack). Use per-service targets so only that service and its dependencies start.

### 1. View logs for a single service

After starting only that service (see below):

```bash
# Last 100 lines, no follow
task docker:dev:logs:crawler -- --tail=100

# Follow live
task docker:dev:logs:crawler
```

Same pattern for other services:

- `task docker:dev:logs:source-manager -- --tail=100`
- `task docker:dev:logs:publisher -- --tail=100`
- `task docker:dev:logs:classifier -- --tail=100`
- `task docker:dev:logs:index-manager -- --tail=100`
- `task docker:dev:logs:auth -- --tail=100`
- `task docker:dev:logs:dashboard -- --tail=100`
- `task docker:dev:logs:nginx -- --tail=100`

### 2. Bring up one app service at a time

Each of these starts **only** that service and its dependencies (Postgres, Elasticsearch, Redis, MinIO, etc. as needed):

```bash
# Start only crawler + its dependencies (postgres-crawler, elasticsearch, minio, redis, source-manager)
task docker:dev:up:crawler

# Then check logs
task docker:dev:logs:crawler -- --tail=200
# When done, stop only this slice (optional):
# docker compose -f docker-compose.base.yml -f docker-compose.dev.yml stop crawler postgres-crawler source-manager postgres-source-manager elasticsearch redis minio minio-init
```

Repeat for another service:

```bash
task docker:dev:up:classifier
task docker:dev:logs:classifier -- --tail=200
```

And so on for `source-manager`, `publisher`, `index-manager`, `auth`, `dashboard`, `nginx`.

### 3. Infra-only (to test Elasticsearch in isolation)

To see only infra and Elasticsearch (no app services):

```bash
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d \
  postgres-crawler postgres-source-manager postgres-classifier postgres-publisher postgres-index-manager \
  elasticsearch redis minio
# Wait for health, then check ES
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs --tail=100 elasticsearch
```

Then bring up one app at a time and watch `docker stats` or task logs to see which service correlates with high memory or a crash.

---

## Suggested order to isolate the culprit

1. **Infra only** – Postgres ×1, Redis, MinIO (no Elasticsearch). Check stability and logs.
2. **Add Elasticsearch** – Often the largest consumer; watch memory and ES logs.
3. **One app** – e.g. `task docker:dev:up:auth` (light), then `task docker:dev:up:source-manager`, then crawler/classifier/publisher.
4. After each step, run `task docker:dev:logs:<service> -- --tail=100` and optionally `docker stats --no-stream` to see memory.

---

## WSL2 memory limit (optional)

If you need the full stack on WSL2, increase the memory cap and restart WSL:

1. Create or edit `%UserProfile%\.wslconfig` (Windows):
   ```ini
   [wsl2]
   memory=12GB
   swap=4GB
   ```
2. In PowerShell (Admin): `wsl --shutdown`, then reopen your distro.

---

## Summary

- **Past crash:** No logs from the previous run; use this guide for the next run.
- **Heaviest suspects:** Elasticsearch (1G + shm), then the five Postgres (512M each), then crawler/classifier/publisher (512M each).
- **Safe way to check logs:** Bring up one service with `task docker:dev:up:<service>`, then `task docker:dev:logs:<service> -- --tail=100`; repeat for the next service without starting the full project at once.
