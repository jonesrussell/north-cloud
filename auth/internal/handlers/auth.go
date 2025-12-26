package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/auth/internal/config"
	"github.com/jonesrussell/auth/internal/logger"
	"github.com/jonesrussell/auth/internal/middleware"
	"github.com/jonesrussell/auth/internal/models"
	"github.com/jonesrussell/auth/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	userRepo *repository.UserRepository
	jwt      *middleware.JWTMiddleware
	config   *config.Config
	logger   logger.Logger
}

func NewAuthHandler(
	userRepo *repository.UserRepository,
	jwt *middleware.JWTMiddleware,
	cfg *config.Config,
	log logger.Logger,
) *AuthHandler {
	return &AuthHandler{
		userRepo: userRepo,
		jwt:      jwt,
		config:   cfg,
		logger:   log,
	}
}

// Login handles user authentication
func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Get user by username
	user, err := h.userRepo.GetByUsername(req.Username)
	if err != nil {
		h.logger.Warn("Login attempt failed - user not found",
			logger.String("username", req.Username),
		)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		h.logger.Warn("Login attempt failed - invalid password",
			logger.String("username", req.Username),
		)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	// Generate tokens
	accessToken, err := h.jwt.GenerateToken(user, h.config.JWT.Expiry)
	if err != nil {
		h.logger.Error("Failed to generate access token",
			logger.String("username", req.Username),
			logger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	refreshToken, err := h.jwt.GenerateToken(user, h.config.JWT.RefreshExpiry)
	if err != nil {
		h.logger.Error("Failed to generate refresh token",
			logger.String("username", req.Username),
			logger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	h.logger.Info("User logged in successfully",
		logger.String("username", req.Username),
	)

	c.JSON(http.StatusOK, models.LoginResponse{
		Token:        accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(h.config.JWT.Expiry),
		User: models.UserInfo{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
		},
	})
}

// Validate checks if a token is valid
func (h *AuthHandler) Validate(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusOK, models.ValidateResponse{Valid: false})
		return
	}

	// Extract token from "Bearer <token>"
	tokenString := ""
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		tokenString = authHeader[7:]
	}

	if tokenString == "" {
		c.JSON(http.StatusOK, models.ValidateResponse{Valid: false})
		return
	}

	claims, err := h.jwt.ParseToken(tokenString)
	if err != nil {
		c.JSON(http.StatusOK, models.ValidateResponse{Valid: false})
		return
	}

	// Get user info
	user, err := h.userRepo.GetByUsername(claims.Username)
	if err != nil {
		c.JSON(http.StatusOK, models.ValidateResponse{Valid: false})
		return
	}

	c.JSON(http.StatusOK, models.ValidateResponse{
		Valid: true,
		User: models.UserInfo{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
		},
	})
}

// Refresh generates a new access token from a refresh token
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req models.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	claims, err := h.jwt.ParseToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
		return
	}

	// Get user
	user, err := h.userRepo.GetByUsername(claims.Username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	// Generate new access token
	accessToken, err := h.jwt.GenerateToken(user, h.config.JWT.Expiry)
	if err != nil {
		h.logger.Error("Failed to generate access token",
			logger.String("username", user.Username),
			logger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, models.LoginResponse{
		Token:     accessToken,
		ExpiresAt: time.Now().Add(h.config.JWT.Expiry),
		User: models.UserInfo{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
		},
	})
}

// Logout is a placeholder - tokens are stateless, so logout is handled client-side
func (h *AuthHandler) Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "logged out successfully"})
}

