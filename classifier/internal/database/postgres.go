// Package database provides database connectivity and operations.
package database

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // PostgreSQL driver
	infracontext "github.com/north-cloud/infrastructure/context"
)

const (
	// DefaultMaxOpenConns is the default maximum number of open connections
	DefaultMaxOpenConns = 25
	// DefaultMaxIdleConns is the default maximum number of idle connections
	DefaultMaxIdleConns = 5
	// DefaultConnMaxLifetime is the default maximum connection lifetime
	DefaultConnMaxLifetime = 5 * time.Minute
	// DefaultPingTimeout is the default timeout for ping operations
	DefaultPingTimeout = 5 * time.Second
)

// Config holds database configuration.
type Config struct {
	Host     string
	Port     string
	User     string
	Password string //nolint:gosec // G117: DB connection config
	DBName   string
	SSLMode  string
}

// NewPostgresConnection creates a new PostgreSQL database connection.
func NewPostgresConnection(cfg Config) (*sqlx.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(DefaultMaxOpenConns)
	db.SetMaxIdleConns(DefaultMaxIdleConns)
	db.SetConnMaxLifetime(DefaultConnMaxLifetime)

	// Verify connection
	ctx, cancel := infracontext.WithPingTimeout()
	defer cancel()

	if pingErr := db.PingContext(ctx); pingErr != nil {
		return nil, fmt.Errorf("failed to ping database: %w", pingErr)
	}

	return db, nil
}
