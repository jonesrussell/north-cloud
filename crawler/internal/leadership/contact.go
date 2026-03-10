package leadership

import (
	"regexp"
	"strings"
)

// ContactInfo holds extracted contact details from a page.
type ContactInfo struct {
	Phone      string `json:"phone,omitempty"`
	TollFree   string `json:"toll_free,omitempty"`
	Fax        string `json:"fax,omitempty"`
	Email      string `json:"email,omitempty"`
	Address    string `json:"address,omitempty"`
	PostalCode string `json:"postal_code,omitempty"`
}

// Compiled regex patterns for contact extraction.
var (
	phonePattern      = regexp.MustCompile(`\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}`)
	tollFreePattern   = regexp.MustCompile(`1[-.\s]?8\d{2}[-.\s]?\d{3}[-.\s]?\d{4}`)
	emailPattern      = regexp.MustCompile(`[\w.+-]+@[\w.-]+\.\w{2,}`)
	postalCodePattern = regexp.MustCompile(`[A-Z]\d[A-Z]\s?\d[A-Z]\d`)
)

// ExtractContact parses contact details from page text.
func ExtractContact(text string) ContactInfo {
	info := ContactInfo{}

	// Extract email (most reliable)
	if match := emailPattern.FindString(text); match != "" {
		info.Email = match
	}

	// Extract postal code
	upper := strings.ToUpper(text)
	if match := postalCodePattern.FindString(upper); match != "" {
		info.PostalCode = match
	}

	// Extract toll-free first (before generic phone, since toll-free matches phone too)
	info.TollFree, info.Phone, info.Fax = extractPhoneNumbers(text)

	return info
}

// extractPhoneNumbers categorizes phone numbers into toll-free, main phone, and fax.
func extractPhoneNumbers(text string) (tollFree, phone, fax string) {
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		lower := strings.ToLower(line)

		// Check for fax first — a toll-free number on a "Fax:" line is a fax, not toll-free
		if fax == "" && strings.Contains(lower, "fax") {
			if match := phonePattern.FindString(line); match != "" {
				fax = match
				continue
			}
		}

		// Check for toll-free
		if tollFree == "" && tollFreePattern.MatchString(line) {
			tollFree = tollFreePattern.FindString(line)
			continue
		}

		// Regular phone
		if phone == "" {
			if match := phonePattern.FindString(line); match != "" {
				phone = match
			}
		}
	}

	return tollFree, phone, fax
}
