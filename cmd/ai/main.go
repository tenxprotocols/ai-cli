package main

import (
	"fmt"
	"os"

	"github.com/tenxprotocols/ai-cli/internal/version"
)

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("ai %s\n", version.Version)
		return
	}
	fmt.Fprintln(os.Stderr, "ai: not yet implemented")
	os.Exit(2)
}
