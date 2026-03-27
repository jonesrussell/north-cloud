package models

import (
	"time"

	"github.com/google/uuid"
)

// ClaudrielLead is a row served as JSON for Claudriel's NorthCloudLeadNormalizer.
type ClaudrielLead struct {
	ID           uuid.UUID `db:"id"`
	Title        string    `db:"title"`
	Description  string    `db:"description"`
	ContactName  string    `db:"contact_name"`
	ContactEmail string    `db:"contact_email"`
	URL          string    `db:"url"`
	ClosingDate  string    `db:"closing_date"`
	Budget       string    `db:"budget"`
	Sector       string    `db:"sector"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}
