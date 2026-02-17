package crawler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	configtypes "github.com/jonesrussell/north-cloud/crawler/internal/config/types"
	"github.com/jonesrussell/north-cloud/crawler/internal/crawler"
)

func TestCheckRedirect_NoRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	source := &configtypes.Source{
		URL:            srv.URL,
		AllowedDomains: []string{"127.0.0.1"},
	}

	if err := crawler.CheckRedirect(context.Background(), source); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCheckRedirect_NoDomainChange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "/other-page")
		w.WriteHeader(http.StatusMovedPermanently)
	}))
	defer srv.Close()

	source := &configtypes.Source{
		URL:            srv.URL,
		AllowedDomains: []string{"127.0.0.1"},
	}

	if err := crawler.CheckRedirect(context.Background(), source); err != nil {
		t.Fatalf("expected no error for same-domain redirect, got: %v", err)
	}
}

func TestCheckRedirect_CrossDomainRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "https://evil.example.com/landing")
		w.WriteHeader(http.StatusMovedPermanently)
	}))
	defer srv.Close()

	source := &configtypes.Source{
		URL:            srv.URL,
		AllowedDomains: []string{"original.example.com"},
	}

	err := crawler.CheckRedirect(context.Background(), source)
	if err == nil {
		t.Fatal("expected error for cross-domain redirect, got nil")
	}
}

func TestCheckRedirect_ConnectionError(t *testing.T) {
	source := &configtypes.Source{
		URL:            "http://127.0.0.1:1", // will fail to connect
		AllowedDomains: []string{"127.0.0.1"},
	}

	// Connection errors should be non-fatal (return nil)
	if err := crawler.CheckRedirect(context.Background(), source); err != nil {
		t.Fatalf("expected nil for connection error (non-fatal), got: %v", err)
	}
}

func TestIsRedirectStatus(t *testing.T) {
	tests := []struct {
		code int
		want bool
	}{
		{http.StatusOK, false},
		{http.StatusCreated, false},
		{http.StatusNoContent, false},
		{http.StatusBadRequest, false},
		{http.StatusNotFound, false},
		{http.StatusInternalServerError, false},
		{http.StatusMovedPermanently, true},
		{http.StatusFound, true},
		{http.StatusSeeOther, true},
		{http.StatusTemporaryRedirect, true},
		{http.StatusPermanentRedirect, true},
	}

	for _, tt := range tests {
		if got := crawler.IsRedirectStatus(tt.code); got != tt.want {
			t.Errorf("IsRedirectStatus(%d) = %v, want %v", tt.code, got, tt.want)
		}
	}
}

func TestHandlePossibleDomainRedirect_AllowedDomain(t *testing.T) {
	source := &configtypes.Source{
		URL:            "https://example.com",
		AllowedDomains: []string{"example.com", "www.example.com"},
	}

	err := crawler.HandlePossibleDomainRedirect(source, "https://www.example.com/page")
	if err != nil {
		t.Fatalf("expected no error for allowed domain redirect, got: %v", err)
	}
}

func TestHandlePossibleDomainRedirect_DisallowedDomain(t *testing.T) {
	source := &configtypes.Source{
		URL:            "https://example.com",
		AllowedDomains: []string{"example.com"},
	}

	err := crawler.HandlePossibleDomainRedirect(source, "https://other.com/page")
	if err == nil {
		t.Fatal("expected error for disallowed domain redirect, got nil")
	}
}

func TestHandlePossibleDomainRedirect_RelativeRedirect(t *testing.T) {
	source := &configtypes.Source{
		URL:            "https://example.com",
		AllowedDomains: []string{"example.com"},
	}

	err := crawler.HandlePossibleDomainRedirect(source, "/relative/path")
	if err != nil {
		t.Fatalf("expected no error for relative redirect, got: %v", err)
	}
}

func TestHostsMatch(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"example.com", "example.com", true},
		{"example.com:443", "example.com", true},
		{"example.com", "example.com:443", true},
		{"example.com", "other.com", false},
	}

	for _, tt := range tests {
		if got := crawler.HostsMatch(tt.a, tt.b); got != tt.want {
			t.Errorf("HostsMatch(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}
