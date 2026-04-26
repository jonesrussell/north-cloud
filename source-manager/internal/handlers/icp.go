package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/source-manager/internal/icpstore"
)

type ICPHandler struct {
	store *icpstore.Store
}

func NewICPHandler(store *icpstore.Store) *ICPHandler {
	return &ICPHandler{store: store}
}

func (h *ICPHandler) GetSegments(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ICP seed store unavailable"})
		return
	}
	seed := h.store.Current()
	if seed == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ICP seed unavailable"})
		return
	}
	c.JSON(http.StatusOK, seed)
}
