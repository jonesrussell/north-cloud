# MCP North Cloud Server

An MCP (Model Context Protocol) server that provides comprehensive tools for managing the North Cloud content platform. This server exposes 23 tools across all North Cloud services for crawling, source management, content classification, publishing, search operations, and development tasks.

## Overview

This MCP server acts as a unified interface to the entire North Cloud microservices platform, allowing you to:
- **Crawl websites** and manage crawl jobs
- **Manage content sources** and test extraction selectors
- **Classify content** for quality, topics, and crime detection
- **Publish articles** to Redis channels with intelligent routing
- **Search content** across all classified articles
- **Manage Elasticsearch indexes** for raw and classified content

## Features

### Crawler Tools (7 tools)
- `start_crawl` - Start an immediate one-time crawl job
- `schedule_crawl` - Schedule recurring crawls with interval-based scheduling
- `list_crawl_jobs` - List all crawl jobs with status filtering
- `pause_crawl_job` - Pause a running or scheduled job
- `resume_crawl_job` - Resume a paused job
- `cancel_crawl_job` - Cancel a job
- `get_crawl_stats` - Get job statistics and execution history

### Source Manager Tools (5 tools)
- `add_source` - Add a new content source with CSS selectors
- `list_sources` - List all configured sources
- `update_source` - Update source configuration
- `delete_source` - Delete a source
- `test_source` - Test crawl a source without saving (validate selectors)

### Publisher Tools (6 tools)
- `create_route` - Create a publishing route with quality/topic filters
- `list_routes` - List all publishing routes
- `delete_route` - Delete a publishing route
- `preview_route` - Preview articles matching route filters
- `get_publish_history` - Get publishing history with pagination
- `get_publisher_stats` - Get publisher statistics

### Search Tools (1 tool)
- `search_articles` - Full-text search with filtering and facets

### Classifier Tools (1 tool)
- `classify_article` - Classify content for type, quality, topics

### Index Manager Tools (2 tools)
- `delete_index` - Delete an Elasticsearch index
- `list_indexes` - List all Elasticsearch indexes

### Development Tools (1 tool)
- `lint_file` - Lint a specific file or entire service (automatically detects Go vs frontend)

## Architecture

The server implements the MCP protocol using:
- **stdio-based communication**: Reads from stdin, writes to stdout
- **JSON-RPC 2.0**: Standard MCP protocol format
- **HTTP clients**: Communicates with all North Cloud services

```
┌─────────────────────────────────────────────────────────┐
│                  MCP North Cloud Server                  │
├─────────────────────────────────────────────────────────┤
│                                                           │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────┐ │
│  │ Crawler  │  │  Source  │  │Publisher │  │ Search  │ │
│  │  Client  │  │ Manager  │  │  Client  │  │ Client  │ │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬────┘ │
│       │             │             │             │       │
│  ┌────┴─────┐  ┌────┴─────┐       │             │       │
│  │Classifier│  │  Index   │       │             │       │
│  │  Client  │  │ Manager  │       │             │       │
│  │          │  │  Client  │       │             │       │
│  └────┬─────┘  └────┬─────┘       │             │       │
│       │             │             │             │       │
│       ▼             ▼             ▼             ▼       │
│  ┌────────────────────────────────────────────────────┐ │
│  │           North Cloud Services (Docker)            │ │
│  │  crawler:8060 | source-manager:8050 |              │ │
│  │  publisher:8080 | search:8090 | classifier:8070    │ │
│  │  index-manager:8090                                  │ │
│  └────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

## Installation

### Running with Docker

```bash
# Start the service (included in docker-compose)
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d mcp-north-cloud

# View logs
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f mcp-north-cloud
```

### Building Locally

```bash
cd mcp-north-cloud
go build -o mcp-north-cloud main.go
```

## Configuration

### Cursor IDE Integration

The project includes a `.cursor/mcp.json` file for Cursor IDE integration:

```json
{
  "mcpServers": {
    "north-cloud": {
      "command": "docker",
      "args": [
        "exec",
        "-i",
        "north-cloud-mcp-north-cloud-1",
        "/app/tmp/mcp-north-cloud"
      ],
      "env": {
        "INDEX_MANAGER_URL": "http://index-manager:8090",
        "CRAWLER_URL": "http://crawler:8060",
        "SOURCE_MANAGER_URL": "http://source-manager:8050",
        "PUBLISHER_URL": "http://publisher-api:8080",
        "SEARCH_URL": "http://search:8090",
        "CLASSIFIER_URL": "http://classifier:8070"
      }
    }
  }
}
```

After modifying the configuration, **restart Cursor** to apply changes.

### Claude Code Hooks Integration

When using Claude Code hooks, MCP tools follow a specific naming pattern. Since the server is configured as `"north-cloud"` in the MCP configuration, all tools are accessible using the pattern:

**Pattern**: `mcp__<server>__<tool>`

**Examples**:
- `mcp__north-cloud__start_crawl` - Start an immediate crawl job
- `mcp__north-cloud__schedule_crawl` - Schedule a recurring crawl
- `mcp__north-cloud__list_crawl_jobs` - List all crawl jobs
- `mcp__north-cloud__add_source` - Add a new content source
- `mcp__north-cloud__create_route` - Create a publishing route
- `mcp__north-cloud__search_articles` - Search classified content
- `mcp__north-cloud__classify_article` - Classify an article
- `mcp__north-cloud__list_indexes` - List Elasticsearch indexes
- `mcp__north-cloud__delete_index` - Delete an Elasticsearch index

**All 23 tools** are available using this naming convention. You can reference them in Claude Code hooks to automate North Cloud operations.

**Hook Example**:
```yaml
# Example hook that uses MCP tools
on:
  - event: file_changed
    pattern: "crawler/**/*.go"
actions:
  - use: mcp__north-cloud__list_crawl_jobs
    args:
      status: "running"
```

**Complete Tool List for Claude Code Hooks**:

All 23 tools available with `mcp__north-cloud__` prefix:

**Crawler Tools (7)**:
- `mcp__north-cloud__start_crawl`
- `mcp__north-cloud__schedule_crawl`
- `mcp__north-cloud__list_crawl_jobs`
- `mcp__north-cloud__pause_crawl_job`
- `mcp__north-cloud__resume_crawl_job`
- `mcp__north-cloud__cancel_crawl_job`
- `mcp__north-cloud__get_crawl_stats`

**Source Manager Tools (5)**:
- `mcp__north-cloud__add_source`
- `mcp__north-cloud__list_sources`
- `mcp__north-cloud__update_source`
- `mcp__north-cloud__delete_source`
- `mcp__north-cloud__test_source`

**Publisher Tools (6)**:
- `mcp__north-cloud__create_route`
- `mcp__north-cloud__list_routes`
- `mcp__north-cloud__delete_route`
- `mcp__north-cloud__preview_route`
- `mcp__north-cloud__get_publish_history`
- `mcp__north-cloud__get_publisher_stats`

**Search Tools (1)**:
- `mcp__north-cloud__search_articles`

**Classifier Tools (1)**:
- `mcp__north-cloud__classify_article`

**Index Manager Tools (2)**:
- `mcp__north-cloud__list_indexes`
- `mcp__north-cloud__delete_index`

**Development Tools (1)**:
- `mcp__north-cloud__lint_file`

### Environment Variables

All service URLs can be configured via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `INDEX_MANAGER_URL` | `http://localhost:8090` | Index manager service URL |
| `CRAWLER_URL` | `http://localhost:8060` | Crawler service URL |
| `SOURCE_MANAGER_URL` | `http://localhost:8050` | Source manager service URL |
| `PUBLISHER_URL` | `http://localhost:8080` | Publisher service URL |
| `SEARCH_URL` | `http://localhost:8090` | Search service URL |
| `CLASSIFIER_URL` | `http://localhost:8070` | Classifier service URL |

## Tool Reference

### Crawler Tools

#### start_crawl

Start a crawl job immediately. Creates a job that runs once without scheduling.

**Parameters:**
- `source_id` (string, required): ID of the source to crawl (from source-manager)
- `url` (string, required): URL to crawl

**Example:**
```json
{
  "source_id": "uuid-12345",
  "url": "https://example.com/news"
}
```

**Response:**
```json
{
  "job_id": "job-uuid",
  "source_id": "uuid-12345",
  "url": "https://example.com/news",
  "status": "pending",
  "created_at": "2026-01-06T10:00:00Z",
  "message": "Crawl job created successfully. Job will run immediately."
}
```

#### schedule_crawl

Schedule a recurring crawl job with interval-based scheduling.

**Parameters:**
- `source_id` (string, required): ID of the source to crawl
- `url` (string, required): URL to crawl
- `interval_minutes` (integer, required): Interval in minutes/hours/days
- `interval_type` (string, required): Type of interval ('minutes', 'hours', 'days')

**Example:**
```json
{
  "source_id": "uuid-12345",
  "url": "https://example.com/news",
  "interval_minutes": 30,
  "interval_type": "minutes"
}
```

**Response:**
```json
{
  "job_id": "job-uuid",
  "status": "scheduled",
  "interval_minutes": 30,
  "interval_type": "minutes",
  "next_run_at": "2026-01-06T10:30:00Z",
  "message": "Scheduled crawl job created. Runs every 30 minutes."
}
```

#### list_crawl_jobs

List all crawl jobs with optional status filter.

**Parameters:**
- `status` (string, optional): Filter by status (pending, scheduled, running, completed, failed, paused, cancelled)

**Example:**
```json
{
  "status": "running"
}
```

**Response:**
```json
{
  "jobs": [
    {
      "id": "job-uuid",
      "source_id": "uuid-12345",
      "url": "https://example.com",
      "status": "running",
      "created_at": "2026-01-06T10:00:00Z"
    }
  ],
  "count": 1
}
```

#### pause_crawl_job, resume_crawl_job, cancel_crawl_job

Control job execution state.

**Parameters:**
- `job_id` (string, required): ID of the job to control

**Example:**
```json
{
  "job_id": "job-uuid"
}
```

#### get_crawl_stats

Get statistics for a crawl job including success rate and execution history.

**Parameters:**
- `job_id` (string, required): ID of the job

**Response:**
```json
{
  "total_executions": 42,
  "success_count": 40,
  "failure_count": 2,
  "avg_duration": 12.5,
  "success_rate": 0.95
}
```

### Source Manager Tools

#### add_source

Add a new content source for crawling.

**Parameters:**
- `name` (string, required): Name of the source
- `url` (string, required): Base URL
- `type` (string, required): Source type (e.g., 'news', 'blog')
- `selectors` (object, required): CSS selectors for content extraction
- `active` (boolean, optional): Whether source is active (default: true)

**Example:**
```json
{
  "name": "Example News",
  "url": "https://example.com",
  "type": "news",
  "selectors": {
    "article": ".article-content",
    "title": "h1.title",
    "body": ".article-body"
  },
  "active": true
}
```

**Response:**
```json
{
  "source_id": "uuid-12345",
  "name": "Example News",
  "url": "https://example.com",
  "type": "news",
  "active": true,
  "created_at": "2026-01-06T10:00:00Z",
  "message": "Source created successfully"
}
```

#### list_sources

List all configured content sources.

**Parameters:** None

**Response:**
```json
{
  "sources": [
    {
      "id": "uuid-12345",
      "name": "Example News",
      "url": "https://example.com",
      "type": "news",
      "active": true
    }
  ],
  "count": 1
}
```

#### update_source

Update an existing source configuration.

**Parameters:**
- `source_id` (string, required): ID of source to update
- `name` (string, optional): New name
- `url` (string, optional): New URL
- `selectors` (object, optional): New selectors
- `active` (boolean, optional): New active status

#### delete_source

Delete a content source.

**Parameters:**
- `source_id` (string, required): ID of source to delete

#### test_source

Test crawl a source without saving results. Useful for validating selectors before adding a source.

**Parameters:**
- `url` (string, required): URL to test
- `selectors` (object, required): CSS selectors to test

**Response:**
```json
{
  "success": true,
  "article_count": 15,
  "success_rate": 0.93,
  "warnings": [],
  "articles": [
    {
      "title": "Example Article",
      "url": "https://example.com/article-1"
    }
  ]
}
```

### Publisher Tools

#### create_route

Create a new publishing route that connects a source to a channel with quality and topic filters.

**Parameters:**
- `source_id` (string, required): ID of publisher source
- `channel_id` (string, required): ID of channel to publish to
- `min_quality_score` (integer, required): Minimum quality score (0-100)
- `topics` (array, optional): Topics to filter by
- `active` (boolean, optional): Whether route is active

**Example:**
```json
{
  "source_id": "source-uuid",
  "channel_id": "channel-uuid",
  "min_quality_score": 70,
  "topics": ["crime", "news"],
  "active": true
}
```

#### list_routes

List all publishing routes with optional filters.

**Parameters:**
- `source_id` (string, optional): Filter by source
- `channel_id` (string, optional): Filter by channel

#### preview_route

Preview articles that would be published by a route without actually publishing them.

**Parameters:**
- `route_id` (string, required): ID of route to preview

**Response:**
```json
{
  "articles": [
    {
      "id": "article-uuid",
      "title": "Crime Report",
      "quality_score": 85,
      "topics": ["crime"],
      "published_at": "2026-01-06T09:00:00Z"
    }
  ],
  "count": 1
}
```

#### get_publish_history

Get publishing history with pagination.

**Parameters:**
- `channel_name` (string, optional): Filter by channel
- `limit` (integer, optional): Number of records (default: 50)
- `offset` (integer, optional): Skip records (default: 0)

#### get_publisher_stats

Get publisher statistics including total published and articles by channel.

**Response:**
```json
{
  "total_published": 1250,
  "articles_by_channel": {
    "articles:crime": 450,
    "articles:news": 800
  }
}
```

### Search Tools

#### search_articles

Full-text search across all classified content with filtering and facets.

**Parameters:**
- `query` (string, required): Search query
- `topics` (array, optional): Filter by topics
- `content_type` (string, optional): Filter by content type
- `min_quality_score` (integer, optional): Minimum quality
- `page` (integer, optional): Page number (default: 1)
- `page_size` (integer, optional): Results per page (default: 20, max: 100)

**Example:**
```json
{
  "query": "crime downtown",
  "topics": ["crime"],
  "min_quality_score": 70,
  "page": 1,
  "page_size": 20
}
```

**Response:**
```json
{
  "results": [
    {
      "id": "article-uuid",
      "title": "Crime Report Downtown",
      "body": "...",
      "quality_score": 85,
      "topics": ["crime"],
      "score": 12.5
    }
  ],
  "total": 42,
  "page": 1,
  "page_size": 20,
  "took_ms": 15
}
```

### Classifier Tools

#### classify_article

Classify a single article to determine content type, quality score, topics, and crime detection.

**Parameters:**
- `title` (string, required): Article title
- `raw_text` (string, required): Article text content
- `url` (string, required): Article URL
- `metadata` (object, optional): Additional metadata

**Example:**
```json
{
  "title": "Breaking: Crime Downtown",
  "raw_text": "A crime was reported...",
  "url": "https://example.com/article",
  "metadata": {
    "author": "John Doe"
  }
}
```

**Response:**
```json
{
  "content_type": "article",
  "quality_score": 85,
  "is_crime_related": true,
  "topics": ["crime", "breaking_news"],
  "source_reputation": 0.92,
  "source_category": "news",
  "confidence": 0.95
}
```

### Index Manager Tools

#### delete_index

Delete an Elasticsearch index. **This operation is irreversible.**

**Parameters:**
- `index_name` (string, required): Name of index to delete

**Example:**
```json
{
  "index_name": "example_com_raw_content"
}
```

#### list_indexes

List all Elasticsearch indexes.

**Response:**
```json
{
  "indexes": [
    "example_com_raw_content",
    "example_com_classified_content"
  ],
  "count": 2
}
```

### Development Tools

#### lint_file

Lint a specific file or entire service. Automatically detects Go files vs Vue.js/TypeScript frontend files and runs the appropriate linter.

**Parameters:**
- `file_path` (string, optional): Absolute or relative path to the file to lint
- `service_name` (string, optional): Service name to lint entire service (Go: crawler, source-manager, classifier, publisher, index-manager, search, auth, mcp-north-cloud | Frontend: dashboard, search-frontend)

**Note:** Either `file_path` or `service_name` must be provided.

**Example (lint a file):**
```json
{
  "file_path": "crawler/main.go"
}
```

**Example (lint entire service):**
```json
{
  "service_name": "publisher"
}
```

**Response:**
```json
{
  "lint_type": "go",
  "service_dir": "/home/jones/dev/north-cloud/publisher",
  "command": "task lint",
  "output": "Running golangci-lint...\n✅ No issues found",
  "success": true
}
```

**Error Response:**
```json
{
  "lint_type": "go",
  "service_dir": "/home/jones/dev/north-cloud/crawler",
  "command": "task lint",
  "output": "internal/scheduler/interval_scheduler.go:45:10: Error: unused variable 'x'",
  "success": false,
  "error": "exit status 1",
  "exit_code": 1
}
```

## Development

### Prerequisites

- Go 1.25+
- Docker and Docker Compose
- Access to North Cloud services

### Building

```bash
go build -o mcp-north-cloud main.go
```

### Running Tests

```bash
go test ./...
```

### Hot Reloading

The service uses Air for hot reloading in development:

```bash
air -c .air.toml
```

## Troubleshooting

### Server not responding

1. Check that all North Cloud services are running:
   ```bash
   docker compose ps
   ```

2. Verify service URLs are correct:
   ```bash
   curl http://localhost:8060/health  # crawler
   curl http://localhost:8050/health  # source-manager
   curl http://localhost:8080/health  # publisher
   ```

3. Check MCP server logs:
   ```bash
   docker logs north-cloud-mcp-north-cloud-1
   ```

### Tool execution fails

1. Verify the service is accessible:
   ```bash
   curl http://localhost:<PORT>/health
   ```

2. Check service-specific logs:
   ```bash
   docker logs north-cloud-<service-name>-1
   ```

3. Ensure required parameters are provided correctly

### Cursor not detecting MCP server

1. Verify `.cursor/mcp.json` exists in project root
2. Check container is running:
   ```bash
   docker ps | grep mcp-north-cloud
   ```
3. **Restart Cursor** after modifying configuration
4. Check Cursor's MCP server status in settings

## Common Workflows

### Add a new source and start crawling

```bash
# 1. Add source
use add_source with selectors

# 2. Test source first
use test_source to validate selectors

# 3. Start immediate crawl
use start_crawl with source_id

# 4. Or schedule recurring crawl
use schedule_crawl with interval
```

### Set up content publishing

```bash
# 1. List available sources and channels
use list_sources
use list_routes to see channels

# 2. Create publishing route
use create_route with filters

# 3. Preview what would be published
use preview_route

# 4. Monitor publishing
use get_publish_history
use get_publisher_stats
```

### Search and classify content

```bash
# 1. Search for articles
use search_articles with query

# 2. Classify new content
use classify_article with article data

# 3. Check crawler job stats
use get_crawl_stats for job performance
```

## Error Handling

The server returns standard JSON-RPC error responses:

| Code | Error | Description |
|------|-------|-------------|
| -32700 | Parse error | Invalid JSON |
| -32600 | Invalid request | Invalid request format |
| -32601 | Method not found | Unknown method |
| -32602 | Invalid params | Invalid parameters |
| -32603 | Internal error | Service error |

## Security Considerations

- The server does not perform authentication - ensure it's only accessible to trusted clients
- Some operations are irreversible (e.g., delete_index, delete_source)
- Consider adding authentication/authorization for production use
- Service URLs should point to internal Docker network addresses

## Protocol Support

- **Protocol Version**: 2024-11-05
- **Transport**: stdio (stdin/stdout)
- **Format**: JSON-RPC 2.0

## License

Part of the North Cloud project.
