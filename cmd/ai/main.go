package main

import (
	"fmt"
	"os"

	"github.com/tenxprotocols/ai-cli/internal/cli"
)

func main() {
	root := cli.NewRoot()
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "ai: %v\n", err)
		os.Exit(1)
	}
}
