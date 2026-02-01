package bootstrap

import (
	"fmt"

	"github.com/jonesrussell/north-cloud/index-manager/internal/config"
	"github.com/jonesrussell/north-cloud/index-manager/internal/database"
)

// SetupDatabase creates a database connection.
func SetupDatabase(cfg *config.Config) (*database.Connection, error) {
	dbConfig := &database.Config{
		Host:            cfg.Database.Host,
		Port:            cfg.Database.Port,
		User:            cfg.Database.User,
		Password:        cfg.Database.Password,
		Database:        cfg.Database.Database,
		SSLMode:         cfg.Database.SSLMode,
		MaxConnections:  cfg.Database.MaxConnections,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnectionMaxLifetime,
	}

	db, err := database.NewConnection(dbConfig)
	if err != nil {
		return nil, fmt.Errorf("database connection: %w", err)
	}
	return db, nil
}
