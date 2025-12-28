package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/publisher/internal/models"
)

// listRoutes returns all routes with joined source/channel details
// GET /api/v1/routes?enabled_only=true
func (r *Router) listRoutes(c *gin.Context) {
	ctx := c.Request.Context()

	const queryTrue = "true"
	// Parse query parameters
	enabledOnly := c.Query("enabled_only") == queryTrue

	routes, err := r.repo.ListRoutesWithDetails(ctx, enabledOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list routes",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"routes": routes,
		"count":  len(routes),
	})
}

// createRoute creates a new route
// POST /api/v1/routes
func (r *Router) createRoute(c *gin.Context) {
	ctx := c.Request.Context()

	var req models.RouteCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request payload",
			"details": err.Error(),
		})
		return
	}

	// Validate request
	if err := req.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	route, err := r.repo.CreateRoute(ctx, &req)
	if err != nil {
		if errors.Is(err, models.ErrDuplicateRoute) {
			c.JSON(http.StatusConflict, gin.H{
				"error": "Route with this source and channel already exists",
			})
			return
		}
		if err.Error() == "source or channel not found" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid source_id or channel_id",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create route",
		})
		return
	}

	// Fetch the created route with details
	routeDetails, err := r.repo.GetRouteWithDetails(ctx, route.ID)
	if err != nil {
		// Route was created but failed to fetch details, return basic route
		c.JSON(http.StatusCreated, route)
		return
	}

	c.JSON(http.StatusCreated, routeDetails)
}

// getRoute retrieves a route by ID with details
// GET /api/v1/routes/:id
func (r *Router) getRoute(c *gin.Context) {
	ctx := c.Request.Context()

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid route ID format",
		})
		return
	}

	route, err := r.repo.GetRouteWithDetails(ctx, id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Route not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get route",
		})
		return
	}

	c.JSON(http.StatusOK, route)
}

// updateRoute updates a route
// PUT /api/v1/routes/:id
func (r *Router) updateRoute(c *gin.Context) {
	ctx := c.Request.Context()

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid route ID format",
		})
		return
	}

	var req models.RouteUpdateRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request payload",
			"details": bindErr.Error(),
		})
		return
	}

	// Validate request
	if validateErr := req.Validate(); validateErr != nil {
		if errors.Is(validateErr, models.ErrNoFieldsToUpdate) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "At least one field must be provided for update",
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error": validateErr.Error(),
		})
		return
	}

	route, err := r.repo.UpdateRoute(ctx, id, &req)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Route not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update route",
		})
		return
	}

	// Fetch updated route with details
	routeDetails, err := r.repo.GetRouteWithDetails(ctx, route.ID)
	if err != nil {
		// Route was updated but failed to fetch details, return basic route
		c.JSON(http.StatusOK, route)
		return
	}

	c.JSON(http.StatusOK, routeDetails)
}

// deleteRoute deletes a route
// DELETE /api/v1/routes/:id
func (r *Router) deleteRoute(c *gin.Context) {
	ctx := c.Request.Context()

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid route ID format",
		})
		return
	}

	err = r.repo.DeleteRoute(ctx, id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Route not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete route",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Route deleted successfully",
	})
}
