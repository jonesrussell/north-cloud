package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jonesrussell/gosources/internal/config"
	"github.com/jonesrussell/gosources/internal/logger"
	_ "github.com/lib/pq" //nolint:blankimports // PostgreSQL driver
)

const (
	defaultPingTimeout = 5
)

type DB struct {
	db     *sql.DB
	logger logger.Logger
}

func New(cfg *config.Config, log logger.Logger) (*DB, error) {
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

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), defaultPingTimeout*time.Second)
	defer cancel()

	if pingErr := db.PingContext(ctx); pingErr != nil {
		return nil, fmt.Errorf("ping database: %w", pingErr)
	}

	log.Info("Database connection established",
		logger.String("host", cfg.Database.Host),
		logger.Int("port", cfg.Database.Port),
		logger.String("dbname", cfg.Database.DBName),
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
