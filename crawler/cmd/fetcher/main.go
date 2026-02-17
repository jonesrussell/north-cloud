package main

import (
	"fmt"
	"os"

	"github.com/jonesrussell/north-cloud/crawler/internal/fetcher"
)

func main() {
	if err := fetcher.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "fetcher: %v\n", err)
		os.Exit(1)
	}
}
