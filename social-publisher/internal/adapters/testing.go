package adapters

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/jonesrussell/north-cloud/social-publisher/internal/domain"
)

const mockMaxLength = 280

// MockAdapter is a test double implementing domain.PlatformAdapter.
type MockAdapter struct {
	name         string
	capabilities domain.PlatformCapabilities
	publishErr   domain.PublishError
	publishCount atomic.Int32
	mu           sync.Mutex
	published    []domain.PlatformPost
}

// NewMockAdapter creates a mock adapter with sensible defaults.
func NewMockAdapter(name string) *MockAdapter {
	return &MockAdapter{
		name: name,
		capabilities: domain.PlatformCapabilities{
			SupportsImages:    true,
			SupportsThreading: false,
			SupportsMarkdown:  false,
			SupportsHTML:      false,
			MaxLength:         mockMaxLength,
		},
	}
}

func (m *MockAdapter) Name() string                              { return m.name }
func (m *MockAdapter) Capabilities() domain.PlatformCapabilities { return m.capabilities }

func (m *MockAdapter) Transform(content domain.PublishMessage) (domain.PlatformPost, error) {
	text := content.Summary
	if content.URL != "" {
		text = fmt.Sprintf("%s %s", text, content.URL)
	}
	return domain.PlatformPost{
		Platform: m.name,
		Content:  text,
		URL:      content.URL,
		Tags:     content.Tags,
	}, nil
}

func (m *MockAdapter) Validate(post domain.PlatformPost) error {
	if post.Content == "" {
		return &domain.ValidationError{Field: "content", Message: "content is required"}
	}
	return nil
}

func (m *MockAdapter) Publish(_ context.Context, post domain.PlatformPost) (domain.DeliveryResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.publishErr != nil {
		return domain.DeliveryResult{}, m.publishErr
	}

	m.publishCount.Add(1)
	m.published = append(m.published, post)

	count := m.publishCount.Load()
	return domain.DeliveryResult{
		PlatformID:  fmt.Sprintf("mock-%d", count),
		PlatformURL: fmt.Sprintf("https://%s.example.com/post/%d", m.name, count),
	}, nil
}

// SetPublishError configures the mock to return an error on Publish calls.
func (m *MockAdapter) SetPublishError(err domain.PublishError) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.publishErr = err
}

// PublishCount returns the number of successful publishes.
func (m *MockAdapter) PublishCount() int {
	return int(m.publishCount.Load())
}

// Published returns copies of all successfully published posts.
func (m *MockAdapter) Published() []domain.PlatformPost {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]domain.PlatformPost{}, m.published...)
}
