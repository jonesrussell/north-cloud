package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// JSONBMap is a custom type for handling JSONB data in PostgreSQL.
// It implements sql.Scanner and driver.Valuer interfaces to seamlessly
// convert between Go's map[string]any and PostgreSQL's JSONB type.
type JSONBMap map[string]any

// Scan implements the sql.Scanner interface.
// It handles scanning JSONB data from the database into a map[string]any.
func (j *JSONBMap) Scan(value any) error {
	if value == nil {
		*j = nil
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		return errors.New("unsupported type for JSONBMap")
	}

	if len(data) == 0 {
		*j = JSONBMap{}
		return nil
	}

	return json.Unmarshal(data, j)
}

// Value implements the driver.Valuer interface.
// It converts the map to JSON bytes for storage in the database.
func (j *JSONBMap) Value() (driver.Value, error) {
	if j == nil || len(*j) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(j)
}
