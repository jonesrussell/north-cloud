package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
)

const (
	defaultPeopleLimit = 50
	maxPeopleLimit     = 200
	trueString         = "true"
)

// PersonHandler handles HTTP requests for the people API.
type PersonHandler struct {
	repo   *repository.PersonRepository
	logger infralogger.Logger
}

// NewPersonHandler creates a new PersonHandler.
func NewPersonHandler(repo *repository.PersonRepository, log infralogger.Logger) *PersonHandler {
	return &PersonHandler{
		repo:   repo,
		logger: log,
	}
}

// ListByCommunity returns people for a community.
func (h *PersonHandler) ListByCommunity(c *gin.Context) {
	communityID := c.Param("id")
	currentOnly := c.DefaultQuery("current_only", trueString) == trueString

	filter := models.PersonFilter{
		CommunityID: communityID,
		Role:        c.Query("role"),
		CurrentOnly: currentOnly,
		Limit:       parseIntQuery(c, "limit", defaultPeopleLimit),
		Offset:      parseIntQuery(c, "offset", 0),
	}
	if filter.Limit > maxPeopleLimit {
		filter.Limit = maxPeopleLimit
	}

	people, err := h.repo.ListByCommunity(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("Failed to list people",
			infralogger.String("community_id", communityID),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list people"})
		return
	}

	total, countErr := h.repo.Count(c.Request.Context(), filter)
	if countErr != nil {
		h.logger.Error("Failed to count people", infralogger.Error(countErr))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count people"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"people": people,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

// GetByID returns a single person by ID.
func (h *PersonHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	person, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		h.logger.Debug("Person not found", infralogger.String("id", id))
		c.JSON(http.StatusNotFound, gin.H{"error": "Person not found"})
		return
	}

	c.JSON(http.StatusOK, person)
}

// Create adds a new person to a community.
func (h *PersonHandler) Create(c *gin.Context) {
	communityID := c.Param("id")

	var person models.Person
	if err := c.ShouldBindJSON(&person); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}
	person.CommunityID = communityID

	if err := h.repo.Create(c.Request.Context(), &person); err != nil {
		h.logger.Error("Failed to create person", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create person"})
		return
	}

	c.JSON(http.StatusCreated, person)
}

// Update modifies an existing person.
func (h *PersonHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var person models.Person
	if err := c.ShouldBindJSON(&person); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}
	person.ID = id

	if err := h.repo.Update(c.Request.Context(), &person); err != nil {
		h.logger.Error("Failed to update person",
			infralogger.String("id", id),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update person"})
		return
	}

	c.JSON(http.StatusOK, person)
}

// Delete archives a person and removes them.
func (h *PersonHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	// Archive the term before deleting
	if archiveErr := h.repo.ArchiveTerm(c.Request.Context(), id); archiveErr != nil {
		h.logger.Warn("Failed to archive person term before delete",
			infralogger.String("id", id),
			infralogger.Error(archiveErr),
		)
	}

	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		h.logger.Error("Failed to delete person",
			infralogger.String("id", id),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete person"})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}
