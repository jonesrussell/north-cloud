# Consumer Integration Guide

This guide explains how to build a service that consumes content from the Publisher Redis pub/sub channels.

## Table of Contents

1. [Overview](#overview)
2. [Crime-only consumers (e.g. StreetCode)](#crime-only-consumers-eg-streetcode)
3. [Mining-only consumers (e.g. OreWire)](#mining-only-consumers-eg-orewire)
4. [Entertainment consumers (e.g. movies-of-war)](#entertainment-consumers-eg-movies-of-war)
5. [Indigenous consumers (e.g. Diidjaaheer)](#indigenous-consumers-eg-diidjaaheer)
6. [Coforge consumers](#coforge-consumers)
7. [Prerequisites](#prerequisites)
8. [Quick Start](#quick-start)
9. [Architecture Patterns](#architecture-patterns)
10. [Implementation Examples](#implementation-examples)
11. [Best Practices](#best-practices)
12. [Production Deployment](#production-deployment)
13. [Verifying content flow](#verifying-content-flow)
14. [Troubleshooting](#troubleshooting)

## Overview

The Publisher service publishes classified content to Redis pub/sub channels based on topic (e.g., `content:crime`, `content:news`). Your consumer service subscribes to one or more channels and processes the content according to your business logic.

### Consumer Responsibilities

As a consumer, you are responsible for:

- ✅ **Subscribing to Redis channels** - Connect and listen for messages
- ✅ **Filtering content** - Apply your own criteria (keywords, geography, etc.)
- ✅ **Deduplication** - Track which content you've already processed
- ✅ **Data transformation** - Map content fields to your database schema
- ✅ **Error handling** - Handle network failures, malformed messages, etc.
- ✅ **Storage** - Save content to your database or CMS

### Crime-only consumers (e.g. StreetCode)

If your site should show **only crime-related content**, subscribe to **both** Layer 1 crime topic channels **and** Layer 3/4 classification channels:

- **Layer 1** (bulk content): `content:crime`, `content:violent_crime`, `content:criminal_justice`, `content:drug_crime`, `content:property_crime`, `content:organized_crime`
- **Layer 3** (classification): `crime:homepage`, `crime:category:violent-crime`, `crime:category:property-crime`, `crime:category:drug-crime`, `crime:category:organized-crime`, `crime:category:court-news`, `crime:category:crime`
- **Layer 4** (location): `crime:canada`, `crime:province:{code}`, `crime:local:{city}`

Layer 1 channels carry the majority of crime content. Layer 3/4 carry a smaller subset with richer classification metadata (homepage eligibility, category pages, location). Subscribe to all layers for complete coverage. Consumer-side deduplication (by content `id`) prevents duplicates across layers.

Do **not** subscribe to non-crime topic channels like `content:news` or `content:politics` (those carry mixed content).

### Mining-only consumers (e.g. OreWire)

Subscribe to **Layer 5 mining channels** for complete coverage:

- **Catch-all**: `content:mining` (all core + peripheral mining content)
- **Relevance**: `mining:core`, `mining:peripheral`
- **Commodity**: `mining:commodity:gold`, `mining:commodity:copper`, `mining:commodity:lithium`, `mining:commodity:nickel`, `mining:commodity:uranium`, `mining:commodity:iron-ore`, `mining:commodity:rare-earths`
- **Stage**: `mining:stage:exploration`, `mining:stage:development`, `mining:stage:production`
- **Location**: `mining:canada`, `mining:international`

`content:mining` carries all mining content. The sub-channels carry overlapping subsets with richer routing metadata. Subscribe to all channels for granular page routing; consumer-side deduplication (by content `id`) prevents duplicates across channels.

Message payload includes `mining.relevance`, `mining.mining_stage`, `mining.commodities`, `mining.location`, and `mining.final_confidence` for additional downstream filtering.

### Entertainment consumers (e.g. movies-of-war)

Subscribe to **Layer 6 channels** for complete coverage:

- **Homepage / relevance**: `entertainment:homepage`, `entertainment:peripheral`
- **Category** (one per classification category; slugs are lowercased, spaces to hyphens): `entertainment:category:film`, `entertainment:category:music`, `entertainment:category:gaming`, `entertainment:category:reviews`, and any other categories produced by the entertainment classifier.

Message payload includes `entertainment_relevance`, `entertainment_categories`, and nested `entertainment` object.

**Note:** The publisher does **not** emit an `content:war` (or `content:entertainment`) channel from any automatic layer. To receive entertainment content, subscribe to the Layer 6 channels above. If you want a single aggregate channel (e.g. `content:war`), create a Layer 2 channel in the publisher DB via the API and configure its rules accordingly.

### Indigenous consumers (e.g. Diidjaaheer)

Subscribe to **Layer 7 channels** for complete coverage:

- **Catch-all**: `content:indigenous` (all core + peripheral Indigenous-classified content above the routing threshold)
- **Category** (one per classification category): `indigenous:category:culture`, `indigenous:category:language`, `indigenous:category:land-rights`, `indigenous:category:education`
- **Region** (when present): `indigenous:region:canada`, `indigenous:region:usa`, etc.

Subscribe to all of the above for full coverage; consumer-side deduplication (by content `id`) prevents duplicates across channels. Do **not** subscribe to `content:default` — the publisher does not emit that channel from any automatic layer.

### Coforge consumers

Subscribe to **Layer 8 channels**. The publisher does **not** emit a catch-all `content:coforge` or `content:default` channel. For Coforge-classified content, subscribe to:

- **Relevance**: `coforge:core`, `coforge:peripheral`
- **Audience** (when set on the content item): `coforge:audience:{slug}` (e.g. `coforge:audience:developers`)
- **Topic** (one per topic): `coforge:topic:{slug}` (e.g. `coforge:topic:digital-transformation`, `coforge:topic:cloud`)
- **Industry** (one per industry): `coforge:industry:{slug}` (e.g. `coforge:industry:banking`, `coforge:industry:insurance`)

Slugs are lowercased with underscores and spaces converted to hyphens. Subscribe at minimum to `coforge:core` and `coforge:peripheral`; add specific audience/topic/industry channels as needed. New slugs (new audiences, topics, or industries from the classifier) require adding those channel names to your consumer config or env.

### Publisher Responsibilities

The publisher handles:

- ✅ Quality score filtering (`quality_score >= threshold`)
- ✅ Topic classification (`topics IN [crime, news, ...]`)
- ✅ Per-channel deduplication (won't publish same content item twice to same channel)
- ✅ Elasticsearch querying and content retrieval

## Prerequisites

### Required

- **Redis client library** for your language
- **Database** for storing content and tracking processed content IDs
- **Network access** to Redis server

### Recommended

- **Queue system** for asynchronous processing (optional but recommended)
- **Monitoring/logging** infrastructure
- **Docker** for containerized deployment

### Redis Connection Details

```bash
# Development
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=  # Optional

# Production
REDIS_HOST=redis  # Docker service name
REDIS_PORT=6379
REDIS_PASSWORD=your-secure-password
```

## Quick Start

### 1. Install Redis Client

**Python**:
```bash
pip install redis
```

**Node.js**:
```bash
npm install redis
```

**PHP/Laravel**:
```bash
# Laravel 12 uses Redis facade (included by default)
# No additional package needed for basic pub/sub

# If using Predis directly (standalone PHP):
composer require predis/predis
```

**Go**:
```bash
go get github.com/redis/go-redis/v9
```

### 2. Subscribe to Channel

**Python**:
```python
import redis
import json

r = redis.Redis(host='localhost', port=6379, decode_responses=True)
pubsub = r.pubsub()
pubsub.subscribe('content:crime')

for message in pubsub.listen():
    if message['type'] == 'message':
        item = json.loads(message['data'])
        print(f"Received: {item['title']}")
```

**Node.js**:
```javascript
const redis = require('redis');
const client = redis.createClient({ host: 'localhost', port: 6379 });

client.subscribe('content:crime');

client.on('message', (channel, message) => {
  const item = JSON.parse(message);
  console.log(`Received: ${item.title}`);
});
```

### 3. Process Content

See [Implementation Examples](#implementation-examples) below for complete examples.

## Architecture Patterns

### Pattern 1: Direct Processing (Simple)

```
Redis → Consumer → Database
```

**Best for**: Low volume (<100 items/hour), simple processing

```python
for message in pubsub.listen():
    item = json.loads(message['data'])
    if not already_processed(item['id']):
        save_to_database(item)
```

### Pattern 2: Queue-Based Processing (Recommended)

```
Redis → Consumer → Queue → Worker(s) → Database
```

**Best for**: Medium to high volume, complex processing, scalability

```python
# Consumer: Add to queue
for message in pubsub.listen():
    item = json.loads(message['data'])
    queue.enqueue('process_item', item)

# Worker: Process from queue
def process_item(item):
    if not already_processed(item['id']):
        save_to_database(item)
```

### Pattern 3: Multi-Consumer (High Availability)

```
        ┌→ Consumer 1 → Queue 1 → Worker Pool 1 → Database
Redis → ├→ Consumer 2 → Queue 2 → Worker Pool 2 → Database
        └→ Consumer 3 → Queue 3 → Worker Pool 3 → Database
```

**Best for**: High availability, load distribution

**Note**: All consumers receive all messages (pub/sub behavior). Use queue-based deduplication.

## Implementation Examples

### Example 1: Python with SQLite

```python
import redis
import json
import sqlite3
import logging
from datetime import datetime

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Database setup
conn = sqlite3.connect('content.db')
conn.execute('''
    CREATE TABLE IF NOT EXISTS content_items (
        id TEXT PRIMARY KEY,
        title TEXT NOT NULL,
        body TEXT,
        url TEXT,
        published_date TEXT,
        quality_score INTEGER,
        topics TEXT,
        processed_at TEXT
    )
''')
conn.commit()

def item_exists(content_id):
    """Check if content item already processed."""
    cursor = conn.execute('SELECT 1 FROM content_items WHERE id = ?', (content_id,))
    return cursor.fetchone() is not None

def save_item(item):
    """Save content item to database."""
    try:
        conn.execute('''
            INSERT INTO content_items (id, title, body, url, published_date, quality_score, topics, processed_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?)
        ''', (
            item['id'],
            item['title'],
            item['body'],
            item['canonical_url'],
            item['published_date'],
            item['quality_score'],
            json.dumps(item.get('topics', [])),
            datetime.utcnow().isoformat()
        ))
        conn.commit()
        logger.info(f"Saved item: {item['id']}")
    except Exception as e:
        logger.error(f"Failed to save item: {e}")

def process_message(message):
    """Process Redis message."""
    try:
        item = json.loads(message['data'])

        # Deduplication
        if item_exists(item['id']):
            logger.debug(f"Skipping duplicate: {item['id']}")
            return

        # Additional filtering (example: minimum quality score)
        if item.get('quality_score', 0) < 70:
            logger.debug(f"Skipping low quality: {item['id']}")
            return

        # Save item
        save_item(item)

    except json.JSONDecodeError as e:
        logger.error(f"Invalid JSON: {e}")
    except Exception as e:
        logger.error(f"Error processing message: {e}")

def main():
    """Main consumer loop."""
    r = redis.Redis(host='localhost', port=6379, decode_responses=True)
    pubsub = r.pubsub()
    pubsub.subscribe('content:crime')

    logger.info("Consumer started. Listening for messages...")

    try:
        for message in pubsub.listen():
            if message['type'] == 'message':
                process_message(message)
    except KeyboardInterrupt:
        logger.info("Shutting down...")
        pubsub.unsubscribe()
        conn.close()

if __name__ == '__main__':
    main()
```

### Example 2: Node.js with PostgreSQL

```javascript
const redis = require('redis');
const { Pool } = require('pg');

// PostgreSQL setup
const pool = new Pool({
  host: 'localhost',
  port: 5432,
  database: 'content',
  user: 'postgres',
  password: 'password'
});

// Create table
pool.query(`
  CREATE TABLE IF NOT EXISTS content_items (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    body TEXT,
    url TEXT,
    published_date TIMESTAMP,
    quality_score INTEGER,
    topics JSONB,
    processed_at TIMESTAMP DEFAULT NOW()
  )
`);

// Check if item exists
async function itemExists(contentId) {
  const result = await pool.query(
    'SELECT 1 FROM content_items WHERE id = $1',
    [contentId]
  );
  return result.rows.length > 0;
}

// Save item
async function saveItem(item) {
  try {
    await pool.query(
      `INSERT INTO content_items (id, title, body, url, published_date, quality_score, topics)
       VALUES ($1, $2, $3, $4, $5, $6, $7)`,
      [
        item.id,
        item.title,
        item.body,
        item.canonical_url,
        item.published_date,
        item.quality_score,
        JSON.stringify(item.topics || [])
      ]
    );
    console.log(`Saved item: ${item.id}`);
  } catch (error) {
    console.error(`Failed to save item: ${error.message}`);
  }
}

// Process message
async function processMessage(channel, message) {
  try {
    const item = JSON.parse(message);

    // Deduplication
    if (await itemExists(item.id)) {
      console.log(`Skipping duplicate: ${item.id}`);
      return;
    }

    // Additional filtering
    if (item.quality_score < 70) {
      console.log(`Skipping low quality: ${item.id}`);
      return;
    }

    // Save item
    await saveItem(item);

  } catch (error) {
    console.error(`Error processing message: ${error.message}`);
  }
}

// Main
async function main() {
  const subscriber = redis.createClient({
    host: 'localhost',
    port: 6379
  });

  subscriber.on('message', processMessage);
  subscriber.subscribe('content:crime');

  console.log('Consumer started. Listening for messages...');

  process.on('SIGINT', () => {
    subscriber.unsubscribe();
    subscriber.quit();
    pool.end();
    process.exit(0);
  });
}

main().catch(console.error);
```

### Example 3: Laravel 12 Integration

```php
<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use Illuminate\Support\Facades\Redis;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Log;

class ConsumeContent extends Command
{
    protected $signature = 'content:consume {--channel=content:crime}';
    protected $description = 'Subscribe to Redis pub/sub and consume content';

    protected $processedCount = 0;
    protected $skippedCount = 0;
    protected $errorCount = 0;

    public function handle()
    {
        $channel = $this->option('channel');
        $this->info("Subscribing to channel: {$channel}");

        Redis::subscribe([$channel], function ($message) {
            $this->processMessage($message);
        });
    }

    protected function processMessage(string $message): void
    {
        try {
            $item = json_decode($message, true);

            if (json_last_error() !== JSON_ERROR_NONE) {
                $this->error('Invalid JSON: ' . json_last_error_msg());
                $this->errorCount++;
                Log::error('Invalid JSON message', [
                    'error' => json_last_error_msg(),
                    'message_preview' => substr($message, 0, 100),
                ]);
                return;
            }

            // Deduplication
            if ($this->itemExists($item['id'])) {
                $this->skippedCount++;
                if ($this->getOutput()->isVerbose()) {
                    $this->line("Skipping duplicate: {$item['id']}");
                }
                return;
            }

            // Additional filtering
            if (isset($item['quality_score']) && $item['quality_score'] < 70) {
                $this->skippedCount++;
                if ($this->getOutput()->isVerbose()) {
                    $this->line("Skipping low quality: {$item['id']} (score: {$item['quality_score']})");
                }
                return;
            }

            // Save item
            $this->saveItem($item);
            $this->processedCount++;

            if ($this->getOutput()->isVerbose()) {
                $this->info("Processed: {$item['title']}");
            }

        } catch (\Exception $e) {
            $this->errorCount++;
            $this->error("Error processing message: {$e->getMessage()}");
            Log::error('Content processing error', [
                'error' => $e->getMessage(),
                'trace' => $e->getTraceAsString(),
            ]);
        }
    }

    protected function itemExists(string $contentId): bool
    {
        return DB::table('content_items')
            ->where('external_id', $contentId)
            ->exists();
    }

    protected function saveItem(array $item): void
    {
        DB::table('content_items')->insert([
            'external_id' => $item['id'],
            'title' => $item['title'],
            'body' => $item['body'] ?? $item['raw_text'] ?? null,
            'canonical_url' => $item['canonical_url'],
            'source' => $item['source'],
            'published_date' => $item['published_date'],
            'quality_score' => $item['quality_score'] ?? null,
            'topics' => json_encode($item['topics'] ?? []),
            'is_crime_related' => $item['is_crime_related'] ?? false,
            'content_type' => $item['content_type'] ?? null,
            'publisher_route_id' => $item['publisher']['route_id'] ?? null,
            'publisher_channel' => $item['publisher']['channel'] ?? null,
            'publisher_published_at' => $item['publisher']['published_at'] ?? now(),
            'created_at' => now(),
            'updated_at' => now(),
        ]);

        Log::info('Content saved', [
            'content_id' => $item['id'],
            'title' => $item['title'],
        ]);
    }

    public function __destruct()
    {
        $this->info("\nSummary:");
        $this->info("Processed: {$this->processedCount}");
        $this->info("Skipped: {$this->skippedCount}");
        $this->info("Errors: {$this->errorCount}");
    }
}
```

**Database Migration:**

```php
// database/migrations/xxxx_create_content_items_table.php
Schema::create('content_items', function (Blueprint $table) {
    $table->id();
    $table->string('external_id')->unique();
    $table->string('title');
    $table->text('body')->nullable();
    $table->string('canonical_url');
    $table->string('source');
    $table->timestamp('published_date');
    $table->integer('quality_score')->nullable();
    $table->json('topics')->nullable();
    $table->boolean('is_crime_related')->default(false);
    $table->string('content_type')->nullable();
    $table->uuid('publisher_route_id')->nullable();
    $table->string('publisher_channel')->nullable();
    $table->timestamp('publisher_published_at')->nullable();
    $table->timestamps();

    $table->index('external_id');
    $table->index('published_date');
    $table->index('quality_score');
});
```

**Running the consumer:**

```bash
# Basic usage
php artisan content:consume

# Custom channel
php artisan content:consume --channel=content:news

# Verbose output
php artisan content:consume -v

# Run as daemon (use supervisor or systemd)
php artisan content:consume > /dev/null 2>&1 &
```

**Using Laravel Queue (Recommended for Production):**

```php
// In your Artisan command or Service Provider
Redis::subscribe(['content:crime'], function ($message) {
    ProcessContentJob::dispatch(json_decode($message, true));
});

// In app/Jobs/ProcessContentJob.php
class ProcessContentJob implements ShouldQueue
{
    use Dispatchable, InteractsWithQueue, Queueable, SerializesModels;

    public function __construct(public array $item) {}

    public function handle(): void
    {
        // Process content with retry logic
        // Laravel queue handles retries automatically
    }
}
```

## Best Practices

### 1. Deduplication

**Always implement deduplication** to prevent processing the same content item multiple times.

```python
# Store processed IDs in database
CREATE TABLE processed_content (
    content_id TEXT PRIMARY KEY,
    processed_at TIMESTAMP DEFAULT NOW()
);

# Or use Redis SET for fast lookups
redis.sadd('processed_content', content_id)
if redis.sismember('processed_content', content_id):
    return  # Skip
```

### 2. Error Handling

**Implement retry logic** with exponential backoff:

```python
from tenacity import retry, stop_after_attempt, wait_exponential

@retry(stop=stop_after_attempt(3), wait=wait_exponential(multiplier=1, min=2, max=10))
def save_item(item):
    # Database operation
    pass
```

### 3. Graceful Shutdown

**Handle SIGTERM/SIGINT** to shutdown cleanly:

```python
import signal
import sys

def signal_handler(sig, frame):
    logger.info('Shutting down gracefully...')
    pubsub.unsubscribe()
    conn.close()
    sys.exit(0)

signal.signal(signal.SIGINT, signal_handler)
signal.signal(signal.SIGTERM, signal_handler)
```

### 4. Monitoring and health

**Track metrics**:
- Messages received per second
- Messages processed successfully
- Messages skipped (duplicates, low quality)
- Processing errors
- Database latency

**Laravel (northcloud-laravel):** Use `php artisan content:status` to verify Redis connection and channel list; use `php artisan content:stats --since=24h` for ingestion volume. Run the subscriber as a long-lived process (systemd or supervisor) and consider including a process or status check in health endpoints.

**Quality and processing:** When using northcloud-laravel, set `NORTHCLOUD_QUALITY_FILTER=true` and `NORTHCLOUD_MIN_QUALITY_SCORE` (e.g. 50 or 70) to skip low-quality content. Use `NORTHCLOUD_PROCESS_SYNC=true` for simplicity; set to `false` and run a queue worker for higher throughput.

```python
from prometheus_client import Counter, Histogram

messages_received = Counter('messages_received_total', 'Total messages received')
messages_processed = Counter('messages_processed_total', 'Total messages processed')
processing_time = Histogram('processing_seconds', 'Time to process message')

@processing_time.time()
def process_message(message):
    messages_received.inc()
    # ... process
    messages_processed.inc()
```

### 5. Logging

**Use structured logging**:

```python
import structlog

logger = structlog.get_logger()

logger.info("content_processed",
    content_id=item['id'],
    quality_score=item['quality_score'],
    topics=item['topics'],
    duration_ms=duration
)
```

## Production Deployment

### Docker Container

**Dockerfile**:
```dockerfile
FROM python:3.11-slim

WORKDIR /app

# Install dependencies
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy application
COPY consumer.py .

# Run as non-root user
RUN useradd -m consumer
USER consumer

CMD ["python", "consumer.py"]
```

**docker-compose.yml**:
```yaml
services:
  consumer:
    build: .
    container_name: content-consumer
    environment:
      - REDIS_HOST=redis
      - REDIS_PORT=6379
      - REDIS_PASSWORD=${REDIS_PASSWORD}
      - DATABASE_URL=postgresql://user:pass@db:5432/content
    depends_on:
      - redis
      - postgres
    restart: unless-stopped
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: content-consumer
spec:
  replicas: 3
  selector:
    matchLabels:
      app: content-consumer
  template:
    metadata:
      labels:
        app: content-consumer
    spec:
      containers:
      - name: consumer
        image: your-registry/content-consumer:latest
        env:
        - name: REDIS_HOST
          value: "redis-service"
        - name: REDIS_PASSWORD
          valueFrom:
            secretKeyRef:
              name: redis-secret
              key: password
        resources:
          limits:
            cpu: "1"
            memory: "512Mi"
          requests:
            cpu: "0.5"
            memory: "256Mi"
```

### Health Checks

```python
from flask import Flask
import threading

app = Flask(__name__)

@app.route('/health')
def health():
    # Check if consumer is running
    return {'status': 'healthy', 'last_message': last_message_time}

# Run Flask in separate thread
threading.Thread(target=lambda: app.run(port=8080), daemon=True).start()
```

### Scaling Considerations

- **Multiple instances**: All consumers receive all messages (pub/sub behavior)
- **Deduplication required**: Use database or shared Redis SET
- **Queue-based**: Use Celery, RabbitMQ, or similar for distribution
- **Resource limits**: Set CPU/memory limits in production

## Verifying content flow

To confirm content is being published and your consumer can receive them:

1. **Publisher**: Check that the router is running and polling (publisher logs). Call `GET /api/v1/stats/overview` (with JWT) to see total published and recent activity. Ensure at least one `*_classified_content` Elasticsearch index exists and the cursor is advancing.
2. **Redis**: From the same host as the publisher, run `redis-cli PING` and `redis-cli PUBSUB CHANNELS` to verify Redis is up and channels are being used.
3. **Laravel (northcloud-laravel)**: Run `php artisan content:status` to confirm Redis connection, channel list, and quality filter. Use `php artisan content:stats --since=24h` to see ingestion volume. Include `content:status` (or a check that the subscriber process is running) in health scripts or monitoring.

Run the subscriber as a long-lived process (systemd or supervisor) so it is always connected when the publisher sends messages; Redis pub/sub does not queue messages for offline consumers.

## Troubleshooting

### Problem: No messages received

**Solution**:
1. Check Redis connectivity: `redis-cli PING`
2. Verify channel name: `redis-cli PUBSUB CHANNELS`
3. Check publisher logs: `docker logs north-cloud-publisher-router`
4. Ensure routes are enabled: `curl http://localhost:8070/api/v1/routes`

### Problem: Duplicate content

**Solution**:
1. Implement deduplication (see Best Practices)
2. Check processed_content table
3. Verify content ID uniqueness

### Problem: High memory usage

**Solution**:
1. Process messages asynchronously (queue-based pattern)
2. Limit concurrent processing
3. Set resource limits in Docker/Kubernetes

### Problem: Consumer crashes

**Solution**:
1. Add error handling (try/catch around message processing)
2. Implement graceful shutdown
3. Use supervisor or systemd for auto-restart
4. Check logs for stack traces

## Next Steps

1. **Start simple**: Use Direct Processing pattern for testing
2. **Add monitoring**: Track metrics and errors
3. **Scale up**: Implement Queue-Based pattern for production
4. **High availability**: Deploy multiple consumer instances

## Support

For questions or issues:
- Check publisher logs: `docker logs north-cloud-publisher-router`
- Review publish history: `GET /api/v1/publish-history`
- Contact: See [REDIS_MESSAGE_FORMAT.md](./REDIS_MESSAGE_FORMAT.md) for message format details
