---
description: Run performance benchmarks with memory stats
variables:
  - name: SERVICE
    description: Service name (crawler, source-manager, classifier, publisher, auth, search)
    default: crawler
---

# Run Benchmarks

Runs Go benchmarks for a service with memory allocation statistics.

## Usage

This command will:
1. Navigate to the project root
2. Run benchmarks for the specified service
3. Display performance metrics and memory stats
4. Optionally save as baseline for comparison

## Command

```bash
cd /home/jones/dev/north-cloud && ./scripts/run-benchmarks.sh -s $SERVICE -m
```

## Example

```bash
# Run crawler benchmarks with memory stats
SERVICE=crawler
```

## Benchmark Output

- Operations per second
- Nanoseconds per operation
- Bytes allocated per operation
- Allocations per operation

## Creating Baselines

To save benchmark results as a baseline:
```bash
cd /home/jones/dev/north-cloud && \
./scripts/run-benchmarks.sh -s $SERVICE -m -b
```

## Comparing Against Baseline

```bash
cd /home/jones/dev/north-cloud && \
./scripts/run-benchmarks.sh -s $SERVICE -m -c baselines/baseline_TIMESTAMP.txt
```

## Available Benchmarks

- Crawler: 18 benchmarks (job processing, scheduler, indexing)
- Source Manager: 7 benchmarks (CRUD, validation, URL parsing)
- Classifier: 6 benchmarks (classification, quality scoring)
- Publisher: 6 benchmarks (filtering, formatting, JSON:API)
- Auth: 10 benchmarks (JWT operations, hashing)
- Search: 7 benchmarks (full-text search, faceting)

## Total: 54 benchmarks across 6 services
