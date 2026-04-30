package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jonesrussell/north-cloud/enrichment/internal/callback"
	"github.com/jonesrussell/north-cloud/enrichment/internal/config"
	"github.com/jonesrussell/north-cloud/enrichment/internal/enricher"
	"github.com/jonesrussell/north-cloud/enrichment/internal/orchestration"
	"github.com/jonesrussell/north-cloud/enrichment/internal/server"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "enrichment: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	searcher, err := orchestration.NewElasticsearchSearcher(orchestration.ElasticsearchConfig{
		BaseURL: elasticsearchURL(),
	})
	if err != nil {
		return fmt.Errorf("build elasticsearch searcher: %w", err)
	}
	runner := orchestration.NewRunner(orchestration.Config{
		Registry: enricher.NewDefaultRegistry(searcher),
		Callback: callback.New(callback.Config{
			RequestTimeout: cfg.WriteTimeout,
		}),
		Logger: logger,
	})
	handler := server.New(logger, runner).Handler()
	httpServer := &http.Server{
		Addr:         cfg.Address(),
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting enrichment service", slog.String("address", cfg.Address()))
		if listenErr := httpServer.ListenAndServe(); listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			errCh <- listenErr
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		return shutdown(httpServer, cfg.ShutdownTimeout)
	case err := <-errCh:
		return err
	}
}

func elasticsearchURL() string {
	if value := os.Getenv("ELASTICSEARCH_URL"); value != "" {
		return value
	}
	if value := os.Getenv("ES_URL"); value != "" {
		return value
	}
	return "http://localhost:9200"
}

func shutdown(httpServer *http.Server, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown server: %w", err)
	}
	return nil
}
