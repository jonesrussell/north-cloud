---
description: Capture CPU or memory profile for debugging
variables:
  - name: SERVICE
    description: Service name (crawler, source-manager, classifier, publisher-api, publisher-router, auth, search)
    default: crawler
  - name: PROFILE_TYPE
    description: Profile type (cpu, heap, goroutine, allocs, block, mutex)
    default: heap
  - name: DURATION
    description: Duration in seconds for CPU profiling (ignored for heap/goroutine)
    default: "30"
---

# Profile Service

Captures performance profiles using Go's pprof for debugging and optimization.

## Usage

This command will:
1. Connect to the service's pprof endpoint
2. Capture the specified profile type
3. Save to `profiles/` directory with timestamp
4. Display profile location for analysis

## Profile Types

- `cpu` - CPU usage during sampling period
- `heap` - Memory allocations on heap
- `goroutine` - All goroutines and their stack traces
- `allocs` - Past memory allocations
- `block` - Blocking operations
- `mutex` - Mutex contention

## Command

```bash
cd /home/jones/dev/north-cloud && ./scripts/profile.sh $SERVICE $PROFILE_TYPE $DURATION
```

## Examples

```bash
# Capture heap profile (memory usage)
SERVICE=crawler
PROFILE_TYPE=heap
DURATION=30

# Capture 60-second CPU profile
SERVICE=publisher-api
PROFILE_TYPE=cpu
DURATION=60

# Capture goroutine dump
SERVICE=classifier
PROFILE_TYPE=goroutine
DURATION=30
```

## Analyzing Profiles

After capturing, analyze with pprof:
```bash
go tool pprof -http=:8080 profiles/service_type_timestamp.pb.gz
```

## Service pprof Ports

- Crawler: 6060
- Source Manager: 6061
- Classifier: 6062
- Publisher API: 6063
- Publisher Router: 6064
- Auth: 6065
- Search: 6066

## When to Use

- Investigating memory leaks
- Identifying CPU hotspots
- Debugging goroutine leaks
- Optimizing performance
- Understanding resource usage
