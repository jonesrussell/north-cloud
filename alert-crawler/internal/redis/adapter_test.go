package redis_test

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	redispkg "github.com/jonesrussell/north-cloud/alert-crawler/internal/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNew_Connect verifies that New returns a ready Publisher when Redis is
// reachable and the Config is valid.
func TestNew_Connect(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)

	pub, err := redispkg.New(redispkg.Config{
		Address: mr.Addr(),
		Channel: "indigenous:alerts",
	})
	require.NoError(t, err)
	require.NotNil(t, pub)

	require.NoError(t, pub.Close())
}

// TestNew_BadAddress verifies that New returns an error when Redis is unreachable.
func TestNew_BadAddress(t *testing.T) {
	t.Parallel()

	pub, err := redispkg.New(redispkg.Config{
		Address: "127.0.0.1:19999", // nothing listening here
		Channel: "indigenous:alerts",
	})
	require.Error(t, err)
	assert.Nil(t, pub)
}

// TestAdapter_Publish verifies that the publish adapter forwards the message
// to the underlying go-redis client and returns nil on success.
// Publisher.Publish is called end-to-end against miniredis so the adapter's
// Publish method is exercised.
func TestAdapter_Publish(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)

	pub, err := redispkg.New(redispkg.Config{
		Address: mr.Addr(),
		Channel: "test-channel",
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	event := fixtureEvent(t)
	require.NoError(t, pub.Publish(context.Background(), event))
}

// TestAdapter_Close verifies that the adapter's Close path is exercised
// without error when the underlying client is healthy.
func TestAdapter_Close(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)

	pub, err := redispkg.New(redispkg.Config{
		Address: mr.Addr(),
		Channel: "test-channel",
	})
	require.NoError(t, err)

	require.NoError(t, pub.Close())
}
