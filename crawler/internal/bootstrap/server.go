package bootstrap

import (
	"github.com/jonesrussell/north-cloud/crawler/internal/api"
	"github.com/jonesrussell/north-cloud/crawler/internal/config"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/job"
	infragin "github.com/north-cloud/infrastructure/gin"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// HTTPServerDeps holds dependencies for the HTTP server.
type HTTPServerDeps struct {
	Config                 config.Interface
	Logger                 infralogger.Logger
	JobsHandler            *api.JobsHandler
	DiscoveredLinksHandler *api.DiscoveredLinksHandler
	LogsHandler            *api.LogsHandler
	LogsV2Handler          *api.LogsStreamV2Handler
	ExecutionRepo          database.ExecutionRepositoryInterface
	SSEHandler             *api.SSEHandler
	Migrator               *job.Migrator
}

// ServerComponents holds the HTTP server and error channel.
type ServerComponents struct {
	Server    *infragin.Server
	ErrorChan <-chan error
}

// SetupHTTPServer creates and starts the HTTP server.
// Returns the server and an error channel for server errors.
func SetupHTTPServer(deps *HTTPServerDeps) *ServerComponents {
	migrationHandler := api.NewMigrationHandler(deps.Migrator, deps.Logger)

	server := api.NewServer(
		deps.Config, deps.JobsHandler, deps.DiscoveredLinksHandler,
		deps.LogsHandler, deps.LogsV2Handler, deps.ExecutionRepo,
		deps.Logger, deps.SSEHandler, migrationHandler,
	)

	deps.Logger.Info("Starting HTTP server", infralogger.String("addr", deps.Config.GetServerConfig().Address))
	errChan := server.StartAsync()

	return &ServerComponents{
		Server:    server,
		ErrorChan: errChan,
	}
}
