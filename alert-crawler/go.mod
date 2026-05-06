module github.com/jonesrussell/north-cloud/alert-crawler

go 1.26.2

replace (
	github.com/jonesrussell/indigenous-taxonomy => ../../indigenous-taxonomy
	github.com/jonesrussell/north-cloud/infrastructure => ../infrastructure
)

require (
	github.com/jonesrussell/north-cloud/infrastructure v0.0.0-20260502205351-34167b1e4b9c
	github.com/mattn/go-sqlite3 v1.14.44
	github.com/redis/go-redis/v9 v9.18.0
	github.com/stretchr/testify v1.11.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/alicebob/miniredis/v2 v2.37.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	go.uber.org/atomic v1.11.0 // indirect
)
