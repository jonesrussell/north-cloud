package bootstrap

import (
	"github.com/jonesrussell/north-cloud/source-manager/internal/api"
	"github.com/jonesrussell/north-cloud/source-manager/internal/config"
	"github.com/jonesrussell/north-cloud/source-manager/internal/database"
	"github.com/jonesrussell/north-cloud/source-manager/internal/events"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
	infragin "github.com/north-cloud/infrastructure/gin"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// SetupHTTPServer creates and configures the HTTP server.
func SetupHTTPServer(
	cfg *config.Config,
	db *database.DB,
	publisher *events.Publisher,
	log infralogger.Logger,
) *infragin.Server {
	sourceRepo := repository.NewSourceRepository(db.DB(), log)
	return api.NewServer(sourceRepo, cfg, log, publisher)
}
