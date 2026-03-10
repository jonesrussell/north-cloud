package models

import "time"

// BandOffice represents the physical office for a community (1:1 relationship).
type BandOffice struct {
	ID          string    `db:"id"           json:"id"`
	CommunityID string    `db:"community_id" json:"community_id"`
	DataSource  string    `db:"data_source"  json:"data_source"`
	Verified    bool      `db:"verified"     json:"verified"`
	CreatedAt   time.Time `db:"created_at"   json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"   json:"updated_at"`

	// Address
	AddressLine1 *string `db:"address_line1" json:"address_line1,omitempty"`
	AddressLine2 *string `db:"address_line2" json:"address_line2,omitempty"`
	City         *string `db:"city"          json:"city,omitempty"`
	Province     *string `db:"province"      json:"province,omitempty"`
	PostalCode   *string `db:"postal_code"   json:"postal_code,omitempty"`

	// Contact
	Phone    *string `db:"phone"     json:"phone,omitempty"`
	Fax      *string `db:"fax"       json:"fax,omitempty"`
	Email    *string `db:"email"     json:"email,omitempty"`
	TollFree *string `db:"toll_free" json:"toll_free,omitempty"`

	// Hours
	OfficeHours *string `db:"office_hours" json:"office_hours,omitempty"`

	// Provenance
	SourceURL  *string    `db:"source_url"  json:"source_url,omitempty"`
	VerifiedAt *time.Time `db:"verified_at" json:"verified_at,omitempty"`
}
