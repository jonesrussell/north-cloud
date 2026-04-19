package signal

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

// ErrNoOrganization is returned by Resolve when none of explicit / email /
// source URL yield a non-empty normalized organization name. Per the
// lead-pipeline spec (docs/specs/lead-pipeline.md §Organization attribution),
// producers MUST fail the signal with a structured log rather than write an
// empty string; callers should treat this error as a producer bug.
var ErrNoOrganization = errors.New("signal: no organization name could be derived from explicit/email/url")

// emailRegexGroups is the expected FindStringSubmatch length (match + domain).
const emailRegexGroups = 2

// corporateTokens are hyphen-separated trailing stems that do not add
// identity — stripped iteratively after normalization so that
// "acme corporation", "acme corp", and "acme-corp.com" collapse to "acme".
// Deliberately excludes "holdings", "group", and country/industry suffixes
// that DO carry identity (e.g. "Acme Holdings" is a distinct entity from
// "Acme Corp" downstream).
var corporateTokens = map[string]struct{}{
	"corporation":  {},
	"corp":         {},
	"inc":          {},
	"incorporated": {},
	"llc":          {},
	"ltd":          {},
	"limited":      {},
	"company":      {},
	"co":           {},
	"plc":          {},
	"sa":           {},
	"ag":           {},
	"gmbh":         {},
}

var (
	nonAlnumRun = regexp.MustCompile(`[^a-z0-9]+`)
	emailRe     = regexp.MustCompile(`^[^@\s]+@([^\s@]+\.[a-zA-Z]{2,})$`)
)

// Normalize returns a stable canonical form of an organization name.
// "Acme Corporation", "Acme Corp", and "acme-corp.com" all map to "acme".
// The result is lowercase and hyphen-separated, with corporate-structure
// suffix tokens stripped. Empty or whitespace-only input returns "".
func Normalize(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	if s == "" {
		return ""
	}
	s = stripTLD(s)
	s = nonAlnumRun.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return stripCorporateTokens(s)
}

// FromEmail returns the normalized organization derived from an email
// address's apex domain. "ops@acme-corp.com" → "acme". Returns "" if the
// email is malformed (no @, no TLD).
func FromEmail(email string) string {
	m := emailRe.FindStringSubmatch(strings.TrimSpace(email))
	if len(m) != emailRegexGroups {
		return ""
	}
	return apexLabel(m[1])
}

// FromURL returns the normalized organization derived from a URL's apex
// domain label. "https://blog.acme-corp.com/a/b" → "acme". Returns "" if
// the URL has no host.
func FromURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return ""
	}
	host := strings.TrimPrefix(strings.ToLower(u.Host), "www.")
	if i := strings.Index(host, ":"); i >= 0 {
		host = host[:i]
	}
	return apexLabel(host)
}

// Resolve implements the attribution fallback chain from the lead-pipeline
// spec: explicit → email → source URL. Returns ErrNoOrganization if every
// stage produces an empty string — the signal is a producer bug.
func Resolve(explicit, email, sourceURL string) (string, error) {
	if n := Normalize(explicit); n != "" {
		return n, nil
	}
	if n := FromEmail(email); n != "" {
		return n, nil
	}
	if n := FromURL(sourceURL); n != "" {
		return n, nil
	}
	return "", ErrNoOrganization
}

// apexLabel picks the label immediately to the left of the public suffix
// and normalizes it. "blog.acme-corp.co.uk" → "acme". Good enough for
// dedup; not an authoritative public-suffix implementation.
func apexLabel(host string) string {
	host = strings.TrimSuffix(host, ".")
	parts := strings.Split(host, ".")
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return Normalize(parts[0])
	}
	last := parts[len(parts)-1]
	secondLast := parts[len(parts)-2]
	if len(parts) >= 3 && isCompoundTLD(secondLast, last) {
		return Normalize(parts[len(parts)-3])
	}
	return Normalize(secondLast)
}

// isCompoundTLD returns true for well-known second-level public suffixes
// where the meaningful org label sits one additional level deeper
// (e.g. ".co.uk", ".com.au", ".gc.ca").
func isCompoundTLD(second, top string) bool {
	switch top {
	case "uk":
		return second == "co" || second == "org" || second == "gov" || second == "ac"
	case "au", "nz", "br", "mx":
		return second == "com" || second == "org" || second == "gov"
	case "ca":
		return second == "gc" || second == "on" || second == "qc" || second == "bc" || second == "ab"
	case "jp":
		return second == "co" || second == "or" || second == "go"
	}
	return false
}

func stripTLD(s string) string {
	i := strings.LastIndex(s, ".")
	if i <= 0 {
		return s
	}
	if isTLDish(s[i+1:]) {
		return s[:i]
	}
	return s
}

func stripCorporateTokens(s string) string {
	for {
		lastHyphen := strings.LastIndex(s, "-")
		if lastHyphen < 0 {
			return s
		}
		if _, ok := corporateTokens[s[lastHyphen+1:]]; !ok {
			return s
		}
		s = s[:lastHyphen]
	}
}

func isTLDish(s string) bool {
	if len(s) < 2 || len(s) > 4 {
		return false
	}
	for _, r := range s {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}
