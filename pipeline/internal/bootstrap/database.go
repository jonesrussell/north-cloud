package bootstrap

import (
	"fmt"

	"github.com/jonesrussell/north-cloud/pipeline/internal/config"
	"github.com/jonesrussell/north-cloud/pipeline/internal/database"
)

// SetupDatabase creates a database connection from config.
func SetupDatabase(cfg *config.Config) (*database.Connection, error) {
	dbCfg := &database.Config{
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

	db, connErr := database.NewConnection(dbCfg)
	if connErr != nil {
		return nil, fmt.Errorf("database connection: %w", connErr)
	}

	return db, nil
}
