package profiling

import (
	"fmt"
	"os"
	"runtime"

	"github.com/grafana/pyroscope-go"
	"github.com/jonesrussell/north-cloud/infrastructure/config"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// PyroscopeProfiler holds the Pyroscope profiler instance
type PyroscopeProfiler struct {
	profiler *pyroscope.Profiler
}

// StartPyroscope initializes and starts Pyroscope continuous profiling
// It reads configuration from the config package, which owns environment lookup:
// - ENABLE_CONTINUOUS_PROFILING: Set to "true" to enable (default: false)
// - PYROSCOPE_SERVER_URL: Pyroscope server address (default: http://pyroscope:4040)
// - PYROSCOPE_ENVIRONMENT: Environment tag (default: development)
//
// Returns a no-op profiler if continuous profiling is disabled.
// Returns error if profiling is enabled but initialization fails.
func StartPyroscope(serviceName string) (*PyroscopeProfiler, error) {
	return StartPyroscopeWithConfig(serviceName, config.LoadContinuousProfilingConfig())
}

// StartPyroscopeWithConfig initializes and starts Pyroscope with explicit configuration.
func StartPyroscopeWithConfig(
	serviceName string,
	cfg config.ContinuousProfilingConfig,
) (*PyroscopeProfiler, error) {
	if !cfg.Enabled {
		return &PyroscopeProfiler{}, nil
	}

	// Configure profiler
	pyroscopeConfig := pyroscope.Config{
		ApplicationName: fmt.Sprintf("north-cloud.%s", serviceName),
		ServerAddress:   cfg.ServerURL,

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
			"environment": cfg.Environment,
			"version":     cfg.Version,
			"hostname":    getHostname(),
			"go_version":  runtime.Version(),
		},
	}

	// Start profiler
	profiler, err := pyroscope.Start(pyroscopeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to start Pyroscope profiler: %w", err)
	}

	log, logErr := infralogger.New(infralogger.Config{
		Level:  "info",
		Format: "json",
	})
	if logErr == nil {
		log.Info("Pyroscope continuous profiling started",
			infralogger.String("service", pyroscopeConfig.ApplicationName),
			infralogger.String("server", cfg.ServerURL),
			infralogger.String("environment", cfg.Environment),
		)
	}

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
