module github.com/jonesrussell/north-cloud/tests/integration/pipeline

go 1.25

require (
	github.com/jonesrussell/north-cloud/index-manager v0.0.0
	github.com/redis/go-redis/v9 v9.17.3
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
)

replace github.com/jonesrussell/north-cloud/index-manager => ../../../index-manager

replace github.com/north-cloud/infrastructure => ../../../infrastructure
