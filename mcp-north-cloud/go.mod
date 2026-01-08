module github.com/jonesrussell/north-cloud/mcp-north-cloud

go 1.25

require github.com/north-cloud/infrastructure v0.0.0

require (
	github.com/joho/godotenv v1.5.1 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/north-cloud/infrastructure => ../infrastructure
