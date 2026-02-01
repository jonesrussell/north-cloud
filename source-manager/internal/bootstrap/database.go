package bootstrap

import (
	"fmt"

	"github.com/jonesrussell/north-cloud/source-manager/internal/config"
	"github.com/jonesrussell/north-cloud/source-manager/internal/database"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// SetupDatabase creates a database connection.
func SetupDatabase(cfg *config.Config, log infralogger.Logger) (*database.DB, error) {
	db, err := database.New(cfg, log)
	if err != nil {
		return nil, fmt.Errorf("database connection: %w", err)
	}
	return db, nil
}
