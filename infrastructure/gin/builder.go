package gin

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/north-cloud/infrastructure/jwt"
	"github.com/north-cloud/infrastructure/logger"
)

// ServerBuilder provides a fluent API for building HTTP servers.
type ServerBuilder struct {
	config       *Config
	logger       logger.Logger
	setupRoutes  func(*gin.Engine)
	healthChecks map[string]HealthChecker
	jwtSecret    string
}

// NewServerBuilder creates a new server builder with the given configuration.
func NewServerBuilder(serviceName string, port int) *ServerBuilder {
	return &ServerBuilder{
		config:       NewConfig(serviceName, port),
		healthChecks: make(map[string]HealthChecker),
	}
}

// WithConfig sets a custom configuration.
func (b *ServerBuilder) WithConfig(cfg *Config) *ServerBuilder {
	b.config = cfg
	return b
}

// WithLogger sets the logger.
func (b *ServerBuilder) WithLogger(log logger.Logger) *ServerBuilder {
	b.logger = log
	return b
}

// WithDebug enables or disables debug mode.
func (b *ServerBuilder) WithDebug(debug bool) *ServerBuilder {
	b.config.Debug = debug
	return b
}

// WithVersion sets the service version.
func (b *ServerBuilder) WithVersion(version string) *ServerBuilder {
	b.config.ServiceVersion = version
	return b
}

// WithCORS configures CORS settings.
func (b *ServerBuilder) WithCORS(cfg CORSConfig) *ServerBuilder {
	b.config.CORS = cfg
	return b
}

// WithCORSOrigins sets allowed CORS origins.
func (b *ServerBuilder) WithCORSOrigins(origins []string) *ServerBuilder {
	b.config.CORS.AllowedOrigins = origins
	return b
}

// WithTimeouts sets all timeout values for the HTTP server.
func (b *ServerBuilder) WithTimeouts(read, write, idle time.Duration) *ServerBuilder {
	b.config.ReadTimeout = read
	b.config.WriteTimeout = write
	b.config.IdleTimeout = idle
	return b
}

// WithJWTAuth enables JWT authentication for protected routes.
// The jwtSecret is used to validate JWT tokens.
func (b *ServerBuilder) WithJWTAuth(jwtSecret string) *ServerBuilder {
	b.jwtSecret = jwtSecret
	return b
}

// WithHealthCheck adds a named health check.
func (b *ServerBuilder) WithHealthCheck(name string, checker HealthChecker) *ServerBuilder {
	b.healthChecks[name] = checker
	return b
}

// WithDatabaseHealthCheck adds a database health check.
func (b *ServerBuilder) WithDatabaseHealthCheck(pingFunc func() error) *ServerBuilder {
	b.healthChecks["database"] = DatabaseHealthChecker(pingFunc)
	return b
}

// WithRedisHealthCheck adds a Redis health check.
func (b *ServerBuilder) WithRedisHealthCheck(pingFunc func() error) *ServerBuilder {
	b.healthChecks["redis"] = RedisHealthChecker(pingFunc)
	return b
}

// WithElasticsearchHealthCheck adds an Elasticsearch health check.
func (b *ServerBuilder) WithElasticsearchHealthCheck(pingFunc func() error) *ServerBuilder {
	b.healthChecks["elasticsearch"] = ElasticsearchHealthChecker(pingFunc)
	return b
}

// WithRoutes sets the route setup function.
func (b *ServerBuilder) WithRoutes(setupRoutes func(*gin.Engine)) *ServerBuilder {
	b.setupRoutes = setupRoutes
	return b
}

// Build creates the server with all configured options.
func (b *ServerBuilder) Build() *Server {
	// Ensure we have a logger
	if b.logger == nil {
		b.logger = logger.Must(logger.Config{
			Level:       "info",
			Development: b.config.Debug,
		})
	}

	// Create wrapper that adds health routes and JWT middleware
	wrappedSetup := func(router *gin.Engine) {
		// Register health routes with checks if any
		if len(b.healthChecks) > 0 {
			RegisterHealthRoutesWithChecks(router, HealthOptions{
				ServiceName:    b.config.ServiceName,
				ServiceVersion: b.config.ServiceVersion,
				Checks:         b.healthChecks,
			})
		} else {
			RegisterHealthRoutes(router, b.config.ServiceName, b.config.ServiceVersion)
		}

		// Call service-specific route setup
		if b.setupRoutes != nil {
			b.setupRoutes(router)
		}
	}

	return NewServer(b.config, b.logger, wrappedSetup)
}

// ProtectedGroup creates a router group with JWT authentication middleware.
// Use this for routes that require authentication.
func ProtectedGroup(router *gin.Engine, path, jwtSecret string) *gin.RouterGroup {
	group := router.Group(path)
	if jwtSecret != "" {
		group.Use(jwt.Middleware(jwtSecret))
	}
	return group
}

// PublicGroup creates a router group without authentication.
// Use this for public routes like health checks or public APIs.
func PublicGroup(router *gin.Engine, path string) *gin.RouterGroup {
	return router.Group(path)
}

// SetupAPIRoutes is a helper that sets up a standard API structure with:
// - Public health endpoints at /health
// - Protected API v1 endpoints at /api/v1 (with optional JWT auth)
// Returns the protected v1 group for adding routes.
func SetupAPIRoutes(router *gin.Engine, serviceName, version, jwtSecret string) *gin.RouterGroup {
	// Health routes are already added by the server builder
	// Just create and return the protected API group
	return ProtectedGroup(router, "/api/v1", jwtSecret)
}

// SetupAPIRoutesWithPublic sets up API structure with both public and protected groups.
// Returns (publicGroup, protectedGroup) for adding routes.
func SetupAPIRoutesWithPublic(router *gin.Engine, jwtSecret string) (publicGroup, protectedGroup *gin.RouterGroup) {
	publicGroup = PublicGroup(router, "/api/v1")
	protectedGroup = ProtectedGroup(router, "/api/v1", jwtSecret)
	return publicGroup, protectedGroup
}
