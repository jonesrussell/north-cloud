package main

import (
	"github.com/jonesrussell/north-cloud/classifier/internal/server"
)

func main() {
	// Allow running httpd command directly: go run cmd/httpd/main.go
	server.StartHTTPServer()
}
