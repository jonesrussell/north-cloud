package gin

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/north-cloud/infrastructure/monitoring"
)

// HealthStatus represents the status of a health check.
type HealthStatus string

const (
	// HealthStatusHealthy indicates the service is healthy.
	HealthStatusHealthy HealthStatus = "healthy"
	// HealthStatusDegraded indicates the service is degraded but functional.
	HealthStatusDegraded HealthStatus = "degraded"
	// HealthStatusUnhealthy indicates the service is unhealthy.
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// Buffer size constants for uptime formatting.
const (
	bufSizeDaysHoursMinutes = 16 // "999d 23h 59m"
	bufSizeHoursMinutes     = 8  // "23h 59m"
	bufSizeMinutesSeconds   = 8  // "59m 59s"
	bufSizeSeconds          = 4  // "59s"
)

// HealthResponse is the standardized health check response format.
type HealthResponse struct {
	Status  HealthStatus           `json:"status"`
	Service string                 `json:"service"`
	Version string                 `json:"version"`
	Uptime  string                 `json:"uptime,omitempty"`
	Checks  map[string]CheckResult `json:"checks,omitempty"`
}

// CheckResult represents the result of an individual health check.
type CheckResult struct {
	Status  HealthStatus `json:"status"`
	Message string       `json:"message,omitempty"`
	Latency string       `json:"latency,omitempty"`
}

// HealthChecker is a function that performs a health check and returns the result.
type HealthChecker func() CheckResult

// HealthOptions configures the health endpoint behavior.
type HealthOptions struct {
	// ServiceName is the name of the service.
	ServiceName string
	// ServiceVersion is the version of the service.
	ServiceVersion string
	// StartTime is when the service started (for uptime calculation).
	StartTime time.Time
	// Checks is a map of named health checkers.
	Checks map[string]HealthChecker
}

// healthState tracks server start time for uptime reporting.
var healthState = struct {
	sync.Once
	startTime time.Time
}{}

// RegisterHealthRoutes adds standardized health endpoints to a Gin router.
// Endpoints:
//   - GET /health - Basic health check with status, service name, version
//   - GET /health/memory - Memory statistics from runtime
//   - HEAD /health - Lightweight health check for load balancers
func RegisterHealthRoutes(router *gin.Engine, serviceName, version string) {
	initStartTime()

	router.GET("/health", healthHandler(serviceName, version, nil))
	router.HEAD("/health", headHealthHandler())
	router.GET("/health/memory", memoryHealthHandler())
}

// RegisterHealthRoutesWithChecks adds health endpoints with custom health checks.
func RegisterHealthRoutesWithChecks(router *gin.Engine, opts HealthOptions) {
	if opts.StartTime.IsZero() {
		initStartTime()
		opts.StartTime = healthState.startTime
	}

	router.GET("/health", healthHandler(opts.ServiceName, opts.ServiceVersion, opts.Checks))
	router.HEAD("/health", headHealthHandler())
	router.GET("/health/memory", memoryHealthHandler())
}

// initStartTime initializes the server start time (only once).
func initStartTime() {
	healthState.Do(func() {
		healthState.startTime = time.Now()
	})
}

// healthHandler returns a Gin handler for the health endpoint.
func healthHandler(serviceName, version string, checks map[string]HealthChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		response := HealthResponse{
			Status:  HealthStatusHealthy,
			Service: serviceName,
			Version: version,
			Uptime:  formatUptime(time.Since(healthState.startTime)),
		}

		// Run health checks if any are configured
		if len(checks) > 0 {
			response.Checks = make(map[string]CheckResult, len(checks))
			for name, checker := range checks {
				result := checker()
				response.Checks[name] = result

				// Update overall status based on check results
				if result.Status == HealthStatusUnhealthy {
					response.Status = HealthStatusUnhealthy
				} else if result.Status == HealthStatusDegraded && response.Status == HealthStatusHealthy {
					response.Status = HealthStatusDegraded
				}
			}
		}

		// Set status code based on health status
		statusCode := http.StatusOK
		if response.Status == HealthStatusUnhealthy {
			statusCode = http.StatusServiceUnavailable
		}

		c.JSON(statusCode, response)
	}
}

// headHealthHandler returns a Gin handler for HEAD /health requests.
func headHealthHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Status(http.StatusOK)
	}
}

// memoryHealthHandler returns a Gin handler for the memory health endpoint.
func memoryHealthHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		monitoring.MemoryHealthHandler(c.Writer, c.Request)
	}
}

// formatUptime formats a duration as a human-readable string.
func formatUptime(d time.Duration) string {
	const (
		hoursPerDay    = 24
		minutesPerHour = 60
		secondsPerMin  = 60
	)

	days := int(d.Hours()) / hoursPerDay
	hours := int(d.Hours()) % hoursPerDay
	minutes := int(d.Minutes()) % minutesPerHour
	seconds := int(d.Seconds()) % secondsPerMin

	if days > 0 {
		return formatDaysHoursMinutes(days, hours, minutes)
	}
	if hours > 0 {
		return formatHoursMinutes(hours, minutes)
	}
	if minutes > 0 {
		return formatMinutesSeconds(minutes, seconds)
	}
	return formatSeconds(seconds)
}

// formatDaysHoursMinutes formats uptime with days, hours, and minutes.
func formatDaysHoursMinutes(days, hours, minutes int) string {
	result := make([]byte, 0, bufSizeDaysHoursMinutes)
	result = appendInt(result, days)
	result = append(result, 'd', ' ')
	result = appendInt(result, hours)
	result = append(result, 'h', ' ')
	result = appendInt(result, minutes)
	result = append(result, 'm')
	return string(result)
}

// formatHoursMinutes formats uptime with hours and minutes.
func formatHoursMinutes(hours, minutes int) string {
	result := make([]byte, 0, bufSizeHoursMinutes)
	result = appendInt(result, hours)
	result = append(result, 'h', ' ')
	result = appendInt(result, minutes)
	result = append(result, 'm')
	return string(result)
}

// formatMinutesSeconds formats uptime with minutes and seconds.
func formatMinutesSeconds(minutes, seconds int) string {
	result := make([]byte, 0, bufSizeMinutesSeconds)
	result = appendInt(result, minutes)
	result = append(result, 'm', ' ')
	result = appendInt(result, seconds)
	result = append(result, 's')
	return string(result)
}

// formatSeconds formats uptime with just seconds.
func formatSeconds(seconds int) string {
	result := make([]byte, 0, bufSizeSeconds)
	result = appendInt(result, seconds)
	result = append(result, 's')
	return string(result)
}

// appendInt appends an integer to a byte slice without using fmt.
func appendInt(buf []byte, n int) []byte {
	if n == 0 {
		return append(buf, '0')
	}
	if n < 0 {
		buf = append(buf, '-')
		n = -n
	}

	// Count digits
	digits := 0
	for temp := n; temp > 0; temp /= 10 {
		digits++
	}

	// Pre-grow buffer
	start := len(buf)
	for range digits {
		buf = append(buf, '0')
	}

	// Fill in digits from right to left
	for i := start + digits - 1; n > 0; i-- {
		buf[i] = byte('0' + n%10)
		n /= 10
	}

	return buf
}

// DatabaseHealthChecker creates a health checker for database connectivity.
// The pingFunc should attempt to ping the database and return an error if it fails.
func DatabaseHealthChecker(pingFunc func() error) HealthChecker {
	return func() CheckResult {
		start := time.Now()
		err := pingFunc()
		latency := time.Since(start)

		if err != nil {
			return CheckResult{
				Status:  HealthStatusUnhealthy,
				Message: "Database connection failed",
				Latency: latency.String(),
			}
		}

		return CheckResult{
			Status:  HealthStatusHealthy,
			Message: "Database connection OK",
			Latency: latency.String(),
		}
	}
}

// RedisHealthChecker creates a health checker for Redis connectivity.
// The pingFunc should attempt to ping Redis and return an error if it fails.
func RedisHealthChecker(pingFunc func() error) HealthChecker {
	return func() CheckResult {
		start := time.Now()
		err := pingFunc()
		latency := time.Since(start)

		if err != nil {
			return CheckResult{
				Status:  HealthStatusDegraded, // Redis often not critical
				Message: "Redis connection failed",
				Latency: latency.String(),
			}
		}

		return CheckResult{
			Status:  HealthStatusHealthy,
			Message: "Redis connection OK",
			Latency: latency.String(),
		}
	}
}

// ElasticsearchHealthChecker creates a health checker for Elasticsearch connectivity.
// The pingFunc should attempt to ping Elasticsearch and return an error if it fails.
func ElasticsearchHealthChecker(pingFunc func() error) HealthChecker {
	return func() CheckResult {
		start := time.Now()
		err := pingFunc()
		latency := time.Since(start)

		if err != nil {
			return CheckResult{
				Status:  HealthStatusDegraded,
				Message: "Elasticsearch connection failed",
				Latency: latency.String(),
			}
		}

		return CheckResult{
			Status:  HealthStatusHealthy,
			Message: "Elasticsearch connection OK",
			Latency: latency.String(),
		}
	}
}
