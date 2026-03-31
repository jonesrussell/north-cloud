package dedup_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/signal-crawler/internal/dedup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_NewItem(t *testing.T) {
	store, err := dedup.New(":memory:")
	require.NoError(t, err)
	defer store.Close()

	seen, err := store.Seen("hn", "12345")
	require.NoError(t, err)
	assert.False(t, seen, "new item should not be seen")
}

func TestStore_MarkAndCheck(t *testing.T) {
	store, err := dedup.New(":memory:")
	require.NoError(t, err)
	defer store.Close()

	err = store.Mark("hn", "12345")
	require.NoError(t, err)

	seen, err := store.Seen("hn", "12345")
	require.NoError(t, err)
	assert.True(t, seen, "marked item should be seen")
}

func TestStore_DifferentSources(t *testing.T) {
	store, err := dedup.New(":memory:")
	require.NoError(t, err)
	defer store.Close()

	err = store.Mark("hn", "12345")
	require.NoError(t, err)

	seen, err := store.Seen("funding", "12345")
	require.NoError(t, err)
	assert.False(t, seen, "same id from different source should not be seen")
}

func TestStore_MarkIdempotent(t *testing.T) {
	store, err := dedup.New(":memory:")
	require.NoError(t, err)
	defer store.Close()

	err = store.Mark("hn", "12345")
	require.NoError(t, err)

	err = store.Mark("hn", "12345")
	require.NoError(t, err, "marking same item twice should not error")
}
