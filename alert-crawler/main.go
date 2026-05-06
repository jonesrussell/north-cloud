package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// Phase order (WP16 will flesh each phase out):
//   1. Config   — load alert-crawler config (YAML + env overrides via SetDefaults)
//   2. Logger   — structured JSON logger via infrastructure/logger
//   3. Catalogue — open/create SQLite catalogue (data/alerts.db)
//   4. Adapters  — build RSS + atom source adapters from config
//   5. Runner    — fetch → severity-score → deduplicate → publish
//   6. Lifecycle — graceful shutdown on SIGINT/SIGTERM

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Stub: WP16 will replace this with the full bootstrap sequence.
	_ = ctx
	fmt.Fprintln(os.Stdout, "alert-crawler scaffold; not yet implemented")
	os.Exit(0)
}
