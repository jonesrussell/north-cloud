module github.com/jonesrussell/north-cloud/mcp-north-cloud

go 1.25

require (
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/north-cloud/infrastructure v0.0.0
)

require (
	github.com/joho/godotenv v1.5.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/north-cloud/infrastructure => ../infrastructure
