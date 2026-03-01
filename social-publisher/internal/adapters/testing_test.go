package adapters_test

import (
	"context"
	"testing"

	"github.com/jonesrussell/north-cloud/social-publisher/internal/adapters"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestMockAdapter_ImplementsInterface(t *testing.T) {
	var _ domain.PlatformAdapter = (*adapters.MockAdapter)(nil)
}

func TestMockAdapter_PublishSuccess(t *testing.T) {
	mock := adapters.NewMockAdapter("test-platform")

	msg := domain.PublishMessage{
		ContentID: "test-1",
		Summary:   "Test post",
		URL:       "https://example.com",
	}

	post, err := mock.Transform(msg)
	assert.NoError(t, err)

	err = mock.Validate(post)
	assert.NoError(t, err)

	result, err := mock.Publish(context.Background(), post)
	assert.NoError(t, err)
	assert.NotEmpty(t, result.PlatformID)
	assert.Equal(t, 1, mock.PublishCount())
}

func TestMockAdapter_PublishFailure(t *testing.T) {
	mock := adapters.NewMockAdapter("test-platform")
	mock.SetPublishError(&domain.TransientError{Message: "timeout"})

	msg := domain.PublishMessage{Summary: "Test"}
	post, err := mock.Transform(msg)
	assert.NoError(t, err)

	_, pubErr := mock.Publish(context.Background(), post)
	assert.Error(t, pubErr)

	var publishErr domain.PublishError
	assert.ErrorAs(t, pubErr, &publishErr)
	assert.True(t, publishErr.IsRetryable())
}

func TestMockAdapter_Capabilities(t *testing.T) {
	mock := adapters.NewMockAdapter("test-platform")
	caps := mock.Capabilities()
	assert.Equal(t, maxTweetLen, caps.MaxLength)
}

const maxTweetLen = 280
