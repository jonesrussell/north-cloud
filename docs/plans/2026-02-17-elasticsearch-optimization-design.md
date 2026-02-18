# Elasticsearch Single-Node Optimization Design

**Date**: 2026-02-17
**Status**: Approved

## Problem

Single ES 9.2.2 node on a 16GB/4vCPU droplet (razor-crest, tor1) is underperforming with 10+ indexes. Root cause: Docker constrains ES to 2GB RAM with only 1GB heap, leaving 14GB unused. No ILM, no backups, replicas wasting disk on a single node.

## Current State

- **Droplet**: 16GB RAM, 4 vCPUs, 320GB root disk, 50GB ES volume (9% used)
- **ES container**: 2GB memory limit, 1GB heap (`-Xms1g -Xmx1g`)
- **Other workloads**: ~2.5GB total (Node SSR workers x10 ~1.2GB, MariaDB ~150MB, Grafana ~230MB, Alloy ~300MB, Loki ~130MB, MinIO ~115MB, ML sidecar ~110MB, Docker daemon ~170MB)
- **Available headroom**: ~8.8GB
- **No ILM policies**, no snapshot backups, replicas=1 on classified_content (useless on single node)

## Design

### 1. Increase ES Resource Allocation

Increase container limits to use available headroom. No droplet resize needed.

| Setting | Current | New |
|---------|---------|-----|
| Container memory | 2G | 8G |
| Container CPU | 2 | 4 |
| ES heap (min/max) | 1g | 4g |
| SHM size | 512mb | 512mb (unchanged) |

The 8GB container gives ES 4GB heap + 4GB Lucene filesystem cache. Leaves ~8GB for OS + other services (need ~2.5GB + OS overhead).

**Files to modify**:
- `docker-compose.prod.yml`: ES deploy resources and `ES_JAVA_OPTS`
- Production `.env`: `ELASTICSEARCH_MIN_HEAP=4g`, `ELASTICSEARCH_MAX_HEAP=4g`
- `.env.example`: Update default comments to document recommended prod values

### 2. Index Lifecycle Management (ILM)

**raw_content indexes**: Delete documents older than 30 days. Raw content is transient — once classified, the source material is re-crawlable. Already use replicas=0.

**classified_content indexes**: Keep indefinitely. Force-merge old indexes to reduce Lucene segment count and free heap. Set replicas to 0 (replicas on a single node just waste disk, provide no redundancy).

**Implementation**: Create ILM policy via ES API, apply to index templates managed by index-manager service.

### 3. Shard & Index Tuning

- Set `number_of_replicas: 0` on all classified_content index templates (saves ~50% disk and heap for those indexes)
- Force-merge indexes no longer receiving writes (reduces segments, improves query performance)
- Increase `refresh_interval` on raw_content indexes from 1s (default) to 30s (raw content doesn't need near-real-time search, reduces indexing overhead)
- Add `indices.memory.index_buffer_size: 20%` to elasticsearch.yml

### 4. Snapshot Backups to DigitalOcean Spaces

Configure automated snapshots for data safety:

- Register S3-compatible snapshot repository pointing to DO Spaces bucket
- Create snapshot lifecycle policy: daily snapshots of `*_classified_content`, retain 7 days
- raw_content excluded from backup (transient, re-crawlable)

### Out of Scope

- Multi-node clustering (not needed at current scale)
- ES security/authentication (internal network only, behind nginx)
- Cross-cluster search
- Kubernetes migration
- Droplet resize
