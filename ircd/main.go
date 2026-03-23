package main

import (
	"fmt"
	"os"
)

var version = "dev"

func main() {
	fmt.Printf("NorthCloud IRCd %s\n", version)
	os.Exit(0)
}
