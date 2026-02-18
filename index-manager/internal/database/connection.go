package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

const (
	// dbConnectionTimeout is the timeout for database connection test
	dbConnectionTimeout = 5 * time.Second
)

// Config holds database configuration
type Config struct {
	Host            string
	Port            int
	User            string
	Password        string //nolint:gosec // G117: DB connection config
	Database        string
	SSLMode         string
	MaxConnections  int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// Connection wraps the database connection
type Connection struct {
	DB *sql.DB
}

// NewConnection creates a new database connection
func NewConnection(cfg *Config) (*Connection, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(cfg.MaxConnections)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Test connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), dbConnectionTimeout)
	defer cancel()
	if pingErr := db.PingContext(ctx); pingErr != nil {
		return nil, fmt.Errorf("failed to ping database: %w", pingErr)
	}

	return &Connection{DB: db}, nil
}

// Close closes the database connection
func (c *Connection) Close() error {
	return c.DB.Close()
}
