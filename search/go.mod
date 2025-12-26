module github.com/jonesrussell/north-cloud/search

go 1.25

require (
	github.com/elastic/go-elasticsearch/v8 v8.11.0
	github.com/gin-gonic/gin v1.10.0
	github.com/jonesrussell/north-cloud/infrastructure v0.0.0
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/jonesrussell/north-cloud/infrastructure => ../infrastructure
