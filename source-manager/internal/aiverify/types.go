package aiverify

import "context"

// VerifyInput holds the record data to send to the LLM.
type VerifyInput struct {
	RecordType    string `json:"record_type"`
	Name          string `json:"name,omitempty"`
	Role          string `json:"role,omitempty"`
	Email         string `json:"email,omitempty"`
	Phone         string `json:"phone,omitempty"`
	CommunityName string `json:"community_name"`
	Province      string `json:"province,omitempty"`
	SourceURL     string `json:"source_url,omitempty"`
	AddressLine1  string `json:"address_line1,omitempty"`
	AddressLine2  string `json:"address_line2,omitempty"`
	City          string `json:"city,omitempty"`
	PostalCode    string `json:"postal_code,omitempty"`
	Fax           string `json:"fax,omitempty"`
	TollFree      string `json:"toll_free,omitempty"`
	OfficeHours   string `json:"office_hours,omitempty"`
}

// VerificationRecord holds a record fetched for verification.
type VerificationRecord struct {
	ID         string
	EntityType string
	Input      VerifyInput
}

// Repository defines the data-access methods needed by the AI verification worker.
type Repository interface {
	ListUnverifiedUnscoredPeople(ctx context.Context, limit int) ([]VerificationRecord, error)
	ListUnverifiedUnscoredBandOffices(ctx context.Context, limit int) ([]VerificationRecord, error)
	UpdatePersonVerificationResult(ctx context.Context, id string, confidence float64, issues string) error
	UpdateBandOfficeVerificationResult(ctx context.Context, id string, confidence float64, issues string) error
	AutoRejectPerson(ctx context.Context, id string) error
	AutoRejectBandOffice(ctx context.Context, id string) error
}
