package handlers

import (
	"context"
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
