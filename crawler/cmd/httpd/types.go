// Package httpd implements the HTTP server for the crawler service.
package httpd

import (
	"github.com/jonesrussell/north-cloud/crawler/internal/config"
	"github.com/jonesrussell/north-cloud/crawler/internal/logger"
	"github.com/jonesrussell/north-cloud/crawler/internal/storage/types"
)

// CommandDeps holds common dependencies for the HTTP server.
type CommandDeps struct {
	Logger logger.Interface
	Config config.Interface
}

// StorageResult holds both storage interface and index manager.
type StorageResult struct {
	Storage      types.Interface
	IndexManager types.IndexManager
}
