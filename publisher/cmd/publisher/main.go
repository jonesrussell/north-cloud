// Package main is the entry point for the publisher service.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/jonesrussell/north-cloud/publisher/internal/app"
	infracontext "github.com/north-cloud/infrastructure/context"
)

var (
	// version can be set at build time via -ldflags
	version = "dev"
)

const flushCacheTimeout = 30 * time.Second

func main() {
	var configPath string
	var flushCache bool
	flag.StringVar(&configPath, "config", "config.yml", "Path to configuration file")
	flag.BoolVar(&flushCache, "flush-cache", false, "Flush Redis deduplication cache and exit")
	flag.Parse()

	// Create application
	application, err := app.New(app.Options{
		ConfigPath: configPath,
		Version:    version,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize application: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if closeErr := application.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "Failed to close application: %v\n", closeErr)
		}
	}()

	// Handle flush-cache command
	if flushCache {
		ctx, cancel := infracontext.WithTimeout(flushCacheTimeout)
		defer cancel()

		if flushErr := application.FlushCache(ctx); flushErr != nil {
			application.Logger().Error("Failed to flush cache")
			os.Exit(1)
		}

		application.Logger().Info("Cache flushed successfully")
		return
	}

	// Run the application
	if runErr := application.Run(context.Background()); runErr != nil {
		application.Logger().Error("Application error")
		os.Exit(1)
	}
}
