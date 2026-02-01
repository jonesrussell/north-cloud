package bootstrap

import (
	"github.com/jonesrussell/north-cloud/source-manager/internal/config"
	"github.com/jonesrussell/north-cloud/source-manager/internal/events"
	infralogger "github.com/north-cloud/infrastructure/logger"
	infraredis "github.com/north-cloud/infrastructure/redis"
)

// SetupEventPublisher creates an optional event publisher if Redis is enabled.
// Returns nil if Redis is disabled or unavailable.
func SetupEventPublisher(cfg *config.Config, log infralogger.Logger) *events.Publisher {
	if !cfg.Redis.Enabled {
		return nil
	}

	redisClient, err := infraredis.NewClient(infraredis.Config{
		Address:  cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err != nil {
		log.Warn("Redis not available, events disabled",
			infralogger.Error(err),
		)
		return nil
	}

	log.Info("Event publisher initialized",
		infralogger.String("redis_address", cfg.Redis.Address),
	)
	return events.NewPublisher(redisClient, log)
}
