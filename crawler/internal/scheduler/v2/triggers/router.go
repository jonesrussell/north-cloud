package triggers

import (
	"context"
	"errors"
	"sync"

	"github.com/redis/go-redis/v9"

	"github.com/jonesrussell/north-cloud/crawler/internal/scheduler/v2/schedule"
)

var (
	// ErrRouterNotRunning is returned when the router is not running.
	ErrRouterNotRunning = errors.New("trigger router not running")

	// ErrWebhooksDisabled is returned when webhooks are disabled.
	ErrWebhooksDisabled = errors.New("webhook triggers disabled")

	// ErrPubSubDisabled is returned when Pub/Sub is disabled.
	ErrPubSubDisabled = errors.New("pubsub triggers disabled")
)

// RouterConfig holds configuration for the trigger router.
type RouterConfig struct {
	// EnableWebhooks enables webhook trigger handling.
	EnableWebhooks bool

	// EnablePubSub enables Redis Pub/Sub trigger handling.
	EnablePubSub bool

	// WebhookConfig is the configuration for webhooks.
	WebhookConfig WebhookConfig

	// PubSubConfig is the configuration for Pub/Sub.
	PubSubConfig PubSubConfig
}

// DefaultRouterConfig returns a config with sensible defaults.
func DefaultRouterConfig() RouterConfig {
	return RouterConfig{
		EnableWebhooks: true,
		EnablePubSub:   true,
		PubSubConfig:   DefaultPubSubConfig(),
	}
}

// Router routes events to appropriate trigger handlers.
type Router struct {
	config         RouterConfig
	matcher        *schedule.EventMatcher
	eventHandler   schedule.EventHandler
	webhookHandler *WebhookHandler
	pubsubListener *PubSubListener

	mu      sync.RWMutex
	running bool
}

// NewRouter creates a new trigger router.
func NewRouter(
	config RouterConfig,
	matcher *schedule.EventMatcher,
	handler schedule.EventHandler,
	redisClient *redis.Client,
) *Router {
	r := &Router{
		config:       config,
		matcher:      matcher,
		eventHandler: handler,
	}

	// Initialize webhook handler if enabled
	if config.EnableWebhooks {
		r.webhookHandler = NewWebhookHandler(config.WebhookConfig, matcher, handler)
	}

	// Initialize Pub/Sub listener if enabled
	if config.EnablePubSub && redisClient != nil {
		r.pubsubListener = NewPubSubListener(redisClient, matcher, handler, config.PubSubConfig)
	}

	return r
}

// Start starts the trigger router.
func (r *Router) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return nil
	}

	// Start Pub/Sub listener if enabled
	if r.pubsubListener != nil {
		if err := r.pubsubListener.Start(ctx); err != nil {
			return err
		}
	}

	r.running = true
	return nil
}

// Stop stops the trigger router.
func (r *Router) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return nil
	}

	// Stop Pub/Sub listener
	if r.pubsubListener != nil {
		if err := r.pubsubListener.Stop(); err != nil {
			return err
		}
	}

	r.running = false
	return nil
}

// WebhookHandler returns the webhook HTTP handler.
func (r *Router) WebhookHandler() (*WebhookHandler, error) {
	if !r.config.EnableWebhooks {
		return nil, ErrWebhooksDisabled
	}
	return r.webhookHandler, nil
}

// RegisterWebhookTrigger registers a job for webhook triggers.
func (r *Router) RegisterWebhookTrigger(jobID, pattern string) error {
	if !r.config.EnableWebhooks {
		return ErrWebhooksDisabled
	}
	return r.matcher.RegisterWebhookTrigger(jobID, pattern)
}

// RegisterChannelTrigger registers a job for Pub/Sub triggers.
func (r *Router) RegisterChannelTrigger(ctx context.Context, jobID, channel string) error {
	if !r.config.EnablePubSub {
		return ErrPubSubDisabled
	}

	// Register in matcher
	if err := r.matcher.RegisterChannelTrigger(jobID, channel); err != nil {
		return err
	}

	// Subscribe to channel if listener is running
	r.mu.RLock()
	running := r.running
	listener := r.pubsubListener
	r.mu.RUnlock()

	if running && listener != nil {
		if subscribeErr := listener.Subscribe(ctx, channel); subscribeErr != nil {
			// Ignore ErrAlreadySubscribed
			if !errors.Is(subscribeErr, ErrAlreadySubscribed) {
				return subscribeErr
			}
		}
	}

	return nil
}

// UnregisterJob removes all triggers for a job.
func (r *Router) UnregisterJob(jobID string) {
	r.matcher.UnregisterJob(jobID)
}

// TriggerWebhook manually triggers a webhook event.
func (r *Router) TriggerWebhook(ctx context.Context, path string, payload WebhookPayload) ([]string, error) {
	if !r.config.EnableWebhooks {
		return nil, ErrWebhooksDisabled
	}
	return r.webhookHandler.HandleWebhook(ctx, path, payload)
}

// TriggerEvent manually triggers an event.
func (r *Router) TriggerEvent(ctx context.Context, event schedule.Event) error {
	r.mu.RLock()
	running := r.running
	r.mu.RUnlock()

	if !running {
		return ErrRouterNotRunning
	}

	var jobIDs []string
	switch event.Type {
	case schedule.EventTypeWebhook:
		if !r.config.EnableWebhooks {
			return ErrWebhooksDisabled
		}
		jobIDs = r.matcher.MatchWebhook(event.Pattern)
	case schedule.EventTypePubSub:
		if !r.config.EnablePubSub {
			return ErrPubSubDisabled
		}
		jobIDs = r.matcher.MatchChannel(event.Pattern)
	}

	for _, jobID := range jobIDs {
		if err := r.eventHandler(ctx, jobID, event); err != nil {
			// Continue triggering other jobs even if one fails
			continue
		}
	}

	return nil
}

// IsRunning returns true if the router is running.
func (r *Router) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

// IsPubSubEnabled returns true if Pub/Sub is enabled.
func (r *Router) IsPubSubEnabled() bool {
	return r.config.EnablePubSub
}

// IsWebhooksEnabled returns true if webhooks are enabled.
func (r *Router) IsWebhooksEnabled() bool {
	return r.config.EnableWebhooks
}

// GetRegisteredWebhooks returns all registered webhook patterns.
func (r *Router) GetRegisteredWebhooks() []string {
	return r.matcher.GetRegisteredWebhooks()
}

// GetRegisteredChannels returns all registered channel names.
func (r *Router) GetRegisteredChannels() []string {
	return r.matcher.GetRegisteredChannels()
}

// Health returns the health status of the router.
func (r *Router) Health() RouterHealth {
	r.mu.RLock()
	defer r.mu.RUnlock()

	health := RouterHealth{
		Running:         r.running,
		WebhooksEnabled: r.config.EnableWebhooks,
		PubSubEnabled:   r.config.EnablePubSub,
	}

	if r.pubsubListener != nil {
		health.PubSubRunning = r.pubsubListener.IsRunning()
		health.PubSubChannels = r.pubsubListener.GetSubscribedChannels()
		health.PubSubError = r.pubsubListener.HealthError()
	}

	health.RegisteredWebhooks = r.matcher.GetRegisteredWebhooks()
	health.RegisteredChannels = r.matcher.GetRegisteredChannels()

	return health
}

// RouterHealth contains health information about the router.
type RouterHealth struct {
	Running            bool
	WebhooksEnabled    bool
	PubSubEnabled      bool
	PubSubRunning      bool
	PubSubChannels     []string
	PubSubError        error
	RegisteredWebhooks []string
	RegisteredChannels []string
}
