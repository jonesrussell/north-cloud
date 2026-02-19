package storage_test

import (
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/click-tracker/internal/domain"
	"github.com/jonesrussell/north-cloud/click-tracker/internal/storage"
)

func newTestEvent(t *testing.T) domain.ClickEvent {
	t.Helper()

	return domain.ClickEvent{
		QueryID:         "q123",
		ResultID:        "r456",
		Position:        1,
		Page:            1,
		DestinationHash: "abc123",
		SessionID:       "sess1",
		UserAgentHash:   "ua1",
		GeneratedAt:     time.Now(),
		ClickedAt:       time.Now(),
	}
}

func TestBuffer_Send(t *testing.T) {
	t.Helper()

	buf := storage.NewBuffer(10)
	defer buf.Close()

	event := newTestEvent(t)
	ok := buf.Send(event)

	if !ok {
		t.Fatal("expected Send to succeed on non-full buffer")
	}
}

func TestBuffer_SendFull(t *testing.T) {
	t.Helper()

	buf := storage.NewBuffer(1)
	defer buf.Close()

	event := newTestEvent(t)

	// Fill the buffer.
	ok := buf.Send(event)
	if !ok {
		t.Fatal("expected first Send to succeed")
	}

	// Second send should fail (non-blocking).
	ok = buf.Send(event)
	if ok {
		t.Fatal("expected Send to return false when buffer is full")
	}
}
