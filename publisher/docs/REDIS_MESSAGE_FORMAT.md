# Redis Message Format - Publisher Service

This document describes the message format published to Redis pub/sub channels by the Publisher Router service.

## Overview

The Publisher Router queries Elasticsearch for classified content, filters them based on route configurations, and publishes matching content to Redis pub/sub channels. Each message is a complete JSON document containing the full content data plus publisher metadata.

## Channel Naming Convention

The publisher routes content across multiple channel layers:

### Layer 1: Topic channels (automatic)
```
content:{topic}
```
Published automatically for each content item topic. Examples: `content:crime`, `content:violent_crime`, `content:criminal_justice`, `content:mining`, `content:technology`, `content:sports`. Topics in `layer1SkipTopics` (currently: `mining`) are excluded from Layer 1 and handled by dedicated layers.

### Layer 2: Custom channels (database-backed)
Same `content:{topic}` pattern but with configurable rules (min quality, include/exclude topics, content types). Stored in the `channels` table.

### Layer 3: Crime classification channels
```
crime:homepage              # Homepage-eligible crime content
crime:category:{type}       # e.g. crime:category:violent-crime, crime:category:drug-crime
crime:courts                # Court-related crime content
crime:context               # Crime context content
```

### Layer 4: Location channels
```
crime:canada                # National Canadian crime
crime:province:{code}       # e.g. crime:province:on, crime:province:bc
crime:local:{city}          # e.g. crime:local:toronto, crime:local:vancouver
crime:international         # International crime
```

### Layer 5: Mining classification channels
```
content:mining             # Catch-all: all mining content (core + peripheral)
mining:core                 # Core mining content (homepage-quality)
mining:peripheral           # Peripheral mining content
mining:commodity:{slug}     # Per-commodity (e.g. mining:commodity:gold, mining:commodity:iron-ore)
mining:stage:{stage}        # Per-stage (exploration, development, production)
mining:canada               # Canadian mining news (local + national)
mining:international        # International mining news
```

### Layer 6: Entertainment classification channels
```
entertainment:homepage          # Core entertainment, homepage-eligible
entertainment:category:{slug}  # e.g. entertainment:category:film
entertainment:peripheral        # Peripheral entertainment
```

## Message Structure

Each message is a JSON object with two main sections:

1. **Publisher Metadata**: Information about when and how the content was published
2. **Content Data**: Complete content item data and classification metadata from Elasticsearch

### Full Message Example

```json
{
  "publisher": {
    "route_id": "a1b2c3d4-e5f6-4789-a0b1-c2d3e4f5g6h7",
    "published_at": "2025-12-28T15:30:45Z",
    "channel": "content:crime"
  },
  "id": "es-doc-id-12345",
  "title": "Local Police Investigate Break-In at Community Center",
  "body": "Full content text content here...",
  "raw_text": "Full content text content here...",
  "raw_html": "<html>Original HTML content...</html>",
  "canonical_url": "https://example.com/articles/police-investigate-break-in",
  "source": "https://example.com/original-content-url",
  "published_date": "2025-12-28T08:00:00Z",

  "quality_score": 85,
  "topics": ["crime", "local"],
  "content_type": "article",
  "content_subtype": "",
  "is_crime_related": true,
  "source_reputation": 78,
  "confidence": 0.92,

  "og_title": "Breaking News: Police Investigate Break-In",
  "og_description": "Community center targeted in overnight incident",
  "og_image": "https://example.com/images/content-image.jpg",
  "og_url": "https://example.com/articles/police-investigate-break-in",

  "intro": "Content introduction or lead paragraph...",
  "description": "Meta description of the content",
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

### Core Content Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | String | Yes | Elasticsearch document ID (unique) |
| `title` | String | Yes | Content headline/title |
| `body` | String | Yes | Content text (alias for raw_text) |
| `raw_text` | String | Yes | Full content text without HTML |
| `raw_html` | String | No | Original HTML content |
| `canonical_url` | String | Yes | Canonical URL for the content |
| `source` | String | Yes | Original source URL |
| `published_date` | ISO 8601 DateTime | Yes | When the content was originally published |

### Classification Metadata

| Field | Type | Range | Description |
|-------|------|-------|-------------|
| `quality_score` | Integer | 0-100 | Content quality rating |
| `topics` | Array[String] | - | Classified topics (e.g., ["crime", "local"]) |
| `content_type` | String | - | Content type (article, page, video, etc.) |
| `content_subtype` | String | - | Content subtype when `content_type` is article: `press_release`, `event`, `advisory`, `report`, `blotter`, `blog_post`, `company_announcement`, or empty for standard news |
| `is_crime_related` | Boolean | - | Whether the content is crime-related |
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
| `intro` | String | Content introduction/lead |
| `description` | String | Meta description |
| `word_count` | Integer | Number of words in content item |
| `category` | String | Content category |
| `section` | String | Site section |
| `keywords` | Array[String] | Content keywords |

### Entertainment Classification (Layer 6)

When the classifier has entertainment classification enabled, messages may include:

| Field | Type | Description |
|-------|------|-------------|
| `entertainment_relevance` | String | `core_entertainment`, `peripheral_entertainment`, or `not_entertainment` |
| `entertainment_categories` | Array[String] | e.g. `["film", "music", "gaming", "reviews"]` |
| `entertainment_homepage_eligible` | Boolean | True if content item qualifies for entertainment homepage |
| `entertainment` | Object | Nested: relevance, categories, final_confidence, homepage_eligible, review_required, model_version |

**Layer 6 channels**: `entertainment:homepage`, `entertainment:category:{slug}`, `entertainment:peripheral`.

## Field Aliases

For backward compatibility and convenience, the following aliases exist:

- **`body`**: Alias for `raw_text` (content text)
- **`source`**: Alias for the original content URL

Both fields are included in every message, so consumers can use whichever field name they prefer.

## Filtering at Consumer Side

The publisher applies the following filters before publishing:

1. **Quality Score**: `quality_score >= route.min_quality_score`
2. **Topics**: Content topics match route configuration (if specified)
3. **Deduplication**: Content item not already published to this channel

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
  WHERE content_id = $1 AND channel_name = $2
)
```

This ensures each content item is published **once per channel**.

### Consumer-Side Deduplication

**Consumers MUST implement their own deduplication** because:

1. Multiple consumers may subscribe to the same channel
2. Consumers may restart and re-process messages
3. Network issues may cause message duplication

**Recommended approach**:

```python
# Example deduplication in consumer
def process_item(message):
    content_id = message['id']

    # Check if already ingested
    if db.item_exists(content_id):
        logger.info(f"Skipping duplicate item: {content_id}")
        return

    # Process and store content
    db.insert_item(message)
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
pubsub.subscribe('content:crime')

print("Listening for crime content...")

for message in pubsub.listen():
    if message['type'] == 'message':
        # Parse JSON message
        item = json.loads(message['data'])

        # Check deduplication
        if db.item_exists(item['id']):
            continue

        # Apply additional filters
        if item['quality_score'] < 70:
            continue

        # Process content
        process_item(item)
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
subscriber.subscribe('content:crime');

subscriber.on('message', (channel, message) => {
  const item = JSON.parse(message);

  // Check deduplication
  if (await db.itemExists(item.id)) {
    return;
  }

  // Apply additional filters
  if (item.quality_score < 70) {
    return;
  }

  // Process content
  await processItem(item);
});

console.log('Listening for crime content...');
```

### PHP/Laravel 12 Example

```php
<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use Illuminate\Support\Facades\Redis;
use Illuminate\Support\Facades\DB;

class RedisSubscribeContent extends Command
{
    protected $signature = 'redis:subscribe-content';
    protected $description = 'Subscribe to Redis pub/sub channels for content';

    public function handle()
    {
        $this->info('Subscribing to content:crime channel...');

        Redis::subscribe(['content:crime'], function ($message) {
            try {
                // Parse JSON message
                $item = json_decode($message, true);

                if (json_last_error() !== JSON_ERROR_NONE) {
                    $this->error('Invalid JSON: ' . json_last_error_msg());
                    return;
                }

                // Check deduplication
                if ($this->itemExists($item['id'])) {
                    $this->info("Skipping duplicate item: {$item['id']}");
                    return;
                }

                // Apply additional filters
                if (isset($item['quality_score']) && $item['quality_score'] < 70) {
                    $this->info("Skipping low quality content: {$item['id']}");
                    return;
                }

                // Process content
                $this->processItem($item);

            } catch (\Exception $e) {
                $this->error('Error processing message: ' . $e->getMessage());
                \Log::error('Redis subscription error', [
                    'error' => $e->getMessage(),
                    'trace' => $e->getTraceAsString(),
                ]);
            }
        });
    }

    protected function itemExists(string $contentId): bool
    {
        return DB::table('content_items')
            ->where('external_id', $contentId)
            ->exists();
    }

    protected function processItem(array $item): void
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
            'publisher_route_id' => $item['publisher']['route_id'] ?? null,
            'publisher_channel' => $item['publisher']['channel'] ?? null,
            'published_at' => $item['publisher']['published_at'] ?? now(),
            'created_at' => now(),
            'updated_at' => now(),
        ]);

        $this->info("Processed content: {$item['title']}");
    }
}
```

**Running the Laravel consumer:**

```bash
php artisan redis:subscribe-content
```

**Using Laravel Queue (Alternative approach):**

For production use, consider using Laravel queues for better reliability:

```php
// In your Artisan command or Event Listener
Redis::subscribe(['content:crime'], function ($message) {
    ProcessContentJob::dispatch(json_decode($message, true));
});
```

## Message Size

- **Typical size**: 5-15 KB per message (depending on content length)
- **Maximum size**: ~100 KB (for very long content)
- **Fields to optimize**: `raw_html` can be large; consumers can choose not to store it

## Error Handling

### Publisher Errors

If the publisher fails to publish to Redis:
- Error is logged
- Content processing continues for other routes
- Router retries on next poll interval (default 5 minutes)

### Consumer Errors

Consumers should handle:
- **Invalid JSON**: Log error and skip message
- **Missing required fields**: Log error and skip message
- **Database errors**: Implement retry with exponential backoff
- **Network errors**: Reconnect to Redis and resume

### Example Error Handling

```python
def safe_process_item(message):
    try:
        item = json.loads(message['data'])

        # Validate required fields
        required = ['id', 'title', 'canonical_url']
        if not all(field in item for field in required):
            logger.error(f"Missing required fields: {message['data']}")
            return

        # Process with retry
        retry_count = 0
        while retry_count < 3:
            try:
                db.insert_item(item)
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

- **Batch size**: 100 items per route per poll (configurable)
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
redis-cli PUBLISH content:crime '{
  "publisher": {
    "route_id": "test-route",
    "published_at": "2025-12-28T10:00:00Z",
    "channel": "content:crime"
  },
  "id": "test-123",
  "title": "Test Content",
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
redis-cli SUBSCRIBE content:crime
```

## Troubleshooting

### No Messages Received

1. **Check Redis connectivity**:
   ```bash
   redis-cli PING
   ```

2. **Verify channel name**:
   ```bash
   redis-cli PUBSUB CHANNELS content:*
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

- **v1.1** (2026-02-14): Added `content_subtype` for multi-content-type support (press_release, event, advisory, report, blotter, blog_post, company_announcement).
- **v1.0** (2025-12-28): Initial message format
  - Full Elasticsearch document payload
  - Publisher metadata section
  - Topic-based channel naming
