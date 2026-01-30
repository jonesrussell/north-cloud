// crawler/internal/api/migration_handler_test.go
package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/crawler/internal/api"
	"github.com/jonesrussell/north-cloud/crawler/internal/job"
)

// MockMigrator implements the migrator interface for testing.
type MockMigrator struct {
	MigrateResult *job.MigrationResult
	Stats         map[string]int
}

func (m *MockMigrator) MigrateJobs(_ context.Context, _ int) (*job.MigrationResult, error) {
	return m.MigrateResult, nil
}

func (m *MockMigrator) GetMigrationStats(_ context.Context) (map[string]int, error) {
	return m.Stats, nil
}

func TestMigrationHandler_RunMigration(t *testing.T) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	migrator := &MockMigrator{
		MigrateResult: &job.MigrationResult{
			Processed: 5,
			Migrated:  3,
			Orphaned:  2,
		},
	}

	handler := api.NewMigrationHandler(migrator, nil)
	router.POST("/api/v1/jobs/migrate", handler.RunMigration)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/migrate", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestMigrationHandler_GetStats(t *testing.T) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	migrator := &MockMigrator{
		Stats: map[string]int{
			"pending":  10,
			"migrated": 5,
			"orphaned": 2,
		},
	}

	handler := api.NewMigrationHandler(migrator, nil)
	router.GET("/api/v1/jobs/migration-stats", handler.GetStats)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/migration-stats", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}
