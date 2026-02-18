package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // PostgreSQL driver
)

const (
	// DefaultMaxOpenConns is the default maximum number of open connections to the database
	DefaultMaxOpenConns = 25

	// DefaultMaxIdleConns is the default maximum number of idle connections
	DefaultMaxIdleConns = 5

	// DefaultConnMaxLifetime is the default maximum lifetime of a connection
	DefaultConnMaxLifetime = 5 * time.Minute

	// DefaultPingTimeout is the default timeout for pinging the database
	DefaultPingTimeout = 5 * time.Second
)

// Config holds database configuration
type Config struct {
	Host     string
	Port     string
	User     string
	Password string //nolint:gosec // G117: DB connection config
	DBName   string
	SSLMode  string
}

// NewPostgresConnection creates a new PostgreSQL connection with connection pooling
func NewPostgresConnection(cfg Config) (*sqlx.DB, error) {
	// Build connection string
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	// Connect to database
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(DefaultMaxOpenConns)
	db.SetMaxIdleConns(DefaultMaxIdleConns)
	db.SetConnMaxLifetime(DefaultConnMaxLifetime)

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), DefaultPingTimeout)
	defer cancel()

	if pingErr := db.PingContext(ctx); pingErr != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", pingErr)
	}

	return db, nil
}

// Close closes the database connection
func Close(db *sqlx.DB) error {
	if db != nil {
		return db.Close()
	}
	return nil
}
