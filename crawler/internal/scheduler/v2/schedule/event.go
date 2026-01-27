package schedule

import (
	"context"
	"errors"
	"path"
	"strings"
	"sync"
)

var (
	// ErrNoTriggerConfigured is returned when a job has no event trigger configured.
	ErrNoTriggerConfigured = errors.New("no event trigger configured")

	// ErrInvalidTriggerPattern is returned when a trigger pattern is invalid.
	ErrInvalidTriggerPattern = errors.New("invalid trigger pattern")
)

// EventHandler is called when an event matches a job trigger.
type EventHandler func(ctx context.Context, jobID string, event Event) error

// Event represents an incoming event that may trigger jobs.
type Event struct {
	Type    EventType
	Source  string
	Pattern string
	Payload map[string]any
}

// EventType represents the type of triggering event.
type EventType string

const (
	// EventTypeWebhook is a webhook event.
	EventTypeWebhook EventType = "webhook"

	// EventTypePubSub is a Redis Pub/Sub event.
	EventTypePubSub EventType = "pubsub"
)

// EventMatcher matches events to job triggers.
type EventMatcher struct {
	mu       sync.RWMutex
	webhooks map[string][]string // pattern -> job IDs
	channels map[string][]string // channel -> job IDs
}

// NewEventMatcher creates a new event matcher.
func NewEventMatcher() *EventMatcher {
	return &EventMatcher{
		webhooks: make(map[string][]string),
		channels: make(map[string][]string),
	}
}

// RegisterWebhookTrigger registers a job to be triggered by a webhook pattern.
func (m *EventMatcher) RegisterWebhookTrigger(jobID, pattern string) error {
	if pattern == "" {
		return ErrInvalidTriggerPattern
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Normalize pattern
	pattern = normalizeWebhookPattern(pattern)

	m.webhooks[pattern] = append(m.webhooks[pattern], jobID)
	return nil
}

// RegisterChannelTrigger registers a job to be triggered by a Redis channel.
func (m *EventMatcher) RegisterChannelTrigger(jobID, channel string) error {
	if channel == "" {
		return ErrInvalidTriggerPattern
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.channels[channel] = append(m.channels[channel], jobID)
	return nil
}

// UnregisterJob removes all triggers for a job.
func (m *EventMatcher) UnregisterJob(jobID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove from webhooks
	for pattern, jobs := range m.webhooks {
		m.webhooks[pattern] = removeFromSlice(jobs, jobID)
		if len(m.webhooks[pattern]) == 0 {
			delete(m.webhooks, pattern)
		}
	}

	// Remove from channels
	for channel, jobs := range m.channels {
		m.channels[channel] = removeFromSlice(jobs, jobID)
		if len(m.channels[channel]) == 0 {
			delete(m.channels, channel)
		}
	}
}

// MatchWebhook returns job IDs that match a webhook path.
func (m *EventMatcher) MatchWebhook(webhookPath string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matches []string
	webhookPath = normalizeWebhookPattern(webhookPath)

	for pattern, jobIDs := range m.webhooks {
		if matchPattern(pattern, webhookPath) {
			matches = append(matches, jobIDs...)
		}
	}

	return uniqueStrings(matches)
}

// MatchChannel returns job IDs that match a channel name.
func (m *EventMatcher) MatchChannel(channel string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Exact match
	if jobIDs, ok := m.channels[channel]; ok {
		return jobIDs
	}

	// Pattern match (supports wildcards)
	var matches []string
	for pattern, jobIDs := range m.channels {
		if matchChannelPattern(pattern, channel) {
			matches = append(matches, jobIDs...)
		}
	}

	return uniqueStrings(matches)
}

// GetRegisteredWebhooks returns all registered webhook patterns.
func (m *EventMatcher) GetRegisteredWebhooks() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	patterns := make([]string, 0, len(m.webhooks))
	for pattern := range m.webhooks {
		patterns = append(patterns, pattern)
	}
	return patterns
}

// GetRegisteredChannels returns all registered channel names.
func (m *EventMatcher) GetRegisteredChannels() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	channels := make([]string, 0, len(m.channels))
	for channel := range m.channels {
		channels = append(channels, channel)
	}
	return channels
}

// normalizeWebhookPattern normalizes a webhook pattern.
func normalizeWebhookPattern(pattern string) string {
	// Ensure leading slash
	if !strings.HasPrefix(pattern, "/") {
		pattern = "/" + pattern
	}
	// Remove trailing slash (except for root)
	if len(pattern) > 1 && strings.HasSuffix(pattern, "/") {
		pattern = strings.TrimSuffix(pattern, "/")
	}
	return pattern
}

// matchPattern matches a path against a pattern with wildcards.
// Supports * for single segment and ** for multiple segments.
func matchPattern(pattern, pathStr string) bool {
	// Exact match
	if pattern == pathStr {
		return true
	}

	// Use path.Match for simple wildcards
	matched, err := path.Match(pattern, pathStr)
	if err == nil && matched {
		return true
	}

	// Handle ** patterns
	if strings.Contains(pattern, "**") {
		// Convert ** to regex-like matching
		// doubleWildcardParts is the expected number of parts when splitting by **
		const doubleWildcardParts = 2
		parts := strings.Split(pattern, "**")
		if len(parts) == doubleWildcardParts {
			prefix := parts[0]
			suffix := parts[1]
			if strings.HasPrefix(pathStr, prefix) && strings.HasSuffix(pathStr, suffix) {
				return true
			}
		}
	}

	return false
}

// matchChannelPattern matches a channel against a pattern.
// Supports * for single segment and ** for any number of segments.
func matchChannelPattern(pattern, channel string) bool {
	// Exact match
	if pattern == channel {
		return true
	}

	// Convert Redis-style patterns to path patterns
	pattern = strings.ReplaceAll(pattern, "*", "[^:]*")
	channel = strings.ReplaceAll(channel, ":", "/")
	pattern = strings.ReplaceAll(pattern, ":", "/")

	matched, err := path.Match(pattern, channel)
	return err == nil && matched
}

// removeFromSlice removes a string from a slice.
func removeFromSlice(slice []string, item string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}

// uniqueStrings removes duplicates from a slice.
func uniqueStrings(slice []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
