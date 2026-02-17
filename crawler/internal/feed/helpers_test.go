package feed_test

import "testing"

// requireNoError fails the test immediately if err is non-nil.
func requireNoError(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// requireLen fails the test immediately if the slice length does not match.
func requireLen[T any](t *testing.T, items []T, expected int) {
	t.Helper()

	if len(items) != expected {
		t.Fatalf("expected %d items, got %d", expected, len(items))
	}
}
