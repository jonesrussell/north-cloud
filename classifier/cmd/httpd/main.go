package main

import (
	"fmt"
	"os"

	"github.com/jonesrussell/north-cloud/classifier/internal/server"
)

func main() {
	// Allow running httpd command directly: go run cmd/httpd/main.go
	if err := server.StartHTTPServer(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
