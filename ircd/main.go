package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/ircd/internal/config"
	"github.com/jonesrussell/north-cloud/ircd/internal/server"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("NorthCloud IRCd %s\n", version)
		os.Exit(0)
	}

	cfgPath := "config.yml"
	if envPath := os.Getenv("IRCD_CONFIG"); envPath != "" {
		cfgPath = envPath
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	log, err := infralogger.New(infralogger.Config{
		Level:  "info",
		Format: "json",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = log.Sync() }()
	log = log.With(infralogger.String("service", "ircd"))

	srv := server.New(cfg, log)
	addr, err := srv.Start()
	if err != nil {
		log.Error("Failed to start server", infralogger.Error(err))
		os.Exit(1)
	}

	log.Info("IRCd started",
		infralogger.String("version", version),
		infralogger.String("address", addr),
		infralogger.String("server_name", cfg.Server.Name),
		infralogger.String("network", cfg.Server.Network),
	)

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	sig := <-shutdown
	log.Info("Shutdown signal received", infralogger.String("signal", sig.String()))

	srv.Shutdown()
	log.Info("IRCd stopped")
}
