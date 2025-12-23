# GoCrawl

A web crawler and search engine built with Go. It crawls websites, extracts content, and stores it in Elasticsearch for efficient searching.

## Features

- Web crawling with configurable rules
- Content extraction and processing
- Elasticsearch storage and search
- REST API for searching content
- Job scheduling for automated crawling

## Prerequisites

- Go 1.25 or later
- Node.js 18+ and npm (for frontend dashboard)
- Elasticsearch 8.x
- Docker (optional, recommended for development)

## Quick Start

1. Clone the repository:
```bash
git clone https://github.com/jonesrussell/north-cloud/crawler.git
cd gocrawl
```

2. Install dependencies:
```bash
go mod download
```

3. Create configuration file:
```bash
cp config.example.yaml config.yaml
# Edit config.yaml with your settings (server port, Elasticsearch URL, etc.)
```

4. Build and run:
```bash
go build -o bin/gocrawl
./bin/gocrawl
```

The service starts an HTTP server that provides:
- REST API for job management (`/api/v1/jobs`)
- Search API for querying content
- Automatic job scheduling for crawling

## Configuration

Configuration can be provided via:
- Environment variables (prefixed with `APP_`, `LOG_`, `ELASTICSEARCH_`, etc.)
- Config file (`config.yaml` in current directory or `./config/`)
- `.env` file in the current directory

See `config.example.yaml` for available configuration options.

## Development

Run tests:
```bash
go test ./...
```

Run linter:
```bash
task lint
```

## License

MIT
