package bootstrap

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/crawler/internal/config"
	dbconfig "github.com/jonesrussell/north-cloud/crawler/internal/config/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
)

// DatabaseComponents holds database connection and all repositories.
type DatabaseComponents struct {
	DB                  *sqlx.DB
	JobRepo             *database.JobRepository
	ExecutionRepo       *database.ExecutionRepository
	DiscoveredLinkRepo  *database.DiscoveredLinkRepository
	ProcessedEventsRepo *database.ProcessedEventsRepository
}

// SetupDatabase connects to PostgreSQL and creates all repositories.
func SetupDatabase(cfg config.Interface) (*DatabaseComponents, error) {
	dbCfg := databaseConfigFromInterface(cfg.GetDatabaseConfig())

	db, err := database.NewPostgresConnection(dbCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	jobRepo, executionRepo, discoveredLinkRepo, processedEventsRepo := setupRepositories(db)

	return &DatabaseComponents{
		DB:                  db,
		JobRepo:             jobRepo,
		ExecutionRepo:       executionRepo,
		DiscoveredLinkRepo:  discoveredLinkRepo,
		ProcessedEventsRepo: processedEventsRepo,
	}, nil
}

// setupRepositories creates all database repositories.
func setupRepositories(db *sqlx.DB) (
	jobRepo *database.JobRepository,
	executionRepo *database.ExecutionRepository,
	discoveredLinkRepo *database.DiscoveredLinkRepository,
	processedEventsRepo *database.ProcessedEventsRepository,
) {
	jobRepo = database.NewJobRepository(db)
	executionRepo = database.NewExecutionRepository(db)
	discoveredLinkRepo = database.NewDiscoveredLinkRepository(db)
	processedEventsRepo = database.NewProcessedEventsRepository(db)
	return jobRepo, executionRepo, discoveredLinkRepo, processedEventsRepo
}

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
