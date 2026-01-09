package database

import (
	"database/sql"
	"fmt"

	"github.com/jonesrussell/north-cloud/source-manager/internal/config"
	_ "github.com/lib/pq" //nolint:blankimports // PostgreSQL driver
	infracontext "github.com/north-cloud/infrastructure/context"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

type DB struct {
	db     *sql.DB
	logger infralogger.Logger
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

	// Test connection
	ctx, cancel := infracontext.WithPingTimeout()
	defer cancel()

	if pingErr := db.PingContext(ctx); pingErr != nil {
		return nil, fmt.Errorf("ping database: %w", pingErr)
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
