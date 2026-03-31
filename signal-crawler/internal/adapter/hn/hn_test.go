package hn_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/jonesrussell/north-cloud/signal-crawler/internal/adapter/hn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHNAdapter_Name(t *testing.T) {
	a := hn.New("", 10)
	assert.Equal(t, "hn", a.Name())
}

func TestHNAdapter_Scan(t *testing.T) {
	// Load fixtures.
	withSignal, err := os.ReadFile(filepath.Join("testdata", "item_with_signal.json"))
	require.NoError(t, err)
	noSignal, err := os.ReadFile(filepath.Join("testdata", "item_no_signal.json"))
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v0/newstories.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[99001,99002]`))
		case "/v0/item/99001.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(withSignal)
		case "/v0/item/99002.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(noSignal)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	a := hn.New(srv.URL, 10)
	signals, err := a.Scan(context.Background())
	require.NoError(t, err)

	require.Len(t, signals, 1)
	s := signals[0]
	assert.Equal(t, "99001", s.ExternalID)
	assert.Equal(t, 90, s.SignalStrength)
	assert.Contains(t, s.Label, "Looking for CTO")
}

func TestHNAdapter_EmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v0/newstories.json" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	defer srv.Close()

	a := hn.New(srv.URL, 10)
	signals, err := a.Scan(context.Background())
	require.NoError(t, err)
	assert.Empty(t, signals)
}
