package redis

import (
	"context"
	"encoding/json"
	"fmt"

	goredis "github.com/redis/go-redis/v9"

	"github.com/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/domain"
)

// ChannelSocialPublish is the Redis Pub/Sub channel for inbound publish requests.
const ChannelSocialPublish = "social:publish"

// Subscriber listens for publish messages on Redis Pub/Sub.
type Subscriber struct {
	client *goredis.Client
	log    logger.Logger
}

// NewSubscriber creates a new Redis subscriber.
func NewSubscriber(client *goredis.Client, log logger.Logger) *Subscriber {
	return &Subscriber{client: client, log: log}
}

// Subscribe blocks and delivers parsed PublishMessage values to the handler until ctx is cancelled.
func (s *Subscriber) Subscribe(ctx context.Context, handler func(msg *domain.PublishMessage)) error {
	pubsub := s.client.Subscribe(ctx, ChannelSocialPublish)
	defer pubsub.Close()

	s.log.Info("Subscribed to Redis channel", logger.String("channel", ChannelSocialPublish))

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			s.log.Info("Redis subscriber shutting down")
			return ctx.Err()
		case redisMsg, ok := <-ch:
			if !ok {
				s.log.Error("Redis Pub/Sub channel closed unexpectedly",
					logger.String("channel", ChannelSocialPublish),
				)
				return fmt.Errorf("redis pub/sub channel %s closed unexpectedly", ChannelSocialPublish)
			}
			var msg domain.PublishMessage
			if err := json.Unmarshal([]byte(redisMsg.Payload), &msg); err != nil {
				s.log.Error("Failed to unmarshal publish message",
					logger.Error(err),
					logger.String("payload", redisMsg.Payload),
				)
				continue
			}
			handler(&msg)
		}
	}
}

// PublishMessage pushes a publish message onto the social:publish channel.
func (s *Subscriber) PublishMessage(ctx context.Context, msg *domain.PublishMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshaling publish message: %w", err)
	}
	return s.client.Publish(ctx, ChannelSocialPublish, data).Err()
}
