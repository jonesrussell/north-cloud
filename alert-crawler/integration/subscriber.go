//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
)

// subscriberDialTimeout is the time allowed for the initial Redis ping and
// subscribe handshake when constructing a Subscriber.
const subscriberDialTimeout = 5 * time.Second

// Subscriber is a thin Redis pub/sub wrapper used by integration tests to
// assert that lifecycle events arrive on the expected channel.
type Subscriber struct {
	client  *goredis.Client
	pubsub  *goredis.PubSub
	channel string
}

// NewSubscriber dials Redis at addr and subscribes to channel.
// Returns an error if the connection or subscribe handshake fails.
func NewSubscriber(addr, channel string) (*Subscriber, error) {
	client := goredis.NewClient(&goredis.Options{Addr: addr})

	ctx, cancel := context.WithTimeout(context.Background(), subscriberDialTimeout)
	defer cancel()

	if pingErr := client.Ping(ctx).Err(); pingErr != nil {
		_ = client.Close()
		return nil, fmt.Errorf("subscriber: ping redis at %s: %w", addr, pingErr)
	}

	ps := client.Subscribe(ctx, channel)

	// Drain the subscription confirmation message.
	if _, confErr := ps.Receive(ctx); confErr != nil {
		_ = ps.Close()
		_ = client.Close()
		return nil, fmt.Errorf("subscriber: subscribe to %s: %w", channel, confErr)
	}

	return &Subscriber{
		client:  client,
		pubsub:  ps,
		channel: channel,
	}, nil
}

// Receive blocks until a lifecycle event arrives or timeout elapses.
// Returns (event, true) on success and (zero, false) on timeout.
func (s *Subscriber) Receive(timeout time.Duration) (domain.LifecycleEvent, bool) {
	ch := s.pubsub.Channel()

	select {
	case msg, ok := <-ch:
		if !ok {
			return domain.LifecycleEvent{}, false
		}

		var ev domain.LifecycleEvent
		if err := json.Unmarshal([]byte(msg.Payload), &ev); err != nil {
			return domain.LifecycleEvent{}, false
		}

		return ev, true

	case <-time.After(timeout):
		return domain.LifecycleEvent{}, false
	}
}

// Close unsubscribes and closes the underlying Redis connection.
func (s *Subscriber) Close() {
	_ = s.pubsub.Close()
	_ = s.client.Close()
}
