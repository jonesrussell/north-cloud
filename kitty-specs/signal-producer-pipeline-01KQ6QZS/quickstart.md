# Quickstart: Signal Producer Pipeline

**Mission**: `signal-producer-pipeline-01KQ6QZS`
**Audience**: maintainer, on-call, or future agent operating the producer.

## What it does, in one paragraph

The signal-producer is a one-shot Go binary scheduled by systemd every 15 minutes. Each run queries `*_classified_content` ES indexes for documents since the last successful checkpoint (minus a 5-minute lookback buffer), maps each hit to the Waaseyaa signal format, and POSTs them in batches of 50 to `https://northops.ca/api/signals`. On success, the checkpoint advances. On failure (after 3 retries on transient errors), the run exits non-zero and the checkpoint stays put for the next fire.

## Build locally

```bash
cd signal-producer
task build               # produces ./signal-producer binary
task test                # unit + fast integration tests
task test:cover          # coverage report (target: ≥ 80% per package)
task lint                # golangci-lint
```

From repo root:

```bash
task build:signal-producer
task test:signal-producer
task lint:signal-producer
```

## Run locally against a fixture ES + mock Waaseyaa

```bash
# Start ES via existing dev compose (if not already running)
task docker:dev:up:search

# Seed two fixture documents
go run ./signal-producer/internal/testutil/seed -url http://localhost:9200

# Start a mock Waaseyaa (writes received batches to stdout)
go run ./signal-producer/internal/testutil/mock-waaseyaa -port 8080 &

# Run one cycle of the producer
WAASEYAA_URL=http://localhost:8080 \
WAASEYAA_API_KEY=local-dev-key \
ES_URL=http://localhost:9200 \
./signal-producer/signal-producer
```

A fresh run with no checkpoint defaults to the last 24 hours; the seed script
creates fixtures within that window.

## Reading journald in production

```bash
# Tail the most recent runs
sudo journalctl -u signal-producer -n 100 --no-pager

# Last 24 hours, only summary lines (one per run)
sudo journalctl -u signal-producer --since "24 hours ago" | grep '"event":"run_summary"'

# Failed runs (non-zero exit)
sudo journalctl -u signal-producer.service -p err --since "7 days ago"

# Source-down alerts
sudo journalctl -u signal-producer | grep '"code":"signal_producer.source_down"'
```

Every run emits at minimum:
- One INFO `event=run_start` line with checkpoint timestamp.
- One INFO `event=es_query` line with `hits` count.
- One INFO `event=batch_post` line per batch (`batch_size`, `ingested`, `skipped`, `duration_ms`).
- One INFO `event=run_summary` line at the end (`total_signals`, `errors`, `duration_ms`).
- One WARN `event=source_down` line (only when 3 consecutive empty runs detected).

## Triaging a failed run

1. Find the failing unit:
   ```bash
   sudo systemctl status signal-producer.service
   ```
2. Look at the most recent journald output:
   ```bash
   sudo journalctl -u signal-producer -n 50 --no-pager
   ```
3. Common patterns:
   - `error="config: WAASEYAA_API_KEY missing"` → `EnvironmentFile=` not loaded; check `/etc/signal-producer/env` permissions and contents.
   - HTTP 401 from Waaseyaa → API key invalid; rotate via 1Password and update env file.
   - HTTP 5xx after all retries → Waaseyaa is down; the next timer fire will retry.
   - `error="checkpoint: write failed: permission denied"` → `/var/lib/signal-producer/` ownership wrong; fix with `chown signal-producer:signal-producer /var/lib/signal-producer/`.
   - Source-down WARN repeating for hours → upstream classifier or crawler isn't producing rfp/need_signal hits; investigate the content pipeline, not the producer.

## Source-down alert recipe

Three consecutive runs with `ingested == 0` emits a single WARN with code `signal_producer.source_down`. To wire it to your alerter of choice:

```bash
# Example: post to Slack via journald watcher
journalctl -fu signal-producer | grep --line-buffered 'signal_producer.source_down' | \
  while read -r line; do curl -X POST -H 'Content-Type: application/json' \
    -d "{\"text\":\"signal-producer source-down alert: $line\"}" \
    "$SLACK_WEBHOOK_URL"; done
```

The producer does not ship an integration; the alert lives in your operator pipeline. See `docs/RUNBOOK.md` for the full triage walkthrough.

## Force a checkpoint rewind (re-send recent signals)

If you need to re-send the last hour of signals (e.g., Waaseyaa lost data and asked for a replay):

```bash
sudo systemctl stop signal-producer.timer
sudo -u signal-producer python3 -c "
import json, datetime
path = '/var/lib/signal-producer/checkpoint.json'
data = json.load(open(path))
new_ts = (datetime.datetime.fromisoformat(data['last_successful_run'].rstrip('Z')) - datetime.timedelta(hours=1)).strftime('%Y-%m-%dT%H:%M:%SZ')
data['last_successful_run'] = new_ts
json.dump(data, open(path, 'w'))
"
sudo systemctl start signal-producer.timer
```

The next fire reprocesses the rewind window. Waaseyaa-side dedup via
`external_id` prevents duplicate leads.

## Deploy

Pushed via the standard CI pipeline:

```bash
git push origin main      # CI runs deploy.sh on the prod VPS
```

For a manual deploy:

```bash
gh workflow run deploy.yml -f services=signal-producer
```

After deploy, verify:

```bash
ssh prod
sudo systemctl status signal-producer.timer
sudo systemctl list-timers | grep signal-producer
sudo journalctl -u signal-producer --since "5 minutes ago"
```

## When to rerun the mission

A follow-up mission is warranted if:
- Waaseyaa changes the `/api/signals` contract (header name, response shape).
- A second content type beyond `rfp`/`need_signal` enters scope.
- The 15-minute cadence proves too slow or too fast based on lead conversion data.
- The single-VPS posture becomes a reliability bottleneck (consider HA or SQS-style queue).

None of those are speculative scope for this mission. Land the producer, run it in prod, learn from real data, then decide.
