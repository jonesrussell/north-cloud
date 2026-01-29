package logs_test

import (
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
)

// verifyMessages checks that entries have expected messages in order.
func verifyMessages(t *testing.T, entries []logs.LogEntry, expected []string) {
	t.Helper()
	if len(entries) != len(expected) {
		t.Errorf("got %d entries, want %d", len(entries), len(expected))
		return
	}
	for i, e := range entries {
		if e.Message != expected[i] {
			t.Errorf("entries[%d].Message = %q, want %q", i, e.Message, expected[i])
		}
	}
}

// writeLetters writes n log entries with messages A, B, C, etc.
func writeLetters(buf logs.Buffer, n int) {
	for i := range n {
		buf.Write(logs.LogEntry{
			Timestamp: time.Now(),
			Message:   string(rune('A' + i)),
		})
	}
}

func TestCircularBuffer_Write(t *testing.T) {
	t.Helper()

	t.Run("writes entries to buffer", func(t *testing.T) {
		buf := logs.NewBuffer(10)

		entry := logs.LogEntry{
			Timestamp: time.Now(),
			Level:     "info",
			Message:   "test message",
		}
		buf.Write(entry)

		if buf.Size() != 1 {
			t.Errorf("Size() = %d, want 1", buf.Size())
		}
		if buf.LineCount() != 1 {
			t.Errorf("LineCount() = %d, want 1", buf.LineCount())
		}
	})

	t.Run("overwrites oldest when full", func(t *testing.T) {
		buf := logs.NewBuffer(3)
		writeLetters(buf, 5)

		if buf.Size() != 3 {
			t.Errorf("Size() = %d, want 3", buf.Size())
		}
		if buf.LineCount() != 5 {
			t.Errorf("LineCount() = %d, want 5", buf.LineCount())
		}

		// Should have C, D, E (oldest A, B overwritten)
		verifyMessages(t, buf.ReadAll(), []string{"C", "D", "E"})
	})
}

func TestCircularBuffer_ReadLast_Empty(t *testing.T) {
	t.Helper()

	buf := logs.NewBuffer(10)
	buf.Write(logs.LogEntry{Message: "test"})

	if len(buf.ReadLast(0)) != 0 {
		t.Error("ReadLast(0) should return empty slice")
	}
	if len(buf.ReadLast(-1)) != 0 {
		t.Error("ReadLast(-1) should return empty slice")
	}
}

func TestCircularBuffer_ReadLast_AllEntries(t *testing.T) {
	t.Helper()

	buf := logs.NewBuffer(10)
	writeLetters(buf, 3)

	entries := buf.ReadLast(10)
	verifyMessages(t, entries, []string{"A", "B", "C"})
}

func TestCircularBuffer_ReadLast_LastN(t *testing.T) {
	t.Helper()

	buf := logs.NewBuffer(10)
	writeLetters(buf, 5)

	// Should get D, E (last 2)
	verifyMessages(t, buf.ReadLast(2), []string{"D", "E"})
}

func TestCircularBuffer_ReadLast_AfterWrap(t *testing.T) {
	t.Helper()

	buf := logs.NewBuffer(3)
	writeLetters(buf, 5) // Buffer has C, D, E

	// Should get D, E (last 2)
	verifyMessages(t, buf.ReadLast(2), []string{"D", "E"})
}

func TestCircularBuffer_ReadLast_LargeWrap(t *testing.T) {
	t.Helper()

	buf := logs.NewBuffer(5)
	writeLetters(buf, 8) // Buffer has D, E, F, G, H

	// Should get F, G, H (last 3)
	verifyMessages(t, buf.ReadLast(3), []string{"F", "G", "H"})
}

func TestCircularBuffer_ReadSince(t *testing.T) {
	t.Helper()

	buf := logs.NewBuffer(10)
	now := time.Now()

	buf.Write(logs.LogEntry{Timestamp: now.Add(-2 * time.Hour), Message: "old"})
	buf.Write(logs.LogEntry{Timestamp: now.Add(-1 * time.Hour), Message: "medium"})
	buf.Write(logs.LogEntry{Timestamp: now, Message: "new"})

	entries := buf.ReadSince(now.Add(-90 * time.Minute))
	if len(entries) != 2 {
		t.Errorf("len(ReadSince) = %d, want 2", len(entries))
	}
}

func TestCircularBuffer_Clear(t *testing.T) {
	t.Helper()

	buf := logs.NewBuffer(10)
	for range 5 {
		buf.Write(logs.LogEntry{Message: "test"})
	}

	buf.Clear()

	if buf.Size() != 0 {
		t.Errorf("Size() after Clear() = %d, want 0", buf.Size())
	}
	// LineCount should be preserved
	if buf.LineCount() != 5 {
		t.Errorf("LineCount() after Clear() = %d, want 5", buf.LineCount())
	}
}

func TestCircularBuffer_Bytes(t *testing.T) {
	t.Helper()

	buf := logs.NewBuffer(10)
	buf.Write(logs.LogEntry{
		Timestamp: time.Date(2026, 1, 28, 12, 0, 0, 0, time.UTC),
		Level:     "info",
		Message:   "test",
	})

	bytes := buf.Bytes()
	if len(bytes) == 0 {
		t.Error("Bytes() returned empty slice")
	}

	// Should be valid JSON lines
	if bytes[len(bytes)-1] != '\n' {
		t.Error("Bytes() should end with newline")
	}
}

func TestNewBuffer_DefaultSize(t *testing.T) {
	t.Helper()

	// Zero or negative size should use default
	buf := logs.NewBuffer(0)
	// We can't directly test the size, but we can verify it works
	for range 1000 {
		buf.Write(logs.LogEntry{Message: "test"})
	}

	if buf.Size() == 0 {
		t.Error("Buffer with default size should allow writes")
	}
}
