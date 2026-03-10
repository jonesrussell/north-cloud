package leadership_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/leadership"
)

func TestExtractContact(t *testing.T) {
	t.Helper()

	text := `Wikwemikong Band Office
123 Main Street
Wikwemikong, ON P0P 2J0

Phone: (705) 859-3122
Toll Free: 1-800-555-1234
Fax: (705) 859-3456
Email: info@wikwemikong.ca`

	info := leadership.ExtractContact(text)

	if info.Phone != "(705) 859-3122" {
		t.Errorf("Phone = %q, want %q", info.Phone, "(705) 859-3122")
	}

	if info.TollFree != "1-800-555-1234" {
		t.Errorf("TollFree = %q, want %q", info.TollFree, "1-800-555-1234")
	}

	if info.Fax != "(705) 859-3456" {
		t.Errorf("Fax = %q, want %q", info.Fax, "(705) 859-3456")
	}

	if info.Email != "info@wikwemikong.ca" {
		t.Errorf("Email = %q, want %q", info.Email, "info@wikwemikong.ca")
	}

	if info.PostalCode != "P0P 2J0" {
		t.Errorf("PostalCode = %q, want %q", info.PostalCode, "P0P 2J0")
	}
}

func TestExtractContact_Minimal(t *testing.T) {
	t.Helper()

	info := leadership.ExtractContact("Call us at 705-555-1234 or email office@fn.ca")

	if info.Phone != "705-555-1234" {
		t.Errorf("Phone = %q, want %q", info.Phone, "705-555-1234")
	}

	if info.Email != "office@fn.ca" {
		t.Errorf("Email = %q, want %q", info.Email, "office@fn.ca")
	}
}
