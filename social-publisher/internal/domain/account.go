package domain

import "time"

// Account represents a social media account. Credentials are stored separately and never loaded into this struct.
type Account struct {
	ID                    string     `db:"id"           json:"id"`
	Name                  string     `db:"name"         json:"name"`
	Platform              string     `db:"platform"     json:"platform"`
	Project               string     `db:"project"      json:"project"`
	Enabled               bool       `db:"enabled"      json:"enabled"`
	CredentialsConfigured bool       `db:"-"            json:"credentials_configured"`
	TokenExpiry           *time.Time `db:"token_expiry" json:"token_expiry,omitempty"`
	CreatedAt             time.Time  `db:"created_at"   json:"created_at"`
	UpdatedAt             time.Time  `db:"updated_at"   json:"updated_at"`
}

// CreateAccountRequest is the input for creating a new account.
type CreateAccountRequest struct {
	Name        string         `binding:"required"  json:"name"`
	Platform    string         `binding:"required"  json:"platform"`
	Project     string         `binding:"required"  json:"project"`
	Enabled     *bool          `json:"enabled"`
	Credentials map[string]any `json:"credentials"`
	TokenExpiry *time.Time     `json:"token_expiry"`
}

// UpdateAccountRequest is the input for updating an existing account.
type UpdateAccountRequest struct {
	Name        *string        `json:"name"`
	Platform    *string        `json:"platform"`
	Project     *string        `json:"project"`
	Enabled     *bool          `json:"enabled"`
	Credentials map[string]any `json:"credentials"`
	TokenExpiry *time.Time     `json:"token_expiry"`
}
