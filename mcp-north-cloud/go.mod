module github.com/jonesrussell/north-cloud/mcp-north-cloud

go 1.26.2

require (
	github.com/go-shiori/go-readability v0.0.0-20251205110129-5db1dc9836f0
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/jonesrussell/north-cloud/infrastructure v0.0.0
)

require (
	github.com/andybalholm/cascadia v1.3.3 // indirect
	github.com/araddon/dateparse v0.0.0-20210429162001-6b43995a97de // indirect
	github.com/go-shiori/dom v0.0.0-20230515143342-73569d674e1c // indirect
	github.com/gogs/chardet v0.0.0-20211120154057-b7413eaefb8f // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.1 // indirect
	golang.org/x/net v0.51.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/jonesrussell/north-cloud/infrastructure => ../infrastructure
