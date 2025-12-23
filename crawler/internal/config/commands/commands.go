// Package commands provides constants and utilities for command handling in the application.
package commands

// Command names
const (
	// Root commands
	Job     = "job"
	Crawl   = "crawl"
	HTTPD   = "httpd"
	Search  = "search"
	Sources = "sources"
)

// ConfigRequirements represents which configuration sections are required for a command
type ConfigRequirements struct {
	// RequiresCrawler indicates if crawler configuration is required
	RequiresCrawler bool
	// RequiresSources indicates if sources configuration is required
	RequiresSources bool
	// RequiresElasticsearchIndex indicates if an Elasticsearch index name is required
	RequiresElasticsearchIndex bool
}

// GetConfigRequirements returns the configuration requirements for a given command
func GetConfigRequirements(command string) ConfigRequirements {
	// By default, require all configurations
	return ConfigRequirements{
		RequiresCrawler:            true,
		RequiresSources:            true,
		RequiresElasticsearchIndex: true,
	}
}

// RequiresCrawlerConfig returns true if the given command requires crawler configuration.
func RequiresCrawlerConfig(command string) bool {
	return GetConfigRequirements(command).RequiresCrawler
}

// RequiresSourcesConfig returns true if the given command requires sources configuration.
func RequiresSourcesConfig(command string) bool {
	return GetConfigRequirements(command).RequiresSources
}

// RequiresElasticsearchIndex returns true if the given command requires an Elasticsearch index name.
func RequiresElasticsearchIndex(command string) bool {
	return GetConfigRequirements(command).RequiresElasticsearchIndex
}
