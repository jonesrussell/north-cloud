package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	goredis "github.com/redis/go-redis/v9"

	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/profiling"

	"github.com/jonesrussell/north-cloud/social-publisher/internal/api"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/config"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/database"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/domain"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/orchestrator"
	spredis "github.com/jonesrussell/north-cloud/social-publisher/internal/redis"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/workers"
)

const (
	version = "0.1.0"

	defaultRetryInterval    = 30 * time.Second
	defaultScheduleInterval = 60 * time.Second
	defaultRealtimeQueueSize = 100
	defaultRetryQueueSize    = 50
	defaultPort              = 8077
	shutdownTimeout          = 30 * time.Second
)

func main() {
	os.Exit(run())
}

func run() int {
	profiling.StartPprofServer()

	cfg, err := config.Load("config.yml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		return 1
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid config: %v\n", err)
		return 1
	}

	log, err := createLogger(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		return 1
	}
	defer func() { _ = log.Sync() }()

	log.Info("Starting social-publisher",
		infralogger.String("version", version),
	)

	db, repo, err := setupDatabase(cfg)
	if err != nil {
		log.Error("Failed to connect to database", infralogger.Error(err))
		return 1
	}
	defer db.Close()

	redisClient := setupRedis(cfg)
	defer redisClient.Close()

	return startService(cfg, log, repo, redisClient)
}

func createLogger(cfg *config.Config) (infralogger.Logger, error) {
	log, err := infralogger.New(infralogger.Config{
		Level:       "info",
		Format:      "json",
		Development: cfg.Debug,
	})
	if err != nil {
		return nil, err
	}
	return log.With(
		infralogger.String("service", "social-publisher"),
		infralogger.String("version", version),
	), nil
}

func setupDatabase(cfg *config.Config) (*sqlx.DB, *database.Repository, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User,
		cfg.Database.Password, cfg.Database.DBName, cfg.Database.SSLMode,
	)
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, nil, err
	}
	return db, database.NewRepository(db), nil
}

func setupRedis(cfg *config.Config) *goredis.Client {
	return goredis.NewClient(&goredis.Options{
		Addr:     cfg.Redis.URL,
		Password: cfg.Redis.Password,
	})
}

func startService(
	cfg *config.Config,
	log infralogger.Logger,
	repo *database.Repository,
	redisClient *goredis.Client,
) int {
	eventPub := spredis.NewEventPublisher(redisClient, log)
	subscriber := spredis.NewSubscriber(redisClient, log)

	adapters := map[string]domain.PlatformAdapter{
		// Adapters added as platform credentials are configured
	}

	orch := orchestrator.NewOrchestrator(adapters, eventPub, repo)
	queue := orchestrator.NewPriorityQueue(defaultRealtimeQueueSize, defaultRetryQueueSize)

	retryInterval := parseDurationOrDefault(cfg.Service.RetryInterval, defaultRetryInterval)
	scheduleInterval := parseDurationOrDefault(cfg.Service.ScheduleInterval, defaultScheduleInterval)

	retryWorker := workers.NewRetryWorker(repo, orch, eventPub, log, retryInterval, cfg.Service.BatchSize)
	scheduler := workers.NewScheduler(repo, queue, log, scheduleInterval, cfg.Service.BatchSize)

	port := extractPort(cfg.Server.Address)
	router := api.NewRouter(repo, orch, cfg)
	server := router.NewServer(log, port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go retryWorker.Run(ctx)
	go scheduler.Run(ctx)
	go func() {
		if subErr := subscriber.Subscribe(ctx, func(msg *domain.PublishMessage) {
			queue.EnqueueRealtime(orchestrator.PublishJob{
				ContentID: msg.ContentID,
				Message:   msg,
			})
		}); subErr != nil && ctx.Err() == nil {
			log.Error("Redis subscriber error", infralogger.Error(subErr))
		}
	}()

	errChan := server.StartAsync()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-shutdown:
		log.Info("Shutdown signal received", infralogger.String("signal", sig.String()))
	case err := <-errChan:
		log.Error("Server error", infralogger.Error(err))
	}

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("Server shutdown error", infralogger.Error(err))
	}

	log.Info("Social publisher stopped")
	return 0
}

func parseDurationOrDefault(s string, fallback time.Duration) time.Duration {
	if d, err := time.ParseDuration(s); err == nil && d > 0 {
		return d
	}
	return fallback
}

func extractPort(address string) int {
	if address == "" {
		return defaultPort
	}
	idx := strings.LastIndex(address, ":")
	if idx < 0 {
		return defaultPort
	}
	port, err := strconv.Atoi(address[idx+1:])
	if err != nil {
		return defaultPort
	}
	return port
}
