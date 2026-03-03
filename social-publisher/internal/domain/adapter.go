package domain

import "context"

// PlatformAdapter defines the interface each social platform must implement.
type PlatformAdapter interface {
	Name() string
	Capabilities() PlatformCapabilities
	Transform(content PublishMessage) (PlatformPost, error)
	Validate(post PlatformPost) error
	Publish(ctx context.Context, post PlatformPost) (DeliveryResult, error)
}

// PlatformCapabilities describes what a platform supports.
type PlatformCapabilities struct {
	SupportsImages    bool
	SupportsThreading bool
	SupportsMarkdown  bool
	SupportsHTML      bool
	MaxLength         int
	RequiresMetadata  []string
}

// PlatformPost is the platform-specific representation of content ready to publish.
type PlatformPost struct {
	Platform string
	Content  string
	Title    string
	URL      string
	Images   []string
	Tags     []string
	Metadata map[string]string
	Thread   []string
}
