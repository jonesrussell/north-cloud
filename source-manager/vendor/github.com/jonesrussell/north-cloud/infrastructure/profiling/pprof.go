package profiling

import (
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
)

// StartPprofServer starts pprof profiling server on a separate port
// Only enabled when ENABLE_PROFILING=true environment variable is set
//
// This provides standard pprof endpoints:
//   - /debug/pprof/heap - Memory allocation profiling
//   - /debug/pprof/goroutine - Goroutine stack traces
//   - /debug/pprof/profile - CPU profiling (30s default)
//   - /debug/pprof/allocs - All past memory allocations
//   - /debug/pprof/block - Blocking operations
//   - /debug/pprof/mutex - Mutex contention
//
// Usage:
//
//	import "north-cloud/infrastructure/profiling"
//
//	func main() {
//	    profiling.StartPprofServer()
//	    // ... rest of application
//	}
func StartPprofServer() {
	if os.Getenv("ENABLE_PROFILING") != "true" {
		return
	}

	pprofPort := os.Getenv("PPROF_PORT")
	if pprofPort == "" {
		pprofPort = "6060"
	}

	// Only bind to localhost for security
	// This prevents external access in production
	addr := "localhost:" + pprofPort

	go func() {
		log.Printf("Starting pprof server on %s", addr)
		log.Printf("Access profiles at http://%s/debug/pprof/", addr)
		log.Printf("Capture CPU profile: curl http://%s/debug/pprof/profile?seconds=30 -o cpu.pprof", addr)
		log.Printf("Capture heap profile: curl http://%s/debug/pprof/heap -o heap.pprof", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Printf("pprof server error: %v", err)
		}
	}()
}
