# Consumer Integration Guide

This guide explains how to build a service that consumes articles from the Publisher Redis pub/sub channels.

## Table of Contents

1. [Overview](#overview)
2. [Crime-only consumers (e.g. StreetCode)](#crime-only-consumers-eg-streetcode)
3. [Prerequisites](#prerequisites)
4. [Quick Start](#quick-start)
5. [Architecture Patterns](#architecture-patterns)
6. [Implementation Examples](#implementation-examples)
7. [Best Practices](#best-practices)
8. [Production Deployment](#production-deployment)

## Overview

The Publisher service publishes classified articles to Redis pub/sub channels based on topic (e.g., `articles:crime`, `articles:news`). Your consumer service subscribes to one or more channels and processes the articles according to your business logic.

### Consumer Responsibilities

As a consumer, you are responsible for:

- ✅ **Subscribing to Redis channels** - Connect and listen for messages
- ✅ **Filtering articles** - Apply your own criteria (keywords, geography, etc.)
- ✅ **Deduplication** - Track which articles you've already processed
- ✅ **Data transformation** - Map article fields to your database schema
- ✅ **Error handling** - Handle network failures, malformed messages, etc.
- ✅ **Storage** - Save articles to your database or CMS

### Crime-only consumers (e.g. StreetCode)

If your site should show **only crime-related content**, subscribe to **both** Layer 1 crime topic channels **and** Layer 3/4 classification channels:

- **Layer 1** (bulk content): `articles:crime`, `articles:violent_crime`, `articles:criminal_justice`, `articles:drug_crime`, `articles:property_crime`, `articles:organized_crime`
- **Layer 3** (classification): `crime:homepage`, `crime:category:violent-crime`, `crime:category:property-crime`, `crime:category:drug-crime`, `crime:category:organized-crime`, `crime:category:court-news`, `crime:category:crime`
- **Layer 4** (location): `crime:canada`, `crime:province:{code}`, `crime:local:{city}`

Layer 1 channels carry the majority of crime articles. Layer 3/4 carry a smaller subset with richer classification metadata (homepage eligibility, category pages, location). Subscribe to all layers for complete coverage. Consumer-side deduplication (by article `id`) prevents duplicates across layers.

Do **not** subscribe to non-crime topic channels like `articles:news` or `articles:politics` (those carry mixed content).

### Mining-only consumers (e.g. OreWire)

Subscribe to the **Layer 5 mining channel**: `articles:mining`. Message payload includes `mining.relevance`, `mining.mining_stage`, `mining.commodities`, `mining.location`, and `mining.final_confidence` for downstream filtering.

### Entertainment consumers

Subscribe to **Layer 6 channels**: `entertainment:homepage`, `entertainment:category:*`, `entertainment:peripheral`. Message payload includes `entertainment_relevance`, `entertainment_categories`, and nested `entertainment` object.

### Publisher Responsibilities

The publisher handles:

- ✅ Quality score filtering (`quality_score >= threshold`)
- ✅ Topic classification (`topics IN [crime, news, ...]`)
- ✅ Per-channel deduplication (won't publish same article twice to same channel)
- ✅ Elasticsearch querying and article retrieval

## Prerequisites

### Required

- **Redis client library** for your language
- **Database** for storing articles and tracking processed article IDs
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
pubsub.subscribe('articles:crime')

for message in pubsub.listen():
    if message['type'] == 'message':
        article = json.loads(message['data'])
        print(f"Received: {article['title']}")
```

**Node.js**:
```javascript
const redis = require('redis');
const client = redis.createClient({ host: 'localhost', port: 6379 });

client.subscribe('articles:crime');

client.on('message', (channel, message) => {
  const article = JSON.parse(message);
  console.log(`Received: ${article.title}`);
});
```

### 3. Process Articles

See [Implementation Examples](#implementation-examples) below for complete examples.

## Architecture Patterns

### Pattern 1: Direct Processing (Simple)

```
Redis → Consumer → Database
```

**Best for**: Low volume (<100 articles/hour), simple processing

```python
for message in pubsub.listen():
    article = json.loads(message['data'])
    if not already_processed(article['id']):
        save_to_database(article)
```

### Pattern 2: Queue-Based Processing (Recommended)

```
Redis → Consumer → Queue → Worker(s) → Database
```

**Best for**: Medium to high volume, complex processing, scalability

```python
# Consumer: Add to queue
for message in pubsub.listen():
    article = json.loads(message['data'])
    queue.enqueue('process_article', article)

# Worker: Process from queue
def process_article(article):
    if not already_processed(article['id']):
        save_to_database(article)
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
conn = sqlite3.connect('articles.db')
conn.execute('''
    CREATE TABLE IF NOT EXISTS articles (
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

def article_exists(article_id):
    """Check if article already processed."""
    cursor = conn.execute('SELECT 1 FROM articles WHERE id = ?', (article_id,))
    return cursor.fetchone() is not None

def save_article(article):
    """Save article to database."""
    try:
        conn.execute('''
            INSERT INTO articles (id, title, body, url, published_date, quality_score, topics, processed_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?)
        ''', (
            article['id'],
            article['title'],
            article['body'],
            article['canonical_url'],
            article['published_date'],
            article['quality_score'],
            json.dumps(article.get('topics', [])),
            datetime.utcnow().isoformat()
        ))
        conn.commit()
        logger.info(f"Saved article: {article['id']}")
    except Exception as e:
        logger.error(f"Failed to save article: {e}")

def process_message(message):
    """Process Redis message."""
    try:
        article = json.loads(message['data'])

        # Deduplication
        if article_exists(article['id']):
            logger.debug(f"Skipping duplicate: {article['id']}")
            return

        # Additional filtering (example: minimum quality score)
        if article.get('quality_score', 0) < 70:
            logger.debug(f"Skipping low quality: {article['id']}")
            return

        # Save article
        save_article(article)

    except json.JSONDecodeError as e:
        logger.error(f"Invalid JSON: {e}")
    except Exception as e:
        logger.error(f"Error processing message: {e}")

def main():
    """Main consumer loop."""
    r = redis.Redis(host='localhost', port=6379, decode_responses=True)
    pubsub = r.pubsub()
    pubsub.subscribe('articles:crime')

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
  database: 'articles',
  user: 'postgres',
  password: 'password'
});

// Create table
pool.query(`
  CREATE TABLE IF NOT EXISTS articles (
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

// Check if article exists
async function articleExists(articleId) {
  const result = await pool.query(
    'SELECT 1 FROM articles WHERE id = $1',
    [articleId]
  );
  return result.rows.length > 0;
}

// Save article
async function saveArticle(article) {
  try {
    await pool.query(
      `INSERT INTO articles (id, title, body, url, published_date, quality_score, topics)
       VALUES ($1, $2, $3, $4, $5, $6, $7)`,
      [
        article.id,
        article.title,
        article.body,
        article.canonical_url,
        article.published_date,
        article.quality_score,
        JSON.stringify(article.topics || [])
      ]
    );
    console.log(`Saved article: ${article.id}`);
  } catch (error) {
    console.error(`Failed to save article: ${error.message}`);
  }
}

// Process message
async function processMessage(channel, message) {
  try {
    const article = JSON.parse(message);

    // Deduplication
    if (await articleExists(article.id)) {
      console.log(`Skipping duplicate: ${article.id}`);
      return;
    }

    // Additional filtering
    if (article.quality_score < 70) {
      console.log(`Skipping low quality: ${article.id}`);
      return;
    }

    // Save article
    await saveArticle(article);

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
  subscriber.subscribe('articles:crime');

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

class ConsumeArticles extends Command
{
    protected $signature = 'articles:consume {--channel=articles:crime}';
    protected $description = 'Subscribe to Redis pub/sub and consume articles';

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
            $article = json_decode($message, true);

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
            if ($this->articleExists($article['id'])) {
                $this->skippedCount++;
                if ($this->getOutput()->isVerbose()) {
                    $this->line("Skipping duplicate: {$article['id']}");
                }
                return;
            }

            // Additional filtering
            if (isset($article['quality_score']) && $article['quality_score'] < 70) {
                $this->skippedCount++;
                if ($this->getOutput()->isVerbose()) {
                    $this->line("Skipping low quality: {$article['id']} (score: {$article['quality_score']})");
                }
                return;
            }

            // Save article
            $this->saveArticle($article);
            $this->processedCount++;

            if ($this->getOutput()->isVerbose()) {
                $this->info("Processed: {$article['title']}");
            }

        } catch (\Exception $e) {
            $this->errorCount++;
            $this->error("Error processing message: {$e->getMessage()}");
            Log::error('Article processing error', [
                'error' => $e->getMessage(),
                'trace' => $e->getTraceAsString(),
            ]);
        }
    }

    protected function articleExists(string $articleId): bool
    {
        return DB::table('articles')
            ->where('external_id', $articleId)
            ->exists();
    }

    protected function saveArticle(array $article): void
    {
        DB::table('articles')->insert([
            'external_id' => $article['id'],
            'title' => $article['title'],
            'body' => $article['body'] ?? $article['raw_text'] ?? null,
            'canonical_url' => $article['canonical_url'],
            'source' => $article['source'],
            'published_date' => $article['published_date'],
            'quality_score' => $article['quality_score'] ?? null,
            'topics' => json_encode($article['topics'] ?? []),
            'is_crime_related' => $article['is_crime_related'] ?? false,
            'content_type' => $article['content_type'] ?? null,
            'publisher_route_id' => $article['publisher']['route_id'] ?? null,
            'publisher_channel' => $article['publisher']['channel'] ?? null,
            'publisher_published_at' => $article['publisher']['published_at'] ?? now(),
            'created_at' => now(),
            'updated_at' => now(),
        ]);

        Log::info('Article saved', [
            'article_id' => $article['id'],
            'title' => $article['title'],
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
// database/migrations/xxxx_create_articles_table.php
Schema::create('articles', function (Blueprint $table) {
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
php artisan articles:consume

# Custom channel
php artisan articles:consume --channel=articles:news

# Verbose output
php artisan articles:consume -v

# Run as daemon (use supervisor or systemd)
php artisan articles:consume > /dev/null 2>&1 &
```

**Using Laravel Queue (Recommended for Production):**

```php
// In your Artisan command or Service Provider
Redis::subscribe(['articles:crime'], function ($message) {
    ProcessArticleJob::dispatch(json_decode($message, true));
});

// In app/Jobs/ProcessArticleJob.php
class ProcessArticleJob implements ShouldQueue
{
    use Dispatchable, InteractsWithQueue, Queueable, SerializesModels;

    public function __construct(public array $article) {}

    public function handle(): void
    {
        // Process article with retry logic
        // Laravel queue handles retries automatically
    }
}
```

## Best Practices

### 1. Deduplication

**Always implement deduplication** to prevent processing the same article multiple times.

```python
# Store processed IDs in database
CREATE TABLE processed_articles (
    article_id TEXT PRIMARY KEY,
    processed_at TIMESTAMP DEFAULT NOW()
);

# Or use Redis SET for fast lookups
redis.sadd('processed_articles', article_id)
if redis.sismember('processed_articles', article_id):
    return  # Skip
```

### 2. Error Handling

**Implement retry logic** with exponential backoff:

```python
from tenacity import retry, stop_after_attempt, wait_exponential

@retry(stop=stop_after_attempt(3), wait=wait_exponential(multiplier=1, min=2, max=10))
def save_article(article):
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

### 4. Monitoring

**Track metrics**:
- Messages received per second
- Messages processed successfully
- Messages skipped (duplicates, low quality)
- Processing errors
- Database latency

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

logger.info("article_processed",
    article_id=article['id'],
    quality_score=article['quality_score'],
    topics=article['topics'],
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
    container_name: article-consumer
    environment:
      - REDIS_HOST=redis
      - REDIS_PORT=6379
      - REDIS_PASSWORD=${REDIS_PASSWORD}
      - DATABASE_URL=postgresql://user:pass@db:5432/articles
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
  name: article-consumer
spec:
  replicas: 3
  selector:
    matchLabels:
      app: article-consumer
  template:
    metadata:
      labels:
        app: article-consumer
    spec:
      containers:
      - name: consumer
        image: your-registry/article-consumer:latest
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

## Troubleshooting

### Problem: No messages received

**Solution**:
1. Check Redis connectivity: `redis-cli PING`
2. Verify channel name: `redis-cli PUBSUB CHANNELS`
3. Check publisher logs: `docker logs north-cloud-publisher-router`
4. Ensure routes are enabled: `curl http://localhost:8070/api/v1/routes`

### Problem: Duplicate articles

**Solution**:
1. Implement deduplication (see Best Practices)
2. Check processed_articles table
3. Verify article ID uniqueness

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
