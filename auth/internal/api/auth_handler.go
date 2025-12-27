package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/auth/internal/auth"
	"github.com/jonesrussell/north-cloud/auth/internal/config"
)

// AuthHandler handles authentication requests
type AuthHandler struct {
	config     *config.Config
	jwtManager *auth.JWTManager
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(cfg *config.Config, jwtManager *auth.JWTManager) *AuthHandler {
	return &AuthHandler{
		config:     cfg,
		jwtManager: jwtManager,
	}
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	Token string `json:"token"`
}

// Login handles login requests
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Validate credentials
	if req.Username != h.config.Username || req.Password != h.config.Password {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	// Generate JWT token
	token, err := h.jwtManager.GenerateToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{Token: token})
}
