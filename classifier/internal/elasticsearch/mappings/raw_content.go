package mappings

import "github.com/jonesrussell/north-cloud/infrastructure/esmapping"

// RawContentMapping wraps the shared SSoT mapping for tooling and tests.
type RawContentMapping struct {
	doc map[string]any
}

// NewRawContentMapping builds a raw_content index mapping with default shard/replica counts.
func NewRawContentMapping() *RawContentMapping {
	return &RawContentMapping{doc: esmapping.RawContentIndex(1, 1)}
}

// GetJSON returns the raw content mapping as a JSON string.
func (m *RawContentMapping) GetJSON() (string, error) {
	return esmapping.ToIndentedJSON(m.doc)
}

// Validate validates the raw content mapping configuration.
func (m *RawContentMapping) Validate() error {
	return ValidateSettings(DefaultSettings())
}
