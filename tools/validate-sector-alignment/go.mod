module github.com/jonesrussell/north-cloud/tools/validate-sector-alignment

go 1.26.2

require (
	github.com/jonesrussell/north-cloud/infrastructure v0.0.0
	gopkg.in/yaml.v3 v3.0.1
)

require github.com/stretchr/testify v1.11.1 // indirect

replace github.com/jonesrussell/north-cloud/infrastructure => ../../infrastructure
