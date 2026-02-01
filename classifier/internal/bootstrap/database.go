package bootstrap

import (
	"fmt"
	"strconv"

	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/classifier/internal/config"
	"github.com/jonesrussell/north-cloud/classifier/internal/database"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// DatabaseComponents holds database connection and repositories.
type DatabaseComponents struct {
	DB                        *sqlx.DB
	RulesRepo                 *database.RulesRepository
	SourceRepRepo             *database.SourceReputationRepository
	ClassificationHistoryRepo *database.ClassificationHistoryRepository
}

// SetupDatabase creates database connection and repositories.
func SetupDatabase(cfg *config.Config, logger infralogger.Logger) (*DatabaseComponents, error) {
	dbPort := strconv.Itoa(cfg.Database.Port)
	if cfg.Database.Port == 0 {
		dbPort = "5432"
	}

	dbConfig := database.Config{
		Host:     cfg.Database.Host,
		Port:     dbPort,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:   cfg.Database.Database,
		SSLMode:  cfg.Database.SSLMode,
	}

	if dbConfig.Host == "" {
		dbConfig.Host = "localhost"
	}
	if dbConfig.User == "" {
		dbConfig.User = "postgres"
	}
	if dbConfig.DBName == "" {
		dbConfig.DBName = "classifier"
	}
	if dbConfig.SSLMode == "" {
		dbConfig.SSLMode = "disable"
	}

	logger.Info("Connecting to PostgreSQL database",
		infralogger.String("host", dbConfig.Host),
		infralogger.String("port", dbConfig.Port),
		infralogger.String("database", dbConfig.DBName),
	)

	db, err := database.NewPostgresConnection(dbConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	logger.Info("Database connected successfully")

	return &DatabaseComponents{
		DB:                        db,
		RulesRepo:                 database.NewRulesRepository(db),
		SourceRepRepo:             database.NewSourceReputationRepository(db),
		ClassificationHistoryRepo: database.NewClassificationHistoryRepository(db),
	}, nil
}
