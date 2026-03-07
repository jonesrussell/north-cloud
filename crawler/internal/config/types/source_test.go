package types

import (
	"testing"
)

func minimalSource() Source {
	return Source{
		Name:      "test",
		URL:       "https://example.com",
		StartURLs: []string{"https://example.com"},
		RateLimit: "1s",
	}
}

func TestSourceValidate_MaxDepthNegativeOneAllowed(t *testing.T) {
	t.Helper()

	s := minimalSource()
	s.MaxDepth = -1
	if err := s.Validate(); err != nil {
		t.Errorf("expected MaxDepth=-1 to be valid, got error: %v", err)
	}
}

func TestSourceValidate_MaxDepthBelowNegativeOneRejected(t *testing.T) {
	t.Helper()

	s := minimalSource()
	s.MaxDepth = -2
	if err := s.Validate(); err == nil {
		t.Error("expected MaxDepth=-2 to be invalid, got nil error")
	}
}

func TestSourceValidate_MaxDepthZeroAllowed(t *testing.T) {
	t.Helper()

	s := minimalSource()
	s.MaxDepth = 0
	if err := s.Validate(); err != nil {
		t.Errorf("expected MaxDepth=0 to be valid, got error: %v", err)
	}
}

func TestSourceValidate_MaxDepthPositiveAllowed(t *testing.T) {
	t.Helper()

	s := minimalSource()
	s.MaxDepth = 5
	if err := s.Validate(); err != nil {
		t.Errorf("expected MaxDepth=5 to be valid, got error: %v", err)
	}
}
