package fetcher

import (
	fetcherconfig "github.com/jonesrussell/north-cloud/crawler/internal/config/fetcher"
)

// Config is an alias for the canonical fetcher configuration type in config/fetcher.
// This alias preserves backward compatibility so existing code referencing fetcher.Config
// continues to compile without import changes.
type Config = fetcherconfig.Config
