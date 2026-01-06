---
description: Detect memory leaks in a service
variables:
  - name: SERVICE
    description: Service name (crawler, source-manager, classifier, publisher-api, publisher-router, auth, search)
    default: crawler
---

# Check Memory Leaks

Automated memory leak detection using heap profile comparison over time.

## Usage

This command will:
1. Take initial heap snapshot
2. Wait specified interval (default 10 minutes)
3. Take second heap snapshot
4. Compare snapshots and analyze growth
5. Report potential memory leaks

## Command

```bash
cd /home/jones/dev/north-cloud && ./scripts/check-memory-leaks.sh -s $SERVICE -i 600 -c 5
```

## Parameters

- `-s SERVICE` - Service to check (crawler, source-manager, etc.)
- `-i INTERVAL` - Seconds between checks (default: 600 = 10 min)
- `-c COUNT` - Number of checks to perform (default: 3)

## Example

```bash
# Check crawler for leaks (5 checks at 10-minute intervals)
SERVICE=crawler
```

## Extended Leak Detection

For thorough leak detection over longer period:
```bash
cd /home/jones/dev/north-cloud && \
./scripts/check-memory-leaks.sh -s $SERVICE -i 900 -c 10 -a
```

Options:
- `-i 900` - 15-minute intervals
- `-c 10` - 10 checks (2.5 hours total)
- `-a` - Enable alerting via logs

## What Gets Checked

- **Heap Growth**: Total heap memory increase
- **Goroutine Leaks**: Increasing goroutine count
- **Allocation Rate**: Memory allocation patterns
- **GC Effectiveness**: Garbage collection metrics

## Warning Thresholds

- Heap growth >50% over 10 minutes
- Goroutine count >2x increase
- Steady upward memory trend

## Output

- Growth percentage per interval
- Goroutine count trends
- Leak warnings if detected
- Recommendations for investigation

## When Leak Detected

1. Capture detailed heap profile:
   ```bash
   ./scripts/profile.sh $SERVICE heap
   ```

2. Analyze with pprof:
   ```bash
   go tool pprof -http=:8080 profiles/*heap*.pb.gz
   ```

3. Look for allocation hotspots in flame graph

## Related Commands

- Use `profile-service.md` for detailed profiling
- Use `run-benchmarks.md` to check for performance regressions
