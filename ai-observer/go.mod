module github.com/jonesrussell/north-cloud/ai-observer

go 1.26

require github.com/north-cloud/infrastructure v0.0.0

require (
	github.com/stretchr/testify v1.8.4 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.1 // indirect
)

replace github.com/north-cloud/infrastructure => ../infrastructure
