package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
)

const (
	entityTypePerson     = "person"
	entityTypeBandOffice = "band_office"
)

// VerificationHandler provides HTTP handlers for the verification queue.
type VerificationHandler struct {
	repo   *repository.VerificationRepository
	logger infralogger.Logger
}

// NewVerificationHandler creates a new VerificationHandler.
func NewVerificationHandler(repo *repository.VerificationRepository, log infralogger.Logger) *VerificationHandler {
	return &VerificationHandler{
		repo:   repo,
		logger: log,
	}
}

// ListPending returns unverified records.
// GET /api/v1/verification/pending?type=person|band_office&limit=50&offset=0
func (h *VerificationHandler) ListPending(c *gin.Context) {
	entityType := c.Query("type")
	if entityType != "" && entityType != entityTypePerson && entityType != entityTypeBandOffice {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type must be 'person' or 'band_office'"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	items, total, err := h.repo.ListPending(c.Request.Context(), entityType, limit, offset)
	if err != nil {
		h.logger.Error("failed to list pending verification",
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list pending items"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
	})
}

// Verify marks a record as verified.
// POST /api/v1/verification/:id/verify?type=person|band_office
func (h *VerificationHandler) Verify(c *gin.Context) {
	h.dispatchAction(c, "verify", func(ctx context.Context, entityType, id string) error {
		if entityType == entityTypePerson {
			return h.repo.VerifyPerson(ctx, id)
		}
		return h.repo.VerifyBandOffice(ctx, id)
	})
}

// Reject removes an unverified record.
// POST /api/v1/verification/:id/reject?type=person|band_office
func (h *VerificationHandler) Reject(c *gin.Context) {
	h.dispatchAction(c, "reject", func(ctx context.Context, entityType, id string) error {
		if entityType == entityTypePerson {
			return h.repo.RejectPerson(ctx, id)
		}
		return h.repo.RejectBandOffice(ctx, id)
	})
}

const maxBulkIDs = 100

// GetStats returns aggregate counts for the verification queue.
// GET /api/v1/verification/stats
func (h *VerificationHandler) GetStats(c *gin.Context) {
	stats, err := h.repo.GetStats(c.Request.Context())
	if err != nil {
		h.logger.Error("failed to get verification stats", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get stats"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// bulkRequest is the request body for bulk verify/reject operations.
type bulkRequest struct {
	IDs  []string `json:"ids"`
	Type string   `json:"type"`
}

// bulkActionFn is a function that applies an action to a list of IDs.
type bulkActionFn func(ctx context.Context, ids []string) (int, error)

// BulkVerify marks multiple records as verified.
// POST /api/v1/verification/bulk-verify
func (h *VerificationHandler) BulkVerify(c *gin.Context) {
	h.dispatchBulkAction(c, "verify", h.repo.BulkVerifyPeople, h.repo.BulkVerifyBandOffices)
}

// BulkReject removes multiple unverified records.
// POST /api/v1/verification/bulk-reject
func (h *VerificationHandler) BulkReject(c *gin.Context) {
	h.dispatchBulkAction(c, "reject", h.repo.BulkRejectPeople, h.repo.BulkRejectBandOffices)
}

func (h *VerificationHandler) dispatchBulkAction(
	c *gin.Context,
	action string,
	peopleFn bulkActionFn,
	officeFn bulkActionFn,
) {
	var req bulkRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ids must not be empty"})
		return
	}
	if len(req.IDs) > maxBulkIDs {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("max %d ids per request", maxBulkIDs)})
		return
	}
	if req.Type != entityTypePerson && req.Type != entityTypeBandOffice {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type must be 'person' or 'band_office'"})
		return
	}

	fn := peopleFn
	if req.Type == entityTypeBandOffice {
		fn = officeFn
	}

	processed, err := fn(c.Request.Context(), req.IDs)
	if err != nil {
		h.logger.Error("failed to "+action+" bulk records",
			infralogger.String("type", req.Type),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"processed": processed, "action": action + "d"})
}

// dispatchAction validates the type param and runs the action, returning a uniform response.
func (h *VerificationHandler) dispatchAction(
	c *gin.Context,
	action string,
	fn func(ctx context.Context, entityType, id string) error,
) {
	id := c.Param("id")
	entityType := c.Query("type")

	if entityType != entityTypePerson && entityType != entityTypeBandOffice {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type query param required: 'person' or 'band_office'"})
		return
	}

	if err := fn(c.Request.Context(), entityType, id); err != nil {
		h.logger.Error("failed to "+action+" record",
			infralogger.String("id", id),
			infralogger.String("type", entityType),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": action + "ed", "id": id, "type": entityType})
}
