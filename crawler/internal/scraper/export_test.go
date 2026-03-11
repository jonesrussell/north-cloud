package scraper

import "github.com/jonesrussell/north-cloud/crawler/internal/leadership"

// PersonKeyForTest exposes personKey for external tests.
func PersonKeyForTest(name, role string) string {
	return personKey(name, role)
}

// PtrEqualsForTest exposes ptrEquals for external tests.
func PtrEqualsForTest(ptr *string, val string) bool {
	return ptrEquals(ptr, val)
}

// BandOfficeUnchangedForTest exposes bandOfficeUnchanged for external tests.
func BandOfficeUnchangedForTest(
	existing *BandOffice, phone, email, fax, tollFree, postalCode string,
) bool {
	return bandOfficeUnchanged(existing, leadership.ContactInfo{
		Phone:      phone,
		Email:      email,
		Fax:        fax,
		TollFree:   tollFree,
		PostalCode: postalCode,
	})
}
