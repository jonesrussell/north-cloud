// crawler/internal/api/migration_handler.go
package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/crawler/internal/job"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// Default migration batch size.
const defaultMigrationBatchSize = 100

// MigratorInterface defines the interface for job migration operations.
type MigratorInterface interface {
	MigrateJobs(ctx context.Context, batchSize int) (*job.MigrationResult, error)
	GetMigrationStats(ctx context.Context) (map[string]int, error)
}

// MigrationHandler handles Phase 3 migration endpoints.
type MigrationHandler struct {
	migrator MigratorInterface
	log      infralogger.Logger
}

// NewMigrationHandler creates a new migration handler.
func NewMigrationHandler(migrator MigratorInterface, log infralogger.Logger) *MigrationHandler {
	return &MigrationHandler{
		migrator: migrator,
		log:      log,
	}
}

// RunMigration runs a batch of job migrations.
// POST /api/v1/jobs/migrate?batch_size=100
func (h *MigrationHandler) RunMigration(c *gin.Context) {
	batchSize := defaultMigrationBatchSize
	if batchStr := c.Query("batch_size"); batchStr != "" {
		parsed, parseErr := strconv.Atoi(batchStr)
		if parseErr == nil && parsed > 0 {
			batchSize = parsed
		}
	}

	result, err := h.migrator.MigrateJobs(c.Request.Context(), batchSize)
	if err != nil {
		h.logError("Migration failed", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "migration failed",
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetStats returns current migration statistics.
// GET /api/v1/jobs/migration-stats
func (h *MigrationHandler) GetStats(c *gin.Context) {
	stats, err := h.migrator.GetMigrationStats(c.Request.Context())
	if err != nil {
		h.logError("Failed to get migration stats", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get stats",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"migration_status": stats,
	})
}

// logError logs at error level if logger is not nil.
func (h *MigrationHandler) logError(msg string, fields ...infralogger.Field) {
	if h.log != nil {
		h.log.Error(msg, fields...)
	}
}
