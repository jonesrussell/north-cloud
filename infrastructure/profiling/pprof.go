package profiling

import (
	"errors"
	"log"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/jonesrussell/north-cloud/infrastructure/config"
)

const (
	pprofServerReadHeaderTimeout = 5 * time.Second
	pprofServerReadTimeout       = 10 * time.Second
	pprofServerWriteTimeout      = 30 * time.Second
	pprofServerIdleTimeout       = 120 * time.Second
)

// StartPprofServer starts pprof profiling server on a separate port
// Only enabled when ENABLE_PROFILING=true environment variable is set.
// Environment parsing is owned by the config package.
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
	StartPprofServerWithConfig(config.LoadPprofConfig())
}

// StartPprofServerWithConfig starts pprof using explicit configuration.
func StartPprofServerWithConfig(cfg config.PprofConfig) {
	if !cfg.Enabled {
		return
	}

	// Only bind to localhost for security
	// This prevents external access in production
	addr := "localhost:" + cfg.Port
	server := &http.Server{
		Addr:              addr,
		Handler:           newPprofMux(),
		ReadHeaderTimeout: pprofServerReadHeaderTimeout,
		ReadTimeout:       pprofServerReadTimeout,
		WriteTimeout:      pprofServerWriteTimeout,
		IdleTimeout:       pprofServerIdleTimeout,
	}

	go func() {
		log.Printf("Starting pprof server on %s", addr)
		log.Printf("Access profiles at http://%s/debug/pprof/", addr)
		log.Printf("Capture CPU profile: curl http://%s/debug/pprof/profile?seconds=30 -o cpu.pprof", addr)
		log.Printf("Capture heap profile: curl http://%s/debug/pprof/heap -o heap.pprof", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("pprof server error: %v", err)
		}
	}()
}

func newPprofMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	mux.Handle("/debug/pprof/allocs", pprof.Handler("allocs"))
	mux.Handle("/debug/pprof/block", pprof.Handler("block"))
	mux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	mux.Handle("/debug/pprof/mutex", pprof.Handler("mutex"))
	mux.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	return mux
}
