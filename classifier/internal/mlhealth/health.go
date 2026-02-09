// Package mlhealth provides a single implementation for ML sidecar health checks.
package mlhealth

import (
	"context"
	"fmt"

	"github.com/jonesrussell/north-cloud/classifier/internal/mltransport"
)

// Check calls GET /health at baseURL and returns reachable, latencyMs, model_version, and any error.
// The API handler builds MLServiceHealth from these values (plus LastChecked).
func Check(ctx context.Context, baseURL string) (reachable bool, latencyMs int64, modelVersion string, err error) {
	reachable, latencyMs, modelVersion, err = mltransport.DoHealth(ctx, baseURL)
	if err != nil {
		return reachable, latencyMs, modelVersion, fmt.Errorf("ml health check: %w", err)
	}
	return reachable, latencyMs, modelVersion, nil
}
