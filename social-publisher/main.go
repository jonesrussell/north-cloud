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

	defaultRetryInterval     = 30 * time.Second
	defaultScheduleInterval  = 60 * time.Second
	defaultRealtimeQueueSize = 100
	defaultRetryQueueSize    = 50
	defaultPort              = 8078
	dequeueTimeout           = 5 * time.Second
	shutdownTimeout          = 30 * time.Second
	redisPingTimeout         = 5 * time.Second
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

	redisClient, err := setupRedis(cfg)
	if err != nil {
		log.Error("Failed to connect to Redis", infralogger.Error(err))
		return 1
	}
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

func setupRedis(cfg *config.Config) (*goredis.Client, error) {
	client := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Redis.URL,
		Password: cfg.Redis.Password,
	})
	ctx, cancel := context.WithTimeout(context.Background(), redisPingTimeout)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}
	return client, nil
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

	retryInterval := parseDurationOrDefault(cfg.Service.RetryInterval, defaultRetryInterval, log, "retry_interval")
	scheduleInterval := parseDurationOrDefault(
		cfg.Service.ScheduleInterval, defaultScheduleInterval, log, "schedule_interval",
	)

	retryWorker := workers.NewRetryWorker(repo, orch, eventPub, log, retryInterval, cfg.Service.BatchSize)
	scheduler := workers.NewScheduler(repo, queue, log, scheduleInterval, cfg.Service.BatchSize)

	port := extractPort(cfg.Server.Address)
	router := api.NewRouter(repo, orch, cfg, log)
	server := router.NewServer(log, port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go retryWorker.Run(ctx)
	go scheduler.Run(ctx)
	go runSubscriber(ctx, subscriber, queue, log)
	go runConsumer(ctx, queue, orch, repo, log)

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

func runSubscriber(
	ctx context.Context,
	subscriber *spredis.Subscriber,
	queue *orchestrator.PriorityQueue,
	log infralogger.Logger,
) {
	subErr := subscriber.Subscribe(ctx, func(msg *domain.PublishMessage) {
		if len(msg.Targets) == 0 {
			log.Warn("Received message with no targets, skipping",
				infralogger.String("content_id", msg.ContentID),
			)
			return
		}
		for _, target := range msg.Targets {
			job := orchestrator.PublishJob{
				ContentID: msg.ContentID,
				Platform:  target.Platform,
				Account:   target.Account,
				Message:   msg,
			}
			if !queue.EnqueueRealtime(job) {
				log.Error("Realtime queue full, dropping job",
					infralogger.String("content_id", msg.ContentID),
					infralogger.String("platform", target.Platform),
				)
			}
		}
	})
	if subErr != nil && ctx.Err() == nil {
		log.Error("Redis subscriber error", infralogger.Error(subErr))
	}
}

func runConsumer(
	ctx context.Context,
	queue *orchestrator.PriorityQueue,
	orch *orchestrator.Orchestrator,
	repo *database.Repository,
	log infralogger.Logger,
) {
	log.Info("Queue consumer started")
	for {
		if ctx.Err() != nil {
			log.Info("Queue consumer shutting down")
			return
		}
		job, ok := queue.Dequeue(dequeueTimeout)
		if !ok {
			continue
		}
		processQueueJob(ctx, job, orch, repo, log)
	}
}

func processQueueJob(
	ctx context.Context,
	job orchestrator.PublishJob,
	orch *orchestrator.Orchestrator,
	repo *database.Repository,
	log infralogger.Logger,
) {
	maxAttempts := len(orchestrator.Backoffs())
	delivery, err := repo.CreateDelivery(ctx, job.ContentID, job.Platform, job.Account, maxAttempts)
	if err != nil {
		log.Error("Failed to create delivery",
			infralogger.Error(err),
			infralogger.String("content_id", job.ContentID),
			infralogger.String("platform", job.Platform),
		)
		return
	}

	result, publishErr := orch.ProcessJob(ctx, job.Platform, job.Message)
	if publishErr != nil {
		log.Error("Publish failed",
			infralogger.Error(publishErr),
			infralogger.String("delivery_id", delivery.ID),
			infralogger.String("platform", job.Platform),
		)
		errMsg := publishErr.Error()
		if failErr := repo.MarkDeliveryFailed(ctx, delivery.ID, errMsg); failErr != nil {
			log.Error("Failed to mark delivery as failed", infralogger.Error(failErr))
		}
		return
	}

	if updateErr := repo.UpdateDeliveryStatus(
		ctx, delivery.ID, domain.StatusDelivered, &result, nil,
	); updateErr != nil {
		log.Error("Failed to update delivery status after publish", infralogger.Error(updateErr))
		return
	}

	log.Info("Published successfully",
		infralogger.String("delivery_id", delivery.ID),
		infralogger.String("platform", job.Platform),
		infralogger.String("platform_id", result.PlatformID),
	)
}

func parseDurationOrDefault(s string, fallback time.Duration, log infralogger.Logger, field string) time.Duration {
	if s == "" {
		return fallback
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		log.Warn("Invalid duration in config, using default",
			infralogger.String("field", field),
			infralogger.String("value", s),
			infralogger.Error(err),
		)
		return fallback
	}
	if d <= 0 {
		log.Warn("Non-positive duration in config, using default",
			infralogger.String("field", field),
			infralogger.String("value", s),
		)
		return fallback
	}
	return d
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
