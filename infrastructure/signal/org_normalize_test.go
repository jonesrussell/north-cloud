package signal_test

import (
	"errors"
	"testing"

	"github.com/jonesrussell/north-cloud/infrastructure/signal"
)

func TestNormalize(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"whitespace", "   ", ""},
		{"simple", "Acme", "acme"},
		{"mixed case", "AcMe", "acme"},
		{"trailing inc", "Acme Inc", "acme"},
		{"trailing inc dot", "Acme Inc.", "acme"},
		{"trailing corporation", "Acme Corporation", "acme"},
		{"trailing llc", "Acme LLC", "acme"},
		{"comma inc", "Acme, Inc.", "acme"},
		{"apex domain collapses via suffix strip", "acme-corp.com", "acme"},
		{"hyphenated corp form", "acme-corp", "acme"},
		{"identity-bearing tokens preserved", "Acme Holdings Group", "acme-holdings-group"},
		{"punctuation collapse", "Acme & Sons!", "acme-sons"},
		{"leading/trailing punctuation trimmed", "!!Acme!!", "acme"},
		{"multi-word org with trailing co", "Big Blue Widget Co.", "big-blue-widget"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := signal.Normalize(tc.input)
			if got != tc.want {
				t.Errorf("Normalize(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestNormalize_CanonicalParity(t *testing.T) {
	t.Parallel()
	// Cross-producer dedup requires that every surface form of the same
	// organization — whether a human-typed name, a domain, or a hyphenated
	// handle — collapse to one canonical string. Per the lead-pipeline spec
	// (§Organization attribution), "Acme Corporation", "Acme Corp", and
	// "acme-corp.com" must converge.
	forms := []string{
		"Acme Corporation",
		"Acme Corp",
		"ACME CORP.",
		"acme-corp.com",
		"Acme-Corp",
		"acme corp",
	}
	const want = "acme"
	for _, f := range forms {
		if got := signal.Normalize(f); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", f, got, want)
		}
	}
}

func TestFromEmail(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"plain", "ops@acme-corp.com", "acme"},
		{"uppercase", "OPS@ACME-CORP.COM", "acme"},
		{"subdomain", "hr@careers.acme.com", "acme"},
		{"compound TLD", "info@acme.co.uk", "acme"},
		{"gov TLD", "contact@agency.gc.ca", "agency"},
		{"whitespace", "  ops@acme.com  ", "acme"},
		{"no at", "not-an-email", ""},
		{"empty", "", ""},
		{"trailing at", "ops@", ""},
		{"no TLD", "ops@acme", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := signal.FromEmail(tc.input)
			if got != tc.want {
				t.Errorf("FromEmail(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestFromURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"https", "https://acme-corp.com/path", "acme"},
		{"http with port", "http://acme.com:8080/path", "acme"},
		{"www prefix stripped", "https://www.acme.com", "acme"},
		{"subdomain", "https://blog.acme-corp.com/a/b", "acme"},
		{"uppercase host", "https://ACME.COM", "acme"},
		{"compound TLD", "https://example.co.uk/", "example"},
		{"gc.ca", "https://agency.gc.ca/page", "agency"},
		{"empty", "", ""},
		{"no scheme no host", "justtext", ""},
		{"path only", "/path/only", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := signal.FromURL(tc.input)
			if got != tc.want {
				t.Errorf("FromURL(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		explicit string
		email    string
		url      string
		want     string
		wantErr  bool
	}{
		{
			"explicit wins over email and url",
			"Acme Corp", "ops@other.com", "https://somewhere.com",
			"acme", false,
		},
		{
			"email falls back when explicit empty",
			"", "ops@acme-corp.com", "https://other.com",
			"acme", false,
		},
		{
			"url falls back when explicit and email empty",
			"", "", "https://acme.com/page",
			"acme", false,
		},
		{
			"url falls back when email malformed",
			"", "not-an-email", "https://acme.com",
			"acme", false,
		},
		{
			"whitespace-only explicit skips to email",
			"   ", "ops@acme.com", "",
			"acme", false,
		},
		{"all empty yields ErrNoOrganization", "", "", "", "", true},
		{
			"all malformed yields ErrNoOrganization",
			"   ", "not-an-email", "/relative/path",
			"", true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := signal.Resolve(tc.explicit, tc.email, tc.url)
			if tc.wantErr {
				if !errors.Is(err, signal.ErrNoOrganization) {
					t.Errorf("got err %v, want ErrNoOrganization", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("Resolve(%q, %q, %q) = %q, want %q",
					tc.explicit, tc.email, tc.url, got, tc.want)
			}
		})
	}
}
