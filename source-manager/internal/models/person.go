package models

import "time"

// Person represents a community leader or official.
type Person struct {
	ID          string    `db:"id"           json:"id"`
	CommunityID string    `db:"community_id" json:"community_id"`
	Name        string    `db:"name"         json:"name"`
	Slug        string    `db:"slug"         json:"slug"`
	Role        string    `db:"role"         json:"role"`
	DataSource  string    `db:"data_source"  json:"data_source"`
	IsCurrent   bool      `db:"is_current"   json:"is_current"`
	Verified    bool      `db:"verified"     json:"verified"`
	CreatedAt   time.Time `db:"created_at"   json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"   json:"updated_at"`

	// Optional fields
	RoleTitle  *string    `db:"role_title"  json:"role_title,omitempty"`
	Email      *string    `db:"email"       json:"email,omitempty"`
	Phone      *string    `db:"phone"       json:"phone,omitempty"`
	TermStart  *time.Time `db:"term_start"  json:"term_start,omitempty"`
	TermEnd    *time.Time `db:"term_end"    json:"term_end,omitempty"`
	SourceURL  *string    `db:"source_url"  json:"source_url,omitempty"`
	VerifiedAt *time.Time `db:"verified_at" json:"verified_at,omitempty"`
}

// PersonHistory is an archived snapshot of a person's term.
type PersonHistory struct {
	ID          string     `db:"id"           json:"id"`
	PersonID    string     `db:"person_id"    json:"person_id"`
	CommunityID string     `db:"community_id" json:"community_id"`
	Name        string     `db:"name"         json:"name"`
	Role        string     `db:"role"         json:"role"`
	TermStart   *time.Time `db:"term_start"   json:"term_start,omitempty"`
	TermEnd     *time.Time `db:"term_end"     json:"term_end,omitempty"`
	DataSource  *string    `db:"data_source"  json:"data_source,omitempty"`
	SourceURL   *string    `db:"source_url"   json:"source_url,omitempty"`
	ArchivedAt  time.Time  `db:"archived_at"  json:"archived_at"`
}

// PersonFilter controls listing/counting queries for people.
type PersonFilter struct {
	CommunityID string
	Role        string
	CurrentOnly bool
	Limit       int
	Offset      int
}
