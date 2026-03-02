package api

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/jonesrussell/north-cloud/social-publisher/internal/crypto"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/database"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/domain"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// AccountsHandler implements account management endpoints.
type AccountsHandler struct {
	repo          *database.Repository
	encryptionKey string
	log           infralogger.Logger
}

// NewAccountsHandler creates a new accounts handler.
func NewAccountsHandler(repo *database.Repository, encryptionKey string, log infralogger.Logger) *AccountsHandler {
	return &AccountsHandler{repo: repo, encryptionKey: encryptionKey, log: log}
}

// List returns all configured accounts with credentials masked.
func (h *AccountsHandler) List(c *gin.Context) {
	accounts, err := h.repo.ListAccounts(c.Request.Context())
	if err != nil {
		h.log.Error("Failed to list accounts", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list accounts"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": accounts,
		"count": len(accounts),
	})
}

// Get returns a single account by ID.
func (h *AccountsHandler) Get(c *gin.Context) {
	id := c.Param("id")
	account, err := h.repo.GetAccountByID(c.Request.Context(), id)
	if err != nil {
		h.log.Error("Failed to get account", infralogger.Error(err), infralogger.String("account_id", id))
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}

	c.JSON(http.StatusOK, account)
}

// Create adds a new social media account.
func (h *AccountsHandler) Create(c *gin.Context) {
	var req domain.CreateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	var encryptedCreds []byte
	if req.Credentials != nil {
		credsJSON, marshalErr := json.Marshal(req.Credentials)
		if marshalErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid credentials format"})
			return
		}
		var encErr error
		encryptedCreds, encErr = crypto.Encrypt(credsJSON, h.encryptionKey)
		if encErr != nil {
			h.log.Error("Failed to encrypt credentials", infralogger.Error(encErr))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encrypt credentials"})
			return
		}
	}

	id := uuid.New().String()
	if err := h.repo.CreateAccount(
		c.Request.Context(), id, req.Name, req.Platform, req.Project,
		enabled, encryptedCreds, req.TokenExpiry,
	); err != nil {
		h.log.Error("Failed to create account", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create account"})
		return
	}

	account, err := h.repo.GetAccountByID(c.Request.Context(), id)
	if err != nil {
		h.log.Error("Failed to retrieve created account", infralogger.Error(err))
		c.JSON(http.StatusCreated, gin.H{"id": id})
		return
	}

	c.JSON(http.StatusCreated, account)
}

// Update modifies an existing account.
func (h *AccountsHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req domain.UpdateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var encryptedCreds []byte
	if req.Credentials != nil {
		credsJSON, marshalErr := json.Marshal(req.Credentials)
		if marshalErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid credentials format"})
			return
		}
		var encErr error
		encryptedCreds, encErr = crypto.Encrypt(credsJSON, h.encryptionKey)
		if encErr != nil {
			h.log.Error("Failed to encrypt credentials", infralogger.Error(encErr))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encrypt credentials"})
			return
		}
	}

	if err := h.repo.UpdateAccount(
		c.Request.Context(), id,
		req.Name, req.Platform, req.Project, req.Enabled,
		encryptedCreds, req.TokenExpiry,
	); err != nil {
		h.log.Error("Failed to update account", infralogger.Error(err), infralogger.String("account_id", id))
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}

	account, err := h.repo.GetAccountByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"id": id, "status": "updated"})
		return
	}

	c.JSON(http.StatusOK, account)
}

// Delete removes an account by ID.
func (h *AccountsHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.repo.DeleteAccount(c.Request.Context(), id); err != nil {
		h.log.Error("Failed to delete account", infralogger.Error(err), infralogger.String("account_id", id))
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}

	c.Status(http.StatusNoContent)
}
