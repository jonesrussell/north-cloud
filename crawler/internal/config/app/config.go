package app

// Config represents application-specific configuration settings.
type Config struct {
	// Name is the name of the application
	Name string `yaml:"name"`
	// Version is the version of the application
	Version string `yaml:"version"`
	// Environment is the application environment (development, staging, production)
	Environment string `yaml:"environment"`
	// Debug indicates whether debug mode is enabled
	Debug bool `yaml:"debug"`
}
