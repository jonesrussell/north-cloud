package logs_test

import (
	"errors"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
)

func assertField(t *testing.T, f logs.Field, wantKey string, wantValue any) {
	t.Helper()
	if f.Key != wantKey || f.Value != wantValue {
		t.Errorf("Field = {%q, %v}, want {%s, %v}", f.Key, f.Value, wantKey, wantValue)
	}
}

func TestStringField(t *testing.T) {
	f := logs.String("key", "value")
	assertField(t, f, "key", "value")
}

func TestIntField(t *testing.T) {
	f := logs.Int("count", 42)
	assertField(t, f, "count", 42)
}

func TestInt64Field(t *testing.T) {
	f := logs.Int64("big", int64(1234567890))
	assertField(t, f, "big", int64(1234567890))
}

func TestDurationField(t *testing.T) {
	f := logs.Duration("elapsed", 1500*time.Millisecond)
	assertField(t, f, "elapsed_ms", int64(1500))
}

func TestURLField(t *testing.T) {
	f := logs.URL("https://example.com")
	assertField(t, f, "url", "https://example.com")
}

func TestErrField(t *testing.T) {
	err := errors.New("something failed")
	f := logs.Err(err)
	assertField(t, f, "error", "something failed")
}

func TestErrFieldNil(t *testing.T) {
	f := logs.Err(nil)
	assertField(t, f, "error", "")
}

func TestBoolField(t *testing.T) {
	f := logs.Bool("enabled", true)
	assertField(t, f, "enabled", true)
}

func TestFieldsToMap(t *testing.T) {
	t.Helper()

	fields := []logs.Field{
		logs.String("name", "test"),
		logs.Int("count", 5),
		logs.URL("https://example.com"),
	}

	m := logs.FieldsToMap(fields)

	if m["name"] != "test" {
		t.Errorf("m[name] = %v, want test", m["name"])
	}
	if m["count"] != 5 {
		t.Errorf("m[count] = %v, want 5", m["count"])
	}
	if m["url"] != "https://example.com" {
		t.Errorf("m[url] = %v, want https://example.com", m["url"])
	}
}
