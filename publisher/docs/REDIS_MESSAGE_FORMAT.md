# Redis Message Format - Publisher Service

This document describes the message format published to Redis pub/sub channels by the Publisher Router service.

## Overview

The Publisher Router queries Elasticsearch for classified articles, filters them based on route configurations, and publishes matching articles to Redis pub/sub channels. Each message is a complete JSON document containing the full article data plus publisher metadata.

## Channel Naming Convention

Channels follow a topic-based naming pattern:

```
articles:{topic}
```

**Examples**:
- `articles:crime` - Crime-related articles
- `articles:news` - General news articles
- `articles:local` - Local community news
- `articles:sports` - Sports articles

## Message Structure

Each message is a JSON object with two main sections:

1. **Publisher Metadata**: Information about when and how the article was published
2. **Article Data**: Complete article content and classification metadata from Elasticsearch

### Full Message Example

```json
{
  "publisher": {
    "route_id": "a1b2c3d4-e5f6-4789-a0b1-c2d3e4f5g6h7",
    "published_at": "2025-12-28T15:30:45Z",
    "channel": "articles:crime"
  },
  "id": "es-doc-id-12345",
  "title": "Local Police Investigate Break-In at Community Center",
  "body": "Full article text content here...",
  "raw_text": "Full article text content here...",
  "raw_html": "<html>Original HTML content...</html>",
  "canonical_url": "https://example.com/articles/police-investigate-break-in",
  "source": "https://example.com/original-article-url",
  "published_date": "2025-12-28T08:00:00Z",

  "quality_score": 85,
  "topics": ["crime", "local"],
  "content_type": "article",
  "is_crime_related": true,
  "source_reputation": 78,
  "confidence": 0.92,

  "og_title": "Breaking News: Police Investigate Break-In",
  "og_description": "Community center targeted in overnight incident",
  "og_image": "https://example.com/images/article-image.jpg",
  "og_url": "https://example.com/articles/police-investigate-break-in",

  "intro": "Article introduction or lead paragraph...",
  "description": "Meta description of the article",
  "word_count": 450,
  "category": "news",
  "section": "local",
  "keywords": ["police", "investigation", "community"]
}
```

## Field Definitions

### Publisher Metadata

| Field | Type | Description |
|-------|------|-------------|
| `publisher.route_id` | UUID | The route ID that triggered this publication |
| `publisher.published_at` | ISO 8601 DateTime | When the publisher sent this message |
| `publisher.channel` | String | The Redis channel name |

### Core Article Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | String | Yes | Elasticsearch document ID (unique) |
| `title` | String | Yes | Article headline/title |
| `body` | String | Yes | Article text content (alias for raw_text) |
| `raw_text` | String | Yes | Full article text without HTML |
| `raw_html` | String | No | Original HTML content |
| `canonical_url` | String | Yes | Canonical URL for the article |
| `source` | String | Yes | Original source URL |
| `published_date` | ISO 8601 DateTime | Yes | When the article was originally published |

### Classification Metadata

| Field | Type | Range | Description |
|-------|------|-------|-------------|
| `quality_score` | Integer | 0-100 | Article quality rating |
| `topics` | Array[String] | - | Classified topics (e.g., ["crime", "local"]) |
| `content_type` | String | - | Content type (article, page, video, etc.) |
| `is_crime_related` | Boolean | - | Whether the article is crime-related |
| `source_reputation` | Integer | 0-100 | Source reliability score |
| `confidence` | Float | 0.0-1.0 | Classifier confidence level |

### Open Graph Metadata

| Field | Type | Description |
|-------|------|-------------|
| `og_title` | String | Open Graph title |
| `og_description` | String | Open Graph description |
| `og_image` | String | Open Graph image URL |
| `og_url` | String | Open Graph URL |

### Additional Fields

| Field | Type | Description |
|-------|------|-------------|
| `intro` | String | Article introduction/lead |
| `description` | String | Meta description |
| `word_count` | Integer | Number of words in article |
| `category` | String | Article category |
| `section` | String | Site section |
| `keywords` | Array[String] | Article keywords |

## Field Aliases

For backward compatibility and convenience, the following aliases exist:

- **`body`**: Alias for `raw_text` (article text content)
- **`source`**: Alias for the original article URL

Both fields are included in every message, so consumers can use whichever field name they prefer.

## Filtering at Consumer Side

The publisher applies the following filters before publishing:

1. **Quality Score**: `quality_score >= route.min_quality_score`
2. **Topics**: Article topics match route configuration (if specified)
3. **Deduplication**: Article not already published to this channel

**Consumers may apply additional filters** such as:
- Specific keywords or phrases
- Geographic filtering (city, region)
- Date range restrictions
- Custom quality thresholds
- Source blacklisting/whitelisting

## Deduplication Strategy

### Publisher-Side Deduplication

The publisher prevents duplicate publications using the `publish_history` table:

```sql
SELECT EXISTS(
  SELECT 1 FROM publish_history
  WHERE article_id = $1 AND channel_name = $2
)
```

This ensures each article is published **once per channel**.

### Consumer-Side Deduplication

**Consumers MUST implement their own deduplication** because:

1. Multiple consumers may subscribe to the same channel
2. Consumers may restart and re-process messages
3. Network issues may cause message duplication

**Recommended approach**:

```python
# Example deduplication in consumer
def process_article(message):
    article_id = message['id']

    # Check if already ingested
    if db.article_exists(article_id):
        logger.info(f"Skipping duplicate article: {article_id}")
        return

    # Process and store article
    db.insert_article(message)
```

## Example Consumer Implementation

### Python Example

```python
import redis
import json

# Connect to Redis
r = redis.Redis(host='localhost', port=6379, decode_responses=True)

# Subscribe to channel
pubsub = r.pubsub()
pubsub.subscribe('articles:crime')

print("Listening for crime articles...")

for message in pubsub.listen():
    if message['type'] == 'message':
        # Parse JSON message
        article = json.loads(message['data'])

        # Check deduplication
        if db.article_exists(article['id']):
            continue

        # Apply additional filters
        if article['quality_score'] < 70:
            continue

        # Process article
        process_article(article)
```

### Node.js Example

```javascript
const redis = require('redis');

// Connect to Redis
const subscriber = redis.createClient({
  host: 'localhost',
  port: 6379
});

// Subscribe to channel
subscriber.subscribe('articles:crime');

subscriber.on('message', (channel, message) => {
  const article = JSON.parse(message);

  // Check deduplication
  if (await db.articleExists(article.id)) {
    return;
  }

  // Apply additional filters
  if (article.quality_score < 70) {
    return;
  }

  // Process article
  await processArticle(article);
});

console.log('Listening for crime articles...');
```

### PHP/Laravel 12 Example

```php
<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use Illuminate\Support\Facades\Redis;
use Illuminate\Support\Facades\DB;

class RedisSubscribeArticles extends Command
{
    protected $signature = 'redis:subscribe-articles';
    protected $description = 'Subscribe to Redis pub/sub channels for articles';

    public function handle()
    {
        $this->info('Subscribing to articles:crime channel...');

        Redis::subscribe(['articles:crime'], function ($message) {
            try {
                // Parse JSON message
                $article = json_decode($message, true);

                if (json_last_error() !== JSON_ERROR_NONE) {
                    $this->error('Invalid JSON: ' . json_last_error_msg());
                    return;
                }

                // Check deduplication
                if ($this->articleExists($article['id'])) {
                    $this->info("Skipping duplicate article: {$article['id']}");
                    return;
                }

                // Apply additional filters
                if (isset($article['quality_score']) && $article['quality_score'] < 70) {
                    $this->info("Skipping low quality article: {$article['id']}");
                    return;
                }

                // Process article
                $this->processArticle($article);

            } catch (\Exception $e) {
                $this->error('Error processing message: ' . $e->getMessage());
                \Log::error('Redis subscription error', [
                    'error' => $e->getMessage(),
                    'trace' => $e->getTraceAsString(),
                ]);
            }
        });
    }

    protected function articleExists(string $articleId): bool
    {
        return DB::table('articles')
            ->where('external_id', $articleId)
            ->exists();
    }

    protected function processArticle(array $article): void
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
            'publisher_route_id' => $article['publisher']['route_id'] ?? null,
            'publisher_channel' => $article['publisher']['channel'] ?? null,
            'published_at' => $article['publisher']['published_at'] ?? now(),
            'created_at' => now(),
            'updated_at' => now(),
        ]);

        $this->info("Processed article: {$article['title']}");
    }
}
```

**Running the Laravel consumer:**

```bash
php artisan redis:subscribe-articles
```

**Using Laravel Queue (Alternative approach):**

For production use, consider using Laravel queues for better reliability:

```php
// In your Artisan command or Event Listener
Redis::subscribe(['articles:crime'], function ($message) {
    ProcessArticleJob::dispatch(json_decode($message, true));
});
```

## Message Size

- **Typical size**: 5-15 KB per message (depending on article length)
- **Maximum size**: ~100 KB (for very long articles)
- **Fields to optimize**: `raw_html` can be large; consumers can choose not to store it

## Error Handling

### Publisher Errors

If the publisher fails to publish to Redis:
- Error is logged
- Article processing continues for other routes
- Router retries on next poll interval (default 5 minutes)

### Consumer Errors

Consumers should handle:
- **Invalid JSON**: Log error and skip message
- **Missing required fields**: Log error and skip message
- **Database errors**: Implement retry with exponential backoff
- **Network errors**: Reconnect to Redis and resume

### Example Error Handling

```python
def safe_process_article(message):
    try:
        article = json.loads(message['data'])

        # Validate required fields
        required = ['id', 'title', 'canonical_url']
        if not all(field in article for field in required):
            logger.error(f"Missing required fields: {message['data']}")
            return

        # Process with retry
        retry_count = 0
        while retry_count < 3:
            try:
                db.insert_article(article)
                break
            except DatabaseError as e:
                retry_count += 1
                time.sleep(2 ** retry_count)  # Exponential backoff
                if retry_count >= 3:
                    logger.error(f"Failed to insert after 3 retries: {e}")

    except json.JSONDecodeError as e:
        logger.error(f"Invalid JSON: {e}")
    except Exception as e:
        logger.error(f"Unexpected error: {e}")
```

## Performance Considerations

### Publisher

- **Batch size**: 100 articles per route per poll (configurable)
- **Poll interval**: 5 minutes (configurable)
- **Redis latency**: <10ms per publish

### Consumer

- **Throughput**: Depends on consumer implementation
- **Recommended**: Process messages asynchronously with queue
- **Scaling**: Run multiple consumer instances if needed (each handles all messages)

### Redis Configuration

For high-volume deployments:

```conf
# /etc/redis/redis.conf

# Increase max clients
maxclients 10000

# Adjust max memory
maxmemory 2gb
maxmemory-policy noeviction

# Persistence (if needed)
save 900 1
save 300 10
save 60 10000
```

## Testing

### Manual Testing

Publish a test message:

```bash
redis-cli PUBLISH articles:crime '{
  "publisher": {
    "route_id": "test-route",
    "published_at": "2025-12-28T10:00:00Z",
    "channel": "articles:crime"
  },
  "id": "test-123",
  "title": "Test Article",
  "body": "Test content",
  "canonical_url": "https://example.com/test",
  "source": "https://example.com/test",
  "published_date": "2025-12-28T10:00:00Z",
  "quality_score": 85,
  "topics": ["crime"],
  "content_type": "article",
  "is_crime_related": true
}'
```

### Subscribe to Monitor

```bash
redis-cli SUBSCRIBE articles:crime
```

## Troubleshooting

### No Messages Received

1. **Check Redis connectivity**:
   ```bash
   redis-cli PING
   ```

2. **Verify channel name**:
   ```bash
   redis-cli PUBSUB CHANNELS articles:*
   ```

3. **Check publisher router logs**:
   ```bash
   docker logs north-cloud-publisher-router
   ```

4. **Verify routes are enabled**:
   ```bash
   curl http://localhost:8070/api/v1/routes
   ```

### Messages Too Large

If messages are too large for your system:
- Omit `raw_html` field in consumer
- Store large fields (raw_html, raw_text) separately
- Use compression at consumer level

### High Message Volume

If receiving too many messages:
- Apply stricter filters in consumer
- Increase quality_score threshold in route configuration
- Create separate channels for different topics

## Support

For issues or questions:
- Check publisher logs: `docker logs north-cloud-publisher-router`
- Review publish history: `GET /api/v1/publish-history`
- Monitor Redis: `redis-cli MONITOR`

## Changelog

- **v1.0** (2025-12-28): Initial message format
  - Full Elasticsearch document payload
  - Publisher metadata section
  - Topic-based channel naming
