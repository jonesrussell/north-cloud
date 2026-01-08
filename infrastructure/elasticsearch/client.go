package elasticsearch

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/retry"
)

// NewClient creates a new Elasticsearch client with retry logic for connection verification.
// It normalizes the URL, configures TLS and authentication, and retries the connection
// verification with exponential backoff if the initial connection fails.
func NewClient(ctx context.Context, cfg Config, log logger.Logger) (*es.Client, error) {
	cfg.SetDefaults()

	// Normalize URL (add http:// if missing)
	url := normalizeURL(cfg.URL)

	// Create transport with TLS if configured
	transport := createTransport(cfg.TLS)

	// Create client configuration
	clientConfig := es.Config{
		Addresses:  []string{url},
		Transport:  transport,
		MaxRetries: cfg.MaxRetries,
	}

	// Configure authentication
	if cfg.APIKey != "" {
		clientConfig.APIKey = cfg.APIKey
	} else if cfg.CloudID != "" && cfg.CloudAPIKey != "" {
		clientConfig.CloudID = cfg.CloudID
		clientConfig.APIKey = cfg.CloudAPIKey
	} else if cfg.Username != "" && cfg.Password != "" {
		clientConfig.Username = cfg.Username
		clientConfig.Password = cfg.Password
	}

	// Create the Elasticsearch client
	esClient, err := es.NewClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Elasticsearch client: %w", err)
	}

	// Verify connection with retry logic
	if log != nil {
		log.Info("Verifying Elasticsearch connection", logger.String("url", url))
	}

	retryCfg := *cfg.RetryConfig
	if err := retry.Retry(ctx, retryCfg, func() error {
		return pingElasticsearch(ctx, esClient, cfg.PingTimeout, log)
	}); err != nil {
		return nil, fmt.Errorf("failed to connect to Elasticsearch after retries: %w", err)
	}

	if log != nil {
		log.Info("Elasticsearch connection established", logger.String("url", url))
	}

	return esClient, nil
}

// normalizeURL normalizes the Elasticsearch URL by adding http:// prefix if missing
func normalizeURL(url string) string {
	if url == "" {
		return "http://localhost:9200"
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return "http://" + url
	}
	return url
}

// createTransport creates an HTTP transport with TLS configuration if provided
func createTransport(tlsConfig *TLSConfig) *http.Transport {
	transport := &http.Transport{}

	if tlsConfig != nil && tlsConfig.Enabled {
		tlsClientConfig := &tls.Config{
			InsecureSkipVerify: tlsConfig.InsecureSkipVerify,
		}

		// Load client certificate if provided
		if tlsConfig.CertFile != "" && tlsConfig.KeyFile != "" {
			cert, err := tls.LoadX509KeyPair(tlsConfig.CertFile, tlsConfig.KeyFile)
			if err == nil {
				tlsClientConfig.Certificates = []tls.Certificate{cert}
			}
		}

		// TODO: Load CA certificate from tlsConfig.CAFile if provided
		// This would require using crypto/x509 to load and parse the CA cert

		transport.TLSClientConfig = tlsClientConfig
	}

	return transport
}

// pingElasticsearch verifies the Elasticsearch connection by pinging it
func pingElasticsearch(ctx context.Context, client *es.Client, timeout time.Duration, log logger.Logger) error {
	// Create a context with timeout for the ping
	pingCtx := ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		pingCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Perform ping
	res, err := client.Ping(client.Ping.WithContext(pingCtx))
	if err != nil {
		if log != nil {
			log.Debug("Elasticsearch ping failed", logger.Error(err))
		}
		return fmt.Errorf("ping failed: %w", err)
	}
	defer func() {
		if closeErr := res.Body.Close(); closeErr != nil && log != nil {
			log.Debug("Failed to close ping response body", logger.Error(closeErr))
		}
	}()

	if res.IsError() {
		body, readErr := io.ReadAll(res.Body)
		errMsg := string(body)
		if readErr != nil {
			errMsg = fmt.Sprintf("error reading response body: %v", readErr)
		}
		if log != nil {
			log.Debug("Elasticsearch ping returned error", logger.String("status", res.Status()), logger.String("body", errMsg))
		}
		return fmt.Errorf("ping returned error [%s]: %s", res.Status(), errMsg)
	}

	return nil
}
