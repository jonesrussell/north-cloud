package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jonesrussell/north-cloud/source-manager/internal/config"
	_ "github.com/lib/pq" //nolint:blankimports // PostgreSQL driver
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/retry"
)

// Connection retry configuration
const (
	maxRetryAttempts  = 10
	initialRetryDelay = 1 * time.Second
	maxRetryDelay     = 30 * time.Second
	retryMultiplier   = 2.0
	connectionTimeout = 2 * time.Minute
	pingTimeout       = 5 * time.Second
)

type DB struct {
	db     *sql.DB
	logger infralogger.Logger
}

// isRetryableDBError checks if a database error is transient and worth retrying
func isRetryableDBError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	retryablePatterns := []string{
		"starting up",
		"connection refused",
		"connection reset",
		"no such host",
		"network is unreachable",
		"timeout",
		"deadline exceeded",
		"too many connections",
		"server closed the connection unexpectedly",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}
	return false
}

func New(cfg *config.Config, log infralogger.Logger) (*DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.DBName,
		cfg.Database.SSLMode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	db.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)

	// Connect with retry for transient errors (e.g., database still starting up)
	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer cancel()

	retryConfig := retry.Config{
		MaxAttempts:  maxRetryAttempts,
		InitialDelay: initialRetryDelay,
		MaxDelay:     maxRetryDelay,
		Multiplier:   retryMultiplier,
		IsRetryable:  isRetryableDBError,
	}

	attempt := 0
	connectErr := retry.Retry(ctx, retryConfig, func() error {
		attempt++
		pingCtx, pingCancel := context.WithTimeout(ctx, pingTimeout)
		defer pingCancel()

		pingErr := db.PingContext(pingCtx)
		if pingErr != nil {
			if isRetryableDBError(pingErr) {
				log.Warn("Database not ready, retrying",
					infralogger.Int("attempt", attempt),
					infralogger.Int("max_attempts", maxRetryAttempts),
					infralogger.Error(pingErr),
				)
			}
			return pingErr
		}
		return nil
	})

	if connectErr != nil {
		if closeErr := db.Close(); closeErr != nil {
			log.Error("Failed to close database after connection failure",
				infralogger.Error(closeErr),
			)
		}
		return nil, fmt.Errorf("ping database: %w", connectErr)
	}

	log.Info("Database connection established",
		infralogger.String("host", cfg.Database.Host),
		infralogger.Int("port", cfg.Database.Port),
		infralogger.String("dbname", cfg.Database.DBName),
	)

	return &DB{
		db:     db,
		logger: log,
	}, nil
}

func (d *DB) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

func (d *DB) DB() *sql.DB {
	return d.db
}
