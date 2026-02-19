module github.com/jonesrussell/north-cloud/click-tracker

go 1.25

require (
	github.com/lib/pq v1.10.9
	github.com/north-cloud/infrastructure v0.0.0-00010101000000-000000000000
)

require (
	github.com/grafana/pyroscope-go v1.2.7 // indirect
	github.com/grafana/pyroscope-go/godeltaprof v0.1.9 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/klauspost/compress v1.18.4 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/north-cloud/infrastructure => ../infrastructure
