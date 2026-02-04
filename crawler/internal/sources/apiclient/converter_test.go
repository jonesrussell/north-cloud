//nolint:testpackage // Testing unexported function buildAllowedDomains
package apiclient

import (
	"testing"
)

func TestBuildAllowedDomains(t *testing.T) {
	tests := []struct {
		name     string
		domain   string
		expected []string
	}{
		{
			name:     "empty domain returns nil",
			domain:   "",
			expected: nil,
		},
		{
			name:     "non-www domain includes both variants",
			domain:   "example.com",
			expected: []string{"example.com", "www.example.com"},
		},
		{
			name:     "www domain includes both variants",
			domain:   "www.example.com",
			expected: []string{"www.example.com", "example.com"},
		},
		{
			name:     "subdomain without www",
			domain:   "blog.example.com",
			expected: []string{"blog.example.com", "www.blog.example.com"},
		},
		{
			name:     "real world case - midnorthmonitor.com",
			domain:   "midnorthmonitor.com",
			expected: []string{"midnorthmonitor.com", "www.midnorthmonitor.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildAllowedDomains(tt.domain)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d domains, got %d: %v", len(tt.expected), len(result), result)
				return
			}

			for i, domain := range tt.expected {
				if result[i] != domain {
					t.Errorf("expected domain[%d] = %q, got %q", i, domain, result[i])
				}
			}
		})
	}
}

func TestConvertAPISourceToConfig_AllowedDomains(t *testing.T) {
	tests := []struct {
		name            string
		url             string
		expectedDomains []string
	}{
		{
			name:            "non-www URL includes both variants",
			url:             "https://example.com",
			expectedDomains: []string{"example.com", "www.example.com"},
		},
		{
			name:            "www URL includes both variants",
			url:             "https://www.example.com",
			expectedDomains: []string{"www.example.com", "example.com"},
		},
		{
			name:            "URL with path",
			url:             "https://example.com/articles",
			expectedDomains: []string{"example.com", "www.example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiSource := &APISource{
				Name: "Test Source",
				URL:  tt.url,
			}

			config, err := ConvertAPISourceToConfig(apiSource)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(config.AllowedDomains) != len(tt.expectedDomains) {
				t.Errorf("expected %d allowed domains, got %d: %v",
					len(tt.expectedDomains), len(config.AllowedDomains), config.AllowedDomains)
				return
			}

			for i, domain := range tt.expectedDomains {
				if config.AllowedDomains[i] != domain {
					t.Errorf("expected AllowedDomains[%d] = %q, got %q",
						i, domain, config.AllowedDomains[i])
				}
			}
		})
	}
}
