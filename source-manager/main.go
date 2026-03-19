package main

import (
	"fmt"
	"os"

	"github.com/jonesrussell/north-cloud/source-manager/internal/bootstrap"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "import-opd" {
		if err := runImportOPD(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := bootstrap.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
