package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := LoadConfig()

	fmt.Printf("nc-http-proxy starting (mode: %s, port: %d)\n", cfg.Mode, cfg.Port)

	proxy, err := NewProxy(cfg)
	if err != nil {
		return fmt.Errorf("failed to create proxy: %w", err)
	}
	admin := NewAdminHandler(proxy)

	mux := http.NewServeMux()

	// Admin routes
	mux.Handle("/admin/", admin)

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// All other requests go to proxy
	mux.Handle("/", proxy)

	// Wrap handler to handle CONNECT method for HTTPS proxying
	// CONNECT requests have a different URL format (host:port) that ServeMux doesn't route properly
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect {
			proxy.ServeHTTP(w, r)
			return
		}
		mux.ServeHTTP(w, r)
	})

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      handler,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	// Graceful shutdown
	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, syscall.SIGINT, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		fmt.Printf("Listening on :%d\n", cfg.Port)
		if listenErr := server.ListenAndServe(); listenErr != http.ErrServerClosed {
			errCh <- listenErr
		}
	}()

	select {
	case serverErr := <-errCh:
		return serverErr
	case sig := <-shutdownCh:
		fmt.Printf("\nReceived %s, shutting down...\n", sig)
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return server.Shutdown(ctx)
	}
}

const (
	readTimeout     = 30 * time.Second
	writeTimeout    = 30 * time.Second
	idleTimeout     = 60 * time.Second
	shutdownTimeout = 10 * time.Second
)
