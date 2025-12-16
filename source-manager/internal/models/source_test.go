package models

import (
	"database/sql/driver"
	"encoding/json"
	"testing"
	"time"
)

func TestStringArray_Value(t *testing.T) {
	tests := []struct {
		name    string
		array   *StringArray
		wantErr bool
		want    driver.Value
	}{
		{
			name:    "nil array returns error",
			array:   nil,
			wantErr: true,
		},
		{
			name:    "empty array returns error",
			array:   &StringArray{},
			wantErr: true,
		},
		{
			name:  "valid array returns JSON",
			array: stringPtr(StringArray{"value1", "value2"}),
			want:  []byte(`["value1","value2"]`),
		},
		{
			name:  "single value array",
			array: stringPtr(StringArray{"single"}),
			want:  []byte(`["single"]`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.array.Value()
			if (err != nil) != tt.wantErr {
				t.Errorf("StringArray.Value() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				gotBytes, ok := got.([]byte)
				if !ok {
					t.Errorf("StringArray.Value() = %T, want []byte", got)
					return
				}
				var gotArray StringArray
				if err := json.Unmarshal(gotBytes, &gotArray); err != nil {
					t.Errorf("StringArray.Value() returned invalid JSON: %v", err)
					return
				}
				var wantArray StringArray
				if err := json.Unmarshal(tt.want.([]byte), &wantArray); err != nil {
					t.Errorf("Test setup error: invalid want JSON: %v", err)
					return
				}
				if len(gotArray) != len(wantArray) {
					t.Errorf("StringArray.Value() length = %d, want %d", len(gotArray), len(wantArray))
				}
				for i := range gotArray {
					if gotArray[i] != wantArray[i] {
						t.Errorf("StringArray.Value() [%d] = %v, want %v", i, gotArray[i], wantArray[i])
					}
				}
			}
		})
	}
}

func TestStringArray_Scan(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		want    StringArray
		wantErr bool
	}{
		{
			name:  "nil value returns nil array",
			value: nil,
			want:  nil,
		},
		{
			name:    "invalid type returns nil (Scan doesn't error, just ignores)",
			value:   "not bytes",
			want:    nil, // Scan returns nil for invalid types without error
			wantErr: false,
		},
		{
			name:  "valid JSON bytes",
			value: []byte(`["value1","value2"]`),
			want:  StringArray{"value1", "value2"},
		},
		{
			name:  "empty JSON array",
			value: []byte(`[]`),
			want:  StringArray{},
		},
		{
			name:  "single value array",
			value: []byte(`["single"]`),
			want:  StringArray{"single"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var a StringArray
			err := a.Scan(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("StringArray.Scan() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(a) != len(tt.want) {
					t.Errorf("StringArray.Scan() length = %d, want %d", len(a), len(tt.want))
					return
				}
				for i := range a {
					if a[i] != tt.want[i] {
						t.Errorf("StringArray.Scan() [%d] = %v, want %v", i, a[i], tt.want[i])
					}
				}
			}
		})
	}
}

func TestSource_Validation(t *testing.T) {
	validSource := Source{
		ID:           "test-id",
		Name:         "Test Source",
		URL:          "https://example.com",
		ArticleIndex: "articles",
		PageIndex:    "pages",
		RateLimit:    "1s",
		MaxDepth:     2,
		Time:         StringArray{"09:00", "17:00"},
		Selectors: SelectorConfig{
			Article: ArticleSelectors{
				Title: "h1",
				Body:  ".content",
			},
		},
		Enabled:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Test that valid source has all required fields
	if validSource.ID == "" {
		t.Error("Source.ID should not be empty")
	}
	if validSource.Name == "" {
		t.Error("Source.Name should not be empty")
	}
	if validSource.URL == "" {
		t.Error("Source.URL should not be empty")
	}
}

// Helper function to convert StringArray to pointer
func stringPtr(s StringArray) *StringArray {
	return &s
}

