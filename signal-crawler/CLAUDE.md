# CLAUDE.md

## Overview

Standalone signal crawler service that scans public sources (Hacker News, government funding portals) for lead signals and POSTs them to NorthOps ingest endpoints.

## Architecture

Single binary with pluggable source adapters. Each adapter implements `adapter.Source` interface.

```
internal/
├── adapter/         Source interface + Signal type
│   ├── hn/          HN Firebase API adapter
│   └── funding/     Grant portal HTML scraper
├── config/          YAML + env config loading
├── dedup/           SQLite deduplication store
├── ingest/          HTTP POST client for NorthOps
├── runner/          Orchestrator: scan → dedup → ingest
└── scoring/         Keyword → signal strength mapping
```

## Development

```bash
task build          # Build binary
task run            # Run all adapters
task run:dry        # Dry run (no POSTing)
task test           # Run tests
task lint           # golangci-lint
```

## Environment Variables

| Variable | Purpose | Required |
|----------|---------|----------|
| `NORTHOPS_URL` | NorthOps base URL | Yes |
| `PIPELINE_API_KEY` | API key for ingest endpoints | Yes |
| `SIGNAL_DB_PATH` | SQLite dedup DB path (default: data/seen.db) | No |
| `LOG_LEVEL` | Log level (default: info) | No |
