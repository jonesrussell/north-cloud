// Package mlhealth provides a single implementation for ML sidecar health checks.
package mlhealth

import (
	"context"
	"fmt"

	"github.com/jonesrussell/north-cloud/classifier/internal/mlclient"
)

// Check calls GET /v1/health at the ML sidecar and returns reachable, latencyMs, model_version, and any error.
// The API handler builds MLServiceHealth from these values (plus LastChecked).
func Check(ctx context.Context, baseURL string) (reachable bool, latencyMs int64, modelVersion string, err error) {
	client := mlclient.NewClient("health-check", baseURL)
	health, healthErr := client.Health(ctx)
	if healthErr != nil {
		return false, 0, "", fmt.Errorf("ml health check: %w", healthErr)
	}
	return true, 0, health.Version, nil
}
