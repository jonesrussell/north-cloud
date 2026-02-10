module github.com/jonesrussell/north-cloud/pipeline

go 1.25

replace github.com/north-cloud/infrastructure => ../infrastructure

require (
	github.com/DATA-DOG/go-sqlmock v1.5.2
	github.com/lib/pq v1.11.2
	github.com/north-cloud/infrastructure v0.0.0-00010101000000-000000000000
)

require (
	github.com/joho/godotenv v1.5.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
