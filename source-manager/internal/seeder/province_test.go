package seeder_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/source-manager/internal/seeder"
)

func TestInferProvince(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		lat      float64
		lng      float64
		expected string
	}{
		{"Toronto ON", 43.65, -79.38, "ON"},
		{"Montreal QC", 45.50, -73.57, "QC"},
		{"Vancouver BC", 49.28, -123.12, "BC"},
		{"Winnipeg MB", 49.90, -97.14, "MB"},
		{"Saskatoon SK", 52.13, -106.67, "SK"},
		{"Edmonton AB", 53.54, -113.49, "AB"},
		{"Halifax NS", 44.65, -63.57, "NS"},
		{"Whitehorse YT", 60.72, -135.05, "YT"},
		{"Yellowknife NT", 62.45, -114.37, "NT"},
		{"Iqaluit NU", 63.75, -68.52, "NU"},
		{"out of range", 10.0, -50.0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := seeder.InferProvince(tt.lat, tt.lng)
			if got != tt.expected {
				t.Errorf("InferProvince(%f, %f) = %q, want %q", tt.lat, tt.lng, got, tt.expected)
			}
		})
	}
}
