package importer

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func Test_parseTimeJSON(t *testing.T) {
	t.Helper()

	tests := []struct {
		name    string
		input   string
		wantLen int
		wantErr bool
	}{
		{"empty", "", 0, false},
		{"whitespace", "  ", 0, false},
		{"valid array", `["a","b"]`, 2, false},
		{"valid empty array", `[]`, 0, false},
		{"invalid json", "not json", 0, true},
		{"object not array", `{"k":"v"}`, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTimeJSON(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTimeJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) != tt.wantLen {
				t.Errorf("parseTimeJSON() length = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func Test_parseSelectorsJSON(t *testing.T) {
	t.Helper()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty", "", false},
		{"whitespace", "  ", false},
		{"valid object", `{"article":{"title":"h1"}}`, false},
		{"invalid json", "not json", true},
		{"array not object", `[]`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseSelectorsJSON(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseSelectorsJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_validateRequiredColumns(t *testing.T) {
	t.Helper()

	tests := []struct {
		name    string
		colMap  columnMap
		wantErr bool
		wantMsg string
	}{
		{
			name:    "both present",
			colMap:  columnMap{name: 0, url: 1},
			wantErr: false,
		},
		{
			name:    "missing both",
			colMap:  columnMap{name: -1, url: -1},
			wantErr: true,
			wantMsg: "missing required columns",
		},
		{
			name:    "missing name",
			colMap:  columnMap{name: -1, url: 1},
			wantErr: true,
			wantMsg: "missing required column",
		},
		{
			name:    "missing url",
			colMap:  columnMap{name: 0, url: -1},
			wantErr: true,
			wantMsg: "missing required column",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateRequiredColumns(tt.colMap)
			if (got != nil) != tt.wantErr {
				t.Errorf("validateRequiredColumns() = %v, wantErr %v", got, tt.wantErr)
				return
			}
			if got != nil && tt.wantMsg != "" && !strings.Contains(got.Error, tt.wantMsg) {
				t.Errorf("validateRequiredColumns() error = %q, want to contain %q", got.Error, tt.wantMsg)
			}
		})
	}
}

func Test_openExcelRows(t *testing.T) {
	t.Helper()

	t.Run("invalid reader", func(t *testing.T) {
		reader := bytes.NewReader([]byte("not excel"))
		rows, err := openExcelRows(reader)
		if err == nil {
			t.Error("openExcelRows() expected error for invalid input")
		}
		if rows != nil {
			t.Errorf("openExcelRows() expected nil rows on error, got len %d", len(rows))
		}
	})

	t.Run("empty sheet", func(t *testing.T) {
		f := excelize.NewFile()
		var buf bytes.Buffer
		if err := f.Write(&buf); err != nil {
			t.Fatalf("write excel: %v", err)
		}
		reader := bytes.NewReader(buf.Bytes())
		rows, err := openExcelRows(reader)
		if err != nil {
			t.Errorf("openExcelRows() unexpected error: %v", err)
		}
		if rows == nil || len(rows) != 0 {
			t.Errorf("openExcelRows() expected empty slice, got %v", rows)
		}
	})

	t.Run("valid one row", func(t *testing.T) {
		f := excelize.NewFile()
		_ = f.SetCellValue("Sheet1", "A1", "name")
		_ = f.SetCellValue("Sheet1", "B1", "url")
		var buf bytes.Buffer
		if err := f.Write(&buf); err != nil {
			t.Fatalf("write excel: %v", err)
		}
		reader := bytes.NewReader(buf.Bytes())
		rows, err := openExcelRows(reader)
		if err != nil {
			t.Errorf("openExcelRows() unexpected error: %v", err)
		}
		if len(rows) != 1 {
			t.Errorf("openExcelRows() got %d rows, want 1", len(rows))
		}
	})
}
