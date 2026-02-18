package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/auth/internal/auth"
	"github.com/jonesrussell/north-cloud/auth/internal/config"
	"github.com/north-cloud/infrastructure/logger"
)

// AuthHandler handles authentication requests.
type AuthHandler struct {
	config     *config.Config
	jwtManager *auth.JWTManager
	log        logger.Logger
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(cfg *config.Config, jwtManager *auth.JWTManager, log logger.Logger) *AuthHandler {
	return &AuthHandler{
		config:     cfg,
		jwtManager: jwtManager,
		log:        log,
	}
}

// LoginRequest represents a login request.
type LoginRequest struct {
	Username string `binding:"required" json:"username"`
	Password string `binding:"required" json:"password"` //nolint:gosec // G117: login request body field required for API
}

// LoginResponse represents a login response.
type LoginResponse struct {
	Token string `json:"token"`
}

// Login handles login requests.
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Debug("Invalid login request", logger.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Validate credentials
	if req.Username != h.config.Auth.Username || req.Password != h.config.Auth.Password {
		h.log.Info("Failed login attempt",
			logger.String("username", req.Username),
			logger.String("client_ip", c.ClientIP()),
		)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	// Generate JWT token
	token, err := h.jwtManager.GenerateToken()
	if err != nil {
		h.log.Error("Failed to generate token", logger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	h.log.Info("Successful login",
		logger.String("username", req.Username),
		logger.String("client_ip", c.ClientIP()),
	)
	c.JSON(http.StatusOK, LoginResponse{Token: token})
}
