package bootstrap

import (
	"time"

	"github.com/jonesrussell/north-cloud/index-manager/internal/api"
	"github.com/jonesrussell/north-cloud/index-manager/internal/config"
	"github.com/jonesrussell/north-cloud/index-manager/internal/database"
	"github.com/jonesrussell/north-cloud/index-manager/internal/elasticsearch"
	"github.com/jonesrussell/north-cloud/index-manager/internal/service"
	infragin "github.com/north-cloud/infrastructure/gin"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const httpTimeoutSeconds = 15

// SetupHTTPServer creates and configures the HTTP server.
func SetupHTTPServer(
	cfg *config.Config,
	esClient *elasticsearch.Client,
	db *database.Connection,
	log infralogger.Logger,
) *infragin.Server {
	indexService := service.NewIndexService(esClient, db, log)
	documentService := service.NewDocumentService(esClient, log)
	handler := api.NewHandler(indexService, documentService, log)

	serverConfig := api.ServerConfig{
		Port:         cfg.Service.Port,
		ReadTimeout:  httpTimeoutSeconds * time.Second,
		WriteTimeout: httpTimeoutSeconds * time.Second,
		Debug:        cfg.Service.Debug,
		ServiceName:  cfg.Service.Name,
	}

	return api.NewServer(handler, serverConfig, log)
}
