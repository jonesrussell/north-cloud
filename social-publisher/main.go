package main

import (
	"fmt"
	"os"
)

const version = "0.1.0"

func main() {
	os.Exit(run())
}

func run() int {
	fmt.Printf("Social Publisher v%s\n", version)
	return 0
}
