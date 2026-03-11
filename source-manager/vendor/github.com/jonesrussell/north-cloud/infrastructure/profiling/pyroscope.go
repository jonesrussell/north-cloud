package profiling

import (
	"fmt"
	"os"
	"runtime"

	"github.com/grafana/pyroscope-go"
)

// PyroscopeProfiler holds the Pyroscope profiler instance
type PyroscopeProfiler struct {
	profiler *pyroscope.Profiler
}

// StartPyroscope initializes and starts Pyroscope continuous profiling
// It reads configuration from environment variables:
// - ENABLE_CONTINUOUS_PROFILING: Set to "true" to enable (default: false)
// - PYROSCOPE_SERVER_URL: Pyroscope server address (default: http://pyroscope:4040)
// - PYROSCOPE_ENVIRONMENT: Environment tag (default: development)
//
// Returns nil if continuous profiling is disabled.
// Returns error if profiling is enabled but initialization fails.
func StartPyroscope(serviceName string) (*PyroscopeProfiler, error) {
	// Check if continuous profiling is enabled
	enabled := os.Getenv("ENABLE_CONTINUOUS_PROFILING")
	if enabled != "true" {
		return nil, nil // Not an error - just disabled
	}

	// Get configuration from environment
	serverURL := os.Getenv("PYROSCOPE_SERVER_URL")
	if serverURL == "" {
		serverURL = "http://pyroscope:4040"
	}

	environment := os.Getenv("PYROSCOPE_ENVIRONMENT")
	if environment == "" {
		environment = "development"
	}

	// Get application version from environment (optional)
	version := os.Getenv("APP_VERSION")
	if version == "" {
		version = "unknown"
	}

	// Configure profiler
	config := pyroscope.Config{
		ApplicationName: fmt.Sprintf("north-cloud.%s", serviceName),
		ServerAddress:   serverURL,

		// Enable all profile types for comprehensive monitoring
		ProfileTypes: []pyroscope.ProfileType{
			pyroscope.ProfileCPU,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileInuseSpace,
			pyroscope.ProfileGoroutines,
		},

		// Add tags for filtering and grouping
		Tags: map[string]string{
			"environment": environment,
			"version":     version,
			"hostname":    getHostname(),
			"go_version":  runtime.Version(),
		},
	}

	// Start profiler
	profiler, err := pyroscope.Start(config)
	if err != nil {
		return nil, fmt.Errorf("failed to start Pyroscope profiler: %w", err)
	}

	fmt.Printf("✓ Pyroscope continuous profiling started\n")
	fmt.Printf("  Service: %s\n", config.ApplicationName)
	fmt.Printf("  Server: %s\n", serverURL)
	fmt.Printf("  Environment: %s\n", environment)
	fmt.Printf("  Profile Types: CPU, Allocations, In-Use Memory, Goroutines\n")

	return &PyroscopeProfiler{profiler: profiler}, nil
}

// Stop gracefully stops the Pyroscope profiler
func (p *PyroscopeProfiler) Stop() error {
	if p == nil || p.profiler == nil {
		return nil
	}
	return p.profiler.Stop()
}

// getHostname returns the container hostname or "unknown"
func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}
