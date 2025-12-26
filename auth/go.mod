module github.com/jonesrussell/auth

go 1.25

require (
	github.com/gin-contrib/cors v1.7.6
	github.com/gin-gonic/gin v1.11.0
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/google/uuid v1.6.0
	github.com/lib/pq v1.10.9
	github.com/north-cloud/infrastructure v0.0.0
	go.uber.org/zap v1.27.1
	golang.org/x/crypto v0.46.0
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/north-cloud/infrastructure => ../infrastructure

