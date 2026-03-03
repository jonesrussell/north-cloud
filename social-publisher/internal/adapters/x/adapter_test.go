package x_test

import (
	"strings"
	"testing"

	"github.com/jonesrussell/north-cloud/social-publisher/internal/adapters/x"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestXAdapter_Name(t *testing.T) {
	adapter := x.NewAdapter(nil)
	assert.Equal(t, "x", adapter.Name())
}

func TestXAdapter_Capabilities(t *testing.T) {
	adapter := x.NewAdapter(nil)
	caps := adapter.Capabilities()
	assert.Equal(t, x.MaxTweetLength, caps.MaxLength)
	assert.True(t, caps.SupportsThreading)
	assert.True(t, caps.SupportsImages)
	assert.Empty(t, caps.RequiresMetadata)
}

func TestXAdapter_Transform_ShortPost(t *testing.T) {
	adapter := x.NewAdapter(nil)
	msg := domain.PublishMessage{
		Summary: "Check out this new feature",
		URL:     "https://example.com/post",
		Tags:    []string{"golang", "dev"},
	}

	post, err := adapter.Transform(msg)
	assert.NoError(t, err)
	assert.Contains(t, post.Content, "Check out this new feature")
	assert.Contains(t, post.Content, "https://example.com/post")
	assert.LessOrEqual(t, len(post.Content), x.MaxTweetLength)
}

func TestXAdapter_Transform_LongPostCreatesThread(t *testing.T) {
	adapter := x.NewAdapter(nil)
	longBody := strings.Repeat("This is a sentence that makes the content longer. ", 50)
	msg := domain.PublishMessage{
		Summary: "A long blog post",
		Body:    longBody,
		URL:     "https://example.com/long-post",
	}

	post, err := adapter.Transform(msg)
	assert.NoError(t, err)
	assert.True(t, len(post.Thread) > 0 || len(post.Content) <= x.MaxTweetLength)
}

func TestXAdapter_Validate_EmptyContent(t *testing.T) {
	adapter := x.NewAdapter(nil)
	post := domain.PlatformPost{Platform: "x", Content: ""}
	err := adapter.Validate(post)
	assert.Error(t, err)
}

func TestXAdapter_Validate_TooLong(t *testing.T) {
	adapter := x.NewAdapter(nil)
	longContent := strings.Repeat("a", 300)
	post := domain.PlatformPost{Platform: "x", Content: longContent}
	err := adapter.Validate(post)
	assert.Error(t, err)
}
