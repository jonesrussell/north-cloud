package bootstrap

import (
	"errors"

	"github.com/jonesrussell/north-cloud/crawler/internal/config"
	infraredis "github.com/north-cloud/infrastructure/redis"
	"github.com/redis/go-redis/v9"
)

// ErrRedisDisabled indicates Redis is disabled or not configured.
var ErrRedisDisabled = errors.New("redis disabled")

// CreateRedisClient creates a Redis client from config.
// Returns ErrRedisDisabled if config is nil or disabled.
func CreateRedisClient(redisCfg *config.RedisConfig) (*redis.Client, error) {
	if redisCfg == nil || !redisCfg.Enabled {
		return nil, ErrRedisDisabled
	}
	return infraredis.NewClient(infraredis.Config{
		Address:  redisCfg.Address,
		Password: redisCfg.Password,
		DB:       redisCfg.DB,
	})
}
