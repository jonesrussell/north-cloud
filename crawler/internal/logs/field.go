package logs

import "time"

// Field represents a structured log field.
type Field struct {
	Key   string
	Value any
}

// String creates a string field.
func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

// Int creates an int field.
func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

// Int64 creates an int64 field.
func Int64(key string, value int64) Field {
	return Field{Key: key, Value: value}
}

// Float64 creates a float64 field.
func Float64(key string, value float64) Field {
	return Field{Key: key, Value: value}
}

// Bool creates a bool field.
func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

// Duration creates a duration field (stored as milliseconds).
func Duration(key string, d time.Duration) Field {
	return Field{Key: key + "_ms", Value: d.Milliseconds()}
}

// URL creates a URL field.
func URL(url string) Field {
	return Field{Key: "url", Value: url}
}

// Err creates an error field.
func Err(err error) Field {
	if err == nil {
		return Field{Key: "error", Value: ""}
	}
	return Field{Key: "error", Value: err.Error()}
}

// Any creates a field with any value.
func Any(key string, value any) Field {
	return Field{Key: key, Value: value}
}

// FieldsToMap converts a slice of fields to a map.
func FieldsToMap(fields []Field) map[string]any {
	if len(fields) == 0 {
		return nil
	}
	m := make(map[string]any, len(fields))
	for _, f := range fields {
		m[f.Key] = f.Value
	}
	return m
}
