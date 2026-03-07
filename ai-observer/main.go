package main

import (
	"fmt"
	"os"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/bootstrap"
)

func main() {
	if err := bootstrap.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
