package dedup

import (
	"context"
	"fmt"
	"time"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/redis/go-redis/v9"
)

// redisScanBatchSize is the number of keys to return per SCAN iteration when clearing tracked content.
const redisScanBatchSize = 100

type Tracker struct {
	client redis.UniversalClient
	ttl    time.Duration
	logger infralogger.Logger
}

func NewTracker(client redis.UniversalClient, ttl time.Duration, log infralogger.Logger) *Tracker {
	return &Tracker{
		client: client,
		ttl:    ttl,
		logger: log,
	}
}

func (t *Tracker) key(contentID string) string {
	return fmt.Sprintf("posted:content:%s", contentID)
}

func (t *Tracker) HasPosted(ctx context.Context, contentID string) bool {
	key := t.key(contentID)

	t.logger.Debug("Checking if content was posted",
		infralogger.String("content_id", contentID),
		infralogger.String("redis_key", key),
	)

	exists, err := t.client.Exists(ctx, key).Result()
	if err != nil {
		t.logger.Error("Redis error checking content",
			infralogger.String("content_id", contentID),
			infralogger.String("redis_key", key),
			infralogger.Error(err),
		)
		// Log error but don't fail - assume not posted
		return false
	}

	alreadyPosted := exists == 1
	if alreadyPosted {
		t.logger.Debug("Content already posted",
			infralogger.String("content_id", contentID),
			infralogger.String("redis_key", key),
		)
	} else {
		t.logger.Debug("Content not yet posted",
			infralogger.String("content_id", contentID),
			infralogger.String("redis_key", key),
		)
	}

	return alreadyPosted
}

func (t *Tracker) MarkPosted(ctx context.Context, contentID string) error {
	key := t.key(contentID)

	t.logger.Debug("Marking content as posted",
		infralogger.String("content_id", contentID),
		infralogger.String("redis_key", key),
		infralogger.Duration("ttl", t.ttl),
	)

	err := t.client.Set(ctx, key, "1", t.ttl).Err()
	if err != nil {
		t.logger.Error("Redis error marking content as posted",
			infralogger.String("content_id", contentID),
			infralogger.String("redis_key", key),
			infralogger.Duration("ttl", t.ttl),
			infralogger.Error(err),
		)
		return err
	}

	t.logger.Debug("Content marked as posted",
		infralogger.String("content_id", contentID),
		infralogger.String("redis_key", key),
	)

	return nil
}

func (t *Tracker) Clear(ctx context.Context, contentID string) error {
	key := t.key(contentID)

	t.logger.Debug("Clearing content from posted cache",
		infralogger.String("content_id", contentID),
		infralogger.String("redis_key", key),
	)

	err := t.client.Del(ctx, key).Err()
	if err != nil {
		t.logger.Error("Redis error clearing content",
			infralogger.String("content_id", contentID),
			infralogger.String("redis_key", key),
			infralogger.Error(err),
		)
		return err
	}

	t.logger.Debug("Content cleared from posted cache",
		infralogger.String("content_id", contentID),
		infralogger.String("redis_key", key),
	)

	return nil
}

// FlushAll removes all posted content keys from Redis
// This will clear the entire deduplication cache
func (t *Tracker) FlushAll(ctx context.Context) error {
	t.logger.Info("Flushing all posted content keys from Redis cache")

	// Use SCAN to find all keys matching the pattern "posted:content:*"
	// This is safer than FLUSHDB which would clear the entire Redis database
	pattern := "posted:content:*"
	var cursor uint64
	var deletedCount int

	for {
		var keys []string
		var err error
		keys, cursor, err = t.client.Scan(ctx, cursor, pattern, redisScanBatchSize).Result()
		if err != nil {
			t.logger.Error("Redis error scanning for keys",
				infralogger.String("pattern", pattern),
				infralogger.Error(err),
			)
			return fmt.Errorf("scan keys: %w", err)
		}

		if len(keys) > 0 {
			deleted, delErr := t.client.Del(ctx, keys...).Result()
			if delErr != nil {
				t.logger.Error("Redis error deleting keys",
					infralogger.Int("key_count", len(keys)),
					infralogger.Error(delErr),
				)
				return fmt.Errorf("delete keys: %w", delErr)
			}
			deletedCount += int(deleted)
		}

		if cursor == 0 {
			break
		}
	}

	t.logger.Info("Flushed Redis cache",
		infralogger.Int("keys_deleted", deletedCount),
		infralogger.String("pattern", pattern),
	)

	return nil
}
