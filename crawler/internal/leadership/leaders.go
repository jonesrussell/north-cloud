package leadership

import (
	"regexp"
	"strings"
)

// Person represents an extracted leader/official.
type Person struct {
	Name  string `json:"name"`
	Role  string `json:"role"`
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"`
}

// Common role patterns in descending priority.
//
//nolint:gochecknoglobals // static lookup
var rolePatterns = []struct {
	pattern *regexp.Regexp
	role    string
}{
	{regexp.MustCompile(`(?i)\bdeputy\s+chief\b`), "deputy_chief"},
	{regexp.MustCompile(`(?i)\bchief\b`), "chief"},
	{regexp.MustCompile(`(?i)\bcouncill?ors?\b`), "councillor"},
	{regexp.MustCompile(`(?i)\bband\s+manager\b`), "band_manager"},
	{regexp.MustCompile(`(?i)\bexecutive\s+director\b`), "executive_director"},
	{regexp.MustCompile(`(?i)\bsecretary\b`), "secretary"},
	{regexp.MustCompile(`(?i)\btreasurer\b`), "treasurer"},
}

// ExtractLeaders parses leadership information from page text.
// It looks for name + role patterns in lines and structured blocks.
// After finding a leader name, it scans the next contactWindowLines lines
// for an email address or phone number to attach to that person.
func ExtractLeaders(text string) []Person {
	lines := strings.Split(text, "\n")
	var leaders []Person
	seen := make(map[string]bool)

	var currentRole string

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Check if this line defines a role heading
		if role := detectRole(trimmed); role != "" {
			currentRole = role
			// Check if the role heading also contains a name (e.g., "Chief John Smith")
			name := extractNameFromRoleLine(trimmed, role)
			if name != "" && !seen[name] {
				seen[name] = true
				email, phone := scanContactWindow(lines, i+1)
				leaders = append(leaders, Person{Name: name, Role: role, Email: email, Phone: phone})
			}
			continue
		}

		// If we have a current role context, this line might be a name
		if currentRole != "" && looksLikeName(trimmed) {
			name := cleanName(trimmed)
			if name != "" && !seen[name] {
				seen[name] = true
				email, phone := scanContactWindow(lines, i+1)
				leaders = append(leaders, Person{Name: name, Role: currentRole, Email: email, Phone: phone})
			}
		}
	}

	return leaders
}

// scanContactWindow scans up to contactWindowLines lines starting at startIdx
// for an email address and phone number. Returns empty strings if not found.
func scanContactWindow(lines []string, startIdx int) (email, phone string) {
	end := startIdx + contactWindowLines
	if end > len(lines) {
		end = len(lines)
	}
	for _, wline := range lines[startIdx:end] {
		if email == "" {
			if match := emailPattern.FindString(wline); match != "" {
				email = match
			}
		}
		if phone == "" {
			if match := phonePattern.FindString(wline); match != "" {
				phone = match
			}
		}
		if email != "" && phone != "" {
			break
		}
	}
	return email, phone
}

// detectRole checks if a line contains a role keyword.
func detectRole(line string) string {
	// Check deputy chief before chief (more specific first)
	for _, rp := range rolePatterns {
		if rp.pattern.MatchString(line) {
			return rp.role
		}
	}
	return ""
}

// extractNameFromRoleLine extracts a name from a line that also contains a role.
// e.g., "Chief John Smith" → "John Smith"
func extractNameFromRoleLine(line, role string) string {
	// Remove the role keyword and see what's left
	cleaned := line
	for _, rp := range rolePatterns {
		if rp.role == role {
			cleaned = rp.pattern.ReplaceAllString(cleaned, "")
			break
		}
	}

	cleaned = strings.TrimSpace(cleaned)
	cleaned = strings.Trim(cleaned, ":-–—")
	cleaned = strings.TrimSpace(cleaned)

	if looksLikeName(cleaned) {
		return cleanName(cleaned)
	}

	return ""
}

const (
	minNameLength      = 5
	maxNameLength      = 50
	contactWindowLines = 5
)

var namePattern = regexp.MustCompile(`^[A-Z][a-zA-Z'-]+(?:\s+[A-Z][a-zA-Z'-]+){1,4}$`)

// looksLikeName returns true if the text looks like a person's name.
func looksLikeName(text string) bool {
	trimmed := strings.TrimSpace(text)
	if len(trimmed) < minNameLength || len(trimmed) > maxNameLength {
		return false
	}

	return namePattern.MatchString(trimmed)
}

// cleanName normalizes a name string.
func cleanName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.Trim(name, "•·●‣▪-–—:,.")
	return strings.TrimSpace(name)
}
