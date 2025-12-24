package httpd

import (
	dbconfig "github.com/jonesrussell/north-cloud/crawler/internal/config/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
)

// databaseConfigFromInterface converts config database config to database.Config.
// This eliminates the DRY violation of repeated field mapping.
func databaseConfigFromInterface(cfg *dbconfig.Config) database.Config {
	return database.Config{
		Host:     cfg.Host,
		Port:     cfg.Port,
		User:     cfg.User,
		Password: cfg.Password,
		DBName:   cfg.DBName,
		SSLMode:  cfg.SSLMode,
	}
}
