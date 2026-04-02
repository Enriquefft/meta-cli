package main

import (
	"fmt"
	"os"
)

var version = "dev"

func main() {
	args := os.Args[1:]
	if len(args) > 0 && (args[0] == "--version" || args[0] == "-v") {
		fmt.Printf("meta version %s\n", version)
		os.Exit(0)
	}

	fmt.Fprintf(os.Stderr, "meta — CLI and MCP server for the Meta Marketing API\n")
	fmt.Fprintf(os.Stderr, "Run 'meta --help' for usage.\n")
	os.Exit(0)
}
