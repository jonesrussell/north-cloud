package api

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/auth/internal/auth"
	"github.com/jonesrussell/north-cloud/auth/internal/config"
	infragin "github.com/north-cloud/infrastructure/gin"
	"github.com/north-cloud/infrastructure/logger"
)

const (
	// HTTP server timeout constants
	readTimeoutSeconds  = 10
	writeTimeoutSeconds = 30
	idleTimeoutSeconds  = 120

	// Service version
	serviceVersion = "1.0.0"
)

// NewServer creates a new HTTP server using the infrastructure gin package.
func NewServer(cfg *config.Config, log logger.Logger) (*infragin.Server, error) {
	// Create JWT manager
	jwtConfig := cfg.GetJWTConfig()
	jwtManager := auth.NewJWTManager(jwtConfig.Secret, jwtConfig.Expiration)

	// Create auth handler
	authHandler := NewAuthHandler(cfg, jwtManager, log)

	// Build server using infrastructure gin package
	server := infragin.NewServerBuilder(cfg.Service.Name, cfg.Service.Port).
		WithLogger(log).
		WithDebug(cfg.Service.Debug).
		WithVersion(serviceVersion).
		WithTimeouts(
			readTimeoutSeconds*time.Second,
			writeTimeoutSeconds*time.Second,
			idleTimeoutSeconds*time.Second,
		).
		WithRoutes(func(router *gin.Engine) {
			// Auth routes (no JWT protection - this IS the auth service)
			v1 := router.Group("/api/v1")
			authGroup := v1.Group("/auth")
			authGroup.POST("/login", authHandler.Login)
		}).
		Build()

	return server, nil
}
