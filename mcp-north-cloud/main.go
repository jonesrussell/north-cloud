package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	infraconfig "github.com/jonesrussell/north-cloud/infrastructure/config"
	"github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/mcp-north-cloud/internal/client"
	"github.com/jonesrussell/north-cloud/mcp-north-cloud/internal/config"
	"github.com/jonesrussell/north-cloud/mcp-north-cloud/internal/mcp"
)

func main() {
	// Load configuration (optional - uses defaults if file doesn't exist)
	configPath := infraconfig.GetConfigPath("config.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		// Config file is optional for MCP server - use defaults
		cfg = config.NewDefault()
	}

	// Initialize logger (writes to stderr, stdout is for MCP protocol)
	log, err := logger.New(logger.Config{
		Level:       cfg.Logging.Level,
		Format:      cfg.Logging.Format,
		OutputPaths: []string{"stderr"},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = log.Sync() }()

	log.Info("Starting MCP server",
		logger.String("env", cfg.Env),
	)

	// Read from stdin, write to stdout
	reader := bufio.NewReader(os.Stdin)
	writer := os.Stdout

	// Initialize service clients
	clients := initializeClients(cfg, log)

	// Build service URL map for health checks
	serviceURLs := map[string]string{
		"crawler":        cfg.Services.CrawlerURL,
		"source-manager": cfg.Services.SourceManagerURL,
		"publisher":      cfg.Services.PublisherURL,
		"search":         cfg.Services.SearchURL,
		"classifier":     cfg.Services.ClassifierURL,
		"index-manager":  cfg.Services.IndexManagerURL,
		"grafana":        cfg.Services.GrafanaURL,
		"auth":           cfg.Services.AuthURL,
		"pipeline":       cfg.Services.PipelineURL,
		"click-tracker":  cfg.Services.ClickTrackerURL,
		"rfp-ingestor":   cfg.Services.RFPIngestorURL,
	}

	// Create MCP server
	server := mcp.NewServer(
		cfg.Env,
		clients.indexManager,
		clients.crawler,
		clients.sourceManager,
		clients.publisher,
		clients.search,
		clients.classifier,
		clients.authClient,
		clients.grafana,
		cfg.Services.OllamaURL,
		cfg.Services.OllamaModel,
		cfg.Services.RendererURL,
		mcp.WithLogger(log),
		mcp.WithServiceURLs(serviceURLs),
	)

	// Process requests
	processRequests(reader, writer, server, log)
}

type serviceClients struct {
	indexManager  *client.IndexManagerClient
	crawler       *client.CrawlerClient
	sourceManager *client.SourceManagerClient
	publisher     *client.PublisherClient
	search        *client.SearchClient
	classifier    *client.ClassifierClient
	authClient    *client.AuthenticatedClient
	grafana       *client.GrafanaClient
}

func initializeClients(cfg *config.Config, log logger.Logger) *serviceClients {
	log.Debug("Initializing service clients",
		logger.String("index_manager_url", cfg.Services.IndexManagerURL),
		logger.String("crawler_url", cfg.Services.CrawlerURL),
		logger.String("source_manager_url", cfg.Services.SourceManagerURL),
		logger.String("publisher_url", cfg.Services.PublisherURL),
		logger.String("search_url", cfg.Services.SearchURL),
		logger.String("classifier_url", cfg.Services.ClassifierURL),
		logger.String("grafana_url", cfg.Services.GrafanaURL),
		logger.Bool("auth_configured", cfg.Auth.JWTSecret != ""),
	)

	// Create authenticated HTTP client for service-to-service calls
	httpTimeout := time.Duration(cfg.Client.HTTPTimeoutSeconds) * time.Second
	authClient := client.NewAuthenticatedClientWithTimeout(cfg.Auth.JWTSecret, httpTimeout)

	return &serviceClients{
		indexManager:  client.NewIndexManagerClient(cfg.Services.IndexManagerURL, authClient),
		crawler:       client.NewCrawlerClient(cfg.Services.CrawlerURL, authClient),
		sourceManager: client.NewSourceManagerClient(cfg.Services.SourceManagerURL, authClient),
		publisher:     client.NewPublisherClient(cfg.Services.PublisherURL, authClient),
		search:        client.NewSearchClient(cfg.Services.SearchURL, authClient),
		classifier:    client.NewClassifierClient(cfg.Services.ClassifierURL, authClient),
		authClient:    authClient,
		grafana:       client.NewGrafanaClient(cfg.Services.GrafanaURL, cfg.Services.GrafanaUsername, cfg.Services.GrafanaPassword, authClient),
	}
}

func processRequests(reader *bufio.Reader, writer io.Writer, server *mcp.Server, log logger.Logger) {
	decoder := json.NewDecoder(reader)
	encoder := json.NewEncoder(writer)

	const requestTimeout = 60 * time.Second // must be >= HTTP client timeout
	for {
		var request mcp.Request
		if err := decoder.Decode(&request); err != nil {
			if err == io.EOF {
				log.Info("EOF received, shutting down")
				break
			}
			log.Error("Failed to parse request", logger.Error(err))
			sendError(encoder, 0, mcp.ParseError, "Failed to parse request", nil)
			continue
		}

		log.Debug("Received request",
			logger.String("method", request.Method),
			logger.Any("id", request.ID),
		)

		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		response := server.HandleRequestWithContext(ctx, &request)
		cancel()
		if response != nil {
			if response.ID == nil && request.ID != nil {
				response.ID = request.ID
			}
			if request.ID == nil {
				continue
			}
			if encodeErr := encoder.Encode(response); encodeErr != nil {
				log.Error("Failed to encode response", logger.Error(encodeErr))
			}
		}
	}
}

func sendError(encoder *json.Encoder, id any, code int, message string, data any) {
	errorResponse := mcp.ErrorResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: mcp.ErrorObject{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	if encodeErr := encoder.Encode(errorResponse); encodeErr != nil {
		fmt.Fprintf(os.Stderr, "Failed to encode error response: %v\n", encodeErr)
	}
}
