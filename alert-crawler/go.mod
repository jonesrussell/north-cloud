module github.com/jonesrussell/north-cloud/alert-crawler

go 1.26.2

replace (
	github.com/jonesrussell/indigenous-taxonomy => ../../indigenous-taxonomy
	github.com/jonesrussell/north-cloud/infrastructure => ../infrastructure
)

require (
	github.com/jonesrussell/north-cloud/infrastructure v0.0.0-20260502205351-34167b1e4b9c
	github.com/stretchr/testify v1.11.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
)
