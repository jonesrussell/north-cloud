package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jonesrussell/north-cloud/index-manager/internal/api"
	"github.com/jonesrussell/north-cloud/index-manager/internal/config"
	"github.com/jonesrussell/north-cloud/index-manager/internal/database"
	"github.com/jonesrussell/north-cloud/index-manager/internal/elasticsearch"
	"github.com/jonesrussell/north-cloud/index-manager/internal/service"
)

// SimpleLogger is a simple logger implementation
type SimpleLogger struct {
	debug bool
}

func NewSimpleLogger(debug bool) *SimpleLogger {
	return &SimpleLogger{debug: debug}
}

func (l *SimpleLogger) Info(msg string, keysAndValues ...interface{}) {
	fmt.Printf("[INFO] %s", msg)
	if len(keysAndValues) > 0 {
		fmt.Print(" ")
		for i := 0; i < len(keysAndValues); i += 2 {
			if i+1 < len(keysAndValues) {
				fmt.Printf("%v=%v ", keysAndValues[i], keysAndValues[i+1])
			}
		}
	}
	fmt.Println()
}

func (l *SimpleLogger) Error(msg string, keysAndValues ...interface{}) {
	fmt.Printf("[ERROR] %s", msg)
	if len(keysAndValues) > 0 {
		fmt.Print(" ")
		for i := 0; i < len(keysAndValues); i += 2 {
			if i+1 < len(keysAndValues) {
				fmt.Printf("%v=%v ", keysAndValues[i], keysAndValues[i+1])
			}
		}
	}
	fmt.Println()
}

func (l *SimpleLogger) Warn(msg string, keysAndValues ...interface{}) {
	fmt.Printf("[WARN] %s", msg)
	if len(keysAndValues) > 0 {
		fmt.Print(" ")
		for i := 0; i < len(keysAndValues); i += 2 {
			if i+1 < len(keysAndValues) {
				fmt.Printf("%v=%v ", keysAndValues[i], keysAndValues[i+1])
			}
		}
	}
	fmt.Println()
}

func (l *SimpleLogger) Debug(msg string, keysAndValues ...interface{}) {
	if l.debug {
		fmt.Printf("[DEBUG] %s", msg)
		if len(keysAndValues) > 0 {
			fmt.Print(" ")
			for i := 0; i < len(keysAndValues); i += 2 {
				if i+1 < len(keysAndValues) {
					fmt.Printf("%v=%v ", keysAndValues[i], keysAndValues[i+1])
				}
			}
		}
		fmt.Println()
	}
}

func main() {
	// Load configuration
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yml"
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger := NewSimpleLogger(cfg.Service.Debug)

	logger.Info("Starting Index Manager Service", "version", cfg.Service.Version)

	// Initialize Elasticsearch client
	esConfig := &elasticsearch.Config{
		URL:        cfg.Elasticsearch.URL,
		Username:   cfg.Elasticsearch.Username,
		Password:   cfg.Elasticsearch.Password,
		MaxRetries: cfg.Elasticsearch.MaxRetries,
		Timeout:    cfg.Elasticsearch.Timeout,
	}

	esClient, err := elasticsearch.NewClient(esConfig)
	if err != nil {
		logger.Error("Failed to create Elasticsearch client", "error", err)
		os.Exit(1)
	}
	logger.Info("Elasticsearch client initialized")

	// Initialize database connection
	dbConfig := &database.Config{
		Host:            cfg.Database.Host,
		Port:            cfg.Database.Port,
		User:            cfg.Database.User,
		Password:        cfg.Database.Password,
		Database:        cfg.Database.Database,
		SSLMode:         cfg.Database.SSLMode,
		MaxConnections:  cfg.Database.MaxConnections,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnectionMaxLifetime,
	}

	db, err := database.NewConnection(dbConfig)
	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	logger.Info("Database connection established")

	// Initialize index service
	indexService := service.NewIndexService(esClient, db, logger)

	// Initialize API handler
	handler := api.NewHandler(indexService, logger)

	// Initialize HTTP server
	serverConfig := api.ServerConfig{
		Port:         cfg.Service.Port,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		Debug:        cfg.Service.Debug,
	}

	server := api.NewServer(handler, serverConfig, logger)

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		if err := server.Start(); err != nil {
			serverErr <- err
		}
	}()

	logger.Info("Index Manager Service started", "port", cfg.Service.Port)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		logger.Error("Server error", "error", err)
	case sig := <-sigChan:
		logger.Info("Received signal", "signal", sig)
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server shutdown error", "error", err)
	}

	logger.Info("Index Manager Service stopped")
}
