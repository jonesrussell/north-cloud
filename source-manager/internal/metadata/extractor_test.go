package metadata //nolint:testpackage // testing unexported SSRF prevention functions

import (
	"net"
	"testing"
)

func TestIsPrivateIP(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"nil IP", "", false},
		{"loopback IPv4", "127.0.0.1", true},
		{"loopback IPv6", "::1", true},
		{"private 10.x", "10.0.0.1", true},
		{"private 172.16.x", "172.16.0.1", true},
		{"private 192.168.x", "192.168.1.1", true},
		{"link-local IPv4", "169.254.1.1", true},
		{"link-local multicast", "ff02::1", true},
		{"unspecified IPv4", "0.0.0.0", true},
		{"unspecified IPv6", "::", true},
		{"public IPv4", "8.8.8.8", false},
		{"public IPv4 alt", "1.1.1.1", false},
		{"public IPv6", "2607:f8b0:4004:800::200e", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ip net.IP
			if tt.ip != "" {
				ip = net.ParseIP(tt.ip)
			}
			result := isPrivateIP(ip)
			if result != tt.expected {
				t.Errorf("isPrivateIP(%s) = %v, want %v", tt.ip, result, tt.expected)
			}
		})
	}
}

func TestValidateURLScheme(t *testing.T) {
	t.Helper()

	tests := []struct {
		name    string
		url     string
		wantErr bool
		errMsg  string
	}{
		{"valid https", "https://example.com", false, ""},
		{"valid http", "http://example.com", false, ""},
		{"ftp rejected", "ftp://example.com", true, "invalid URL scheme"},
		{"javascript rejected", "javascript:alert(1)", true, "invalid URL scheme"},
		{"file rejected", "file:///etc/passwd", true, "invalid URL scheme"},
		{"empty scheme rejected", "://example.com", true, "invalid URL"},
		{"blocked localhost", "http://localhost/admin", true, "blocked hostname"},
		{"blocked metadata GCP", "http://metadata.google.internal/", true, "blocked hostname"},
		{"blocked AWS metadata", "http://169.254.169.254/latest/meta-data/", true, "blocked hostname"},
		{"blocked localhost uppercase", "http://LOCALHOST/admin", true, "blocked hostname"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateURLScheme(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateURLScheme(%q) = nil, want error containing %q", tt.url, tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Errorf("validateURLScheme(%q) = %v, want nil", tt.url, err)
			}
		})
	}
}

func TestValidateAndGetRequestURL_Valid(t *testing.T) {
	t.Helper()

	requestURL, parsed, err := validateAndGetRequestURL("https://example.com/path?q=1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requestURL == "" {
		t.Error("requestURL is empty")
	}
	if parsed == nil {
		t.Fatal("parsed URL is nil")
	}
	if parsed.Host != "example.com" {
		t.Errorf("parsed host = %q, want %q", parsed.Host, "example.com")
	}
}

func TestValidateAndGetRequestURL_BlockedHost(t *testing.T) {
	t.Helper()

	_, _, err := validateAndGetRequestURL("http://localhost/admin")
	if err == nil {
		t.Fatal("expected error for blocked host, got nil")
	}
}

func TestValidateAndGetRequestURL_InvalidScheme(t *testing.T) {
	t.Helper()

	_, _, err := validateAndGetRequestURL("ftp://example.com")
	if err == nil {
		t.Fatal("expected error for invalid scheme, got nil")
	}
}
