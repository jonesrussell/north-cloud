package main

import (
	"fmt"
	"os"

	"github.com/jonesrussell/gocrawl/cmd/httpd"
)

func main() {
	if err := httpd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
